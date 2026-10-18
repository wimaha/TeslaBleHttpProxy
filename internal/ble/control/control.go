package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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

// instance is the active BleControl. Handlers read it once per request with Instance(); the
// dashboard swaps it when the active key changes.
var (
	instance atomic.Pointer[BleControl]
	setupMu  sync.Mutex
)

// bleMu is held for as long as a BLE connection is open, and while the adapter is released.
// The command loop, a loop being replaced after a key change, and SendKeysToVehicle each hold
// it, so there is never more than one connection and the adapter is never closed under one.
var bleMu sync.Mutex

// Instance returns the active BleControl, or nil if no key is loaded.
func Instance() *BleControl {
	return instance.Load()
}

// Replace makes bc the active BleControl and returns the previous one. It neither starts nor
// stops a loop; SetupBleControl and CloseBleControl do that.
func Replace(bc *BleControl) *BleControl {
	return instance.Swap(bc)
}

// SetupBleControl loads the active key, starts a new command loop and stops the previous one.
func SetupBleControl() {
	setupMu.Lock()
	defer setupMu.Unlock()

	bc, err := NewBleControl()
	if err != nil {
		logging.Warn("BleControl could not be initialized!")
		bc = nil
	} else {
		go bc.Loop()
		logging.Info("BleControl initialized")
	}
	if previous := Replace(bc); previous != nil {
		previous.Stop()
	}
}

// CloseBleControl stops the active command loop.
func CloseBleControl() {
	setupMu.Lock()
	defer setupMu.Unlock()

	if previous := Replace(nil); previous != nil {
		previous.Stop()
	}
}

// errStopped answers commands that reach a BleControl after it was replaced.
var errStopped = errors.New("BLE control was restarted (active key changed), please retry")

// errQueueFull answers commands while the queue is full, which means the car has been
// unreachable for a while and requests piled up.
var errQueueFull = errors.New("command queue is full, the vehicle is not reachable at the moment")

// Car is a connected vehicle: everything Send needs plus session handling.
// *vehicle.Vehicle implements it; tests use internal/ble/fakecar.
type Car interface {
	commands.Vehicle
	StartSession(ctx context.Context, domains []universalmessage.Domain) error
	Disconnect()
}

// Dialer opens a connection to the vehicle with the given VIN. retry reports whether a failed
// attempt is worth repeating. The default dials over BLE.
type Dialer func(ctx context.Context, vin string) (car Car, retry bool, err error)

// Timing holds the waits of the command loop. Tests shorten them.
type Timing struct {
	LoopDelay         time.Duration // pause before each new connection
	RetryDelay        time.Duration // first wait before a retry, doubled each time
	ConnectionTimeout time.Duration // how long a connection is kept open for further commands
}

var DefaultTiming = Timing{
	LoopDelay:         1 * time.Second,
	RetryDelay:        3 * time.Second,
	ConnectionTimeout: 29 * time.Second,
}

type BleControl struct {
	privateKey protocol.ECDHPrivateKey
	dial       Dialer
	Timing     Timing

	// ReleaseAdapterWhenIdle hands the BLE adapter back to the system (BlueZ) whenever no
	// connection is open, so other Bluetooth services on the host can use it.
	ReleaseAdapterWhenIdle bool
	releaseAdapter         func() error

	commandStack  chan commands.Command
	providerStack chan commands.Command

	// stopMu orders pushes against Stop: after Stop no command can enter the queue, so the
	// loop can answer everything still queued and exit.
	stopMu  sync.RWMutex
	stopped bool
	done    chan struct{}

	closeRequest chan struct{} // see RequestClose

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

	bc := New(privateKey, nil)
	if config.AppConfig != nil {
		if config.AppConfig.ConnectionTimeout > 0 {
			bc.Timing.ConnectionTimeout = time.Duration(config.AppConfig.ConnectionTimeout) * time.Second
		}
		bc.ReleaseAdapterWhenIdle = config.AppConfig.ReleaseAdapterWhenIdle
	}
	return bc, nil
}

// New returns a BleControl that connects through dial, or over BLE if dial is nil.
// Without a private key only unauthenticated requests (add-key-request) are possible.
func New(privateKey protocol.ECDHPrivateKey, dial Dialer) *BleControl {
	bc := &BleControl{
		privateKey:     privateKey,
		dial:           dial,
		Timing:         DefaultTiming,
		releaseAdapter: ble.CloseAdapter,
		commandStack:   make(chan commands.Command, 50),
		providerStack:  make(chan commands.Command),
		done:           make(chan struct{}),
		closeRequest:   make(chan struct{}, 1),
		lastAwakeTime:  make(map[string]time.Time),
	}
	if bc.dial == nil {
		bc.dial = bc.bleDial
	}
	return bc
}

func (bc *BleControl) Loop() {
	var retryCommand *commands.Command
	defer func() {
		if retryCommand != nil {
			failCommand(retryCommand, errStopped)
		}
		bc.failQueued()
		logging.Debug("Command loop stopped")
	}()
	for {
		time.Sleep(bc.Timing.LoopDelay)
		if bc.isStopped() {
			return
		}
		if retryCommand != nil {
			logging.Info("Retrying command", "Command", retryCommand.Command, "Body", retryCommand.Body)
			retryCommand = bc.connectToVehicleAndOperateConnection(retryCommand)
		} else {
			if bc.ReleaseAdapterWhenIdle {
				bleMu.Lock()
				if err := bc.releaseAdapter(); err != nil {
					logging.Debug("Failed to release BLE adapter", "error", err)
				}
				bleMu.Unlock()
			}
			logging.Debug("Waiting for next command ...")
			// Wait for the next command
			select {
			case <-bc.done:
				return
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

// Stop ends the command loop. An open connection is closed, and commands that are queued or
// pushed later are answered with an error instead of waiting forever.
func (bc *BleControl) Stop() {
	bc.stopMu.Lock()
	defer bc.stopMu.Unlock()
	if !bc.stopped {
		bc.stopped = true
		close(bc.done)
	}
}

func (bc *BleControl) isStopped() bool {
	select {
	case <-bc.done:
		return true
	default:
		return false
	}
}

// failQueued answers every queued command with errStopped. Called by the loop after Stop, when
// no further command can be pushed.
func (bc *BleControl) failQueued() {
	for {
		select {
		case command := <-bc.commandStack:
			failCommand(&command, errStopped)
		default:
			return
		}
	}
}

// failCommand reports err to whoever waits for the command.
func failCommand(command *commands.Command, err error) {
	if command.Response == nil {
		return
	}
	command.Response.Error = err.Error()
	command.Response.Result = false
	if command.Response.Wait != nil {
		command.Response.Wait.Done()
	}
}

func (bc *BleControl) PushCommand(command string, vin string, body map[string]interface{}, response *models.ApiResponse, autoWakeup bool) {
	bc.Push(commands.Command{
		Command:    command,
		Vin:        vin,
		Body:       body,
		Response:   response,
		AutoWakeup: autoWakeup,
	})
}

// Push queues a command. After Stop, or if the queue is full, it is answered with an error
// right away. It never blocks, so Stop cannot be held up by a waiting push.
func (bc *BleControl) Push(command commands.Command) {
	bc.stopMu.RLock()
	defer bc.stopMu.RUnlock()
	if bc.stopped {
		failCommand(&command, errStopped)
		return
	}
	select {
	case bc.commandStack <- command:
	default:
		logging.Error("Command queue is full, rejecting command", "Command", command.Command)
		failCommand(&command, errQueueFull)
	}
}

// RequestClose asks the loop to close its connection once the current command is done, instead
// of keeping it open for the connection timeout. Used before opening a connection outside the
// loop (SendKeysToVehicle).
func (bc *BleControl) RequestClose() {
	select {
	case bc.closeRequest <- struct{}{}:
	default: // already requested
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
	bleMu.Lock()
	defer bleMu.Unlock()
	logging.Info("Connecting to Vehicle ...")
	//defer log.Debug("connecting to Vehicle done")

	var sleep = bc.Timing.RetryDelay
	var retryCount = 3
	var lastErr error

	commandError := func(err error) *commands.Command {
		logging.Error("Cannot connect to vehicle", "Error", err)
		failCommand(firstCommand, err)
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
			logging.Debug(fmt.Sprintf("Retrying in %s", sleep))
			select {
			case <-time.After(sleep):
			case <-parentCtx.Done():
				return commandError(parentCtx.Err())
			}
			sleep *= 2
		}
		logging.Debugf("Connecting to vehicle (Attempt %d) ...", i+1)
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Second)
		car, retry, err := bc.TryConnectToVehicle(ctx, firstCommand)
		if err == nil {
			//Successful - cancel the connection attempt context since we're done with it
			cancel()
			defer car.Disconnect()
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

// bleDial scans for the vehicle's beacon and connects to it over BLE.
func (bc *BleControl) bleDial(ctx context.Context, vin string) (Car, bool, error) {
	var conn *ble.Connection
	var car *vehicle.Vehicle
	var shouldDefer = true

	defer func() {
		if shouldDefer {
			if car != nil {
				car.Disconnect()
			}
			if conn != nil {
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

	scanResult, err := ble.ScanVehicleBeacon(scanCtx, vin)
	if err != nil {
		if scanCtx.Err() != nil {
			// Scan timed out - allow retry as vehicle might be temporarily out of range or experiencing transient BLE issues
			return nil, true, fmt.Errorf("Vehicle is not in range: %s", err)
		} else {
			if strings.Contains(err.Error(), "operation not permitted") {
				// The underlying BLE package calls HCIDEVDOWN on the BLE device, presumably as a
				// heavy-handed way of dealing with devices that are in a bad state.
				return nil, false, fmt.Errorf("failed to connect to vehicle (A): %s\nTry again after granting this application CAP_NET_ADMIN:\nsudo setcap 'cap_net_admin=eip' \"$(which %s)\"", err, os.Args[0])
			} else {
				return nil, true, fmt.Errorf("failed to connect to vehicle (A): %s", err)
			}
		}
	}

	logging.Debug("Beacon found", "LocalName", scanResult.LocalName, "Address", scanResult.Address, "RSSI", scanResult.RSSI)
	//log.Debug("Connecting to vehicle ...")
	conn, err = ble.NewConnectionFromScanResult(ctx, vin, scanResult)
	if err != nil {
		return nil, true, fmt.Errorf("failed to connect to vehicle (A): %s", err)
	}

	/*conn, err = ble.NewConnection(ctx, vin)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			// The underlying BLE package calls HCIDEVDOWN on the BLE device, presumably as a
			// heavy-handed way of dealing with devices that are in a bad state.
			return nil, false, fmt.Errorf("failed to connect to vehicle (A): %s\nTry again after granting this application CAP_NET_ADMIN:\nsudo setcap 'cap_net_admin=eip' \"$(which %s)\"", err, os.Args[0])
		} else {
			return nil, true, fmt.Errorf("failed to connect to vehicle (A): %s", err)
		}
	}*/

	logging.Debug("Creating vehicle object ...")
	car, err = vehicle.NewVehicle(conn, bc.privateKey, nil)
	if err != nil {
		return nil, true, fmt.Errorf("failed to connect to vehicle (B): %s", err)
	}

	logging.Debug("Connecting ...")
	if err := car.Connect(ctx); err != nil {
		return nil, true, fmt.Errorf("failed to connect to vehicle (C): %s", err)
	}

	shouldDefer = false
	return &bleCar{Vehicle: car, conn: conn}, false, nil
}

// bleCar closes the BLE connection together with the vehicle session.
type bleCar struct {
	*vehicle.Vehicle
	conn *ble.Connection
}

func (c *bleCar) Disconnect() {
	c.Vehicle.Disconnect()
	c.conn.Close()
}

// TryConnectToVehicle connects, starts the sessions the command needs and wakes the car if
// required. On success the caller must Disconnect the returned car.
func (bc *BleControl) TryConnectToVehicle(ctx context.Context, firstCommand *commands.Command) (Car, bool, error) {
	var shouldDefer = true

	car, retry, err := bc.dial(ctx, firstCommand.Vin)
	if err != nil {
		return nil, retry, err
	}
	defer func() {
		if shouldDefer {
			car.Disconnect()
		}
	}()

	//Start Session only if privateKey is available
	if bc.privateKey != nil {
		logging.Debug("Starting VCSEC session ...")
		// First connect just VCSEC
		if err := car.StartSession(ctx, []universalmessage.Domain{
			protocol.DomainVCSEC,
		}); err != nil {
			return nil, true, fmt.Errorf("failed to perform handshake with vehicle (A): %s", err)
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
									return nil, true, fmt.Errorf("failed to wake up car: %s", err)
								}
								logging.Debug("Car wakeup command sent")
								// Mark as awake after successful wakeup
								bc.markVehicleAwake(firstCommand.Vin)
							} else {
								return nil, false, fmt.Errorf("vehicle sleep status unknown and wakeup not requested")
							}
						} else {
							sleepStatus := vs.GetVehicleSleepStatus().String()
							if strings.Contains(sleepStatus, "ASLEEP") {
								logging.Debug("Vehicle is asleep")
								if firstCommand.AutoWakeup {
									logging.Debug("Waking up vehicle as requested ...")
									if err := car.Wakeup(ctx); err != nil {
										return nil, true, fmt.Errorf("failed to wake up car: %s", err)
									}
									logging.Debug("Car successfully wakeup")
									// Mark as awake after successful wakeup
									bc.markVehicleAwake(firstCommand.Vin)
								} else {
									return nil, false, fmt.Errorf("vehicle is sleeping")
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
									return nil, false, fmt.Errorf("vehicle sleep status unknown and wakeup not requested")
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
						return nil, true, fmt.Errorf("failed to wake up car: %s", err)
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
					return nil, true, fmt.Errorf("failed to perform handshake with vehicle (B): %s", err)
				}
				logging.Info("Connection to vehicle established")
			}
		}
	} else {
		logging.Info("Key-Request connection established ...")
	}

	// everything fine
	shouldDefer = false
	return car, false, nil
}

func (bc *BleControl) operateConnection(car Car, firstCommand *commands.Command) *commands.Command {
	logging.Debug("Operating connection ...")
	//defer log.Debug("operating connection done")
	connectionCtx, cancel := context.WithTimeout(context.Background(), bc.Timing.ConnectionTimeout)
	defer cancel()

	cmd, err, _ := bc.ExecuteCommand(car, firstCommand, connectionCtx)
	if err != nil {
		return cmd
	}

	// TryConnectToVehicle starts Infotainment only for commands that need it.
	infotainment := needsInfotainment(firstCommand)

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
			infotainment = true
		}
	}

	handleCommand := func(command *commands.Command) (doReturn bool, retryCommand *commands.Command) {
		//If new VIN, close connection
		if command.Vin != firstCommand.Vin {
			logging.Debug("New VIN, closing connection ...")
			return true, command
		}
		// A VCSEC-only connection (body-controller-state) cannot serve this command. Reconnect,
		// which runs the wake-up and session logic for it.
		if needsInfotainment(command) && !infotainment {
			logging.Debug("Command needs Infotainment, reconnecting ...", "Command", command.Command)
			return true, command
		}

		cmd, err, ctx := bc.ExecuteCommand(car, command, connectionCtx)

		// If the connection context is done, return to reoperate the connection
		if connectionCtx.Err() != nil {
			return true, cmd
		}
		// A request the caller got wrong says nothing about the connection: keep it open for the
		// commands queued behind it.
		var badInput *commands.InvalidInputError
		if errors.As(err, &badInput) {
			return false, nil
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
		case <-bc.done:
			logging.Debug("Command loop stopped, closing connection ...")
			return nil
		case <-bc.closeRequest:
			logging.Debug("Connection needed elsewhere, closing connection ...")
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

// needsInfotainment reports whether a command needs an Infotainment session. wake_up and
// VCSEC commands (body-controller-state) work with VCSEC alone.
func needsInfotainment(command *commands.Command) bool {
	return command.Domain != commands.Domain.VCSEC && command.Command != "wake_up"
}

func (bc *BleControl) ExecuteCommand(car commands.Vehicle, command *commands.Command, connectionCtx context.Context) (retryCommand *commands.Command, retErr error, ctx context.Context) {
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

	var sleep = bc.Timing.RetryDelay
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
			logging.Info(fmt.Sprintf("Retrying in %s", sleep))

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
			var badInput *commands.InvalidInputError
			if errors.As(err, &badInput) {
				return nil, err, ctx
			}
			// Any other error that is not worth retrying (a toggle that must not be repeated, an
			// unknown command, a missing key file) is reported, never turned into success. The
			// returned command is nil, so it is not re-run.
			logging.Error("Failed, not retried", "Command", command.Command, "Body", command.Body, "Error", err)
			return nil, err, ctx
		}

		if strings.Contains(err.Error(), "closed pipe") {
			return command, err, ctx
		}

		lastErr = err
	}

	logging.Error("Canceled", "Command", command.Command, "Body", command.Body, "Error", lastErr)
	return nil, lastErr, ctx
}
