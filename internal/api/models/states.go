package models

// ChargeState contains the current charge states that exist within the vehicle.
type ChargeState struct {
	Timestamp                      int64       `json:"timestamp"`                         //
	ChargingState                  string      `json:"charging_state"`                    //
	ChargeLimitSoc                 int32       `json:"charge_limit_soc"`                  //
	ChargeLimitSocStd              int32       `json:"charge_limit_soc_std"`              //
	ChargeLimitSocMin              int32       `json:"charge_limit_soc_min"`              //
	ChargeLimitSocMax              int32       `json:"charge_limit_soc_max"`              //
	BatteryHeaterOn                bool        `json:"battery_heater_on"`                 //
	NotEnoughPowerToHeat           bool        `json:"not_enough_power_to_heat"`          //
	MaxRangeChargeCounter          int32       `json:"max_range_charge_counter"`          //
	FastChargerPresent             bool        `json:"fast_charger_present"`              //
	FastChargerType                *string     `json:"fast_charger_type,omitempty"`       //
	BatteryRange                   float32     `json:"battery_range"`                     //
	EstBatteryRange                float32     `json:"est_battery_range"`                 //
	IdealBatteryRange              float32     `json:"ideal_battery_range"`               //
	BatteryLevel                   int32       `json:"battery_level"`                     //
	UsableBatteryLevel             int32       `json:"usable_battery_level"`              //
	ChargeEnergyAdded              float32     `json:"charge_energy_added"`               //
	ChargeMilesAddedRated          float32     `json:"charge_miles_added_rated"`          //
	ChargeMilesAddedIdeal          float32     `json:"charge_miles_added_ideal"`          //
	ChargerVoltage                 int32       `json:"charger_voltage"`                   //
	ChargerPilotCurrent            int32       `json:"charger_pilot_current"`             //
	ChargerActualCurrent           int32       `json:"charger_actual_current"`            //
	ChargerPower                   int32       `json:"charger_power"`                     //
	TripCharging                   bool        `json:"trip_charging"`                     //
	ChargeRate                     float32     `json:"charge_rate"`                       //
	ChargePortDoorOpen             bool        `json:"charge_port_door_open"`             //
	ScheduledChargingMode          string      `json:"scheduled_charging_mode"`           //
	ScheduledDepatureTime          int64       `json:"scheduled_departure_time"`          //
	ScheduledDepatureTimeMinutes   uint32      `json:"scheduled_departure_time_minutes"`  //
	SuperchargerSessionTripPlanner bool        `json:"supercharger_session_trip_planner"` //
	ScheduledChargingStartTime     uint64      `json:"scheduled_charging_start_time"`     //
	ScheduledChargingPending       bool        `json:"scheduled_charging_pending"`        //
	UserChargeEnableRequest        interface{} `json:"user_charge_enable_request"`        //
	ChargeEnableRequest            bool        `json:"charge_enable_request"`             //
	ChargerPhases                  int32       `json:"charger_phases"`                    //
	ChargePortLatch                string      `json:"charge_port_latch"`                 //
	ChargeCurrentRequest           int32       `json:"charge_current_request"`            //
	ChargeCurrentRequestMax        int32       `json:"charge_current_request_max"`        //
	ChargeAmps                     int32       `json:"charge_amps"`                       //
	OffPeakChargingEnabled         bool        `json:"off_peak_charging_enabled"`         //
	OffPeakChargingTimes           string      `json:"off_peak_charging_times"`           //
	OffPeakHoursEndTime            uint32      `json:"off_peak_hours_end_time"`           //
	PreconditioningEnabled         bool        `json:"preconditioning_enabled"`           //
	PreconditioningTimes           string      `json:"preconditioning_times"`             //
	ManagedChargingActive          bool        `json:"managed_charging_active"`           //
	ManagedChargingUserCanceled    bool        `json:"managed_charging_user_canceled"`    //
	ManagedChargingStartTime       interface{} `json:"managed_charging_start_time"`       //
	ChargePortcoldWeatherMode      bool        `json:"charge_port_cold_weather_mode"`     //
	ChargePortColor                string      `json:"charge_port_color"`                 //
	ConnChargeCable                *string     `json:"conn_charge_cable,omitempty"`       //
	FastChargerBrand               *string     `json:"fast_charger_brand,omitempty"`      //
	MinutesToFullCharge            int32       `json:"minutes_to_full_charge"`            //
}

// ClimateState contains the current climate states available from the vehicle.
type ClimateState struct {
	Timestamp                              int64       `json:"timestamp"`                                  //
	AllowCabinOverheatProtection           bool        `json:"allow_cabin_overheat_protection"`            //
	AutoSeatClimateLeft                    bool        `json:"auto_seat_climate_left"`                     //
	AutoSeatClimateRight                   bool        `json:"auto_seat_climate_right"`                    //
	AutoSteeringWheelHeat                  bool        `json:"auto_steering_wheel_heat"`                   //
	BioweaponMode                          bool        `json:"bioweapon_mode"`                             //
	CabinOverheatProtection                string      `json:"cabin_overheat_protection"`                  //
	CabinOverheatProtectionActivelyCooling bool        `json:"cabin_overheat_protection_actively_cooling"` //
	CopActivationTemperature               string      `json:"cop_activation_temperature"`                 //
	InsideTemp                             float32     `json:"inside_temp"`                                //
	OutsideTemp                            float32     `json:"outside_temp"`                               //
	DriverTempSetting                      float32     `json:"driver_temp_setting"`                        //
	PassengerTempSetting                   float32     `json:"passenger_temp_setting"`                     //
	LeftTempDirection                      int32       `json:"left_temp_direction"`                        //
	RightTempDirection                     int32       `json:"right_temp_direction"`                       //
	IsAutoConditioningOn                   bool        `json:"is_auto_conditioning_on"`                    //
	IsFrontDefrosterOn                     bool        `json:"is_front_defroster_on"`                      //
	IsRearDefrosterOn                      bool        `json:"is_rear_defroster_on"`                       //
	FanStatus                              int32       `json:"fan_status"`                                 //
	HvacAutoRequest                        string      `json:"hvac_auto_request"`                          //
	IsClimateOn                            bool        `json:"is_climate_on"`                              //
	MinAvailTemp                           float32     `json:"min_avail_temp"`                             //
	MaxAvailTemp                           float32     `json:"max_avail_temp"`                             //
	SeatHeaterLeft                         int32       `json:"seat_heater_left"`                           //
	SeatHeaterRight                        int32       `json:"seat_heater_right"`                          //
	SeatHeaterRearLeft                     int32       `json:"seat_heater_rear_left"`                      //
	SeatHeaterRearRight                    int32       `json:"seat_heater_rear_right"`                     //
	SeatHeaterRearCenter                   int32       `json:"seat_heater_rear_center"`                    //
	SeatHeaterRearRightBack                int32       `json:"seat_heater_rear_right_back"`
	SeatHeaterRearLeftBack                 int32       `json:"seat_heater_rear_left_back"`
	SteeringWheelHeatLevel                 int32       `json:"steering_wheel_heat_level"`                   //
	SteeringWheelHeater                    bool        `json:"steering_wheel_heater"`                       //
	SupportsFanOnlyCabinOverheatProtection bool        `json:"supports_fan_only_cabin_overheat_protection"` //
	BatteryHeater                          bool        `json:"battery_heater"`                              //
	BatteryHeaterNoPower                   interface{} `json:"battery_heater_no_power"`                     //
	ClimateKeeperMode                      string      `json:"climate_keeper_mode"`                         //
	DefrostMode                            string      `json:"defrost_mode"`                                //
	IsPreconditioning                      bool        `json:"is_preconditioning"`                          //
	RemoteHeaterControlEnabled             bool        `json:"remote_heater_control_enabled"`               //
	SideMirrorHeaters                      bool        `json:"side_mirror_heaters"`                         //
	WiperBladeHeater                       bool        `json:"wiper_blade_heater"`                          //
}

// ClosureStatuses contains the current closure states available from the vehicle.
type ClosureStatuses struct {
	FrontDriverDoor    string `json:"front_driver_door"`
	FrontPassengerDoor string `json:"front_passenger_door"`
	RearDriverDoor     string `json:"rear_driver_door"`
	RearPassengerDoor  string `json:"rear_passenger_door"`
	RearTrunk          string `json:"rear_trunk"`
	FrontTrunk         string `json:"front_trunk"`
	ChargePort         string `json:"charge_port"`
	Tonneau            string `json:"tonneau"`
}

// DetailedClosureStatus contains the current detailed closure states available from the vehicle.
type DetailedClosureStatus struct {
	TonneauPercentOpen int32 `json:"tonneau_percent_open"`
}

// VehicleStatus contains the current vehicle status available from the vehicle.
type VehicleStatus struct {
	ClosureStatuses       *ClosureStatuses       `json:"closure_statuses"`
	DetailedClosureStatus *DetailedClosureStatus `json:"detailed_closure_status"`
	UserPresence          string                 `json:"user_presence"`
	VehicleLockState      string                 `json:"vehicle_lock_state"`
	VehicleSleepStatus    string                 `json:"vehicle_sleep_status"`
}
