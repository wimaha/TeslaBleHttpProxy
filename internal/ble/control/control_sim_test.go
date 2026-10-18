package control

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/fakecar"
)

// These tests run the real command loop against simulated cars instead of BLE.

const (
	vin1 = "5YJ3E1EA1JF000001"
	vin2 = "5YJ3E1EA1JF000002"
)

var _ Car = (*fakecar.Car)(nil)

var (
	vcsecOnly        = []universal.Domain{protocol.DomainVCSEC}
	withInfotainment = []universal.Domain{protocol.DomainVCSEC, protocol.DomainInfotainment}
)

// startControl runs a command loop that reaches the given cars by VIN.
func startControl(t *testing.T, connectionTimeout time.Duration, cars ...*fakecar.Car) *BleControl {
	t.Helper()
	byVin := map[string]*fakecar.Car{}
	for _, c := range cars {
		byVin[c.Vin] = c
	}
	bc := New(fakecar.PrivateKey(), func(ctx context.Context, vin string) (Car, bool, error) {
		c, ok := byVin[vin]
		if !ok {
			return nil, true, errors.New("vehicle is not in range")
		}
		car, retry, err := c.Dial(ctx, vin)
		if err != nil {
			return nil, retry, err
		}
		return car, false, nil
	})
	bc.Timing = Timing{LoopDelay: time.Millisecond, RetryDelay: time.Millisecond, ConnectionTimeout: connectionTimeout}
	go bc.Loop()
	return bc
}

// send pushes a command like the HTTP handler with wait=true and returns the response.
func send(t *testing.T, ctx context.Context, bc *BleControl, vin, command string, body map[string]interface{}, autoWakeup bool) *models.ApiResponse {
	t.Helper()
	wg := &sync.WaitGroup{}
	resp := &models.ApiResponse{Wait: wg, Ctx: ctx}
	wg.Add(1)
	bc.PushCommand(command, vin, body, resp, autoWakeup)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s: no response within 5s", command)
	}
	return resp
}

func mustSucceed(t *testing.T, resp *models.ApiResponse) {
	t.Helper()
	if !resp.Result {
		t.Fatalf("result = false, error %q", resp.Error)
	}
}

func mustFail(t *testing.T, resp *models.ApiResponse, contains string) {
	t.Helper()
	if resp.Result || !strings.Contains(resp.Error, contains) {
		t.Fatalf("response = (result %v, error %q), want failure containing %q", resp.Result, resp.Error, contains)
	}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func sessions(c *fakecar.Car) [][]universal.Domain {
	var out [][]universal.Domain
	for _, call := range c.Calls() {
		if call.Method == "StartSession" {
			out = append(out, call.Args[0].([]universal.Domain))
		}
	}
	return out
}

func TestCommandWakesSleepingCar(t *testing.T) {
	car := fakecar.New(vin1)
	car.Asleep = true
	bc := startControl(t, time.Second, car)

	mustSucceed(t, send(t, context.Background(), bc, vin1, "flash_lights", nil, true))

	wantMethods := []string{"Dial", "StartSession", "Wakeup", "StartSession", "FlashLights"}
	if got := car.Methods(); !reflect.DeepEqual(got, wantMethods) {
		t.Errorf("calls = %v, want %v", got, wantMethods)
	}
	if got := sessions(car); !reflect.DeepEqual(got, [][]universal.Domain{vcsecOnly, withInfotainment}) {
		t.Errorf("sessions = %v, want VCSEC, then VCSEC+Infotainment", got)
	}
}

// wake_up needs only VCSEC; the session is upgraded afterwards for the commands that follow.
func TestWakeUpStartsWithVCSECOnly(t *testing.T) {
	car := fakecar.New(vin1)
	car.Asleep = true
	bc := startControl(t, time.Second, car)

	mustSucceed(t, send(t, context.Background(), bc, vin1, "wake_up", nil, false))

	wantMethods := []string{"Dial", "StartSession", "Wakeup", "StartSession"}
	if got := car.Methods(); !reflect.DeepEqual(got, wantMethods) {
		t.Errorf("calls = %v, want %v", got, wantMethods)
	}
	mustSucceed(t, send(t, context.Background(), bc, vin1, "charge_start", nil, true))
	if n := car.Count("Dial"); n != 1 {
		t.Errorf("Dial count = %d, want 1 (charge_start should reuse the upgraded session)", n)
	}
}

func TestVehicleDataDoesNotWakeSleepingCar(t *testing.T) {
	car := fakecar.New(vin1)
	car.Asleep = true
	bc := startControl(t, time.Second, car)

	resp := send(t, context.Background(), bc, vin1, "vehicle_data", map[string]interface{}{"endpoints": []string{"charge_state"}}, false)

	mustFail(t, resp, "vehicle is sleeping")
	if n := car.Count("Wakeup"); n != 0 {
		t.Errorf("Wakeup count = %d, want 0", n)
	}
	if n := car.Count("Dial"); n != 1 {
		t.Errorf("Dial count = %d, want 1: a sleeping car is not a reason to retry", n)
	}
	eventually(t, "disconnect", func() bool { return car.Count("Disconnect") == 1 })
}

func TestVehicleDataWakesWhenRequested(t *testing.T) {
	car := fakecar.New(vin1)
	car.Asleep = true
	car.BatteryLevel = 72
	bc := startControl(t, time.Second, car)

	resp := send(t, context.Background(), bc, vin1, "vehicle_data", map[string]interface{}{"endpoints": []string{"charge_state"}}, true)

	mustSucceed(t, resp)
	if n := car.Count("Wakeup"); n != 1 {
		t.Errorf("Wakeup count = %d, want 1", n)
	}
	if !strings.Contains(string(resp.Response), `"battery_level":72`) {
		t.Errorf("response = %s, want battery_level 72", resp.Response)
	}
}

// After the car was seen awake, vehicle_data skips the sleep check for 9 minutes.
func TestVehicleDataRemembersAwakeCar(t *testing.T) {
	car := fakecar.New(vin1)
	bc := startControl(t, 20*time.Millisecond, car)
	body := map[string]interface{}{"endpoints": []string{"charge_state"}}

	mustSucceed(t, send(t, context.Background(), bc, vin1, "vehicle_data", body, false))
	eventually(t, "first disconnect", func() bool { return car.Count("Disconnect") == 1 })
	mustSucceed(t, send(t, context.Background(), bc, vin1, "vehicle_data", body, false))

	if n := car.Count("Dial"); n != 2 {
		t.Fatalf("Dial count = %d, want 2 separate connections", n)
	}
	if n := car.Count("BodyControllerState"); n != 1 {
		t.Errorf("BodyControllerState count = %d, want 1 (second request should use the awake cache)", n)
	}
}

func TestConnectRetriesThenSucceeds(t *testing.T) {
	car := fakecar.New(vin1)
	car.FailNext("Dial", errors.New("scan timeout"), errors.New("scan timeout"))
	bc := startControl(t, time.Second, car)

	mustSucceed(t, send(t, context.Background(), bc, vin1, "flash_lights", nil, true))

	if n := car.Count("Dial"); n != 3 {
		t.Errorf("Dial count = %d, want 3", n)
	}
}

func TestConnectGivesUpAfterThreeAttempts(t *testing.T) {
	car := fakecar.New(vin1)
	car.FailNext("Dial", errors.New("scan timeout"), errors.New("scan timeout"), errors.New("scan timeout"))
	bc := startControl(t, time.Second, car)

	mustFail(t, send(t, context.Background(), bc, vin1, "flash_lights", nil, true), "scan timeout")

	if n := car.Count("Dial"); n != 3 {
		t.Errorf("Dial count = %d, want 3", n)
	}
	if n := car.Count("FlashLights"); n != 0 {
		t.Errorf("FlashLights count = %d, want 0", n)
	}
}

func TestConnectDoesNotRetryPermanentError(t *testing.T) {
	car := fakecar.New(vin1)
	car.FailNext("Dial", fakecar.ErrNotPermitted)
	bc := startControl(t, time.Second, car)

	mustFail(t, send(t, context.Background(), bc, vin1, "flash_lights", nil, true), "not permitted")

	if n := car.Count("Dial"); n != 1 {
		t.Errorf("Dial count = %d, want 1", n)
	}
}

// A failed handshake closes the half-open connection before the next attempt.
func TestFailedHandshakeDisconnects(t *testing.T) {
	car := fakecar.New(vin1)
	car.FailNext("StartSession", errors.New("handshake failed"))
	bc := startControl(t, time.Second, car)

	mustSucceed(t, send(t, context.Background(), bc, vin1, "flash_lights", nil, true))

	wantStart := []string{"Dial", "StartSession", "Disconnect", "Dial"}
	if got := car.Methods()[:4]; !reflect.DeepEqual(got, wantStart) {
		t.Errorf("calls start with %v, want %v", got, wantStart)
	}
}

func TestCommandRetriesThenSucceeds(t *testing.T) {
	car := fakecar.New(vin1)
	car.FailNext("FlashLights", errors.New("busy"), errors.New("busy"))
	bc := startControl(t, time.Second, car)

	mustSucceed(t, send(t, context.Background(), bc, vin1, "flash_lights", nil, true))

	if n := car.Count("FlashLights"); n != 3 {
		t.Errorf("FlashLights count = %d, want 3", n)
	}
}

func TestCommandFailsAfterThreeAttempts(t *testing.T) {
	car := fakecar.New(vin1)
	car.FailNext("SetChargingAmps", errors.New("busy"), errors.New("busy"), errors.New("busy"))
	bc := startControl(t, time.Second, car)

	resp := send(t, context.Background(), bc, vin1, "set_charging_amps", map[string]interface{}{"charging_amps": 10.0}, true)

	mustFail(t, resp, "busy")
	if n := car.Count("SetChargingAmps"); n != 3 {
		t.Errorf("SetChargingAmps count = %d, want 3", n)
	}
}

// When the HTTP client gives up, the command is not repeated.
func TestCanceledRequestIsNotRetried(t *testing.T) {
	car := fakecar.New(vin1)
	car.HangNext("FlashLights")
	bc := startControl(t, time.Second, car)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	mustFail(t, send(t, ctx, bc, vin1, "flash_lights", nil, true), "deadline exceeded")

	if n := car.Count("FlashLights"); n != 1 {
		t.Errorf("FlashLights count = %d, want 1", n)
	}
}

func TestRearTrunkFailureIsNotRetried(t *testing.T) {
	car := fakecar.New(vin1)
	car.FailNext("ActuateTrunk", errors.New("reply lost"))
	bc := startControl(t, time.Second, car)

	resp := send(t, context.Background(), bc, vin1, "actuate_trunk", map[string]interface{}{"which_trunk": "rear"}, true)

	mustFail(t, resp, "not retried")
	if n := car.Count("ActuateTrunk"); n != 1 {
		t.Errorf("ActuateTrunk count = %d, want 1", n)
	}
}

func TestCommandsShareOneConnection(t *testing.T) {
	car := fakecar.New(vin1)
	bc := startControl(t, time.Second, car)

	mustSucceed(t, send(t, context.Background(), bc, vin1, "set_charging_amps", map[string]interface{}{"charging_amps": 12.0}, true))
	mustSucceed(t, send(t, context.Background(), bc, vin1, "set_charge_limit", map[string]interface{}{"percent": 85.0}, true))
	mustSucceed(t, send(t, context.Background(), bc, vin1, "charge_start", nil, true))

	if n := car.Count("Dial"); n != 1 {
		t.Errorf("Dial count = %d, want 1", n)
	}
	car.Do(func(c *fakecar.Car) {
		if c.ChargingAmps != 12 || c.ChargeLimit != 85 || !c.Charging {
			t.Errorf("car state amps %d, limit %d, charging %v", c.ChargingAmps, c.ChargeLimit, c.Charging)
		}
	})
}

// A bad body on an open connection is reported, and the connection stays usable.
func TestInvalidInputKeepsConnection(t *testing.T) {
	car := fakecar.New(vin1)
	bc := startControl(t, time.Second, car)

	mustSucceed(t, send(t, context.Background(), bc, vin1, "flash_lights", nil, true))
	mustFail(t, send(t, context.Background(), bc, vin1, "set_temps", map[string]interface{}{"driver_temp": 40.0}, true), "out of range")
	mustSucceed(t, send(t, context.Background(), bc, vin1, "honk_horn", nil, true))

	if n := car.Count("Dial"); n != 1 {
		t.Errorf("Dial count = %d, want 1", n)
	}
}

func TestOtherVinOpensNewConnection(t *testing.T) {
	car1, car2 := fakecar.New(vin1), fakecar.New(vin2)
	bc := startControl(t, time.Second, car1, car2)

	mustSucceed(t, send(t, context.Background(), bc, vin1, "flash_lights", nil, true))
	mustSucceed(t, send(t, context.Background(), bc, vin2, "flash_lights", nil, true))

	if car1.Count("Disconnect") != 1 {
		t.Errorf("car1 calls = %v, want a disconnect before car2 is used", car1.Methods())
	}
	if car2.Count("FlashLights") != 1 {
		t.Errorf("car2 calls = %v, want FlashLights", car2.Methods())
	}
}

func TestConnectionClosesAfterTimeout(t *testing.T) {
	car := fakecar.New(vin1)
	bc := startControl(t, 20*time.Millisecond, car)

	mustSucceed(t, send(t, context.Background(), bc, vin1, "flash_lights", nil, true))

	eventually(t, "disconnect", func() bool { return car.Count("Disconnect") == 1 })
	if s := car.Session(); s != nil {
		t.Errorf("session after disconnect = %v, want none", s)
	}
}
