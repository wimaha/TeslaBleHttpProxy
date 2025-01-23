package models

import (
	"strings"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

func flatten(s string) any {
	if s == "<nil>" {
		return nil
	}
	return strings.ReplaceAll(s, ":{}", "")
}

func BodyControllerStateFromBle(vehicleData *vcsec.VehicleStatus) map[string]interface{} {
	cs := &vehicleData.ClosureStatuses
	dcs := &vehicleData.DetailedClosureStatus
	return map[string]interface{}{
		"closure_statuses": map[string]interface{}{
			"front_driver_door":    flatten((*cs).GetFrontDriverDoor().String()),
			"front_passenger_door": flatten((*cs).GetFrontPassengerDoor().String()),
			"rear_driver_door":     flatten((*cs).GetRearDriverDoor().String()),
			"rear_passenger_door":  flatten((*cs).GetRearPassengerDoor().String()),
			"rear_trunk":           flatten((*cs).GetRearTrunk().String()),
			"front_trunk":          flatten((*cs).GetFrontTrunk().String()),
			"charge_port":          flatten((*cs).GetChargePort().String()),
			"tonneau":              flatten((*cs).GetTonneau().String()),
		},
		"detailed_closure_status": map[string]interface{}{
			"tonneau_percent_open": (*dcs).GetTonneauPercentOpen(),
		},
		"user_presence":        flatten(vehicleData.GetUserPresence().String()),
		"vehicle_lock_state":   flatten(vehicleData.GetVehicleLockState().String()),
		"vehicle_sleep_status": flatten(vehicleData.GetVehicleSleepStatus().String()),
	}
}

func ChargeStateFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {
	cs := vehicleData.ChargeState
	return map[string]interface{}{
		"timestamp":                         (*cs).GetTimestamp().AsTime().Unix(),
		"charging_state":                    flatten((*cs).GetChargingState().String()),
		"charge_limit_soc":                  (*cs).GetChargeLimitSoc(),
		"charge_limit_soc_std":              (*cs).GetChargeLimitSocStd(),
		"charge_limit_soc_min":              (*cs).GetChargeLimitSocMin(),
		"charge_limit_soc_max":              (*cs).GetChargeLimitSocMax(),
		"max_range_charge_counter":          (*cs).GetMaxRangeChargeCounter(),
		"fast_charger_present":              (*cs).GetFastChargerPresent(),
		"fast_charger_type":                 flatten((*cs).GetFastChargerType().String()),
		"battery_range":                     (*cs).GetBatteryRange(),
		"est_battery_range":                 (*cs).GetEstBatteryRange(),
		"ideal_battery_range":               (*cs).GetIdealBatteryRange(),
		"battery_level":                     (*cs).GetBatteryLevel(),
		"usable_battery_level":              (*cs).GetUsableBatteryLevel(),
		"charge_energy_added":               (*cs).GetChargeEnergyAdded(),
		"charge_miles_added_rated":          (*cs).GetChargeMilesAddedRated(),
		"charge_miles_added_ideal":          (*cs).GetChargeMilesAddedIdeal(),
		"charger_voltage":                   (*cs).GetChargerVoltage(),
		"charger_pilot_current":             (*cs).GetChargerPilotCurrent(),
		"charger_actual_current":            (*cs).GetChargerActualCurrent(),
		"charger_power":                     (*cs).GetChargerPower(),
		"trip_charging":                     (*cs).GetTripCharging(),
		"charge_rate":                       (*cs).GetChargeRateMphFloat(),
		"charge_port_door_open":             (*cs).GetChargePortDoorOpen(),
		"scheduled_charging_mode":           flatten((*cs).GetScheduledChargingMode().String()),
		"scheduled_departure_time":          (*cs).GetScheduledDepartureTime().AsTime().Unix(),
		"scheduled_departure_time_minutes":  (*cs).GetScheduledDepartureTimeMinutes(),
		"supercharger_session_trip_planner": (*cs).GetSuperchargerSessionTripPlanner(),
		"scheduled_charging_start_time":     (*cs).GetScheduledChargingStartTime(),
		"scheduled_charging_pending":        (*cs).GetScheduledChargingPending(),
		"user_charge_enable_request":        (*cs).GetUserChargeEnableRequest(),
		"charge_enable_request":             (*cs).GetChargeEnableRequest(),
		"charger_phases":                    (*cs).GetChargerPhases(),
		"charge_port_latch":                 flatten((*cs).GetChargePortLatch().String()),
		"charge_current_request":            (*cs).GetChargeCurrentRequest(),
		"charge_current_request_max":        (*cs).GetChargeCurrentRequestMax(),
		"charge_amps":                       (*cs).GetChargingAmps(),
		"off_peak_charging_times":           flatten((*cs).GetOffPeakChargingTimes().String()),
		"off_peak_hours_end_time":           (*cs).GetOffPeakHoursEndTime(),
		"preconditioning_enabled":           (*cs).GetPreconditioningEnabled(),
		"preconditioning_times":             flatten((*cs).GetPreconditioningTimes().String()),
		"managed_charging_active":           (*cs).GetManagedChargingActive(),
		"managed_charging_user_canceled":    (*cs).GetManagedChargingUserCanceled(),
		"managed_charging_start_time":       (*cs).GetManagedChargingStartTime(),
		"charge_port_cold_weather_mode":     (*cs).GetChargePortColdWeatherMode(),
		"charge_port_color":                 flatten((*cs).GetChargePortColor().String()),
		"conn_charge_cable":                 flatten((*cs).GetConnChargeCable().String()),
		"fast_charger_brand":                flatten((*cs).GetFastChargerBrand().String()),
		"minutes_to_full_charge":            (*cs).GetMinutesToFullCharge(),
		// "battery_heater_on":              (*cs).GetBatteryHeaterOn(),
		// "not_enough_power_to_heat":       (*cs).GetNotEnoughPowerToHeat(),
		// "off_peak_charging_enabled":      (*cs).GetOffPeakChargingEnabled(),
	}
}

/*
MISSING
	BatteryHeaterOn             bool        `json:"battery_heater_on"`
	NotEnoughPowerToHeat        bool        `json:"not_enough_power_to_heat"`
	TimeToFullCharge            float64     `json:"time_to_full_charge"`
	OffPeakChargingEnabled      bool        `json:"off_peak_charging_enabled"`
*/

func ClimateStateFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {
	cs := &vehicleData.ClimateState
	return map[string]interface{}{
		"timestamp":                                   (*cs).GetTimestamp().AsTime().Unix(),
		"allow_cabin_overheat_protection":             (*cs).GetAllowCabinOverheatProtection(),
		"auto_seat_climate_left":                      (*cs).GetAutoSeatClimateLeft(),
		"auto_seat_climate_right":                     (*cs).GetAutoSeatClimateRight(),
		"auto_steering_wheel_heat":                    (*cs).GetAutoSteeringWheelHeat(),
		"bioweapon_mode":                              (*cs).GetBioweaponModeOn(),
		"cabin_overheat_protection":                   flatten((*cs).GetCabinOverheatProtection().String()),
		"cabin_overheat_protection_actively_cooling":  (*cs).GetCabinOverheatProtectionActivelyCooling(),
		"cop_activation_temperature":                  flatten((*cs).GetCopActivationTemperature().String()),
		"inside_temp":                                 (*cs).GetInsideTempCelsius(),
		"outside_temp":                                (*cs).GetOutsideTempCelsius(),
		"driver_temp_setting":                         (*cs).GetDriverTempSetting(),
		"passenger_temp_setting":                      (*cs).GetPassengerTempSetting(),
		"left_temp_direction":                         (*cs).GetLeftTempDirection(),
		"right_temp_direction":                        (*cs).GetRightTempDirection(),
		"is_auto_conditioning_on":                     (*cs).GetIsAutoConditioningOn(),
		"is_front_defroster_on":                       (*cs).GetIsFrontDefrosterOn(),
		"is_rear_defroster_on":                        (*cs).GetIsRearDefrosterOn(),
		"fan_status":                                  (*cs).GetFanStatus(),
		"hvac_auto_request":                           flatten((*cs).GetHvacAutoRequest().String()),
		"is_climate_on":                               (*cs).GetIsClimateOn(),
		"min_avail_temp":                              (*cs).GetMinAvailTempCelsius(),
		"max_avail_temp":                              (*cs).GetMaxAvailTempCelsius(),
		"seat_heater_left":                            (*cs).GetSeatHeaterLeft(),
		"seat_heater_right":                           (*cs).GetSeatHeaterRight(),
		"seat_heater_rear_left":                       (*cs).GetSeatHeaterRearLeft(),
		"seat_heater_rear_right":                      (*cs).GetSeatHeaterRearRight(),
		"seat_heater_rear_center":                     (*cs).GetSeatHeaterRearCenter(),
		"seat_heater_rear_right_back":                 (*cs).GetSeatHeaterRearRightBack(),
		"seat_heater_rear_left_back":                  (*cs).GetSeatHeaterRearLeftBack(),
		"steering_wheel_heat_level":                   int32(*(*cs).GetSteeringWheelHeatLevel().Enum()),
		"steering_wheel_heater":                       (*cs).GetSteeringWheelHeater(),
		"supports_fan_only_cabin_overheat_protection": (*cs).GetSupportsFanOnlyCabinOverheatProtection(),
		"battery_heater":                              (*cs).GetBatteryHeater(),
		"battery_heater_no_power":                     (*cs).GetBatteryHeaterNoPower(),
		"climate_keeper_mode":                         flatten((*cs).GetClimateKeeperMode().String()),
		"defrost_mode":                                flatten((*cs).GetDefrostMode().String()),
		"is_preconditioning":                          (*cs).GetIsPreconditioning(),
		"remote_heater_control_enabled":               (*cs).GetRemoteHeaterControlEnabled(),
		"side_mirror_heaters":                         (*cs).GetSideMirrorHeaters(),
		"wiper_blade_heater":                          (*cs).GetWiperBladeHeater(),
	}
}

/*
MISSING
	SmartPreconditioning       bool        `json:"smart_preconditioning"`
*/

// func ChargeStateFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {}
// func ClimateStateFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {)
// func DriveStateFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {
// func LocationDataFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {)
// func ClosuresStateFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {)
// func ChargeScheduleDataFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {)
// func PreconditioningScheduleDataFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {)
// func TirePressureFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {)
// func MediaFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {
// func MediaDetailFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {
// func SoftwareUpdateFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {)
// func ParentalControlsFromBle(vehicleData *carserver.VehicleData) map[string]interface{} {)
