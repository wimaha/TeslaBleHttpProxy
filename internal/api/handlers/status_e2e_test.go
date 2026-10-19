package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
)

// The dashboard status: reading it never contacts the car, refreshing goes through the queue and
// wakes the car only on request.

type statusVehicle struct {
	Vin         string                     `json:"vin"`
	SleepStatus string                     `json:"sleep_status"`
	LastSuccess *struct{ Command string }  `json:"last_success"`
	Fields      map[string]json.RawMessage `json:"-"`
}

func (p *proxy) status() (map[string]json.RawMessage, []statusVehicle) {
	p.t.Helper()
	code, resp := p.do("GET", "/api/proxy/1/status", "")
	wantStatus(p.t, code, resp, http.StatusOK)
	var raw struct {
		Proxy    map[string]json.RawMessage   `json:"proxy"`
		Vehicles []map[string]json.RawMessage `json:"vehicles"`
	}
	if err := json.Unmarshal(resp.Response, &raw); err != nil {
		p.t.Fatalf("status %s: %v", resp.Response, err)
	}
	vehicles := make([]statusVehicle, len(raw.Vehicles))
	for i, fields := range raw.Vehicles {
		b, _ := json.Marshal(fields)
		if err := json.Unmarshal(b, &vehicles[i]); err != nil {
			p.t.Fatal(err)
		}
		vehicles[i].Fields = fields
	}
	return raw.Proxy, vehicles
}

func (p *proxy) vehicle(vin string) *statusVehicle {
	p.t.Helper()
	_, vehicles := p.status()
	for i := range vehicles {
		if vehicles[i].Vin == vin {
			return &vehicles[i]
		}
	}
	return nil
}

func (p *proxy) refresh(vin string, wakeup bool, headers map[string]string) (int, models.Response) {
	p.t.Helper()
	form := url.Values{"vin": {vin}, "wakeup": {map[bool]string{true: "true", false: "false"}[wakeup]}}
	req, err := http.NewRequest("POST", p.server.URL+"/api/proxy/1/status/refresh", strings.NewReader(form.Encode()))
	if err != nil {
		p.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		p.t.Fatal(err)
	}
	defer res.Body.Close()
	var ret models.Ret
	if res.StatusCode != http.StatusForbidden {
		if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
			p.t.Fatalf("decoding refresh answer: %v", err)
		}
	}
	return res.StatusCode, ret.Response
}

func TestHTTPStatusNeverContactsCar(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)

	proxy, _ := p.status()
	if string(proxy["ble_control_running"]) != "true" {
		t.Errorf("ble_control_running = %s, want true", proxy["ble_control_running"])
	}
	if v := p.vehicle(vin); v != nil {
		t.Errorf("unknown vehicle listed: %+v", v)
	}
	if n := len(p.car.Calls()); n != 0 {
		t.Errorf("car got %v, want no contact", p.car.Methods())
	}
}

func TestHTTPStatusShowsCachedDataWithoutLocation(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)
	p.car.BatteryLevel = 64

	code, resp := p.do("GET", "/api/1/vehicles/"+vin+"/vehicle_data?endpoints=charge_state;climate_state;drive_state", "")
	wantStatus(t, code, resp, http.StatusOK)
	calls := len(p.car.Calls())

	v := p.vehicle(vin)
	if v == nil {
		t.Fatal("vehicle missing from status")
	}
	var charge struct {
		Data      models.ChargeState `json:"data"`
		UpdatedAt string             `json:"updated_at"`
	}
	if err := json.Unmarshal(v.Fields["charge_state"], &charge); err != nil {
		t.Fatalf("charge_state %s: %v", v.Fields["charge_state"], err)
	}
	if charge.Data.BatteryLevel != 64 || charge.UpdatedAt == "" {
		t.Errorf("charge_state = %+v, want battery 64 with updated_at", charge)
	}
	if string(v.Fields["climate_state"]) == "null" {
		t.Error("climate_state missing")
	}
	if _, ok := v.Fields["drive_state"]; ok {
		t.Error("drive_state must not be part of the status")
	}
	if v.LastSuccess == nil || v.LastSuccess.Command != "vehicle_data" {
		t.Errorf("last_success = %+v, want vehicle_data", v.LastSuccess)
	}
	if n := len(p.car.Calls()); n != calls {
		t.Errorf("reading the status contacted the car: %v", p.car.Methods()[calls:])
	}
}

func TestHTTPStatusShowsBodyControllerState(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)
	code, resp := p.do("GET", "/api/1/vehicles/"+vin+"/body_controller_state", "")
	wantStatus(t, code, resp, http.StatusOK)

	v := p.vehicle(vin)
	if v == nil || string(v.Fields["body_controller"]) == "null" {
		t.Fatalf("body_controller missing from status: %+v", v)
	}
	if !strings.Contains(string(v.Fields["body_controller"]), "VEHICLELOCKSTATE_LOCKED") {
		t.Errorf("body_controller = %s", v.Fields["body_controller"])
	}
}

func TestHTTPRefreshDoesNotWakeSleepingCar(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)
	p.car.Asleep = true

	code, resp := p.refresh(vin, false, nil)

	wantStatus(t, code, resp, http.StatusServiceUnavailable)
	if !strings.Contains(resp.Reason, "sleeping") {
		t.Errorf("reason = %q, want sleeping", resp.Reason)
	}
	if n := p.car.Count("Wakeup"); n != 0 {
		t.Errorf("Wakeup count = %d, want 0", n)
	}
	if v := p.vehicle(vin); v == nil || v.SleepStatus != "asleep" {
		t.Errorf("status = %+v, want sleep_status asleep", v)
	}
}

func TestHTTPRefreshWithWakeup(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)
	p.car.Asleep = true

	code, resp := p.refresh(vin, true, nil)

	wantStatus(t, code, resp, http.StatusOK)
	if n := p.car.Count("Wakeup"); n != 1 {
		t.Errorf("Wakeup count = %d, want 1", n)
	}
	v := p.vehicle(vin)
	if v == nil || string(v.Fields["charge_state"]) == "null" || v.SleepStatus != "awake" {
		t.Errorf("status after refresh = %+v", v)
	}
}

// Refresh always asks the car, even when the cache is fresh.
func TestHTTPRefreshBypassesCache(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)
	code, resp := p.do("GET", "/api/1/vehicles/"+vin+"/vehicle_data", "")
	wantStatus(t, code, resp, http.StatusOK)
	before := p.car.Count("GetState")

	code, resp = p.refresh(vin, false, nil)

	wantStatus(t, code, resp, http.StatusOK)
	if p.car.Count("GetState") == before {
		t.Error("refresh was answered from the cache")
	}
}

func TestHTTPRefreshRejects(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)

	code, resp := p.refresh("NOTAVIN", false, nil)
	wantStatus(t, code, resp, http.StatusServiceUnavailable)
	if !strings.Contains(resp.Reason, "17 characters") {
		t.Errorf("reason = %q", resp.Reason)
	}
	if code, _ := p.refresh(vin, true, map[string]string{"Sec-Fetch-Site": "cross-site"}); code != http.StatusForbidden {
		t.Errorf("cross-site refresh: status %d, want 403", code)
	}
	res, err := http.Get(p.server.URL + "/api/proxy/1/status/refresh")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET refresh: status %d, want 405", res.StatusCode)
	}
	if n := len(p.car.Calls()); n != 0 {
		t.Errorf("car got %v, want no contact", p.car.Methods())
	}
}
