package commands

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

type commandArgs map[string]interface{}

func (args commandArgs) validateString(key string, required bool) (string, error) {
	if _, ok := args[key]; !ok {
		if required {
			return "", fmt.Errorf("missing '%s' in request body", key)
		} else {
			return "", nil
		}
	}
	value, ok := args[key].(string)
	if !ok {
		return "", fmt.Errorf("expected '%s' to be a string", key)
	}
	return value, nil
}
func (args commandArgs) str(key string) string {
	value, _ := args[key].(string)
	return value
}

func (args commandArgs) optStr(key string, def string) string {
	if value, ok := args[key].(string); ok {
		return value
	}
	return def
}
func (args commandArgs) validateBool(key string, required bool) (bool, error) {
	if _, ok := args[key]; !ok {
		if required {
			return false, fmt.Errorf("missing '%s' in request body", key)
		} else {
			return false, nil
		}
	}
	value, ok := args[key].(bool)
	if !ok {
		return false, fmt.Errorf("expected '%s' to be a boolean", key)
	}
	return value, nil
}
func (args commandArgs) bool(key string) bool {
	value, _ := args[key].(bool)
	return value
}
func (args commandArgs) optBool(key string, def bool) bool {
	if value, ok := args[key].(bool); ok {
		return value
	}
	return def
}
func (args commandArgs) validateInt(key string, required bool) (int, error) {
	if _, ok := args[key]; !ok {
		if required {
			return 0, fmt.Errorf("missing '%s' in request body", key)
		} else {
			return 0, nil
		}
	}
	value, ok := args[key].(int)
	if !ok {
		if value, ok := args[key].(float64); ok {
			// Ensure that the float is actually an integer
			if value != math.Trunc(value) || value < math.MinInt32 || value > math.MaxInt32 || math.IsNaN(value) {
				return 0, fmt.Errorf("expected '%s' to be an integer", key)
			}
			args[key] = int(value)
			return int(value), nil
		}
		return 0, fmt.Errorf("expected '%s' to be an integer", key)
	}
	return value, nil
}
func (args commandArgs) int(key string) int {
	value, _ := args[key].(int)
	return value
}
func (args commandArgs) optInt(key string, def int) int {
	if value, ok := args[key].(float64); ok {
		return int(value)
	}
	return def
}
func (args commandArgs) validateFloat(key string, required bool) (float64, error) {
	if _, ok := args[key]; !ok {
		if required {
			return 0, fmt.Errorf("missing '%s' in request body", key)
		} else {
			return 0, nil
		}
	}
	value, ok := args[key].(float64)
	if !ok {
		value, ok := args[key].(int)
		if !ok {
			return 0, fmt.Errorf("expected '%s' to be a number", key)
		}
		args[key] = float64(value)
		return float64(value), nil
	}
	return value, nil
}
func (args commandArgs) float(key string) float64 {
	value, _ := args[key].(float64)
	return value
}
func (args commandArgs) float32(key string) float32 {
	return float32(args.float(key))
}
func (args commandArgs) optFloat(key string, def float64) float64 {
	if value, ok := args[key].(float64); ok {
		return value
	}
	return def
}

type fleetVehicleCommandHandler struct {
	validate   func(commandArgs) error
	execute    func(context.Context, *vehicle.Vehicle, commandArgs) error
	checkError func(error) error
}

// TODO: Eventually replace all of this with ExtractCommandAction
// from https://github.com/teslamotors/vehicle-command/blob/main/pkg/proxy/command.go
// however, for now, that implementation is missing some stuff and is not the same as Fleet API (for now)
// Progress: https://github.com/teslamotors/vehicle-command/issues/188
var fleetVehicleCommands = map[string]fleetVehicleCommandHandler{
	"FIXME/shut_up_warnings_for_unused_opt_methods": {
		validate: func(args commandArgs) error {
			// FIXME: Some of these methods are not used yet,
			// but might be in the future so we keep them here
			// to avoid warnings about unused methods
			args.optBool("", false)
			args.optInt("", 0)
			args.optStr("", "")
			args.optFloat("", 0)
			return fmt.Errorf("not intended to be used")
		},
	},
	"actuate_trunk": {
		validate: func(args commandArgs) error {
			which_trunk, err := args.validateString("which_trunk", true)
			if err != nil {
				return err
			}
			if which_trunk != "front" && which_trunk != "rear" {
				return fmt.Errorf("invalid 'which_trunk' value: %s", which_trunk)
			}

			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			switch args.str("which_trunk") {
			case "front":
				return car.OpenFrunk(ctx)
			case "rear":
				return car.ActuateTrunk(ctx)
			default:
				return fmt.Errorf("invalid 'which_trunk' value: %s", args["which_trunk"])
			}
		},
	},
	"add_charge_schedule": {
		validate: func(args commandArgs) error {
			// TODO: implement
			return fmt.Errorf("not implemented")
		},
	},
	"add_precondition_schedule": {
		validate: func(args commandArgs) error {
			// TODO: implement
			return fmt.Errorf("not implemented")
		},
	},
	"adjust_volume": {
		validate: func(args commandArgs) error {
			volume, err := args.validateFloat("volume", true)
			if err != nil {
				return err
			}

			if volume < 0 || volume > 11 {
				return fmt.Errorf("invalid 'volume' (should be in [0, 11])")
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			volume := args.float32("volume")
			if volume > 10 {
				log.Warn("volume greater than 10 can not be set via BLE, clamping to 10")
				volume = 10
			}
			return car.SetVolume(ctx, volume)
		},
	},
	"auto_conditioning_start": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ClimateOn(ctx)
		},
	},
	"auto_conditioning_stop": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ClimateOff(ctx)
		},
	},
	"cancel_software_update": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.CancelSoftwareUpdate(ctx)
		},
	},
	"charge_max_range": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ChargeMaxRange(ctx)
		},
	},
	"charge_port_door_close": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ChargePortClose(ctx)
		},
	},
	"charge_port_door_open": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ChargePortOpen(ctx)
		},
	},
	"charge_standard": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ChargeStandardRange(ctx)
		},
	},
	"charge_start": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ChargeStart(ctx)
		},
		checkError: func(err error) error {
			if strings.Contains(err.Error(), "is_charging") {
				return nil
			} else if strings.Contains(err.Error(), "complete") {
				return nil
			}
			return err
		},
	},
	"charge_stop": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ChargeStop(ctx)
		},
		checkError: func(err error) error {
			if strings.Contains(err.Error(), "not_charging") {
				return nil
			}
			return err
		},
	},
	"clear_pin_to_drive_admin": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.SetPINToDrive(ctx, false, "")
		},
	},
	"door_lock": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.Lock(ctx)
		},
	},
	"door_unlock": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.Unlock(ctx)
		},
	},
	"erase_user_data": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.EraseGuestData(ctx)
		},
	},
	"flash_lights": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.FlashLights(ctx)
		},
	},
	"guest_mode": {
		validate: func(args commandArgs) error {
			if _, err := args.validateBool("enable", true); err != nil {
				return err
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.SetGuestMode(ctx, args.bool("enable"))
		},
	},
	"honk_horn": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.HonkHorn(ctx)
		},
	},
	"media_next_fav": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not implemented") // TODO: https://github.com/teslamotors/vehicle-command/pull/356
		},
	},
	"media_next_track": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not implemented") // TODO: https://github.com/teslamotors/vehicle-command/pull/356
		},
	},
	"media_prev_fav": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not implemented") // TODO: https://github.com/teslamotors/vehicle-command/pull/356
		},
	},
	"media_prev_track": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not implemented") // TODO: https://github.com/teslamotors/vehicle-command/pull/356
		},
	},
	"media_toggle_playback": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ToggleMediaPlayback(ctx)
		},
	},
	"media_volume_up": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not implemented") // TODO: https://github.com/teslamotors/vehicle-command/pull/356
		},
	},
	"media_volume_down": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not implemented") // TODO: https://github.com/teslamotors/vehicle-command/pull/356
		},
	},
	"navigation_gps_request": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not implemented") // TODO: https://github.com/teslamotors/vehicle-command/issues/334
		},
	},
	"navigation_request": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not implemented") // TODO: https://github.com/teslamotors/vehicle-command/issues/334
		},
	},
	"navigation_sc_request": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not implemented") // TODO: https://github.com/teslamotors/vehicle-command/issues/334
		},
	},
	"navigation_waypoints_request": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not implemented") // TODO: https://github.com/teslamotors/vehicle-command/issues/334
		},
	},
	"remote_auto_seat_climate_request": {
		validate: func(args commandArgs) error {
			if _, err := args.validateBool("auto_climate_on", true); err != nil {
				return err
			}

			position, err := args.validateInt("auto_seat_position", true)
			if err != nil {
				return err
			}

			if _, ok := carserver.AutoSeatClimateAction_AutoSeatPosition_E_name[int32(position)]; !ok {
				return fmt.Errorf("invalid 'auto_seat_position' value: %d", position)
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			autoClimateOn := args.bool("auto_climate_on")
			position := args.int("auto_seat_position")
			var seat vehicle.SeatPosition
			switch carserver.AutoSeatClimateAction_AutoSeatPosition_E(position) {
			case carserver.AutoSeatClimateAction_AutoSeatPosition_FrontLeft:
				seat = vehicle.SeatFrontLeft
			case carserver.AutoSeatClimateAction_AutoSeatPosition_FrontRight:
				seat = vehicle.SeatFrontRight
			default:
				seat = vehicle.SeatUnknown
			}
			return car.AutoSeatAndClimate(ctx, []vehicle.SeatPosition{seat}, autoClimateOn)
		},
	},
	"remote_auto_steering_wheel_heat_climate_request": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not supported via BLE")
		},
	},
	"remote_boombox": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not supported via BLE")
		},
	},
	"remote_seat_cooler_request": {
		validate: func(args commandArgs) error {
			level, err := args.validateInt("seat_cooler_level", true)
			if err != nil {
				return err
			}
			if level < int(vehicle.LevelOff) || level > int(vehicle.LevelHigh) {
				return fmt.Errorf("invalid 'seat_cooler_level' value: %d", level)
			}

			position, err := args.validateInt("seat_position", true)
			if err != nil {
				return err
			}
			if position != int(vehicle.SeatFrontLeft) && position != int(vehicle.SeatFrontRight) {
				return fmt.Errorf("invalid 'seat_position' value: %d", position)
			}
			return nil

		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			level := args.int("seat_cooler_level")
			position := args.int("seat_position")
			return car.SetSeatCooler(ctx, vehicle.Level(level), vehicle.SeatPosition(position))
		},
	},
	"remote_seat_heater_request": {
		validate: func(args commandArgs) error {
			heater, err := args.validateInt("heater", true)
			if err != nil {
				return err
			}
			if heater < int(vehicle.SeatFrontLeft) || heater > int(vehicle.SeatThirdRowRight) {
				return fmt.Errorf("invalid 'heater' value: %d", heater)
			}

			level, err := args.validateInt("level", true)
			if err != nil {
				return err
			}
			if level < int(vehicle.LevelOff) || level > int(vehicle.LevelHigh) {
				return fmt.Errorf("invalid 'level' value: %d", level)
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			heater := args.int("heater")
			level := args.int("level")
			return car.SetSeatHeater(ctx, map[vehicle.SeatPosition]vehicle.Level{vehicle.SeatPosition(heater): vehicle.Level(level)})
		},
	},
	"remote_start_drive": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.RemoteDrive(ctx)
		},
	},
	"remote_steering_wheel_heat_level_request": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not supported via BLE")
		},
	},
	"remote_steering_wheel_heater_request": {
		validate: func(args commandArgs) error {
			if _, err := args.validateBool("on", true); err != nil {
				return err
			}

			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.SetSteeringWheelHeater(ctx, args.bool("on"))
		},
	},
	"remove_charge_schedule": {
		validate: func(args commandArgs) error {
			// TODO: implement
			return fmt.Errorf("not implemented")
		},
	},
	"remove_precondition_schedule": {
		validate: func(args commandArgs) error {
			// TODO: implement
			return fmt.Errorf("not implemented")
		},
	},
	"reset_pin_to_drive_pin": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ResetPIN(ctx)
		},
	},
	"reset_valet_pin": {
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ResetValetPin(ctx)
		},
	},
	"schedule_software_update": {
		validate: func(args commandArgs) error {
			sec, err := args.validateFloat("offset_sec", true)
			if err != nil {
				return err
			}
			if sec < 0 {
				return fmt.Errorf("invalid 'offset_sec' value: %f", sec)
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			sec := args.float("offset_sec")
			return car.ScheduleSoftwareUpdate(ctx, time.Duration(sec)*time.Second)
		},
	},
	"set_bioweapon_mode": {
		validate: func(args commandArgs) error {
			if _, err := args.validateBool("manual_override", true); err != nil {
				return err
			}
			if _, err := args.validateBool("on", true); err != nil {
				return err
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.SetBioweaponDefenseMode(ctx, args.bool("on"), args.bool("manual_override"))
		},
	},
	"set_cabin_overheat_protection": {
		validate: func(args commandArgs) error {
			if _, err := args.validateBool("fan_only", true); err != nil {
				return err
			}
			if _, err := args.validateBool("on", true); err != nil {
				return err
			}

			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			on := args.bool("on")
			onlyFans := args.bool("fan_only")
			return car.SetCabinOverheatProtection(ctx, on, onlyFans)
		},
	},
	"set_charge_limit": {
		validate: func(args commandArgs) error {
			percent, err := args.validateInt("percent", true)
			if err != nil {
				return err
			}
			if percent < 50 || percent > 100 {
				return fmt.Errorf("invalid 'percent' value: %d", percent)
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ChangeChargeLimit(ctx, int32(args.int("percent")))
		},
	},
	"set_charging_amps": {
		validate: func(args commandArgs) error {
			chargingAmps, err := args.validateInt("charging_amps", true)
			if err != nil {
				return err
			}
			if chargingAmps < 0 || chargingAmps > 48 {
				return fmt.Errorf("invalid 'charging_amps' value: %d", chargingAmps)
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.SetChargingAmps(ctx, int32(args.int("charging_amps")))
		},
	},
	"set_climate_keeper_mode": {
		validate: func(args commandArgs) error {
			mode, err := args.validateInt("climate_keeper_mode", true)
			if err != nil {
				return err
			}
			if mode < int(vehicle.ClimateKeeperModeOff) || mode > int(vehicle.ClimateKeeperModeCamp) {
				return fmt.Errorf("invalid 'climate_keeper_mode' value: %d", mode)
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.SetClimateKeeperMode(ctx, vehicle.ClimateKeeperMode(args.int("climate_keeper_mode")), true)
		},
	},
	"set_cop_temp": {
		validate: func(args commandArgs) error {
			copTemp, err := args.validateInt("cop_temp", true)
			if err != nil {
				return err
			}
			// NOTE: The vehicle.Level this function requires starts at 0 for low,
			// however we want to start with 1 for Low in Cop temp (0 is unspecified)
			// (check carserver.ClimateState_CopActivationTemp)
			copTemp += 1

			if copTemp < int(vehicle.LevelLow) || copTemp > int(vehicle.LevelHigh) {
				return fmt.Errorf("invalid 'cop_temp' value: %d", copTemp)
			}

			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.SetCabinOverheatProtectionTemperature(ctx, vehicle.Level(args.int("cop_temp")+1))
		},
	},
	"set_pin_to_drive": {
		validate: func(args commandArgs) error {
			if _, err := args.validateBool("enable", true); err != nil {
				return err
			}

			password, err := args.validateString("password", true)
			if err != nil {
				return err
			}
			if len(password) != 4 {
				return fmt.Errorf("invalid 'password' length: %d", len(password))
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.SetPINToDrive(ctx, args.bool("enable"), args.str("password"))
		},
	},
	"set_preconditioning_max": {
		validate: func(args commandArgs) error {
			if _, err := args.validateBool("on", true); err != nil {
				return err
			}
			if _, err := args.validateBool("manual_override", true); err != nil {
				return err
			}

			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.SetPreconditioningMax(ctx, args.bool("on"), args.bool("manual_override"))
		},
	},
	"set_scheduled_charging": {
		validate: func(args commandArgs) error {
			if _, err := args.validateBool("enable", true); err != nil {
				return err
			}

			time, err := args.validateFloat("time", true)
			if err != nil {
				return err
			}
			if time < 0 || time > 24*60-1 {
				return fmt.Errorf("invalid 'time' value: %f", time)
			}

			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ScheduleCharging(ctx, args.bool("enable"), time.Duration(args.float("time"))*time.Minute)
		},
	},
	"set_scheduled_departure": {
		validate: func(args commandArgs) error {
			// TODO: implement
			return fmt.Errorf("not implemented")
		},
	},
	"set_sentry_mode": {
		validate: func(args commandArgs) error {
			if _, err := args.validateBool("on", true); err != nil {
				return err
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.SetSentryMode(ctx, args.bool("on"))
		},
	},
	"set_temps": {
		validate: func(args commandArgs) error {
			// NOTE: The temp is always in Celsius, regardless of the car's region
			if _, err := args.validateFloat("driver_temp", true); err != nil {
				return err
			}
			if _, err := args.validateFloat("passenger_temp", true); err != nil {
				return err
			}

			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			driverTemp := args.float32("driver_temp")
			passengerTemp := args.float32("passenger_temp")
			return car.ChangeClimateTemp(ctx, driverTemp, passengerTemp)
		},
	},
	"set_valet_mode": {
		validate: func(args commandArgs) error {
			if _, err := args.validateBool("on", true); err != nil {
				return err
			}

			password, err := args.validateString("password", true)
			if err != nil {
				return err
			}
			if len(password) != 4 {
				return fmt.Errorf("invalid 'password' length: %d", len(password))
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			if args.bool("on") {
				return car.EnableValetMode(ctx, args.str("password"))
			} else {
				return car.DisableValetMode(ctx)
			}
		},
	},
	"set_vehicle_name": {
		validate: func(args commandArgs) error {
			if _, err := args.validateString("vehicle_name", true); err != nil {
				return err
			}
			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.SetVehicleName(ctx, args.str("vehicle_name"))
		},
	},
	"speed_limit_activate": {
		validate: func(args commandArgs) error {
			pin, err := args.validateString("pin", true)
			if err != nil {
				return err
			}
			if len(pin) != 4 {
				return fmt.Errorf("invalid 'pin' length: %d", len(pin))
			}

			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ActivateSpeedLimit(ctx, args.str("pin"))
		},
	},
	"speed_limit_clear_pin": {
		validate: func(args commandArgs) error {
			pin, err := args.validateString("pin", true)
			if err != nil {
				return err
			}
			if len(pin) != 4 {
				return fmt.Errorf("invalid 'pin' length: %d", len(pin))
			}

			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.ClearSpeedLimitPIN(ctx, args.str("pin"))
		},
	},
	"speed_limit_clear_pin_admin": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not supported via BLE")
		},
	},
	"speed_limit_deactivate": {
		validate: func(args commandArgs) error {
			pin, err := args.validateString("pin", true)
			if err != nil {
				return err
			}
			if len(pin) != 4 {
				return fmt.Errorf("invalid 'pin' length: %d", len(pin))
			}

			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.DeactivateSpeedLimit(ctx, args.str("pin"))
		},
	},
	"speed_limit_set_limit": {
		validate: func(args commandArgs) error {
			limitMph, err := args.validateFloat("limit_mph", true)
			if err != nil {
				return err
			}
			if limitMph < 50 || limitMph > 90 {
				return fmt.Errorf("invalid 'limit_mph' value: %f", limitMph)
			}

			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.SpeedLimitSetLimitMPH(ctx, args.float("limit_mph"))
		},
	},
	"sun_roof_control": {
		validate: func(args commandArgs) error {
			// https://tesla-api.timdorr.com/vehicle/commands/sunroof#post-api-1-vehicles-id-command-sun_roof_control
			// TODO: implement - car.ChangeSunroofState
			return fmt.Errorf("not implemented")
		},
	},
	"trigger_homelink": {
		validate: func(args commandArgs) error {
			lat, err := args.validateFloat("lat", false)
			if err != nil {
				return err
			}
			if lat < -90 || lat > 90 {
				return fmt.Errorf("invalid 'lat' value: %f", lat)
			}

			lon, err := args.validateFloat("lon", false)
			if err != nil {
				return err
			}
			if lon < -180 || lon > 180 {
				return fmt.Errorf("invalid 'lon' value: %f", lon)
			}

			// Official API requires token, but it's not needed here
			// so we just validate it for completeness and to avoid errors
			if _, ok := args.validateString("token", false); ok != nil {
				return err
			}

			return nil
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			return car.TriggerHomelink(ctx, args.float32("lat"), args.float32("lon"))
		},
	},
	"upcoming_calendar_entries": {
		validate: func(args commandArgs) error {
			return fmt.Errorf("not implemented")
		},
	},
	"window_control": {
		validate: func(args commandArgs) error {
			// https://tesla-api.timdorr.com/vehicle/commands/windows#post-api-1-vehicles-id-command-window_control
			// lat and lon values must be near the current location of the car for
			// close operation to succeed. For vent, the lat and lon values are ignored,
			// and may both be 0 (which has been observed from the app itself).
			state, err := args.validateString("command", true)
			if err != nil {
				return err
			}
			// In actuallity, the lat and lon values are not required for any operation
			// but we still validate them here for completeness and to avoid errors
			if _, err := args.validateFloat("lat", false); err != nil {
				return err
			}
			if _, err := args.validateFloat("lon", false); err != nil {
				return err
			}
			switch state {
			case "vent":
				fallthrough
			case "close":
				return nil
			default:
				return fmt.Errorf("invalid 'command' value: %s", args["command"])
			}
		},
		execute: func(ctx context.Context, car *vehicle.Vehicle, args commandArgs) error {
			switch args.str("command") {
			case "vent":
				return car.VentWindows(ctx)
			case "close":
				return car.CloseWindows(ctx)
			default:
				return fmt.Errorf("invalid 'command' value: %s", args["command"])
			}
		},
	},
}

func ValidateFleetVehicleCommand(command string, body map[string]interface{}) error {
	handler, ok := fleetVehicleCommands[command]
	if !ok {
		return fmt.Errorf("unrecognized vehicle command: %s", command)
	}

	if handler.validate != nil {
		return handler.validate(body)
	}
	return nil
}
