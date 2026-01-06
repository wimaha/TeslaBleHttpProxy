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
		log.Error("Failed to load private key.", "err", err)
		return nil, fmt.Errorf("Failed to load private key: %s", err)
	}
	log.Debug("PrivateKeyFile loaded", "PrivateKeyFile", config.PrivateKeyFile)

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
			log.Info("Retrying command", "Command", retryCommand.Command, "Body", retryCommand.Body)
			retryCommand = bc.connectToVehicleAndOperateConnection(retryCommand)
		} else {
			log.Debug("Waiting for next command ...")
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

func (bc *BleControl) PushCommand(command string, vin string, body map[string]interface{}, response *models.ApiResponse, autoWakeup bool) {
	bc.commandStack <- commands.Command{
		Command:    command,
		Vin:        vin,
		Body:       body,
		Response:   response,
		AutoWakeup: autoWakeup,
	}
}

func (bc *BleControl) connectToVehicleAndOperateConnection(firstCommand *commands.Command) *commands.Command {
	log.Info("Connecting to Vehicle ...")
	//defer log.Debug("connecting to Vehicle done")

	var sleep = 3 * time.Second
	var retryCount = 3
	var lastErr error

	commandError := func(err error) *commands.Command {
		log.Error("Cannot connect to vehicle", "Error", err)
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
			log.Warn("No context provided, using default", "Command", firstCommand.Command, "Body", firstCommand.Body)
		}
		parentCtx = context.Background()
	}

	for i := 0; i < retryCount; i++ {
		if i > 0 {
			log.Warn(lastErr)
			log.Debug(fmt.Sprintf("Retrying in %d seconds", sleep/time.Second))
			select {
			case <-time.After(sleep):
			case <-parentCtx.Done():
				return commandError(parentCtx.Err())
			}
			sleep *= 2
		}
		log.Debugf("Connecting to vehicle (Attempt %d) ...", i+1)
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Second)
		defer cancel()
		conn, car, retry, err := bc.TryConnectToVehicle(ctx, firstCommand)
		if err == nil {
			//Successful
			defer conn.Close()
			//defer log.Debug("close connection (A)")
			defer car.Disconnect()
			//defer log.Debug("disconnect vehicle (A)")
			cmd := bc.operateConnection(car, firstCommand)
			return cmd
		} else if !retry || parentCtx.Err() != nil {
			//Failed but no retry possible
			return commandError(err)
		} else {
			lastErr = err
		}
	}
	log.Error(fmt.Sprintf("Stop retrying after %d attempts", retryCount), "Error", lastErr)
	return commandError(lastErr)
}

func (bc *BleControl) TryConnectToVehicle(ctx context.Context, firstCommand *commands.Command) (*ble.Connection, *vehicle.Vehicle, bool, error) {
	//log.Debug("Trying to connect to vehicle ...")
	var conn *ble.Connection
	var car *vehicle.Vehicle
	var shouldDefer = true

	defer func() {
		if shouldDefer {
			if car != nil {
				//log.Debug("disconnect vehicle (B)")
				car.Disconnect()
			}
			if conn != nil {
				//log.Debug("close connection (B)")
				conn.Close()
			}
		}
	}()

	var err error
	log.Debug("Scanning for vehicle ...")
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
			return nil, nil, false, fmt.Errorf("Vehicle is not in range: %s", err)
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

	log.Debug("Beacon found", "LocalName", scanResult.LocalName, "Address", scanResult.Address, "RSSI", scanResult.RSSI)
	//log.Debug("Connecting to vehicle ...")
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

	log.Debug("Creating vehicle object ...")
	car, err = vehicle.NewVehicle(conn, bc.privateKey, nil)
	if err != nil {
		return nil, nil, true, fmt.Errorf("failed to connect to vehicle (B): %s", err)
	}

	log.Debug("Connecting ...")
	if err := car.Connect(ctx); err != nil {
		return nil, nil, true, fmt.Errorf("failed to connect to vehicle (C): %s", err)
	}
	//defer car.Disconnect()

	//Start Session only if privateKey is available
	if bc.privateKey != nil {
		log.Debug("Starting VCSEC session ...")
		// First connect just VCSEC
		if err := car.StartSession(ctx, []universalmessage.Domain{
			protocol.DomainVCSEC,
		}); err != nil {
			return nil, nil, true, fmt.Errorf("failed to perform handshake with vehicle (A): %s", err)
		}

		if firstCommand.Domain != commands.Domain.VCSEC {
			// Always check if the vehicle is awake before starting Infotainment session
			log.Debug("Checking vehicle sleep status ...")
			vs, err := car.BodyControllerState(ctx)
			if err != nil {
				log.Debug("Failed to get body controller state", "Error", err)
				// If we can't check status and AutoWakeup is requested, try to wake up anyway
				if firstCommand.AutoWakeup {
					log.Debug("Attempting wakeup since status check failed and AutoWakeup is enabled")
					if err := car.Wakeup(ctx); err != nil {
						return nil, nil, true, fmt.Errorf("failed to wake up car: %s", err)
					}
					log.Debug("Car wakeup command sent")
				} else {
					return nil, nil, false, fmt.Errorf("vehicle sleep status unknown and wakeup not requested")
				}
			} else {
				sleepStatus := vs.GetVehicleSleepStatus().String()
				if strings.Contains(sleepStatus, "ASLEEP") {
					log.Debug("Vehicle is asleep")
					if firstCommand.AutoWakeup {
						log.Debug("Waking up vehicle as requested ...")
						if err := car.Wakeup(ctx); err != nil {
							return nil, nil, true, fmt.Errorf("failed to wake up car: %s", err)
						}
						log.Debug("Car successfully wakeup")
					} else {
						return nil, nil, false, fmt.Errorf("vehicle is sleeping")
					}
				} else if strings.Contains(sleepStatus, "AWAKE") {
					log.Debug("Vehicle is already awake")
				} else {
					log.Debug("Vehicle sleep status unknown")
					// If status is unknown and AutoWakeup is requested, attempt wakeup to be safe
					if firstCommand.AutoWakeup {
						log.Debug("Attempting wakeup since status is unknown and AutoWakeup is enabled")
						if err := car.Wakeup(ctx); err != nil {
							log.Debug("Wakeup failed but continuing", "Error", err)
						}
					} else {
						return nil, nil, false, fmt.Errorf("vehicle sleep status unknown and wakeup not requested")
					}
				}
			}

			log.Debug("Starting Infotainment session ...")
			// Then we can also connect the infotainment
			if err := car.StartSession(ctx, []universalmessage.Domain{
				protocol.DomainVCSEC,
				protocol.DomainInfotainment,
			}); err != nil {
				return nil, nil, true, fmt.Errorf("failed to perform handshake with vehicle (B): %s", err)
			}
			log.Info("Connection to vehicle established")
		}
	} else {
		log.Info("Key-Request connection established ...")
	}

	// everything fine
	shouldDefer = false
	return conn, car, false, nil
}

func (bc *BleControl) operateConnection(car *vehicle.Vehicle, firstCommand *commands.Command) *commands.Command {
	log.Debug("Operating connection ...")
	//defer log.Debug("operating connection done")
	connectionCtx, cancel := context.WithTimeout(context.Background(), 29*time.Second)
	defer cancel()

	cmd, err, _ := bc.ExecuteCommand(car, firstCommand, connectionCtx)
	if err != nil {
		return cmd
	}

	handleCommand := func(command *commands.Command) (doReturn bool, retryCommand *commands.Command) {
		//If new VIN, close connection
		if command.Vin != firstCommand.Vin {
			log.Debug("New VIN, closing connection ...")
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
			log.Debug("Connection timeout ...")
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
	log.Info("Executing command", "Command", command.Command, "Body", command.Body)
	if command.Response != nil && command.Response.Ctx != nil {
		ctx = command.Response.Ctx
	} else {
		if command.Response != nil {
			log.Debug("No context provided, using default", "Command", command.Command, "Body", command.Body)
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

	// Create a single goroutine to handle both context cancellations
	go func() {
		select {
		case <-connectionCtx.Done():
			cancel()
		case <-ctx.Done():
		}
	}()

	for i := 0; i < retryCount; i++ {
		if i > 0 {
			log.Warn(lastErr)
			log.Info(fmt.Sprintf("Retrying in %d seconds", sleep/time.Second))

			select {
			case <-time.After(sleep):
			case <-ctx.Done():
				if connectionCtx.Err() != nil {
					return command, ctx.Err(), ctx
				}
				return nil, ctx.Err(), ctx
			}
			sleep *= 2
		}

		retry, err := command.Send(ctx, car)
		if err == nil {
			log.Info("Successfully executed", "Command", command.Command, "Body", command.Body)
			return nil, nil, ctx
		}

		if !retry {
			return nil, nil, ctx
		}

		if strings.Contains(err.Error(), "closed pipe") {
			return command, err, ctx
		}

		lastErr = err
	}

	log.Error("Canceled", "Command", command.Command, "Body", command.Body, "Error", lastErr)
	return nil, lastErr, ctx
}
