package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/control"
	"github.com/wimaha/TeslaBleHttpProxy/internal/logging"
)

// The dashboard status is assembled from what the proxy already knows: the command loop's view of
// each connection and the cached responses. Reading it never contacts the vehicle; only the
// refresh endpoint does, through the command queue.

var startTime = time.Now()

// statusEndpoints are the cached vehicle_data endpoints the dashboard shows. Location and drive
// data are left out on purpose, even when cached.
var statusEndpoints = []string{"charge_state", "climate_state"}

type proxyStatus struct {
	Version                string    `json:"version"`
	StartedAt              time.Time `json:"started_at"`
	UptimeSeconds          int64     `json:"uptime_seconds"`
	ActiveKeyRole          string    `json:"active_key_role"`
	BleControlRunning      bool      `json:"ble_control_running"`
	QueueLength            int       `json:"queue_length"`
	ConnectionTimeout      int       `json:"connection_timeout"`
	ReleaseAdapterWhenIdle bool      `json:"release_adapter_when_idle"`
	VehicleDataCacheTime   int       `json:"vehicle_data_cache_time"`
}

type vehicleStatus struct {
	control.VehicleConnection
	ChargeState    *cachedEntry `json:"charge_state"`
	ClimateState   *cachedEntry `json:"climate_state"`
	BodyController *cachedEntry `json:"body_controller"`
}

type statusResponse struct {
	Proxy    proxyStatus     `json:"proxy"`
	Vehicles []vehicleStatus `json:"vehicles"`
	Now      time.Time       `json:"now"`
}

func buildStatus() statusResponse {
	bc := control.Instance()
	proxy := proxyStatus{
		Version:           config.Version,
		StartedAt:         startTime,
		UptimeSeconds:     int64(time.Since(startTime).Seconds()),
		ActiveKeyRole:     control.GetActiveKeyRole(),
		BleControlRunning: bc != nil,
	}
	if config.AppConfig != nil {
		proxy.ConnectionTimeout = config.AppConfig.ConnectionTimeout
		proxy.ReleaseAdapterWhenIdle = config.AppConfig.ReleaseAdapterWhenIdle
		proxy.VehicleDataCacheTime = config.AppConfig.VehicleDataCacheTime
	}

	byVin := map[string]*vehicleStatus{}
	vehicle := func(vin string) *vehicleStatus {
		if v, ok := byVin[vin]; ok {
			return v
		}
		v := &vehicleStatus{VehicleConnection: control.VehicleConnection{Vin: vin}}
		byVin[vin] = v
		return v
	}
	if bc != nil {
		proxy.QueueLength = bc.QueueLength()
		for _, c := range bc.Vehicles() {
			vehicle(c.Vin).VehicleConnection = c
		}
	}
	for _, vin := range cachedVins() {
		v := vehicle(vin)
		v.ChargeState = latestVehicleData(vin, statusEndpoints[0])
		v.ClimateState = latestVehicleData(vin, statusEndpoints[1])
		v.BodyController = latestBodyControllerState(vin)
	}

	vehicles := make([]vehicleStatus, 0, len(byVin))
	for _, v := range byVin {
		vehicles = append(vehicles, *v)
	}
	slices.SortFunc(vehicles, func(a, b vehicleStatus) int { return strings.Compare(a.Vin, b.Vin) })
	return statusResponse{Proxy: proxy, Vehicles: vehicles, Now: time.Now()}
}

// Status answers GET /api/proxy/1/status for the dashboard. It never contacts the vehicle.
func Status(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal(buildStatus())
	response := models.Ret{Response: models.Response{Result: err == nil, Reason: "The request was successfully processed.", Response: data}}
	if err != nil {
		logging.Error("Failed to build status", "error", err)
		response.Response.Reason = err.Error()
		response.Response.Response = nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logging.Error("failed to send response", "error", err)
	}
}

var vinPattern = regexp.MustCompile(`^[A-HJ-NPR-Z0-9]{17}$`)

// RefreshStatus answers POST /api/proxy/1/status/refresh (form fields vin, wakeup). It fetches
// the dashboard's vehicle data over BLE through the command queue, bypassing the cache, and
// wakes the vehicle only if wakeup=true. The result is cached like any vehicle_data response.
func RefreshStatus(w http.ResponseWriter, r *http.Request) {
	vin := strings.ToUpper(strings.TrimSpace(r.FormValue("vin")))
	wakeup := r.FormValue("wakeup") == "true"

	var response models.Response
	response.Vin = vin
	response.Command = "vehicle_data"
	defer commonDefer(w, &response)
	logging.Info("Dashboard refresh", "VIN", vin, "Wakeup", wakeup)

	if !vinPattern.MatchString(vin) {
		response.Reason = "A VIN has 17 characters (letters and digits, no I, O or Q)."
		return
	}
	bc := bleControl(&response)
	if bc == nil {
		return
	}
	fetchVehicleData(r.Context(), bc, vin, statusEndpoints, 0, wakeup, &response)
}
