package commands

import (
	"context"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

func TestParseTemps(t *testing.T) {
	tests := []struct {
		name          string
		body          map[string]interface{}
		wantDriver    float32
		wantPassenger float32
		wantErr       bool
	}{
		{"numbers", map[string]interface{}{"driver_temp": 21.5, "passenger_temp": 20.0}, 21.5, 20, false},
		{"strings", map[string]interface{}{"driver_temp": "22", "passenger_temp": " 19.5 "}, 22, 19.5, false},
		{"passenger defaults to driver", map[string]interface{}{"driver_temp": 21.0}, 21, 21, false},
		{"range bounds inclusive", map[string]interface{}{"driver_temp": 15.0, "passenger_temp": 28.0}, 15, 28, false},
		{"nil body", nil, 0, 0, true},
		{"driver missing", map[string]interface{}{"passenger_temp": 21.0}, 0, 0, true},
		{"driver not a number", map[string]interface{}{"driver_temp": "warm"}, 0, 0, true},
		{"unsupported type", map[string]interface{}{"driver_temp": true}, 0, 0, true},
		{"driver too high", map[string]interface{}{"driver_temp": 200.0}, 0, 0, true},
		{"passenger too low", map[string]interface{}{"driver_temp": 21.0, "passenger_temp": 5.0}, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, p, err := parseTemps(tt.body)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && (d != tt.wantDriver || p != tt.wantPassenger) {
				t.Errorf("got %.1f/%.1f, want %.1f/%.1f", d, p, tt.wantDriver, tt.wantPassenger)
			}
		})
	}
}

func TestParseWhichTrunk(t *testing.T) {
	tests := []struct {
		name    string
		body    map[string]interface{}
		want    string
		wantErr bool
	}{
		{"rear", map[string]interface{}{"which_trunk": "rear"}, "rear", false},
		{"front", map[string]interface{}{"which_trunk": "front"}, "front", false},
		{"case and spaces", map[string]interface{}{"which_trunk": " REAR "}, "rear", false},
		{"nil body", nil, "", true},
		{"missing", map[string]interface{}{}, "", true},
		{"unknown value", map[string]interface{}{"which_trunk": "side"}, "", true},
		{"unsupported type", map[string]interface{}{"which_trunk": 1.0}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWhichTrunk(tt.body)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateBody(t *testing.T) {
	tests := []struct {
		command string
		body    map[string]interface{}
		wantErr bool
	}{
		{"set_temps", map[string]interface{}{"driver_temp": 21.0}, false},
		{"set_temps", map[string]interface{}{"driver_temp": 40.0}, true},
		{"set_temps", nil, true},
		{"actuate_trunk", map[string]interface{}{"which_trunk": "rear"}, false},
		{"actuate_trunk", map[string]interface{}{"which_trunk": "side"}, true},
		{"actuate_trunk", nil, true},
		// commands without a pre-queue check pass through unchanged
		{"door_unlock", nil, false},
		{"set_charging_amps", nil, false},
	}
	for _, tt := range tests {
		err := ValidateBody(tt.command, tt.body)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateBody(%s, %v) err = %v, wantErr %v", tt.command, tt.body, err, tt.wantErr)
		}
	}
}

// Invalid bodies must be rejected before anything is sent to the car, and must not be retried.
func TestInvalidBodyNotRetried(t *testing.T) {
	for _, cmd := range []string{"set_temps", "actuate_trunk"} {
		command := &Command{Command: cmd, Body: map[string]interface{}{}}
		retry, err := command.Send(context.Background(), &vehicle.Vehicle{})
		if err == nil {
			t.Errorf("%s with empty body: expected error", cmd)
		}
		if retry {
			t.Errorf("%s with empty body: shouldRetry = true, want false", cmd)
		}
	}
}
