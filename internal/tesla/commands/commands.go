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

var ExceptedCommands = []string{"vehicle_data", "auto_conditioning_start", "auto_conditioning_stop", "charge_port_door_open", "charge_port_door_close", "flash_lights", "wake_up", "set_charging_amps", "set_charge_limit", "charge_start", "charge_stop", "session_info", "honk_horn", "door_lock", "door_unlock", "set_sentry_mode", "add_charge_schedule", "remove_charge_schedule"}
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
	    schedule := &vehicle.ChargeSchedule{}
	    if v, ok := command.Body["id"].(float64); ok {
	        schedule.Id = uint64(v)
	    }
	    if v, ok := command.Body["start_enabled"].(bool); ok {
	        schedule.StartEnabled = v
	    }
	    if v, ok := command.Body["start_time"].(float64); ok {
	        schedule.StartTime = int32(v)
	    }
	    if v, ok := command.Body["end_enabled"].(bool); ok {
	        schedule.EndEnabled = v
	    }
	    if v, ok := command.Body["end_time"].(float64); ok {
	        schedule.EndTime = int32(v)
	    }
	    if v, ok := command.Body["one_time"].(bool); ok {
	        schedule.OneTime = v
	    }
	    if v, ok := command.Body["enabled"].(bool); ok {
	        schedule.Enabled = v
	    }
	    if v, ok := command.Body["lat"].(float64); ok {
	        schedule.Latitude = float32(v)
	    }
	    if v, ok := command.Body["lon"].(float64); ok {
	        schedule.Longitude = float32(v)
	    }
	    if v, ok := command.Body["days_of_week"].(string); ok {
	        schedule.DaysOfWeek = parseDaysOfWeek(v)
	    }
	    if err := car.AddChargeSchedule(ctx, schedule); err != nil {
	        return true, fmt.Errorf("failed to add charge schedule: %s", err)
	    }
	case "remove_charge_schedule":
	    id, ok := command.Body["id"].(float64)
	    if !ok {
	        return false, fmt.Errorf("id missing in body")
	    }
	    if err := car.RemoveChargeSchedule(ctx, uint64(id)); err != nil {
	        return true, fmt.Errorf("failed to remove charge schedule: %s", err)
	}
	default:
	    return false, fmt.Errorf("unrecognized command: %s", command.Command)
    }

	// everything fine
	return false, nil
}

func parseDaysOfWeek(days string) int32 {
    dayMap := map[string]int32{
        "sunday": 1, "monday": 2, "tuesday": 4, "wednesday": 8,
        "thursday": 16, "friday": 32, "saturday": 64,
    }
    if strings.EqualFold(days, "all") {
        return 127
    }
    if strings.EqualFold(days, "weekdays") {
        return 62
    }
    var mask int32
    for _, d := range strings.Split(days, ",") {
        if bit, ok := dayMap[strings.ToLower(strings.TrimSpace(d))]; ok {
            mask |= bit
        }
    }
    return mask
}
