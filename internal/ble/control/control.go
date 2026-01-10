package control

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/logging"
	"github.com/wimaha/TeslaBleHttpProxy/internal/tesla/commands"
)

var BleControlInstance *BleControl = nil

func SetupBleControl() {
	var err error
	if BleControlInstance, err = NewBleControl(); err != nil {
		logging.Warn("BleControl could not be initialized!")
	} else {
		go BleControlInstance.Loop()
		logging.Info("BleControl initialized")
	}
}

func CloseBleControl() {
	BleControlInstance = nil
}

type BleControl struct {
	privateKey protocol.ECDHPrivateKey

	commandStack  chan commands.Command
	providerStack chan commands.Command

	// Cache to track when each vehicle was last confirmed awake
	lastAwakeTime map[string]time.Time
	awakeTimeMu   sync.RWMutex
}

func NewBleControl() (*BleControl, error) {
	var privateKey protocol.ECDHPrivateKey
	var err error

	// Get active key files
	privateKeyFile, _ := config.GetActiveKeyFiles()

	// Load private key (protected by UNIX file permissions)
	if privateKey, err = LoadPrivateKey(privateKeyFile); err != nil {
		logging.Error("Failed to load private key.", "err", err)
		return nil, fmt.Errorf("Failed to load private key: %s", err)
	}
	logging.Debug("PrivateKeyFile loaded", "PrivateKeyFile", privateKeyFile, "Role", GetActiveKeyRole())

	return &BleControl{
		privateKey:    privateKey,
		commandStack:  make(chan commands.Command, 50),
		providerStack: make(chan commands.Command),
		lastAwakeTime: make(map[string]time.Time),
	}, nil
}

func (bc *BleControl) Loop() {
	var retryCommand *commands.Command
	for {
		time.Sleep(1 * time.Second)
		if retryCommand != nil {
			logging.Info("Retrying command", "Command", retryCommand.Command, "Body", retryCommand.Body)
			retryCommand = bc.connectToVehicleAndOperateConnection(retryCommand)
		} else {
			logging.Debug("Waiting for next command ...")
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

// shouldCheckSleepStatus returns true if we need to check the vehicle's sleep status
// (i.e., if it's been more than 9 minutes since we last confirmed it was awake)
func (bc *BleControl) shouldCheckSleepStatus(vin string) bool {
	bc.awakeTimeMu.RLock()
	lastAwake, exists := bc.lastAwakeTime[vin]
	bc.awakeTimeMu.RUnlock()

	if !exists {
		return true // No cache entry, need to check
	}

	// Check if it's been more than 9 minutes
	return time.Since(lastAwake) > 9*time.Minute
}

// markVehicleAwake records that the vehicle was confirmed awake at this time
func (bc *BleControl) markVehicleAwake(vin string) {
	bc.awakeTimeMu.Lock()
	bc.lastAwakeTime[vin] = time.Now()
	bc.awakeTimeMu.Unlock()
}

func (bc *BleControl) connectToVehicleAndOperateConnection(firstCommand *commands.Command) *commands.Command {
	logging.Info("Connecting to Vehicle ...")
	//defer log.Debug("connecting to Vehicle done")

	var sleep = 3 * time.Second
	var retryCount = 3
	var lastErr error

	commandError := func(err error) *commands.Command {
		logging.Error("Cannot connect to vehicle", "Error", err)
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
			logging.Warn("No context provided, using default", "Command", firstCommand.Command, "Body", firstCommand.Body)
		}
		parentCtx = context.Background()
	}

	for i := 0; i < retryCount; i++ {
		if i > 0 {
			logging.Warn("Retry error", "error", lastErr)
			logging.Debug(fmt.Sprintf("Retrying in %d seconds", sleep/time.Second))
			select {
			case <-time.After(sleep):
			case <-parentCtx.Done():
				return commandError(parentCtx.Err())
			}
			sleep *= 2
		}
		logging.Debugf("Connecting to vehicle (Attempt %d) ...", i+1)
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Second)
		conn, car, retry, err := bc.TryConnectToVehicle(ctx, firstCommand)
		if err == nil {
			//Successful - cancel the connection attempt context since we're done with it
			cancel()
			defer conn.Close()
			//defer log.Debug("close connection (A)")
			defer car.Disconnect()
			//defer log.Debug("disconnect vehicle (A)")
			cmd := bc.operateConnection(car, firstCommand)
			return cmd
		} else if !retry || parentCtx.Err() != nil {
			//Failed but no retry possible - cancel context before returning
			cancel()
			return commandError(err)
		} else {
			// Will retry - cancel this attempt's context before next iteration
			cancel()
			lastErr = err
		}
	}
	logging.Error(fmt.Sprintf("Stop retrying after %d attempts", retryCount), "Error", lastErr)
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
	logging.Debug("Scanning for vehicle ...")
	// Vehicle sends a beacon every ~200ms, so if it is not found in scanTimeout seconds, it is likely not in range and not worth retrying.
	// The scan context is created independently to ensure it gets the full scanTimeout duration,
	// regardless of how much time remains on the parent context.
	scanTimeout := config.AppConfig.ScanTimeout
	var scanCtx context.Context
	var cancelScan context.CancelFunc
	if scanTimeout > 0 {
		// Create scan context with full timeout duration, but also respect parent context cancellation
		// This ensures the scan gets the full scanTimeout seconds, not limited by parent context's remaining time
		baseCtx := context.Background()
		scanCtx, cancelScan = context.WithTimeout(baseCtx, time.Duration(scanTimeout)*time.Second)
		// Also cancel scan if parent context is cancelled (to allow early termination)
		// ctx.Done() always returns a non-nil channel, so no nil check needed
		go func() {
			select {
			case <-ctx.Done():
				cancelScan()
			case <-scanCtx.Done():
				// Scan completed or timed out, exit goroutine
			}
		}()
	} else {
		scanCtx, cancelScan = context.WithCancel(ctx)
	}
	defer cancelScan()

	scanResult, err := ble.ScanVehicleBeacon(scanCtx, firstCommand.Vin)
	if err != nil {
		if scanCtx.Err() != nil {
			// Scan timed out - allow retry as vehicle might be temporarily out of range or experiencing transient BLE issues
			return nil, nil, true, fmt.Errorf("Vehicle is not in range: %s", err)
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

	logging.Debug("Beacon found", "LocalName", scanResult.LocalName, "Address", scanResult.Address, "RSSI", scanResult.RSSI)
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

	logging.Debug("Creating vehicle object ...")
	car, err = vehicle.NewVehicle(conn, bc.privateKey, nil)
	if err != nil {
		return nil, nil, true, fmt.Errorf("failed to connect to vehicle (B): %s", err)
	}

	logging.Debug("Connecting ...")
	if err := car.Connect(ctx); err != nil {
		return nil, nil, true, fmt.Errorf("failed to connect to vehicle (C): %s", err)
	}
	//defer car.Disconnect()

	//Start Session only if privateKey is available
	if bc.privateKey != nil {
		logging.Debug("Starting VCSEC session ...")
		// First connect just VCSEC
		if err := car.StartSession(ctx, []universalmessage.Domain{
			protocol.DomainVCSEC,
		}); err != nil {
			return nil, nil, true, fmt.Errorf("failed to perform handshake with vehicle (A): %s", err)
		}

		// wake_up command can execute with just VCSEC, but we still need Infotainment for other commands
		isWakeUpCommand := firstCommand.Command == "wake_up"

		if firstCommand.Domain != commands.Domain.VCSEC || isWakeUpCommand {
			// For wake_up, skip sleep check and Infotainment setup (it only needs VCSEC)
			if isWakeUpCommand {
				logging.Debug("Wake_up command detected, VCSEC session is sufficient")
				logging.Info("Connection to vehicle established (VCSEC only for wake_up)")
			} else {
				// For vehicle_data, use conditional wakeup (check cache, only wake if needed)
				// For all other commands, always wake up if needed
				isVehicleData := firstCommand.Command == "vehicle_data"

				if isVehicleData {
					// Conditional wakeup for vehicle_data: check cache first
					needToCheck := bc.shouldCheckSleepStatus(firstCommand.Vin)

					if needToCheck {
						logging.Debug("Checking vehicle sleep status for vehicle_data (cache expired or not available) ...")
						vs, err := car.BodyControllerState(ctx)
						if err != nil {
							logging.Debug("Failed to get body controller state", "Error", err)
							// If we can't check status and AutoWakeup is requested, try to wake up anyway
							if firstCommand.AutoWakeup {
								logging.Debug("Attempting wakeup since status check failed and AutoWakeup is enabled")
								if err := car.Wakeup(ctx); err != nil {
									return nil, nil, true, fmt.Errorf("failed to wake up car: %s", err)
								}
								logging.Debug("Car wakeup command sent")
								// Mark as awake after successful wakeup
								bc.markVehicleAwake(firstCommand.Vin)
							} else {
								return nil, nil, false, fmt.Errorf("vehicle sleep status unknown and wakeup not requested")
							}
						} else {
							sleepStatus := vs.GetVehicleSleepStatus().String()
							if strings.Contains(sleepStatus, "ASLEEP") {
								logging.Debug("Vehicle is asleep")
								if firstCommand.AutoWakeup {
									logging.Debug("Waking up vehicle as requested ...")
									if err := car.Wakeup(ctx); err != nil {
										return nil, nil, true, fmt.Errorf("failed to wake up car: %s", err)
									}
									logging.Debug("Car successfully wakeup")
									// Mark as awake after successful wakeup
									bc.markVehicleAwake(firstCommand.Vin)
								} else {
									return nil, nil, false, fmt.Errorf("vehicle is sleeping")
								}
							} else if strings.Contains(sleepStatus, "AWAKE") {
								logging.Debug("Vehicle is already awake")
								// Update cache - vehicle is confirmed awake
								bc.markVehicleAwake(firstCommand.Vin)
							} else {
								logging.Debug("Vehicle sleep status unknown")
								// If status is unknown and AutoWakeup is requested, attempt wakeup to be safe
								if firstCommand.AutoWakeup {
									logging.Debug("Attempting wakeup since status is unknown and AutoWakeup is enabled")
									if err := car.Wakeup(ctx); err != nil {
										logging.Debug("Wakeup failed but continuing", "Error", err)
									} else {
										// Mark as awake after successful wakeup
										bc.markVehicleAwake(firstCommand.Vin)
									}
								} else {
									return nil, nil, false, fmt.Errorf("vehicle sleep status unknown and wakeup not requested")
								}
							}
						}
					} else {
						logging.Debug("Skipping sleep status check for vehicle_data (vehicle was awake less than 9 minutes ago)")
					}
				} else {
					// For commands, always send wakeup (no need to check sleep status first)
					logging.Debug("Command detected, sending wakeup ...")
					if err := car.Wakeup(ctx); err != nil {
						return nil, nil, true, fmt.Errorf("failed to wake up car: %s", err)
					}
					logging.Debug("Car successfully wakeup")
					// Mark as awake after successful wakeup
					bc.markVehicleAwake(firstCommand.Vin)
				}

				logging.Debug("Starting Infotainment session ...")
				// Then we can also connect the infotainment
				if err := car.StartSession(ctx, []universalmessage.Domain{
					protocol.DomainVCSEC,
					protocol.DomainInfotainment,
				}); err != nil {
					return nil, nil, true, fmt.Errorf("failed to perform handshake with vehicle (B): %s", err)
				}
				logging.Info("Connection to vehicle established")
			}
		}
	} else {
		logging.Info("Key-Request connection established ...")
	}

	// everything fine
	shouldDefer = false
	return conn, car, false, nil
}

func (bc *BleControl) operateConnection(car *vehicle.Vehicle, firstCommand *commands.Command) *commands.Command {
	logging.Debug("Operating connection ...")
	//defer log.Debug("operating connection done")
	connectionCtx, cancel := context.WithTimeout(context.Background(), 29*time.Second)
	defer cancel()

	cmd, err, _ := bc.ExecuteCommand(car, firstCommand, connectionCtx)
	if err != nil {
		return cmd
	}

	// If wake_up command executed successfully, upgrade session to include Infotainment
	// for subsequent commands that might need it
	if firstCommand.Command == "wake_up" {
		logging.Debug("Wake_up executed successfully, upgrading session to include Infotainment for subsequent commands")
		ctx, cancelUpgrade := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelUpgrade()
		if err := car.StartSession(ctx, []universalmessage.Domain{
			protocol.DomainVCSEC,
			protocol.DomainInfotainment,
		}); err != nil {
			logging.Debug("Failed to upgrade session to Infotainment, subsequent commands may fail", "Error", err)
		} else {
			logging.Debug("Session upgraded to include Infotainment")
		}
	}

	handleCommand := func(command *commands.Command) (doReturn bool, retryCommand *commands.Command) {
		//If new VIN, close connection
		if command.Vin != firstCommand.Vin {
			logging.Debug("New VIN, closing connection ...")
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
			logging.Debug("Connection timeout ...")
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
	logging.Info("Executing command", "Command", command.Command, "Body", command.Body)
	if command.Response != nil && command.Response.Ctx != nil {
		ctx = command.Response.Ctx
	} else {
		if command.Response != nil {
			logging.Debug("No context provided, using default", "Command", command.Command, "Body", command.Body)
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
			logging.Warn("Retry error", "error", lastErr)
			logging.Info(fmt.Sprintf("Retrying in %d seconds", sleep/time.Second))

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
			logging.Info("Successfully executed", "Command", command.Command, "Body", command.Body)
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

	logging.Error("Canceled", "Command", command.Command, "Body", command.Body, "Error", lastErr)
	return nil, lastErr, ctx
}
