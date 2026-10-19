package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Expired cache entries are not used to answer vehicle_data, but the dashboard still shows them
// with their age instead of fetching them again.
func TestStatusShowsExpiredCacheEntries(t *testing.T) {
	vin := "5YJ3E1EA1JF999999"
	old := time.Now().Add(-2 * time.Hour)
	vehicleDataCacheMux.Lock()
	vehicleDataCache[generateVehicleDataCacheKey(vin, "charge_state")] = &vehicleDataCacheEntry{data: json.RawMessage(`{"battery_level":42}`), timestamp: old}
	vehicleDataCache[generateVehicleDataCacheKey(vin, "location_data")] = &vehicleDataCacheEntry{data: json.RawMessage(`{"latitude":1}`), timestamp: old}
	vehicleDataCacheMux.Unlock()
	t.Cleanup(func() {
		vehicleDataCacheMux.Lock()
		delete(vehicleDataCache, generateVehicleDataCacheKey(vin, "charge_state"))
		delete(vehicleDataCache, generateVehicleDataCacheKey(vin, "location_data"))
		vehicleDataCacheMux.Unlock()
	})

	var found *vehicleStatus
	st := buildStatus()
	for i := range st.Vehicles {
		if st.Vehicles[i].Vin == vin {
			found = &st.Vehicles[i]
		}
	}
	if found == nil || found.ChargeState == nil {
		t.Fatalf("expired charge_state not in status: %+v", found)
	}
	if !found.ChargeState.UpdatedAt.Equal(old) {
		t.Errorf("updated_at = %s, want %s", found.ChargeState.UpdatedAt, old)
	}
	b, _ := json.Marshal(found)
	if strings.Contains(string(b), "latitude") {
		t.Errorf("location data leaked into the status: %s", b)
	}
}

// The status section renders data only through status.js, which must not insert data as HTML.
func TestDashboardStatusSection(t *testing.T) {
	rec := httptest.NewRecorder()
	ShowDashboard(os.DirFS("../../.."))(rec, httptest.NewRequest("GET", "/dashboard", nil))
	body := rec.Body.String()
	for _, want := range []string{`id="vehicle-status"`, `src="static/js/status.js"`} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard is missing %s", want)
		}
	}

	script, err := os.ReadFile("../../../static/js/status.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(string(script), forbidden) {
			t.Errorf("status.js uses %s; set values with textContent instead", forbidden)
		}
	}
}
