package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/control"
	"github.com/wimaha/TeslaBleHttpProxy/internal/logging"
	"github.com/wimaha/TeslaBleHttpProxy/internal/tesla/commands"
)

// vehicleDataCacheEntry stores a cached endpoint data with timestamp
type vehicleDataCacheEntry struct {
	data      json.RawMessage // The JSON data for this specific endpoint
	timestamp time.Time
}

// vehicleDataCache is a thread-safe cache for VehicleData endpoints
// Key format: "VIN:endpoint" (e.g., "5YJ3E1EA1JF123456:charge_state")
var (
	vehicleDataCache    = make(map[string]*vehicleDataCacheEntry)
	vehicleDataCacheMux sync.RWMutex
)

// Limits of the opt-in options. They are variables only so that tests can shorten them.
var (
	// staleMaxAge is the oldest cached value that serveStaleWhenAsleep still serves.
	staleMaxAge = 24 * time.Hour
	// wakeRetryInterval is the minimum time between two wake-ups for the same never-cached endpoint.
	wakeRetryInterval = time.Hour
)

// wakeAttempted remembers when wakeWhenNotCached last woke the car for an endpoint (key
// "VIN:endpoint"), whether or not that filled the cache. An endpoint is woken for at most once per
// wakeRetryInterval, so one that never gets cached (the car keeps failing) cannot wake the car on
// every poll. It is kept in memory only, so it is empty again after a restart.
var (
	wakeAttempted    = make(map[string]time.Time)
	wakeAttemptedMux sync.Mutex
)

// shouldWakeForUncached reports whether the car should be woken because of endpoints that were
// never cached, and records the attempt so that each endpoint is woken for at most once per
// wakeRetryInterval.
func shouldWakeForUncached(vin string, endpoints []string) bool {
	wakeAttemptedMux.Lock()
	defer wakeAttemptedMux.Unlock()
	now := time.Now()
	wake := false
	for _, endpoint := range endpoints {
		key := generateVehicleDataCacheKey(vin, endpoint)
		if last, ok := wakeAttempted[key]; !ok || now.Sub(last) >= wakeRetryInterval {
			wakeAttempted[key] = now
			wake = true
		}
	}
	return wake
}

// bodyControllerCache keeps the last successful body_controller_state per VIN for the dashboard.
// It is never used to answer body_controller_state itself.
var (
	bodyControllerCache    = make(map[string]*vehicleDataCacheEntry)
	bodyControllerCacheMux sync.RWMutex
)

// cachedEntry is a cached response and when it was fetched.
type cachedEntry struct {
	Data      json.RawMessage `json:"data"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// latestVehicleData returns the last cached data of an endpoint, however old, or nil.
func latestVehicleData(vin, endpoint string) *cachedEntry {
	vehicleDataCacheMux.RLock()
	defer vehicleDataCacheMux.RUnlock()
	if e, ok := vehicleDataCache[generateVehicleDataCacheKey(vin, endpoint)]; ok {
		return &cachedEntry{Data: e.data, UpdatedAt: e.timestamp}
	}
	return nil
}

// latestBodyControllerState returns the last successful body_controller_state, or nil.
func latestBodyControllerState(vin string) *cachedEntry {
	bodyControllerCacheMux.RLock()
	defer bodyControllerCacheMux.RUnlock()
	if e, ok := bodyControllerCache[vin]; ok {
		return &cachedEntry{Data: e.data, UpdatedAt: e.timestamp}
	}
	return nil
}

// cachedVins returns every VIN with cached vehicle data or body controller state.
func cachedVins() []string {
	seen := map[string]bool{}
	vehicleDataCacheMux.RLock()
	for key := range vehicleDataCache {
		vin, _, _ := strings.Cut(key, ":")
		seen[vin] = true
	}
	vehicleDataCacheMux.RUnlock()
	bodyControllerCacheMux.RLock()
	for vin := range bodyControllerCache {
		seen[vin] = true
	}
	bodyControllerCacheMux.RUnlock()
	vins := make([]string, 0, len(seen))
	for vin := range seen {
		vins = append(vins, vin)
	}
	return vins
}

func commonDefer(w http.ResponseWriter, response *models.Response) {
	var ret models.Ret
	ret.Response = *response

	w.Header().Set("Content-Type", "application/json")
	status := http.StatusOK
	if !response.Result {
		status = http.StatusServiceUnavailable
	}
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(ret); err != nil {
		logging.Error("failed to send response", "error", err)
	}
	logging.Debug("Response", "Command", response.Command, "Status", status, "Result", response.Result, "Reason", response.Reason)
}

// bleControl returns the active BleControl, read once so a key change during the request
// cannot swap it out halfway. If there is none, it sets the response reason and returns nil.
func bleControl(response *models.Response) *control.BleControl {
	bc := control.Instance()
	if bc == nil {
		response.Reason = "BleControl is not initialized. Maybe private.pem is missing."
		response.Result = false
	}
	return bc
}

func Command(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	vin := params["vin"]
	command := params["command"]

	wait := r.URL.Query().Get("wait") == "true"
	// Commands always wake up the car automatically (except wake_up itself)
	// The wakeup parameter is ignored for commands, only used for vehicle_data
	autoWakeup := command != "wake_up"

	var response models.Response
	response.Vin = vin
	response.Command = command

	defer commonDefer(w, &response)

	bc := bleControl(&response)
	if bc == nil {
		return
	}

	//Body
	var body map[string]interface{} = nil
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" && !strings.Contains(err.Error(), "cannot unmarshal bool") {
		logging.Error("Decoding body", "Error", err)
	}

	logRequestWithBody(r, "Command", body)

	if !slices.Contains(commands.ExceptedCommands, command) {
		logging.Error("Command not supported", "Command", command)
		response.Reason = fmt.Sprintf("The command \"%s\" is not supported.", command)
		response.Result = false
		return
	}

	// Reject a bad body before queuing, so it never connects to or wakes the car.
	if err := commands.ValidateBody(command, body); err != nil {
		logging.Error("Invalid body", "Command", command, "Error", err)
		response.Reason = fmt.Sprintf("Invalid body for \"%s\": %s", command, err)
		response.Result = false
		return
	}

	if wait {
		var apiResponse models.ApiResponse
		wg := sync.WaitGroup{}
		apiResponse.Wait = &wg
		apiResponse.Ctx = r.Context()

		wg.Add(1)
		bc.PushCommand(command, vin, body, &apiResponse, autoWakeup)

		wg.Wait()

		if apiResponse.Result {
			response.Result = true
			response.Reason = "The command was successfully processed."
			response.Response = apiResponse.Response
		} else {
			response.Result = false
			response.Reason = apiResponse.Error
		}
		return
	}

	// Nobody waits for the outcome, but a command that was not even queued must not look accepted.
	if err := bc.PushCommand(command, vin, body, nil, autoWakeup); err != nil {
		response.Result = false
		response.Reason = err.Error()
		return
	}
	response.Result = true
	response.Reason = "The command was successfully received and will be processed shortly."
}

// generateVehicleDataCacheKey creates a unique cache key for a specific VIN and endpoint
func generateVehicleDataCacheKey(vin string, endpoint string) string {
	return vin + ":" + endpoint
}

func VehicleData(w http.ResponseWriter, r *http.Request) {
	logRequest(r, "VehicleData")
	params := mux.Vars(r)
	vin := params["vin"]
	command := "vehicle_data"

	var endpoints []string
	endpointsString := r.URL.Query().Get("endpoints")
	if endpointsString != "" {
		endpoints = strings.Split(endpointsString, ";")
	} else {
		endpoints = []string{"charge_state", "climate_state"} //'charge_state', 'climate_state', 'closures_state', 'drive_state', 'gui_settings', 'location_data', 'charge_schedule_data', 'preconditioning_schedule_data', 'vehicle_config', 'vehicle_state', 'vehicle_data_combo'
	}

	var response models.Response
	response.Vin = vin
	response.Command = command

	for _, endpoint := range endpoints {
		if !slices.Contains(commands.ExceptedEndpoints, endpoint) {
			logging.Error("Endpoint not supported", "Endpoint", endpoint)
			response.Reason = fmt.Sprintf("The endpoint \"%s\" is not supported.", endpoint)
			response.Result = false
			commonDefer(w, &response)
			return
		}
	}

	defer commonDefer(w, &response)

	bc := bleControl(&response)
	if bc == nil {
		return
	}

	cacheTime := time.Duration(config.AppConfig.VehicleDataCacheTime) * time.Second
	fetchVehicleData(r.Context(), bc, vin, endpoints, cacheTime, r.URL.Query().Get("wakeup") == "true", &response)
}

// isSleepError reports whether a failed read means the car cannot be read without waking it:
// it is asleep, or its sleep status could not be determined (for example while it is waking up
// or the Bluetooth connection is unstable) and the request did not ask to wake it.
func isSleepError(message string) bool {
	return strings.Contains(message, "vehicle is sleeping") ||
		strings.Contains(message, "vehicle sleep status unknown and wakeup not requested")
}

// fetchVehicleData fills response with the endpoints: from the cache where an entry is younger
// than cacheTime, the rest over BLE, which is then cached. If the BLE request fails, it falls
// back to what is cached. wakeup wakes the car if it is asleep.
func fetchVehicleData(ctx context.Context, bc *control.BleControl, vin string, endpoints []string, cacheTime time.Duration, wakeup bool, response *models.Response) {
	// Check cache for each endpoint
	vehicleDataCacheMux.RLock()
	cachedData := make(map[string]json.RawMessage) // endpoint -> cached data
	missingEndpoints := []string{}
	neverCached := []string{} // endpoints without any cache entry, not even an expired one

	for _, endpoint := range endpoints {
		cacheKey := generateVehicleDataCacheKey(vin, endpoint)
		cachedEntry, exists := vehicleDataCache[cacheKey]
		if exists {
			age := time.Since(cachedEntry.timestamp)
			if age < cacheTime {
				// Cache hit for this endpoint
				cachedData[endpoint] = cachedEntry.data
				logging.Debug("VehicleData endpoint cache hit", "VIN", vin, "Endpoint", endpoint, "Age", age)
			} else {
				// Cache expired for this endpoint
				logging.Debug("VehicleData endpoint cache expired", "VIN", vin, "Endpoint", endpoint, "Age", age)
				missingEndpoints = append(missingEndpoints, endpoint)
			}
		} else {
			// Cache miss for this endpoint
			logging.Debug("VehicleData endpoint cache miss", "VIN", vin, "Endpoint", endpoint)
			neverCached = append(neverCached, endpoint)
			missingEndpoints = append(missingEndpoints, endpoint)
		}
	}
	vehicleDataCacheMux.RUnlock()

	// If all endpoints are cached, construct response from cache
	if len(missingEndpoints) == 0 {
		logging.Debug("VehicleData fully served from cache", "VIN", vin)
		// Build response from cached endpoints
		combinedResponse := make(map[string]json.RawMessage)
		for _, endpoint := range endpoints {
			combinedResponse[endpoint] = cachedData[endpoint]
		}
		responseJson, err := json.Marshal(combinedResponse)
		if err != nil {
			response.Result = false
			response.Reason = fmt.Sprintf("Failed to marshal cached response: %s", err)
			return
		}
		response.Result = true
		response.Reason = "The request was successfully processed."
		response.Response = responseJson
		return
	}

	// Some endpoints missing/expired - fetch from BLE
	// Opt-in (wakeWhenNotCached): an endpoint that was never cached has nothing to serve while the
	// car sleeps, so wake it to fill the cache, at most once per hour and endpoint. Without the option
	// the car is only woken when the client asks with wakeup=true.
	if len(neverCached) > 0 && !wakeup && config.AppConfig.WakeWhenNotCached && shouldWakeForUncached(vin, neverCached) {
		logging.Debug("VehicleData endpoint never cached, waking the vehicle once to fill the cache", "VIN", vin, "Endpoints", neverCached)
		wakeup = true
	}
	var apiResponse models.ApiResponse
	wg := sync.WaitGroup{}
	apiResponse.Wait = &wg
	apiResponse.Ctx = ctx

	wg.Add(1)
	bc.PushCommand("vehicle_data", vin, map[string]interface{}{"endpoints": endpoints}, &apiResponse, wakeup)

	wg.Wait()

	if apiResponse.Result {
		// Parse the BLE response to extract individual endpoint data
		var fetchedData map[string]json.RawMessage
		if err := json.Unmarshal(apiResponse.Response, &fetchedData); err != nil {
			response.Result = false
			response.Reason = fmt.Sprintf("Failed to unmarshal BLE response: %s", err)
			return
		}

		// Store each endpoint separately in cache and merge with cached data
		vehicleDataCacheMux.Lock()
		combinedResponse := make(map[string]json.RawMessage)

		// Add cached endpoints that are still valid
		for endpoint, data := range cachedData {
			combinedResponse[endpoint] = data
		}

		// Add freshly fetched endpoints and cache them
		for endpoint, data := range fetchedData {
			combinedResponse[endpoint] = data
			cacheKey := generateVehicleDataCacheKey(vin, endpoint)
			vehicleDataCache[cacheKey] = &vehicleDataCacheEntry{
				data:      data,
				timestamp: time.Now(),
			}
			logging.Debug("VehicleData endpoint cached", "VIN", vin, "Endpoint", endpoint)
		}
		vehicleDataCacheMux.Unlock()

		// Build final response combining cached and fresh data
		responseJson, err := json.Marshal(combinedResponse)
		if err != nil {
			response.Result = false
			response.Reason = fmt.Sprintf("Failed to marshal combined response: %s", err)
			return
		}

		response.Result = true
		response.Reason = "The request was successfully processed."
		response.Response = responseJson
	} else {
		// Opt-in (serveStaleWhenAsleep): the car is asleep, or its sleep status is unknown, and the
		// request does not wake it. Answer from the last cached values (at most 24 hours old) and mark
		// the answer as stale. This never wakes the car. Without the option the client gets the 503
		// ("vehicle is sleeping" or "sleep status unknown") as before.
		if config.AppConfig.ServeStaleWhenAsleep && isSleepError(apiResponse.Error) {
			vehicleDataCacheMux.RLock()
			var oldest time.Duration
			complete := true
			combinedResponse := make(map[string]json.RawMessage)
			for _, endpoint := range endpoints {
				entry, ok := vehicleDataCache[generateVehicleDataCacheKey(vin, endpoint)]
				if !ok {
					complete = false
					break
				}
				combinedResponse[endpoint] = entry.data
				if age := time.Since(entry.timestamp); age > oldest {
					oldest = age
				}
			}
			vehicleDataCacheMux.RUnlock()

			if complete && oldest > staleMaxAge {
				logging.Debug("Cached data is too old to serve while the vehicle sleeps", "VIN", vin, "Age", oldest, "Max", staleMaxAge)
				complete = false
			}

			if complete {
				if responseJson, err := json.Marshal(combinedResponse); err == nil {
					ageSeconds := int(oldest.Seconds())
					logging.Debug("Vehicle is sleeping, serving stale data from cache", "VIN", vin, "AgeSeconds", ageSeconds)
					response.Result = true
					response.Reason = fmt.Sprintf("The vehicle is sleeping. Served from cache without waking it; the data is up to %ds old.", ageSeconds)
					response.Response = responseJson
					response.Stale = true
					response.CacheAgeSeconds = &ageSeconds
					return
				}
			}
		}

		// BLE fetch failed - try to serve from cache if available
		if len(cachedData) > 0 {
			logging.Debug("BLE fetch failed, serving partial data from cache", "VIN", vin, "CachedEndpoints", len(cachedData))
			combinedResponse := make(map[string]json.RawMessage)
			for endpoint, data := range cachedData {
				combinedResponse[endpoint] = data
			}
			responseJson, err := json.Marshal(combinedResponse)
			if err != nil {
				response.Result = false
				response.Reason = apiResponse.Error
				return
			}
			response.Result = true
			response.Reason = "The request was partially processed from cache. Some data may be stale."
			response.Response = responseJson
		} else {
			response.Result = false
			response.Reason = apiResponse.Error
		}
	}
}

func BodyControllerState(w http.ResponseWriter, r *http.Request) {
	logRequest(r, "BodyControllerState")
	params := mux.Vars(r)
	vin := params["vin"]

	var response models.Response
	response.Vin = vin
	response.Command = "body-controller-state"

	defer commonDefer(w, &response)

	bc := bleControl(&response)
	if bc == nil {
		return
	}

	// Through the queue like every other request, so it never opens a second BLE connection. If
	// a connection to the car is already open, it is answered on it right away. It only needs
	// VCSEC, so it does not wake the car.
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	var apiResponse models.ApiResponse
	wg := sync.WaitGroup{}
	apiResponse.Wait = &wg
	apiResponse.Ctx = ctx
	wg.Add(1)
	bc.Push(commands.Command{
		Command:  "body-controller-state",
		Domain:   commands.Domain.VCSEC,
		Vin:      vin,
		Response: &apiResponse,
	})

	// The loop answers once it reaches the command; do not wait past the timeout for that.
	answered := make(chan struct{})
	go func() {
		wg.Wait()
		close(answered)
	}()
	select {
	case <-answered:
	case <-ctx.Done():
		response.Result = false
		response.Reason = fmt.Sprintf("failed to get body controller state: %s", ctx.Err())
		return
	}

	if apiResponse.Result {
		bodyControllerCacheMux.Lock()
		bodyControllerCache[vin] = &vehicleDataCacheEntry{data: apiResponse.Response, timestamp: time.Now()}
		bodyControllerCacheMux.Unlock()
		SetCacheControl(w, config.AppConfig.CacheMaxAge)
		response.Result = true
		response.Reason = "The request was successfully processed."
		response.Response = apiResponse.Response
	} else {
		response.Result = false
		response.Reason = apiResponse.Error
	}
}

func logRequest(r *http.Request, handler string) {
	logging.Debug("Received HTTP request", "Handler", handler, "Method", r.Method, "Endpoint", r.URL, "Client", r.RemoteAddr)
}

func logRequestWithBody(r *http.Request, handler string, body map[string]interface{}) {
	logging.Debug("Received HTTP request", "Handler", handler, "Method", r.Method, "Endpoint", r.URL, "Client", r.RemoteAddr, "Body", body)
}

func SetCacheControl(w http.ResponseWriter, maxAge int) {
	if maxAge > 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, must-revalidate", maxAge))
	} else {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	}
}
