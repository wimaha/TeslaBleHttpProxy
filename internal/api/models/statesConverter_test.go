package models

import (
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

func TestDriveStateFromBleOdometer(t *testing.T) {
	vehicleData := &carserver.VehicleData{
		DriveState: &carserver.DriveState{
			OptionalOdometerInHundredthsOfAMile: &carserver.DriveState_OdometerInHundredthsOfAMile{
				OdometerInHundredthsOfAMile: 1234567,
			},
			OptionalPower: &carserver.DriveState_Power{
				Power: 42,
			},
			OptionalSpeedFloat: &carserver.DriveState_SpeedFloat{
				SpeedFloat: 12.5,
			},
			ShiftState: &carserver.ShiftState{
				Type: &carserver.ShiftState_D{D: &carserver.Void{}},
			},
		},
	}

	got := DriveStateFromBle(vehicleData)

	if got.Odometer != 12345.67 {
		t.Errorf("Odometer = %v, want %v", got.Odometer, 12345.67)
	}
	if got.Power != 42 {
		t.Errorf("Power = %v, want %v", got.Power, 42)
	}
	if got.Speed != 12.5 {
		t.Errorf("Speed = %v, want %v", got.Speed, 12.5)
	}
	if got.ShiftState != "D" {
		t.Errorf("ShiftState = %q, want %q", got.ShiftState, "D")
	}
}

func TestShiftStateFromBle(t *testing.T) {
	tests := []struct {
		name  string
		input *carserver.ShiftState
		want  string
	}{
		{"nil", nil, ""},
		{"unset", &carserver.ShiftState{}, ""},
		{"invalid", &carserver.ShiftState{Type: &carserver.ShiftState_Invalid{Invalid: &carserver.Void{}}}, ""},
		{"park", &carserver.ShiftState{Type: &carserver.ShiftState_P{P: &carserver.Void{}}}, "P"},
		{"reverse", &carserver.ShiftState{Type: &carserver.ShiftState_R{R: &carserver.Void{}}}, "R"},
		{"neutral", &carserver.ShiftState{Type: &carserver.ShiftState_N{N: &carserver.Void{}}}, "N"},
		{"drive", &carserver.ShiftState{Type: &carserver.ShiftState_D{D: &carserver.Void{}}}, "D"},
		{"sna", &carserver.ShiftState{Type: &carserver.ShiftState_SNA{SNA: &carserver.Void{}}}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shiftStateFromBle(tt.input); got != tt.want {
				t.Errorf("shiftStateFromBle(%s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestDriveStateFromBleSpeedFallback(t *testing.T) {
	// When speed_float is not present, the conversion must fall back to the older
	// integer speed field.
	vehicleData := &carserver.VehicleData{
		DriveState: &carserver.DriveState{
			OptionalSpeed: &carserver.DriveState_Speed{
				Speed: 37,
			},
		},
	}

	got := DriveStateFromBle(vehicleData)

	if got.Speed != 37 {
		t.Errorf("Speed = %v, want %v", got.Speed, 37)
	}
}

func TestDriveStateFromBleMissingOptionalOdometer(t *testing.T) {
	// When the optional odometer is not present, the conversion must default to 0
	// instead of panicking or returning an unexpected value.
	vehicleData := &carserver.VehicleData{
		DriveState: &carserver.DriveState{},
	}

	got := DriveStateFromBle(vehicleData)

	if got.Odometer != 0 {
		t.Errorf("Odometer = %v, want %v", got.Odometer, 0)
	}
	if got.ShiftState != "" {
		t.Errorf("ShiftState = %q, want empty string", got.ShiftState)
	}
}
