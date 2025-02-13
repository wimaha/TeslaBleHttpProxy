package control

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"
	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/tesla/commands"
)

var BleControlInstance *BleControl = nil

func SetupBleControl() {
	var err error
	if BleControlInstance, err = NewBleControl(); err != nil {
		log.Warn("BleControl could not be initialized!")
	} else {
		go BleControlInstance.Loop()
		log.Info("BleControl initialized")
	}
}

func CloseBleControl() {
	BleControlInstance = nil
}

type BleControl struct {
	privateKey          protocol.ECDHPrivateKey
	operatedBeacon      *ble.ScanResult
	infotainmentSession bool

	commandStack  chan commands.Command
	providerStack chan commands.Command
}

func NewBleControl() (*BleControl, error) {
	var privateKey protocol.ECDHPrivateKey
	var err error
	if privateKey, err = protocol.LoadPrivateKey(config.PrivateKeyFile); err != nil {
		log.Error("failed to load private key.", "err", err)
		return nil, fmt.Errorf("failed to load private key: %s", err)
	}
	log.Debug("privateKeyFile loaded")

	return &BleControl{
		privateKey:    privateKey,
		commandStack:  make(chan commands.Command, 50),
		providerStack: make(chan commands.Command),
	}, nil
}

func (bc *BleControl) Loop() {
	var retryCommand *commands.Command
	for {
		time.Sleep(1 * time.Second)
		if retryCommand != nil {
			log.Debug("retrying command from loop", "command", retryCommand.Command, "body", retryCommand.Body)
			retryCommand = bc.connectToVehicleAndOperateConnection(retryCommand)
		} else {
			log.Debug("waiting for command")
			// Wait for the next command
		outer:
			for {
				select {
				case command, ok := <-bc.providerStack:
					if ok {
						retryCommand = bc.connectToVehicleAndOperateConnection(&command)
					}
					break outer
				case command, ok := <-bc.commandStack:
					if ok {
						log.Debug("command popped", "command", command.Command, "body", command.Body, "stack size", len(bc.commandStack))
						if command.IsContextDone() {
							log.Debug("context done, skipping command", "command", command.Command, "body", command.Body)
							continue
						}
						retryCommand = bc.connectToVehicleAndOperateConnection(&command)
					}
					break outer
				}
			}
		}
	}
}

func (bc *BleControl) PushCommand(command commands.Command) {
	bc.commandStack <- command
}

func processIfConnectionStatusCommand(command *commands.Command, operated bool) bool {
	if command.Command != "connection_status" {
		return false
	}

	defer func() {
		if command.Response.Wait != nil {
			command.Response.Wait.Done()
		}
	}()

	if BleControlInstance == nil {
		command.Response.Error = "BleControl is not initialized. Maybe private.pem is missing."
		command.Response.Result = false
		return true
	} else {
		command.Response.Result = true
	}

	var beacon *ble.ScanResult = nil

	if operated {
		if BleControlInstance.operatedBeacon != nil {
			beacon = BleControlInstance.operatedBeacon
		} else {
			log.Warn("operated beacon is nil but operated is true")
		}
	} else {
		var err error
		scanTimeout := config.AppConfig.ScanTimeout
		scanCtx, cancelScan := context.WithCancel(command.Response.Ctx)
		if scanTimeout > 0 {
			scanCtx, cancelScan = context.WithTimeout(command.Response.Ctx, time.Duration(scanTimeout)*time.Second)
		}
		defer cancelScan()
		beacon, err = ble.ScanVehicleBeacon(scanCtx, command.Vin)
		if err != nil && !strings.Contains(err.Error(), "context deadline exceeded") {
			command.Response.Error = err.Error()
			command.Response.Result = false
			return true
		}
	}

	var resp map[string]interface{}
	if beacon != nil {
		resp = map[string]interface{}{
			"local_name":  beacon.LocalName,
			"connectable": true,
			"address":     beacon.Address.String(),
			"rssi":        beacon.RSSI,
			"operated":    operated,
		}
	} else {
		resp = map[string]interface{}{
			"local_name":  ble.VehicleLocalName(command.Vin),
			"connectable": false,
			"address":     nil,
			"rssi":        nil,
			"operated":    false,
		}
	}
	respBytes, err := json.Marshal(resp)

	if err != nil {
		command.Response.Error = err.Error()
		command.Response.Result = false
	} else {
		command.Response.Response = json.RawMessage(respBytes)
	}

	return true
}

func (bc *BleControl) connectToVehicleAndOperateConnection(firstCommand *commands.Command) *commands.Command {
	if processIfConnectionStatusCommand(firstCommand, false) {
		return nil
	}

	log.Info("connecting to Vehicle ...")
	defer log.Debug("connecting to Vehicle done")

	var sleep = 3 * time.Second
	var retryCount = 3
	var lastErr error

	commandError := func(err error) *commands.Command {
		log.Error("can't connect to vehicle", "error", err)
		if firstCommand.Response != nil {
			firstCommand.Response.Error = err.Error()
			firstCommand.Response.Result = false
			if firstCommand.Response.Wait != nil {
				firstCommand.Response.Wait.Done()
			}
		}
		return nil
	}

	var parentCtx context.Context
	if firstCommand.Response != nil && firstCommand.Response.Ctx != nil {
		parentCtx = firstCommand.Response.Ctx
		if parentCtx.Err() != nil {
			return commandError(parentCtx.Err())
		}
	} else {
		if firstCommand.Response != nil {
			log.Warn("no context provided, using default", "command", firstCommand.Command, "body", firstCommand.Body)
		}
		parentCtx = context.Background()
	}

	for i := 0; i < retryCount; i++ {
		if i > 0 {
			log.Warn(lastErr)
			log.Info(fmt.Sprintf("retrying in %d seconds", sleep/time.Second))
			select {
			case <-time.After(sleep):
			case <-parentCtx.Done():
				return commandError(parentCtx.Err())
			}
			sleep *= 2
		}
		log.Debug("trying connecting to vehicle", "attempt", i+1)
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Second)
		defer cancel()
		conn, car, retry, err := bc.TryConnectToVehicle(ctx, firstCommand)
		if err == nil {
			//Successful
			defer conn.Close()
			defer log.Debug("close connection (A)")
			defer car.Disconnect()
			defer log.Debug("disconnect vehicle (A)")
			cmd := bc.operateConnection(car, firstCommand)
			return cmd
		} else if !retry || parentCtx.Err() != nil {
			//Failed but no retry possible
			return commandError(err)
		} else {
			lastErr = err
		}
	}
	log.Error(fmt.Sprintf("stop retrying after %d attempts", retryCount), "error", lastErr)
	return commandError(lastErr)
}

func (bc *BleControl) startInfotainmentSession(ctx context.Context, car *vehicle.Vehicle) error {
	log.Debug("start Infotainment session...")

	for {
		// FIX: https://github.com/teslamotors/vehicle-command/issues/366
		// Timeout since we can't rely on car.StartSession to return (even an error) if
		// the car is not ready yet. Maybe it's a bug in the vehicle package.
		ctxTry, cancel := context.WithTimeout(ctx, 1000*time.Millisecond)
		defer cancel()
		// Measure time to connect startSession
		start := time.Now()
		// Then we can also connect the infotainment
		if err := car.StartSession(ctxTry, []universalmessage.Domain{
			protocol.DomainVCSEC,
			protocol.DomainInfotainment,
		}); err != nil {
			if errors.Cause(err) == context.DeadlineExceeded && ctx.Err() == nil {
				log.Debug("retrying handshake with vehicle")
				continue
			}
			return fmt.Errorf("failed to perform handshake with vehicle (B): %s", err)
		}
		log.Debug("handshake with vehicle successful", "duration", time.Since(start))

		log.Info("connection established")
		bc.infotainmentSession = true
		return nil
	}
}

func (bc *BleControl) TryConnectToVehicle(ctx context.Context, firstCommand *commands.Command) (*ble.Connection, *vehicle.Vehicle, bool, error) {
	log.Debug("connecting to vehicle (A)...")
	var conn *ble.Connection
	var car *vehicle.Vehicle
	var shouldDefer = true

	defer func() {
		if shouldDefer {
			if car != nil {
				log.Debug("disconnect vehicle (B)")
				car.Disconnect()
			}
			if conn != nil {
				log.Debug("close connection (B)")
				conn.Close()
			}
		}
	}()

	var err error
	log.Debug("scan for vehicle ...")
	// Vehicle sends a beacon every ~200ms, so if it is not found in (scanTimeout=1) seconds,
	// it is likely not in range and not worth retrying.
	scanTimeout := config.AppConfig.ScanTimeout
	scanCtx, cancelScan := context.WithCancel(ctx)
	if scanTimeout > 0 {
		scanCtx, cancelScan = context.WithTimeout(ctx, time.Duration(scanTimeout)*time.Second)
	}
	defer cancelScan()

	scanResult, err := ble.ScanVehicleBeacon(scanCtx, firstCommand.Vin)
	if err != nil {
		if scanCtx.Err() != nil {
			return nil, nil, false, fmt.Errorf("vehicle not in range: %s", err)
		} else {
			return nil, nil, true, fmt.Errorf("failed to scan for vehicle: %s", err)
		}
	}

	log.Debug("beacon found", "localName", scanResult.LocalName, "addr", scanResult.Address.String(), "rssi", scanResult.RSSI)

	log.Debug("dialing to vehicle ...")
	conn, err = ble.NewConnectionToBleTarget(ctx, firstCommand.Vin, scanResult)
	if err != nil {
		return nil, nil, true, fmt.Errorf("failed to connect to vehicle (A): %s", err)
	}
	bc.infotainmentSession = false
	//defer conn.Close()

	log.Debug("create vehicle object ...")
	car, err = vehicle.NewVehicle(conn, bc.privateKey, nil)
	if err != nil {
		return nil, nil, true, fmt.Errorf("failed to connect to vehicle (B): %s", err)
	}

	log.Debug("connecting to vehicle (B)...")
	if err := car.Connect(ctx); err != nil {
		return nil, nil, true, fmt.Errorf("failed to connect to vehicle (C): %s", err)
	}
	//defer car.Disconnect()

	//Start Session only if privateKey is available
	if bc.privateKey != nil {
		log.Debug("start VCSEC session...")
		// First connect just VCSEC so we can Wakeup() the car if needed.
		if err := car.StartSession(ctx, []universalmessage.Domain{
			protocol.DomainVCSEC,
		}); err != nil {
			return nil, nil, true, fmt.Errorf("failed to perform handshake with vehicle (A): %s", err)
		}

		if firstCommand.Domain() != commands.Domain.VCSEC {
			if err := car.Wakeup(ctx); err != nil {
				return nil, nil, true, fmt.Errorf("failed to wake up car: %s", err)
			} else {
				log.Debug("car successfully wakeup")
			}

			if err := bc.startInfotainmentSession(ctx, car); err != nil {
				return nil, nil, true, err
			}
		}
	} else {
		log.Info("Key-Request connection established")
	}

	bc.operatedBeacon = scanResult

	// everything fine
	shouldDefer = false
	return conn, car, false, nil
}

func (bc *BleControl) operateConnection(car *vehicle.Vehicle, firstCommand *commands.Command) *commands.Command {
	log.Debug("operating connection ...")
	defer log.Debug("operating connection done")
	connectionCtx, cancel := context.WithTimeout(context.Background(), 29*time.Second)
	defer cancel()

	defer func() { bc.operatedBeacon = nil }()

	handleCommand := func(command *commands.Command) *commands.Command {
		if processIfConnectionStatusCommand(command, command.Vin == firstCommand.Vin) {
			return nil
		}

		//If new VIN, close connection
		if command.Vin != firstCommand.Vin {
			log.Debug("new VIN, so close connection")
			return command
		}

		cmd, err := bc.ExecuteCommand(car, command, connectionCtx)

		if err != nil {
			if command.TotalRetries >= 3 {
				log.Error("failed to execute command after 3 retries", "command", command.Command, "body", command.Body, "error", err.Error())
				return nil
			}
			if cmd == nil {
				log.Error("failed to execute command", "command", command.Command, "body", command.Body, "error", err.Error())
				return nil
			} else {
				log.Debug("failed to execute command", "command", command.Command, "body", command.Body, "total retires", command.TotalRetries, "error", err.Error())
				return cmd
			}
		}

		// Successful or api context done so no retry
		return nil
	}

	retryCommand := handleCommand(firstCommand)
	if retryCommand != nil {
		return retryCommand
	}

	for {
		select {
		case <-connectionCtx.Done():
			log.Debug("connection Timeout")
			return nil
		case command, ok := <-bc.providerStack:
			if !ok {
				return nil
			}

			retryCommand := handleCommand(&command)
			if retryCommand != nil {
				return retryCommand
			}
		case command, ok := <-bc.commandStack:
			if !ok {
				return nil
			}

			retryCommand := handleCommand(&command)
			if retryCommand != nil {
				return retryCommand
			}
		}
	}
}

func (bc *BleControl) ExecuteCommand(car *vehicle.Vehicle, command *commands.Command, connectionCtx context.Context) (retryCommand *commands.Command, retErr error) {
	log.Debug("sending", "command", command.Command, "body", command.Body)
	var ctx context.Context
	if command.Response != nil && command.Response.Ctx != nil {
		ctx = command.Response.Ctx
	} else {
		if command.Response != nil {
			log.Debug("no context provided, using default", "command", command.Command, "body", command.Body)
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}

	var sleep = 3 * time.Second
	var retryCount = 3
	var lastErr error

	defer func() {
		if command.Response != nil {
			if retErr != nil {
				command.Response.Error = retErr.Error()
				command.Response.Result = false
			} else {
				command.Response.Result = true
			}
			if command.Response.Wait != nil && retryCommand == nil {
				command.Response.Wait.Done()
			}
		}
	}()

	// If the context is already done, return immediately
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Wrap ctx with connectionCtx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-connectionCtx.Done():
			cancel()
		case <-ctx.Done():
		}
	}()

	dontSkipWait := false
	for ; command.TotalRetries < retryCount; command.TotalRetries++ {
		if dontSkipWait {
			log.Warn(lastErr)
			log.Info(fmt.Sprintf("retrying in %d seconds", sleep/time.Second))
			select {
			case <-time.After(sleep):
			case <-ctx.Done():
				if connectionCtx.Err() != nil {
					log.Debug("operated connection expired")
					return command, errors.Wrap(ctx.Err(), "operated connection expired")
				}
				return nil, ctx.Err()
			}
			sleep *= 2
		}
		dontSkipWait = true

		if !bc.infotainmentSession && command.Domain() == commands.Domain.Infotainment {
			if err := car.Wakeup(ctx); err != nil {
				lastErr = fmt.Errorf("failed to wake up car: %s", err)
				continue
			} else {
				log.Debug("car successfully wakeup")
			}

			if err := bc.startInfotainmentSession(ctx, car); err != nil {
				lastErr = err
				continue
			}
		}

		retry, err := command.Send(ctx, car)
		if err == nil {
			//Successful
			log.Info("successfully executed", "command", command.Command, "body", command.Body)
			return nil, nil
		} else if !retry {
			return nil, nil
		} else {
			//closed pipe
			if strings.Contains(err.Error(), "closed pipe") {
				//connection lost, returning the command so it can be executed again
				return command, err
			}
			lastErr = err
		}
	}
	log.Warn("max retries reached", "command", command.Command, "body", command.Body, "err", lastErr)
	return nil, lastErr
}
