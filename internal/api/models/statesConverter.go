package models

import (
	"strings"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

func flatten(s string) string {
	return strings.ReplaceAll(s, ":{}", "")
}

func strPtr(s string) *string { return &s }

func ChargeStateFromBle(VehicleData *carserver.VehicleData) ChargeState {
	return ChargeState{
		Timestamp:                      VehicleData.ChargeState.GetTimestamp().AsTime().Unix(),
		ChargingState:                  flatten(VehicleData.ChargeState.GetChargingState().String()),
		ChargeLimitSoc:                 VehicleData.ChargeState.GetChargeLimitSoc(),
		ChargeLimitSocStd:              VehicleData.ChargeState.GetChargeLimitSocStd(),
		ChargeLimitSocMin:              VehicleData.ChargeState.GetChargeLimitSocMin(),
		ChargeLimitSocMax:              VehicleData.ChargeState.GetChargeLimitSocMax(),
		MaxRangeChargeCounter:          VehicleData.ChargeState.GetMaxRangeChargeCounter(),
		FastChargerPresent:             VehicleData.ChargeState.GetFastChargerPresent(),
		FastChargerType:                strPtr(flatten(VehicleData.ChargeState.GetFastChargerType().String())),
		BatteryRange:                   VehicleData.ChargeState.GetBatteryRange(),
		EstBatteryRange:                VehicleData.ChargeState.GetEstBatteryRange(),
		IdealBatteryRange:              VehicleData.ChargeState.GetIdealBatteryRange(),
		BatteryLevel:                   VehicleData.ChargeState.GetUsableBatteryLevel(), //VehicleData.ChargeState.GetBatteryLevel(),
		UsableBatteryLevel:             VehicleData.ChargeState.GetUsableBatteryLevel(),
		ChargeEnergyAdded:              VehicleData.ChargeState.GetChargeEnergyAdded(),
		ChargeMilesAddedRated:          VehicleData.ChargeState.GetChargeMilesAddedRated(),
		ChargeMilesAddedIdeal:          VehicleData.ChargeState.GetChargeMilesAddedIdeal(),
		ChargerVoltage:                 VehicleData.ChargeState.GetChargerVoltage(),
		ChargerPilotCurrent:            VehicleData.ChargeState.GetChargerPilotCurrent(),
		ChargerActualCurrent:           VehicleData.ChargeState.GetChargerActualCurrent(),
		ChargerPower:                   VehicleData.ChargeState.GetChargerPower(),
		TripCharging:                   VehicleData.ChargeState.GetTripCharging(),
		ChargeRate:                     VehicleData.ChargeState.GetChargeRateMphFloat(),
		ChargePortDoorOpen:             VehicleData.ChargeState.GetChargePortDoorOpen(),
		ScheduledChargingMode:          flatten(VehicleData.ChargeState.GetScheduledChargingMode().String()),
		ScheduledDepatureTime:          VehicleData.ChargeState.GetScheduledDepartureTime().AsTime().Unix(),
		ScheduledDepatureTimeMinutes:   VehicleData.ChargeState.GetScheduledDepartureTimeMinutes(),
		SuperchargerSessionTripPlanner: VehicleData.ChargeState.GetSuperchargerSessionTripPlanner(),
		ScheduledChargingStartTime:     VehicleData.ChargeState.GetScheduledChargingStartTime(),
		ScheduledChargingPending:       VehicleData.ChargeState.GetScheduledChargingPending(),
		UserChargeEnableRequest:        VehicleData.ChargeState.GetUserChargeEnableRequest(),
		ChargeEnableRequest:            VehicleData.ChargeState.GetChargeEnableRequest(),
		ChargerPhases:                  VehicleData.ChargeState.GetChargerPhases(),
		ChargePortLatch:                flatten(VehicleData.ChargeState.GetChargePortLatch().String()),
		ChargeCurrentRequest:           VehicleData.ChargeState.GetChargeCurrentRequest(),
		ChargeCurrentRequestMax:        VehicleData.ChargeState.GetChargeCurrentRequestMax(),
		ChargeAmps:                     VehicleData.ChargeState.GetChargingAmps(),
		OffPeakChargingTimes:           flatten(VehicleData.ChargeState.GetOffPeakChargingTimes().String()),
		OffPeakHoursEndTime:            VehicleData.ChargeState.GetOffPeakHoursEndTime(),
		PreconditioningEnabled:         VehicleData.ChargeState.GetPreconditioningEnabled(),
		PreconditioningTimes:           flatten(VehicleData.ChargeState.GetPreconditioningTimes().String()),
		ManagedChargingActive:          VehicleData.ChargeState.GetManagedChargingActive(),
		ManagedChargingUserCanceled:    VehicleData.ChargeState.GetManagedChargingUserCanceled(),
		ManagedChargingStartTime:       VehicleData.ChargeState.GetManagedChargingStartTime(),
		ChargePortcoldWeatherMode:      VehicleData.ChargeState.GetChargePortColdWeatherMode(),
		ChargePortColor:                flatten(VehicleData.ChargeState.GetChargePortColor().String()),
		ConnChargeCable:                strPtr(flatten(VehicleData.ChargeState.GetConnChargeCable().String())),
		FastChargerBrand:               strPtr(flatten(VehicleData.ChargeState.GetFastChargerBrand().String())),
		MinutesToFullCharge:            VehicleData.ChargeState.GetMinutesToFullCharge(),
	}
}

/*
MISSING
	BatteryHeaterOn             bool        `json:"battery_heater_on"`
	NotEnoughPowerToHeat        bool        `json:"not_enough_power_to_heat"`
	TimeToFullCharge            float64     `json:"time_to_full_charge"`
	OffPeakChargingEnabled      bool        `json:"off_peak_charging_enabled"`
*/

func ClimateStateFromBle(VehicleData *carserver.VehicleData) ClimateState {
	return ClimateState{
		Timestamp:                              VehicleData.ClimateState.GetTimestamp().AsTime().Unix(),
		AllowCabinOverheatProtection:           VehicleData.ClimateState.GetAllowCabinOverheatProtection(),
		AutoSeatClimateLeft:                    VehicleData.ClimateState.GetAutoSeatClimateLeft(),
		AutoSeatClimateRight:                   VehicleData.ClimateState.GetAutoSeatClimateRight(),
		AutoSteeringWheelHeat:                  VehicleData.ClimateState.GetAutoSteeringWheelHeat(),
		BioweaponMode:                          VehicleData.ClimateState.GetBioweaponModeOn(),
		CabinOverheatProtection:                flatten(VehicleData.ClimateState.GetCabinOverheatProtection().String()),
		CabinOverheatProtectionActivelyCooling: VehicleData.ClimateState.GetCabinOverheatProtectionActivelyCooling(),
		CopActivationTemperature:               flatten(VehicleData.ClimateState.GetCopActivationTemperature().String()),
		InsideTemp:                             VehicleData.ClimateState.GetInsideTempCelsius(),
		OutsideTemp:                            VehicleData.ClimateState.GetOutsideTempCelsius(),
		DriverTempSetting:                      VehicleData.ClimateState.GetDriverTempSetting(),
		PassengerTempSetting:                   VehicleData.ClimateState.GetPassengerTempSetting(),
		LeftTempDirection:                      VehicleData.ClimateState.GetLeftTempDirection(),
		RightTempDirection:                     VehicleData.ClimateState.GetRightTempDirection(),
		IsAutoConditioningOn:                   VehicleData.ClimateState.GetIsAutoConditioningOn(),
		IsFrontDefrosterOn:                     VehicleData.ClimateState.GetIsFrontDefrosterOn(),
		IsRearDefrosterOn:                      VehicleData.ClimateState.GetIsRearDefrosterOn(),
		FanStatus:                              VehicleData.ClimateState.GetFanStatus(),
		HvacAutoRequest:                        flatten(VehicleData.ClimateState.GetHvacAutoRequest().String()),
		IsClimateOn:                            VehicleData.ClimateState.GetIsClimateOn(),
		MinAvailTemp:                           VehicleData.ClimateState.GetMinAvailTempCelsius(),
		MaxAvailTemp:                           VehicleData.ClimateState.GetMaxAvailTempCelsius(),
		SeatHeaterLeft:                         VehicleData.ClimateState.GetSeatHeaterLeft(),
		SeatHeaterRight:                        VehicleData.ClimateState.GetSeatHeaterRight(),
		SeatHeaterRearLeft:                     VehicleData.ClimateState.GetSeatHeaterRearLeft(),
		SeatHeaterRearRight:                    VehicleData.ClimateState.GetSeatHeaterRearRight(),
		SeatHeaterRearCenter:                   VehicleData.ClimateState.GetSeatHeaterRearCenter(),
		SeatHeaterRearRightBack:                VehicleData.ClimateState.GetSeatHeaterRearRightBack(),
		SeatHeaterRearLeftBack:                 VehicleData.ClimateState.GetSeatHeaterRearLeftBack(),
		SteeringWheelHeatLevel:                 int32(*VehicleData.ClimateState.GetSteeringWheelHeatLevel().Enum()),
		SteeringWheelHeater:                    VehicleData.ClimateState.GetSteeringWheelHeater(),
		SupportsFanOnlyCabinOverheatProtection: VehicleData.ClimateState.GetSupportsFanOnlyCabinOverheatProtection(),
		BatteryHeater:                          VehicleData.ClimateState.GetBatteryHeater(),
		BatteryHeaterNoPower:                   VehicleData.ClimateState.GetBatteryHeaterNoPower(),
		ClimateKeeperMode:                      flatten(VehicleData.ClimateState.GetClimateKeeperMode().String()),
		DefrostMode:                            flatten(VehicleData.ClimateState.GetDefrostMode().String()),
		IsPreconditioning:                      VehicleData.ClimateState.GetIsPreconditioning(),
		RemoteHeaterControlEnabled:             VehicleData.ClimateState.GetRemoteHeaterControlEnabled(),
		SideMirrorHeaters:                      VehicleData.ClimateState.GetSideMirrorHeaters(),
		WiperBladeHeater:                       VehicleData.ClimateState.GetWiperBladeHeater(),
	}
}

/*
MISSING
	SmartPreconditioning       bool        `json:"smart_preconditioning"`
*/

func ClosureStatusesFromBle(cs *vcsec.ClosureStatuses) *ClosureStatuses {
	return &ClosureStatuses{
		FrontDriverDoor:    flatten(cs.GetFrontDriverDoor().String()),
		FrontPassengerDoor: flatten(cs.GetFrontPassengerDoor().String()),
		RearDriverDoor:     flatten(cs.GetRearDriverDoor().String()),
		RearPassengerDoor:  flatten(cs.GetRearPassengerDoor().String()),
		RearTrunk:          flatten(cs.GetRearTrunk().String()),
		FrontTrunk:         flatten(cs.GetFrontTrunk().String()),
		ChargePort:         flatten(cs.GetChargePort().String()),
		Tonneau:            flatten(cs.GetTonneau().String()),
	}
}

func DetailedClosureStatusFromBle(dcs *vcsec.DetailedClosureStatus) *DetailedClosureStatus {
	return &DetailedClosureStatus{
		TonneauPercentOpen: int32(dcs.GetTonneauPercentOpen()),
	}
}

func VehicleStatusFromBle(vs *vcsec.VehicleStatus) VehicleStatus {
	return VehicleStatus{
		ClosureStatuses:       ClosureStatusesFromBle(vs.GetClosureStatuses()),
		DetailedClosureStatus: DetailedClosureStatusFromBle(vs.GetDetailedClosureStatus()),
		UserPresence:          flatten(vs.GetUserPresence().String()),
		VehicleLockState:      flatten(vs.GetVehicleLockState().String()),
		VehicleSleepStatus:    flatten(vs.GetVehicleSleepStatus().String()),
	}
}
