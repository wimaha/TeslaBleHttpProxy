package control

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
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
	privateKey protocol.ECDHPrivateKey

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
			select {
			case command, ok := <-bc.providerStack:
				if ok {
					retryCommand = bc.connectToVehicleAndOperateConnection(&command)
				}
			case command, ok := <-bc.commandStack:
				if ok {
					retryCommand = bc.connectToVehicleAndOperateConnection(&command)
				}
			}
		}
	}
}

func (bc *BleControl) PushCommand(command string, vin string, body map[string]interface{}, response *models.ApiResponse) {
	bc.commandStack <- commands.Command{
		Command:  command,
		Vin:      vin,
		Body:     body,
		Response: response,
	}
}

func (bc *BleControl) connectToVehicleAndOperateConnection(firstCommand *commands.Command) *commands.Command {
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

func (bc *BleControl) TryConnectToVehicle(ctx context.Context, firstCommand *commands.Command) (*ble.Connection, *vehicle.Vehicle, bool, error) {
	log.Debug("try connecting to vehicle ...")
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
	// Vehicle sends a beacon every ~200ms, so if it is not found in (scanTimeout=2) seconds, it is likely not in range and not worth retrying.
	scanTimeout := config.AppConfig.ScanTimeout
	var scanCtx context.Context
	var cancelScan context.CancelFunc
	if scanTimeout > 0 {
		scanCtx, cancelScan = context.WithTimeout(ctx, time.Duration(scanTimeout)*time.Second)
	} else {
		scanCtx, cancelScan = context.WithCancel(ctx)
	}
	defer cancelScan()

	scanResult, err := ble.ScanVehicleBeacon(scanCtx, firstCommand.Vin)
	if err != nil {
		if scanCtx.Err() != nil {
			return nil, nil, false, fmt.Errorf("vehicle not in range: %s", err)
		} else {
			if strings.Contains(err.Error(), "operation not permitted") {
				// The underlying BLE package calls HCIDEVDOWN on the BLE device, presumably as a
				// heavy-handed way of dealing with devices that are in a bad state.
				return nil, nil, false, fmt.Errorf("failed to connect to vehicle (A): %s\nTry again after granting this application CAP_NET_ADMIN:\nsudo setcap 'cap_net_admin=eip' \"$(which %s)\"", err, os.Args[0])
			} else {
				return nil, nil, true, fmt.Errorf("failed to connect to vehicle (A): %s", err)
			}
		}
	}

	log.Debug("beacon found", "localName", scanResult.LocalName, "addr", scanResult.Address, "rssi", scanResult.RSSI)
	log.Debug("connect to vehicle ...")
	conn, err = ble.NewConnectionFromScanResult(ctx, firstCommand.Vin, scanResult)
	if err != nil {
		return nil, nil, true, fmt.Errorf("failed to connect to vehicle (A): %s", err)
	}

	/*conn, err = ble.NewConnection(ctx, firstCommand.Vin)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			// The underlying BLE package calls HCIDEVDOWN on the BLE device, presumably as a
			// heavy-handed way of dealing with devices that are in a bad state.
			return nil, nil, false, fmt.Errorf("failed to connect to vehicle (A): %s\nTry again after granting this application CAP_NET_ADMIN:\nsudo setcap 'cap_net_admin=eip' \"$(which %s)\"", err, os.Args[0])
		} else {
			return nil, nil, true, fmt.Errorf("failed to connect to vehicle (A): %s", err)
		}
	}*/
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

		if firstCommand.Domain != commands.Domain.VCSEC {
			if err := car.Wakeup(ctx); err != nil {
				return nil, nil, true, fmt.Errorf("failed to wake up car: %s", err)
			} else {
				log.Debug("car successfully wakeup")
			}

			log.Debug("start Infotainment session...")
			// Then we can also connect the infotainment
			if err := car.StartSession(ctx, []universalmessage.Domain{
				protocol.DomainVCSEC,
				protocol.DomainInfotainment,
			}); err != nil {
				return nil, nil, true, fmt.Errorf("failed to perform handshake with vehicle (B): %s", err)
			}
			log.Info("connection established")
		}
	} else {
		log.Info("Key-Request connection established")
	}

	// everything fine
	shouldDefer = false
	return conn, car, false, nil
}

func (bc *BleControl) operateConnection(car *vehicle.Vehicle, firstCommand *commands.Command) *commands.Command {
	log.Debug("operating connection ...")
	defer log.Debug("operating connection done")
	connectionCtx, cancel := context.WithTimeout(context.Background(), 29*time.Second)
	defer cancel()

	if firstCommand.Command != "wake_up" {
		cmd, err, _ := bc.ExecuteCommand(car, firstCommand, connectionCtx)
		if err != nil {
			return cmd
		}
	}

	handleCommand := func(command *commands.Command) (doReturn bool, retryCommand *commands.Command) {
		//If new VIN, close connection
		if command.Vin != firstCommand.Vin {
			log.Debug("new VIN, so close connection")
			return true, command
		}

		cmd, err, ctx := bc.ExecuteCommand(car, command, connectionCtx)

		// If the connection context is done, return to reoperate the connection
		if connectionCtx.Err() != nil {
			return true, cmd
		}
		// If the context is not done, return to retry the command
		if err != nil && ctx.Err() == nil {
			return true, cmd
		}

		// Successful or api context done so no retry
		return false, nil
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

			doReturn, retryCommand := handleCommand(&command)
			if doReturn {
				return retryCommand
			}
		case command, ok := <-bc.commandStack:
			if !ok {
				return nil
			}

			doReturn, retryCommand := handleCommand(&command)
			if doReturn {
				return retryCommand
			}
		}
	}
}

func (bc *BleControl) ExecuteCommand(car *vehicle.Vehicle, command *commands.Command, connectionCtx context.Context) (retryCommand *commands.Command, retErr error, ctx context.Context) {
	log.Info("sending", "command", command.Command, "body", command.Body)
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
		return nil, ctx.Err(), ctx
	}

	// Wrap ctx with connectionCtx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	connectionCtxDone := false
	go func() {
		select {
		case <-connectionCtx.Done():
			connectionCtxDone = true
			cancel()
		case <-ctx.Done():
		}
	}()

	for i := 0; i < retryCount; i++ {
		if i > 0 {
			log.Warn(lastErr)
			log.Info(fmt.Sprintf("retrying in %d seconds", sleep/time.Second))
			select {
			case <-time.After(sleep):
			case <-ctx.Done():
				if connectionCtxDone {
					return command, ctx.Err(), ctx
				}
				return nil, ctx.Err(), ctx
			}
			sleep *= 2
		}

		retry, err := command.Send(ctx, car)
		if err == nil {
			//Successful
			log.Info("successfully executed", "command", command.Command, "body", command.Body)
			return nil, nil, ctx
		} else if !retry {
			return nil, nil, ctx
		} else {
			//closed pipe
			if strings.Contains(err.Error(), "closed pipe") {
				//connection lost, returning the command so it can be executed again
				return command, err, ctx
			}
			lastErr = err
		}
	}
	log.Error("canceled", "command", command.Command, "body", command.Body, "err", lastErr)
	return nil, lastErr, ctx
}
