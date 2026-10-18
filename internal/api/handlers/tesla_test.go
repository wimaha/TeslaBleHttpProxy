package handlers_test

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/routes"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/control"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/fakecar"
)

// End-to-end: HTTP request -> router -> handler -> command queue -> simulated car -> HTTP answer.
// The handlers use the global control.BleControlInstance, so these tests do not run in parallel.
// Each test uses a fresh VIN because the vehicle_data cache is global as well.

var vinCounter atomic.Int32

func newVin() string { return fmt.Sprintf("5YJ3E1EA1JF%06d", vinCounter.Add(1)) }

type proxy struct {
	t      *testing.T
	car    *fakecar.Car
	server *httptest.Server
}

func startProxy(t *testing.T, vin string) *proxy {
	t.Helper()
	config.AppConfig = &config.Config{VehicleDataCacheTime: 60}
	car := fakecar.New(vin)
	bc := control.New(fakecar.PrivateKey(), func(ctx context.Context, vin string) (control.Car, bool, error) {
		c, retry, err := car.Dial(ctx, vin)
		if err != nil {
			return nil, retry, err
		}
		return c, false, nil
	})
	bc.Timing = control.Timing{LoopDelay: time.Millisecond, RetryDelay: time.Millisecond, ConnectionTimeout: 200 * time.Millisecond}
	go bc.Loop()

	previous := control.BleControlInstance
	control.BleControlInstance = bc
	server := httptest.NewServer(routes.SetupRoutes(embed.FS{}, embed.FS{}))
	t.Cleanup(func() {
		server.Close()
		control.BleControlInstance = previous
	})
	return &proxy{t: t, car: car, server: server}
}

func (p *proxy) do(method, path, body string) (int, models.Response) {
	p.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, p.server.URL+path, reader)
	if err != nil {
		p.t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		p.t.Fatal(err)
	}
	defer res.Body.Close()
	var ret models.Ret
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		p.t.Fatalf("%s %s: decoding answer: %v", method, path, err)
	}
	return res.StatusCode, ret.Response
}

func wantStatus(t *testing.T, code int, resp models.Response, want int) {
	t.Helper()
	if code != want {
		t.Fatalf("status = %d, want %d (reason %q)", code, want, resp.Reason)
	}
}

func TestHTTPCommandReachesCar(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)

	code, resp := p.do("POST", "/api/1/vehicles/"+vin+"/command/set_charging_amps?wait=true", `{"charging_amps": 9}`)

	wantStatus(t, code, resp, http.StatusOK)
	if !resp.Result || resp.Vin != vin || resp.Command != "set_charging_amps" {
		t.Errorf("response = %+v", resp)
	}
	p.car.Do(func(c *fakecar.Car) {
		if c.ChargingAmps != 9 {
			t.Errorf("car charging amps = %d, want 9", c.ChargingAmps)
		}
	})
}

// Without wait=true the proxy answers at once and sends the command in the background.
func TestHTTPCommandWithoutWait(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)

	code, resp := p.do("POST", "/api/1/vehicles/"+vin+"/command/flash_lights", "")

	wantStatus(t, code, resp, http.StatusOK)
	if !strings.Contains(resp.Reason, "will be processed") {
		t.Errorf("reason = %q", resp.Reason)
	}
	deadline := time.Now().Add(3 * time.Second)
	for p.car.Count("FlashLights") == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("car never flashed; calls %v", p.car.Methods())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHTTPRejectsWithoutContactingCar(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)

	for _, tc := range []struct{ path, body, reason string }{
		{"/command/self_destruct?wait=true", "", "not supported"},
		{"/command/set_temps?wait=true", `{"driver_temp": 40}`, "out of range"},
		{"/command/actuate_trunk?wait=true", `{"which_trunk": "side"}`, "which_trunk"},
	} {
		code, resp := p.do("POST", "/api/1/vehicles/"+vin+tc.path, tc.body)
		wantStatus(t, code, resp, http.StatusServiceUnavailable)
		if resp.Result || !strings.Contains(resp.Reason, tc.reason) {
			t.Errorf("%s: response = %+v, want reason containing %q", tc.path, resp, tc.reason)
		}
	}
	code, resp := p.do("GET", "/api/1/vehicles/"+vin+"/vehicle_data?endpoints=gui_settings", "")
	wantStatus(t, code, resp, http.StatusServiceUnavailable)

	if n := len(p.car.Calls()); n != 0 {
		t.Errorf("car got %v, want no contact", p.car.Methods())
	}
}

func TestHTTPCarErrorIsReported(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)
	p.car.FailNext("HonkHorn", errBusy, errBusy, errBusy)

	code, resp := p.do("POST", "/api/1/vehicles/"+vin+"/command/honk_horn?wait=true", "")

	wantStatus(t, code, resp, http.StatusServiceUnavailable)
	if resp.Result || !strings.Contains(resp.Reason, "busy") {
		t.Errorf("response = %+v, want the car's error", resp)
	}
}

func TestHTTPVehicleDataIsCached(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)
	p.car.BatteryLevel = 81
	path := "/api/1/vehicles/" + vin + "/vehicle_data?endpoints=charge_state"

	code, first := p.do("GET", path, "")
	wantStatus(t, code, first, http.StatusOK)
	code, second := p.do("GET", path, "")
	wantStatus(t, code, second, http.StatusOK)

	var data struct {
		ChargeState models.ChargeState `json:"charge_state"`
	}
	if err := json.Unmarshal(second.Response, &data); err != nil {
		t.Fatalf("response %s: %v", second.Response, err)
	}
	if data.ChargeState.BatteryLevel != 81 {
		t.Errorf("battery_level = %d, want 81", data.ChargeState.BatteryLevel)
	}
	if n := p.car.Count("GetState"); n != 1 {
		t.Errorf("GetState count = %d, want 1 (second request from cache)", n)
	}
}

// If the car cannot be reached, cached endpoints are still served.
func TestHTTPVehicleDataFallsBackToCache(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)
	base := "/api/1/vehicles/" + vin + "/vehicle_data?endpoints="

	code, resp := p.do("GET", base+"charge_state", "")
	wantStatus(t, code, resp, http.StatusOK)
	p.car.Do(func(c *fakecar.Car) { c.Asleep = true }) // and wakeup is not requested

	code, resp = p.do("GET", base+"charge_state;climate_state", "")

	wantStatus(t, code, resp, http.StatusOK)
	if !strings.Contains(resp.Reason, "partially") {
		t.Errorf("reason = %q, want partial answer", resp.Reason)
	}
	if !strings.Contains(string(resp.Response), "charge_state") || strings.Contains(string(resp.Response), "climate_state") {
		t.Errorf("response = %s, want only the cached charge_state", resp.Response)
	}
}

func TestHTTPVehicleDataSleepingCar(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)
	p.car.Asleep = true

	code, resp := p.do("GET", "/api/1/vehicles/"+vin+"/vehicle_data", "")

	wantStatus(t, code, resp, http.StatusServiceUnavailable)
	if !strings.Contains(resp.Reason, "sleeping") {
		t.Errorf("reason = %q", resp.Reason)
	}
	if n := p.car.Count("Wakeup"); n != 0 {
		t.Errorf("Wakeup count = %d, want 0", n)
	}
}

func TestHTTPBodyControllerState(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)
	p.car.Asleep = true

	code, resp := p.do("GET", "/api/1/vehicles/"+vin+"/body_controller_state", "")

	wantStatus(t, code, resp, http.StatusOK)
	var vs models.VehicleStatus
	if err := json.Unmarshal(resp.Response, &vs); err != nil {
		t.Fatalf("response %s: %v", resp.Response, err)
	}
	if vs.VehicleSleepStatus != "VEHICLE_SLEEP_STATUS_ASLEEP" || vs.VehicleLockState != "VEHICLELOCKSTATE_LOCKED" {
		t.Errorf("status = %+v", vs)
	}
	if n := p.car.Count("Wakeup"); n != 0 {
		t.Errorf("Wakeup count = %d, want 0: reading the body controller must not wake the car", n)
	}
	if n := p.car.Count("Disconnect"); n != 1 {
		t.Errorf("Disconnect count = %d, want 1", n)
	}
}

var errBusy = errors.New("busy")
