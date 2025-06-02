package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
)

var ExceptedCommands = []string{"vehicle_data", "auto_conditioning_start", "auto_conditioning_stop", "charge_port_door_open", "charge_port_door_close", "flash_lights", "wake_up", "set_charging_amps", "set_charge_limit", "charge_start", "charge_stop", "session_info", "honk_horn", "door_lock", "door_unlock", "set_sentry_mode"}
var ExceptedEndpoints = []string{"charge_state", "climate_state"}

func (command *Command) Send(ctx context.Context, car *vehicle.Vehicle) (shouldRetry bool, err error) {
	switch command.Command {
	case "auto_conditioning_start":
		if err := car.ClimateOn(ctx); err != nil {
			return true, fmt.Errorf("failed to start auto conditioning: %s", err)
		}
	case "auto_conditioning_stop":
		if err := car.ClimateOff(ctx); err != nil {
			return true, fmt.Errorf("failed to stop auto conditioning: %s", err)
		}
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
	case "honk_horn":
		if err := car.HonkHorn(ctx); err != nil {
			return true, fmt.Errorf("failed to honk horn %s", err)
		}
	case "door_lock":
		if err := car.Lock(ctx); err != nil {
			return true, fmt.Errorf("failed to lock %s", err)
		}
	case "door_unlock":
		if err := car.Unlock(ctx); err != nil {
			return true, fmt.Errorf("failed to unlock %s", err)
		}
	case "set_sentry_mode":
		var on bool
		switch v := command.Body["on"].(type) {
		case bool:
			on = v
		case string:
			if onBool, err := strconv.ParseBool(v); err == nil {
				on = onBool
			} else {
				return false, fmt.Errorf("on parsing error: %s", err)
			}
		default:
			return false, fmt.Errorf("on missing in body")
		}
		if err := car.SetSentryMode(ctx, on); err != nil {
			return true, fmt.Errorf("failed to set sentry mode %s", err)
		}
	case "charge_start":
		if err := car.ChargeStart(ctx); err != nil {
			if strings.Contains(err.Error(), "is_charging") {
				//The car is already charging, so the command is somehow successfully executed.
				log.Info("the car is already charging")
				return false, nil
			} else if strings.Contains(err.Error(), "complete") {
				//The charging is completed, so the command is somehow successfully executed.
				log.Info("the charging is completed")
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
		publicKey, err := protocol.LoadPublicKey(config.PublicKeyFile)
		if err != nil {
			return false, fmt.Errorf("failed to load public key: %s", err)
		}

		info, err := car.SessionInfo(ctx, publicKey, protocol.DomainVCSEC)
		if err != nil {
			return true, fmt.Errorf("failed session_info: %s", err)
		}
		fmt.Printf("%s\n", info)
	case "add-key-request":
		publicKey, err := protocol.LoadPublicKey(config.PublicKeyFile)
		if err != nil {
			return false, fmt.Errorf("failed to load public key: %s", err)
		}

		if err := car.SendAddKeyRequest(ctx, publicKey, true, vcsec.KeyFormFactor_KEY_FORM_FACTOR_CLOUD_KEY); err != nil {
			return true, fmt.Errorf("failed to add key: %s", err)
		} else {
			log.Info(fmt.Sprintf("Sent add-key request to %s. Confirm by tapping NFC card on center console.", car.VIN()))
		}
	case "vehicle_data":
		if command.Body == nil {
			return false, fmt.Errorf("request body is nil")
		}

		endpoints, ok := command.Body["endpoints"].([]string)
		if !ok {
			return false, fmt.Errorf("missing or invalid 'endpoints' in request body")
		}

		response := make(map[string]json.RawMessage)
		for _, endpoint := range endpoints {
			log.Debugf("get: %s", endpoint)
			category, err := GetCategory(endpoint)
			if err != nil {
				return false, fmt.Errorf("unrecognized state category charge")
			}
			data, err := car.GetState(ctx, category)
			if err != nil {
				return true, fmt.Errorf("failed to get vehicle data: %s", err)
			}
			/*d, err := protojson.Marshal(data)
			if err != nil {
				return true, fmt.Errorf("failed to marshal vehicle data: %s", err)
			}
			log.Debugf("data: %s", d)*/

			var converted interface{}
			switch endpoint {
			case "charge_state":
				converted = models.ChargeStateFromBle(data)
			case "climate_state":
				converted = models.ClimateStateFromBle(data)
			}
			d, err := json.Marshal(converted)
			if err != nil {
				return true, fmt.Errorf("failed to marshal vehicle data: %s", err)
			}

			response[endpoint] = d
		}

		responseJson, err := json.Marshal(response)
		if err != nil {
			return false, fmt.Errorf("failed to marshal vehicle data: %s", err)
		}
		command.Response.Response = responseJson
	case "body-controller-state":
		vs, err := car.BodyControllerState(ctx)
		if err != nil {
			return true, fmt.Errorf("failed to get body controller state: %s", err)
		}
		vsJson, err := json.Marshal(models.VehicleStatusFromBle(vs))
		if err != nil {
			return true, fmt.Errorf("failed to marshal body-controller-state: %s", err)
		}
		command.Response.Response = vsJson
	default:
		return false, fmt.Errorf("unrecognized command: %s", command.Command)
	}

	// everything fine
	return false, nil
}
