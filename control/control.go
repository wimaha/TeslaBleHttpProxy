package control

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

var PublicKeyFile = "key/public.pem"
var PrivateKeyFile = "key/private.pem"

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

	commandStack chan Command
}

func NewBleControl() (*BleControl, error) {
	var privateKey protocol.ECDHPrivateKey
	var err error
	if privateKey, err = protocol.LoadPrivateKey(PrivateKeyFile); err != nil {
		log.Error("failed to load private key.", "err", err)
		return nil, fmt.Errorf("failed to load private key: %s", err)
	}
	log.Debug("privateKeyFile loaded")

	return &BleControl{
		privateKey:   privateKey,
		commandStack: make(chan Command, 50),
	}, nil
}

func (bc *BleControl) Loop() {
	var retryCommand *Command
	for {
		time.Sleep(1 * time.Second)
		if retryCommand != nil {
			retryCommand = bc.connectToVehicleAndOperateConnection(retryCommand)
		} else {
			// Wait for the next command
			command, ok := <-bc.commandStack
			if ok {
				retryCommand = bc.connectToVehicleAndOperateConnection(&command)
			}
		}
	}
}

func (bc *BleControl) PushCommand(command string, vin string, body map[string]interface{}) {
	bc.commandStack <- Command{
		Command: command,
		Vin:     vin,
		Body:    body,
	}
	/*bc.commandStack.Push(Command{
		Command: command,
		Vin:     vin,
		Body:    body,
	})*/
}

func (bc *BleControl) connectToVehicleAndOperateConnection(firstCommand *Command) *Command {
	log.Info("connecting to Vehicle ...")

	var sleep = 3 * time.Second
	var retryCount = 3
	var lastErr error

	for i := 0; i < retryCount; i++ {
		if i > 0 {
			log.Warn(lastErr)
			log.Info(fmt.Sprintf("retrying in %d seconds", sleep/time.Second))
			time.Sleep(sleep)
			sleep *= 2
		}
		log.Debug("trying connecting to vehicle", "attempt", i+1)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		conn, car, retry, err := bc.tryConnectToVehicle(ctx, firstCommand)
		if err == nil {
			//Successful
			defer conn.Close()
			defer log.Debug("close connection (A)")
			defer car.Disconnect()
			defer log.Debug("disconnect vehicle (A)")
			cmd := bc.operateConnection(car, firstCommand)
			return cmd
		} else if !retry {
			//Failed but no retry possible
			log.Error("can't connect to vehicle", "error", err)
			return nil
		} else {
			lastErr = err
		}
	}
	log.Error(fmt.Sprintf("stop retrying after %d attempts", retryCount), "error", lastErr)
	return nil
}

func (bc *BleControl) tryConnectToVehicle(ctx context.Context, firstCommand *Command) (*ble.Connection, *vehicle.Vehicle, bool, error) {
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
	conn, err = ble.NewConnection(ctx, firstCommand.Vin)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			// The underlying BLE package calls HCIDEVDOWN on the BLE device, presumably as a
			// heavy-handed way of dealing with devices that are in a bad state.
			return nil, nil, false, fmt.Errorf("failed to connect to vehicle (A): %s\nTry again after granting this application CAP_NET_ADMIN:\nsudo setcap 'cap_net_admin=eip' \"$(which %s)\"", err, os.Args[0])
		} else {
			return nil, nil, true, fmt.Errorf("failed to connect to vehicle (A): %s", err)
		}
	}
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
	} else {
		log.Info("Key-Request connection established")
	}

	// everything fine
	shouldDefer = false
	return conn, car, false, nil
}

func (bc *BleControl) operateConnection(car *vehicle.Vehicle, firstCommand *Command) *Command {
	if firstCommand.Command != "wake_up" {
		cmd, err := bc.executeCommand(car, firstCommand)
		if err != nil {
			return cmd
		}
	}

	timeout := time.After(29 * time.Second)
	for {
		select {
		case <-timeout:
			log.Debug("connection Timeout")
			return nil
		case command, ok := <-bc.commandStack:
			if !ok {
				return nil
			}

			//If new VIN, close connection
			if command.Vin != firstCommand.Vin {
				log.Debug("new VIN, so close connection")
				return &command
			}

			cmd, err := bc.executeCommand(car, &command)
			if err != nil {
				return cmd
			}
		}
	}
}

func (bc *BleControl) executeCommand(car *vehicle.Vehicle, command *Command) (*Command, error) {
	log.Info("sending", "command", command.Command, "body", command.Body)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var sleep = 3 * time.Second
	var retryCount = 3
	var lastErr error

	for i := 0; i < retryCount; i++ {
		if i > 0 {
			log.Warn(lastErr)
			log.Info(fmt.Sprintf("retrying in %d seconds", sleep/time.Second))
			time.Sleep(sleep)
			sleep *= 2
		}

		retry, err := bc.sendCommand(ctx, car, command)
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
	log.Error("canceled", "command", command.Command, "body", command.Body, "err", lastErr)
	return nil, lastErr
}

func (bc *BleControl) sendCommand(ctx context.Context, car *vehicle.Vehicle, command *Command) (bool, error) {
	switch command.Command {
	case "charge_port_door_open":
		if err := car.ChargePortOpen(ctx); err != nil {
			return true, fmt.Errorf("failed to open charge port: %s", err)
		}
	case "charge_port_door_close":
		if err := car.ChargePortClose(ctx); err != nil {
			return true, fmt.Errorf("failed to close charge port: %s", err)
		}
	case "flash_lights":
		if err := car.FlashLights(ctx); err != nil {
			return true, fmt.Errorf("failed to flash lights: %s", err)
		}
	case "wake_up":
		if err := car.Wakeup(ctx); err != nil {
			return true, fmt.Errorf("failed to wake up car: %s", err)
		}
	case "charge_start":
		if err := car.ChargeStart(ctx); err != nil {
			if strings.Contains(err.Error(), "is_charging") {
				//The car is already charging, so the command is somehow successfully executed.
				log.Info("the car is already charging")
				return false, nil
			}
			return true, fmt.Errorf("failed to start charge: %s", err)
		}
	case "charge_stop":
		if err := car.ChargeStop(ctx); err != nil {
			if strings.Contains(err.Error(), "not_charging") {
				//The car has already stopped charging, so the command is somehow successfully executed.
				log.Info("the car has already stopped charging")
				return false, nil
			}
			return true, fmt.Errorf("failed to stop charge: %s", err)
		}
	case "set_charging_amps":
		var chargingAmps int32
		switch v := command.Body["charging_amps"].(type) {
		case float64:
			chargingAmps = int32(v)
		case string:
			if chargingAmps64, err := strconv.ParseInt(v, 10, 32); err == nil {
				chargingAmps = int32(chargingAmps64)
			} else {
				return false, fmt.Errorf("charing Amps parsing error: %s", err)
			}
		default:
			return false, fmt.Errorf("charing Amps missing in body")
		}
		if err := car.SetChargingAmps(ctx, chargingAmps); err != nil {
			return true, fmt.Errorf("failed to set charging Amps to %d: %s", chargingAmps, err)
		}
	case "set_charge_limit":
		var chargeLimit int32
		switch v := command.Body["percent"].(type) {
		case float64:
			chargeLimit = int32(v)
		case string:
			if chargeLimit64, err := strconv.ParseInt(v, 10, 32); err == nil {
				chargeLimit = int32(chargeLimit64)
			} else {
				return false, fmt.Errorf("charing Amps parsing error: %s", err)
			}
		default:
			return false, fmt.Errorf("charing Amps missing in body")
		}
		if err := car.ChangeChargeLimit(ctx, chargeLimit); err != nil {
			return true, fmt.Errorf("failed to set charge limit to %d %%: %s", chargeLimit, err)
		}
	case "session_info":
		publicKey, err := protocol.LoadPublicKey(PublicKeyFile)
		if err != nil {
			return false, fmt.Errorf("failed to load public key: %s", err)
		}

		info, err := car.SessionInfo(ctx, publicKey, protocol.DomainVCSEC)
		if err != nil {
			return true, fmt.Errorf("failed session_info: %s", err)
		}
		fmt.Printf("%s\n", info)
	case "add-key-request":
		publicKey, err := protocol.LoadPublicKey(PublicKeyFile)
		if err != nil {
			return false, fmt.Errorf("failed to load public key: %s", err)
		}

		if err := car.SendAddKeyRequest(ctx, publicKey, true, vcsec.KeyFormFactor_KEY_FORM_FACTOR_CLOUD_KEY); err != nil {
			return true, fmt.Errorf("failed to add key: %s", err)
		} else {
			log.Info(fmt.Sprintf("Sent add-key request to %s. Confirm by tapping NFC card on center console.", car.VIN()))
		}
	}

	// everything fine
	return false, nil
}
