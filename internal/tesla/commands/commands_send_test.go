package commands

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/fakecar"
)

const testVin = "5YJ3E1EA1JF000001"

// Each command must reach the right car method with the parsed arguments and change the
// simulated state accordingly.
func TestSendCallsCar(t *testing.T) {
	tests := []struct {
		command  string
		body     map[string]interface{}
		prepare  func(c *fakecar.Car)
		wantCall fakecar.Call
		check    func(t *testing.T, c *fakecar.Car)
	}{
		{command: "auto_conditioning_start", wantCall: fakecar.Call{Method: "ClimateOn"},
			check: func(t *testing.T, c *fakecar.Car) { want(t, c.ClimateRunning, true) }},
		{command: "auto_conditioning_stop", prepare: func(c *fakecar.Car) { c.ClimateRunning = true },
			wantCall: fakecar.Call{Method: "ClimateOff"},
			check:    func(t *testing.T, c *fakecar.Car) { want(t, c.ClimateRunning, false) }},
		{command: "charge_port_door_open", wantCall: fakecar.Call{Method: "ChargePortOpen"}},
		{command: "charge_port_door_close", wantCall: fakecar.Call{Method: "ChargePortClose"}},
		{command: "flash_lights", wantCall: fakecar.Call{Method: "FlashLights"}},
		{command: "honk_horn", wantCall: fakecar.Call{Method: "HonkHorn"}},
		{command: "wake_up", prepare: func(c *fakecar.Car) { c.Asleep = true },
			wantCall: fakecar.Call{Method: "Wakeup"},
			check:    func(t *testing.T, c *fakecar.Car) { want(t, c.Asleep, false) }},
		{command: "door_unlock", wantCall: fakecar.Call{Method: "Unlock"},
			check: func(t *testing.T, c *fakecar.Car) { want(t, c.Locked, false) }},
		{command: "door_lock", prepare: func(c *fakecar.Car) { c.Locked = false },
			wantCall: fakecar.Call{Method: "Lock"},
			check:    func(t *testing.T, c *fakecar.Car) { want(t, c.Locked, true) }},
		{command: "set_charging_amps", body: map[string]interface{}{"charging_amps": 10.0},
			wantCall: fakecar.Call{Method: "SetChargingAmps", Args: []any{int32(10)}},
			check:    func(t *testing.T, c *fakecar.Car) { want(t, c.ChargingAmps, int32(10)) }},
		{command: "set_charging_amps", body: map[string]interface{}{"charging_amps": "6"},
			wantCall: fakecar.Call{Method: "SetChargingAmps", Args: []any{int32(6)}}},
		{command: "set_charge_limit", body: map[string]interface{}{"percent": 90.0},
			wantCall: fakecar.Call{Method: "ChangeChargeLimit", Args: []any{int32(90)}},
			check:    func(t *testing.T, c *fakecar.Car) { want(t, c.ChargeLimit, int32(90)) }},
		{command: "set_charge_limit", body: map[string]interface{}{"percent": "70"},
			wantCall: fakecar.Call{Method: "ChangeChargeLimit", Args: []any{int32(70)}}},
		{command: "charge_start", wantCall: fakecar.Call{Method: "ChargeStart"},
			check: func(t *testing.T, c *fakecar.Car) { want(t, c.Charging, true) }},
		{command: "charge_stop", prepare: func(c *fakecar.Car) { c.Charging = true },
			wantCall: fakecar.Call{Method: "ChargeStop"},
			check:    func(t *testing.T, c *fakecar.Car) { want(t, c.Charging, false) }},
		{command: "set_sentry_mode", body: map[string]interface{}{"on": true},
			wantCall: fakecar.Call{Method: "SetSentryMode", Args: []any{true}},
			check:    func(t *testing.T, c *fakecar.Car) { want(t, c.SentryMode, true) }},
		{command: "set_sentry_mode", body: map[string]interface{}{"on": "false"},
			prepare:  func(c *fakecar.Car) { c.SentryMode = true },
			wantCall: fakecar.Call{Method: "SetSentryMode", Args: []any{false}}},
		{command: "set_temps", body: map[string]interface{}{"driver_temp": 22.5, "passenger_temp": 20.0},
			wantCall: fakecar.Call{Method: "ChangeClimateTemp", Args: []any{float32(22.5), float32(20)}}},
		{command: "set_temps", body: map[string]interface{}{"driver_temp": "23"},
			wantCall: fakecar.Call{Method: "ChangeClimateTemp", Args: []any{float32(23), float32(23)}}},
		{command: "actuate_trunk", body: map[string]interface{}{"which_trunk": "front"},
			wantCall: fakecar.Call{Method: "OpenFrunk"},
			check:    func(t *testing.T, c *fakecar.Car) { want(t, c.FrunkOpen, true) }},
		{command: "actuate_trunk", body: map[string]interface{}{"which_trunk": "rear"},
			wantCall: fakecar.Call{Method: "ActuateTrunk"},
			check:    func(t *testing.T, c *fakecar.Car) { want(t, c.TrunkOpen, true) }},
		{command: "remove_charge_schedule", body: map[string]interface{}{"id": 7.0},
			wantCall: fakecar.Call{Method: "RemoveChargeSchedule", Args: []any{uint64(7)}}},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			car := fakecar.New(testVin)
			if tt.prepare != nil {
				tt.prepare(car)
			}
			cmd := &Command{Command: tt.command, Vin: testVin, Body: tt.body}

			retry, err := cmd.Send(context.Background(), car)

			if err != nil || retry {
				t.Fatalf("Send = (retry %v, %v), want success", retry, err)
			}
			calls := car.Calls()
			if len(calls) != 1 {
				t.Fatalf("calls = %v, want exactly %s", car.Methods(), tt.wantCall.Method)
			}
			if !reflect.DeepEqual(calls[0], tt.wantCall) {
				t.Errorf("call = %+v, want %+v", calls[0], tt.wantCall)
			}
			if tt.check != nil {
				tt.check(t, car)
			}
		})
	}
}

func TestAddChargeScheduleReachesCar(t *testing.T) {
	car := fakecar.New(testVin)
	cmd := &Command{Command: "add_charge_schedule", Body: map[string]interface{}{
		"days_of_week": "Monday,Friday", "lat": 52.5, "lon": 13.4, "enabled": true, "start_enabled": true, "start_time": 120.0,
	}}
	if retry, err := cmd.Send(context.Background(), car); err != nil || retry {
		t.Fatalf("Send = (retry %v, %v), want success", retry, err)
	}
	if len(car.Schedules) != 1 {
		t.Fatalf("schedules = %v, want one", car.Schedules)
	}
	for _, s := range car.Schedules {
		if !s.Enabled || !s.StartEnabled || s.StartTime != 120 {
			t.Errorf("schedule = %+v", s)
		}
	}
}

// A failing car call is retried, except where repeating it is unsafe.
func TestSendRetryDecision(t *testing.T) {
	errCar := errors.New("car did not answer")
	tests := []struct {
		name      string
		command   string
		body      map[string]interface{}
		method    string
		wantRetry bool
		wantErr   bool
	}{
		{"plain command", "flash_lights", nil, "FlashLights", true, true},
		{"charging amps", "set_charging_amps", map[string]interface{}{"charging_amps": 8.0}, "SetChargingAmps", true, true},
		{"frunk", "actuate_trunk", map[string]interface{}{"which_trunk": "front"}, "OpenFrunk", true, true},
		// the rear trunk toggles: a retry after a lost reply would close it again
		{"rear trunk", "actuate_trunk", map[string]interface{}{"which_trunk": "rear"}, "ActuateTrunk", false, true},
		// a new schedule without id could be stored twice
		{"new schedule", "add_charge_schedule", map[string]interface{}{"days_of_week": "Monday", "lat": 52.5, "lon": 13.4}, "AddChargeSchedule", false, true},
		{"schedule update", "add_charge_schedule", map[string]interface{}{"id": 3.0, "days_of_week": "Monday", "lat": 52.5, "lon": 13.4}, "AddChargeSchedule", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			car := fakecar.New(testVin)
			car.FailNext(tt.method, errCar)
			cmd := &Command{Command: tt.command, Body: tt.body}

			retry, err := cmd.Send(context.Background(), car)

			if car.Count(tt.method) != 1 {
				t.Fatalf("calls = %v, want %s once", car.Methods(), tt.method)
			}
			if (err != nil) != tt.wantErr || retry != tt.wantRetry {
				t.Errorf("Send = (retry %v, %v), want retry %v, error %v", retry, err, tt.wantRetry, tt.wantErr)
			}
		})
	}
}

func TestRearTrunkFailureIsReported(t *testing.T) {
	car := fakecar.New(testVin)
	car.FailNext("ActuateTrunk", errors.New("timeout"))
	_, err := (&Command{Command: "actuate_trunk", Body: map[string]interface{}{"which_trunk": "rear"}}).Send(context.Background(), car)
	var nr *NotRetriedError
	if !errors.As(err, &nr) {
		t.Errorf("err = %v, want NotRetriedError", err)
	}
}

// The car answers is_charging / not_charging when there is nothing to do: that is success.
func TestChargeStartStopAlreadyDone(t *testing.T) {
	car := fakecar.New(testVin)
	car.Charging = true
	if retry, err := (&Command{Command: "charge_start"}).Send(context.Background(), car); err != nil || retry {
		t.Errorf("charge_start while charging = (retry %v, %v), want success", retry, err)
	}
	car.Charging = false
	if retry, err := (&Command{Command: "charge_stop"}).Send(context.Background(), car); err != nil || retry {
		t.Errorf("charge_stop while stopped = (retry %v, %v), want success", retry, err)
	}
}

// Bad bodies are rejected before the car is touched.
func TestSendBadBodyDoesNotTouchCar(t *testing.T) {
	tests := []struct {
		command string
		body    map[string]interface{}
	}{
		{"set_charging_amps", nil},
		{"set_charging_amps", map[string]interface{}{"charging_amps": "lots"}},
		{"set_charge_limit", map[string]interface{}{}},
		{"set_sentry_mode", map[string]interface{}{"on": "maybe"}},
		{"set_temps", map[string]interface{}{"driver_temp": 40.0}},
		{"actuate_trunk", map[string]interface{}{"which_trunk": "side"}},
		{"add_charge_schedule", map[string]interface{}{"days_of_week": "Mondey"}},
		{"remove_charge_schedule", map[string]interface{}{}},
		{"vehicle_data", nil},
		{"vehicle_data", map[string]interface{}{"endpoints": []string{"nonsense"}}},
		{"add-key-request", map[string]interface{}{"role": "../owner"}},
		{"no_such_command", nil},
	}
	for _, tt := range tests {
		car := fakecar.New(testVin)
		cmd := &Command{Command: tt.command, Body: tt.body, Response: &models.ApiResponse{}}
		retry, err := cmd.Send(context.Background(), car)
		if err == nil || retry {
			t.Errorf("%s %v: Send = (retry %v, %v), want error without retry", tt.command, tt.body, retry, err)
		}
		if n := len(car.Calls()); n != 0 {
			t.Errorf("%s %v: car got %v, want no call", tt.command, tt.body, car.Methods())
		}
	}
}

// vehicle_data turns the car's protobuf state into the Fleet API JSON.
func TestVehicleDataJSON(t *testing.T) {
	car := fakecar.New(testVin)
	car.BatteryLevel = 63
	car.ChargingAmps = 11
	car.Charging = true
	car.InsideTemp = 18.5
	cmd := &Command{
		Command:  "vehicle_data",
		Body:     map[string]interface{}{"endpoints": []string{"charge_state", "climate_state"}},
		Response: &models.ApiResponse{},
	}

	if retry, err := cmd.Send(context.Background(), car); err != nil || retry {
		t.Fatalf("Send = (retry %v, %v), want success", retry, err)
	}

	var got struct {
		ChargeState  models.ChargeState  `json:"charge_state"`
		ClimateState models.ClimateState `json:"climate_state"`
	}
	if err := json.Unmarshal(cmd.Response.Response, &got); err != nil {
		t.Fatalf("response %s: %v", cmd.Response.Response, err)
	}
	want(t, got.ChargeState.BatteryLevel, int32(63))
	want(t, got.ChargeState.ChargeAmps, int32(11))
	want(t, got.ChargeState.ChargingState, "Charging")
	want(t, got.ClimateState.InsideTemp, float32(18.5))
	want(t, car.Count("GetState"), 2)
}

func TestVehicleDataReadErrorIsRetried(t *testing.T) {
	car := fakecar.New(testVin)
	car.FailNext("GetState", errors.New("timeout"))
	cmd := &Command{
		Command:  "vehicle_data",
		Body:     map[string]interface{}{"endpoints": []string{"charge_state"}},
		Response: &models.ApiResponse{},
	}
	if retry, err := cmd.Send(context.Background(), car); err == nil || !retry {
		t.Errorf("Send = (retry %v, %v), want retryable error", retry, err)
	}
}

func want[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
