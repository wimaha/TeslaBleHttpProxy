package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/keys"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/logging"
)

var ExceptedCommands = []string{"vehicle_data", "auto_conditioning_start", "auto_conditioning_stop", "charge_port_door_open", "charge_port_door_close", "flash_lights", "wake_up", "set_charging_amps", "set_charge_limit", "charge_start", "charge_stop", "session_info", "honk_horn", "door_lock", "door_unlock", "set_sentry_mode", "add_charge_schedule", "remove_charge_schedule", "set_temps", "actuate_trunk"}
var ExceptedEndpoints = []string{"charge_state", "climate_state", "drive_state"}

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
	case "set_temps":
		driverTemp, passengerTemp, err := parseTemps(command.Body)
		if err != nil {
			return false, err
		}
		if err := car.ChangeClimateTemp(ctx, driverTemp, passengerTemp); err != nil {
			return true, fmt.Errorf("failed to set temps to %.1f/%.1f: %s", driverTemp, passengerTemp, err)
		}
	case "actuate_trunk":
		whichTrunk, err := parseWhichTrunk(command.Body)
		if err != nil {
			return false, err
		}
		if whichTrunk == "front" {
			if err := car.OpenFrunk(ctx); err != nil {
				return true, fmt.Errorf("failed to open frunk: %s", err)
			}
		} else {
			// The rear trunk is a toggle. If the car acted but the reply was lost, a retry
			// would move it back, so never retry: report the error and let the client
			// check closure_statuses.rear_trunk before sending again.
			if err := car.ActuateTrunk(ctx); err != nil {
				return false, fmt.Errorf("failed to actuate trunk (not retried, it is a toggle): %s", err)
			}
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
				logging.Info("The car is already charging")
				return false, nil
			} else if strings.Contains(err.Error(), "complete") {
				//The charging is completed, so the command is somehow successfully executed.
				logging.Info("The charging is completed")
				return false, nil
			}
			return true, fmt.Errorf("failed to start charge: %s", err)
		}
	case "charge_stop":
		if err := car.ChargeStop(ctx); err != nil {
			if strings.Contains(err.Error(), "not_charging") {
				//The car has already stopped charging, so the command is somehow successfully executed.
				logging.Info("The car has already stopped charging")
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
		// Get active key files
		_, publicKeyFile := config.GetActiveKeyFiles()
		publicKey, err := protocol.LoadPublicKey(publicKeyFile)
		if err != nil {
			return false, fmt.Errorf("failed to load public key: %s", err)
		}

		info, err := car.SessionInfo(ctx, publicKey, protocol.DomainVCSEC)
		if err != nil {
			return true, fmt.Errorf("failed session_info: %s", err)
		}
		fmt.Printf("%s\n", info)
	case "add-key-request":
		// Get role from command body, default to charging_manager (recommended for security)
		roleStr := "charging_manager"
		if command.Body != nil {
			if role, ok := command.Body["role"].(string); ok && role != "" {
				roleStr = role
			}
		}

		// Validate role to prevent path traversal
		// Check against valid roles
		validRoles := []string{"owner", "charging_manager"}
		isValid := false
		for _, validRole := range validRoles {
			if roleStr == validRole {
				isValid = true
				break
			}
		}
		if !isValid {
			return false, fmt.Errorf("invalid role: %s. Valid roles are: owner, charging_manager", roleStr)
		}
		// Prevent path traversal attempts
		if strings.Contains(roleStr, "..") || strings.Contains(roleStr, "/") || strings.Contains(roleStr, "\\") {
			return false, fmt.Errorf("invalid role: contains path traversal characters")
		}

		// Get public key file for the specified role
		_, publicKeyFile := config.GetKeyFilesForRole(roleStr)
		publicKey, err := protocol.LoadPublicKey(publicKeyFile)
		if err != nil {
			return false, fmt.Errorf("failed to load public key: %s", err)
		}

		// Map role string to keys.Role enum
		var keyRole keys.Role
		switch roleStr {
		case "owner":
			keyRole = keys.Role_ROLE_OWNER
		case "charging_manager":
			keyRole = keys.Role_ROLE_CHARGING_MANAGER
		default:
			// Default to charging_manager (recommended for security)
			keyRole = keys.Role_ROLE_CHARGING_MANAGER
		}

		// Get display name for logging
		displayName := roleStr
		if roleStr == "" {
			displayName = "Legacy (Owner)"
		} else {
			switch roleStr {
			case "owner":
				displayName = "Owner"
			case "charging_manager":
				displayName = "Charging Manager"
			}
		}

		if err := car.SendAddKeyRequestWithRole(ctx, publicKey, keyRole, vcsec.KeyFormFactor_KEY_FORM_FACTOR_CLOUD_KEY); err != nil {
			return true, fmt.Errorf("failed to add key: %s", err)
		} else {
			logging.Info(fmt.Sprintf("Sent add-key request to %s with role %s. Confirm by tapping NFC card on center console.", car.VIN(), displayName))
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
			//log.Debugf("get: %s", endpoint)
			category, err := GetCategory(endpoint)
			if err != nil {
				return false, err
			}
			data, err := car.GetState(ctx, category)
			if err != nil {
				return true, fmt.Errorf("Failed to get vehicle data: %s", err)
			}
			/*d, err := protojson.Marshal(data)
			if err != nil {
				return true, fmt.Errorf("failed to marshal vehicle data: %s", err)
			}
			logging.Debugf("data: %s", d)*/

			var converted interface{}
			switch endpoint {
			case "charge_state":
				converted = models.ChargeStateFromBle(data)
			case "climate_state":
				converted = models.ClimateStateFromBle(data)
			case "drive_state":
				converted = models.DriveStateFromBle(data)
			}
			d, err := json.Marshal(converted)
			if err != nil {
				return true, fmt.Errorf("Failed to marshal vehicle data: %s", err)
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
	case "add_charge_schedule":
		schedule, err := chargeScheduleFromBody(command.Body)
		if err != nil {
			return false, err
		}
		if err := car.AddChargeSchedule(ctx, schedule); err != nil {
			// Without an id the car creates a new schedule: if it stored it but the reply was lost,
			// a retry would add a second one. Updating by id is safe to retry.
			return schedule.Id != 0, fmt.Errorf("failed to add charge schedule: %s", err)
		}
	case "remove_charge_schedule":
		id, err := chargeScheduleIDFromBody(command.Body)
		if err != nil {
			return false, err
		}
		if err := car.RemoveChargeSchedule(ctx, id); err != nil {
			return true, fmt.Errorf("failed to remove charge schedule: %s", err)
		}
	default:
		return false, fmt.Errorf("unrecognized command: %s", command.Command)
	}

	// everything fine
	return false, nil
}

// ValidateBody checks a command body before the command is queued, so a bad body is
// rejected without connecting to (and possibly waking) the car. Commands without a
// check here are validated in Send only.
func ValidateBody(command string, body map[string]interface{}) error {
	switch command {
	case "set_temps":
		_, _, err := parseTemps(body)
		return err
	case "actuate_trunk":
		_, err := parseWhichTrunk(body)
		return err
	}
	return nil
}

// parseFloatField reads a numeric body field that may arrive as a JSON number or a string.
func parseFloatField(body map[string]interface{}, key string) (float32, bool, error) {
	raw, ok := body[key]
	if !ok || raw == nil {
		return 0, false, nil
	}
	switch v := raw.(type) {
	case float64:
		return float32(v), true, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 32)
		if err != nil {
			return 0, true, fmt.Errorf("%s parsing error: %s", key, err)
		}
		return float32(f), true, nil
	default:
		return 0, true, fmt.Errorf("%s has unsupported type %T", key, raw)
	}
}

// parseTemps reads the Fleet API set_temps body: {"driver_temp": 21, "passenger_temp": 21} in Celsius.
// passenger_temp is optional and defaults to driver_temp.
func parseTemps(body map[string]interface{}) (float32, float32, error) {
	driver, found, err := parseFloatField(body, "driver_temp")
	if err != nil {
		return 0, 0, err
	}
	if !found {
		return 0, 0, fmt.Errorf("driver_temp missing in body")
	}
	passenger, found, err := parseFloatField(body, "passenger_temp")
	if err != nil {
		return 0, 0, err
	}
	if !found {
		passenger = driver
	}
	for _, t := range []float32{driver, passenger} {
		if t < 15 || t > 28 {
			return 0, 0, fmt.Errorf("temperature %.1f out of range 15-28 °C", t)
		}
	}
	return driver, passenger, nil
}

// parseWhichTrunk reads the Fleet API actuate_trunk body: {"which_trunk": "rear"} or {"which_trunk": "front"}.
func parseWhichTrunk(body map[string]interface{}) (string, error) {
	raw, ok := body["which_trunk"]
	if !ok || raw == nil {
		return "", fmt.Errorf("which_trunk missing in body")
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("which_trunk has unsupported type %T", raw)
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "rear":
		return "rear", nil
	case "front":
		return "front", nil
	default:
		return "", fmt.Errorf("which_trunk must be \"rear\" or \"front\", got %q", s)
	}
}
