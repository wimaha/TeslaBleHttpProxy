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

var vehicleOutOfRange = true

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
		logging.Fatal("failed to send response", "error", err)
	}
	logging.Debug("Response", "Command", response.Command, "Status", status, "Result", response.Result, "Reason", response.Reason)
}

func checkBleControl(response *models.Response) bool {
	if control.BleControlInstance == nil {
		response.Reason = "BleControl is not initialized. Maybe private.pem is missing."
		response.Result = false
		return false
	}
	return true
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

	if !checkBleControl(&response) {
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

	if wait {
		var apiResponse models.ApiResponse
		wg := sync.WaitGroup{}
		apiResponse.Wait = &wg
		apiResponse.Ctx = r.Context()

		wg.Add(1)
		control.BleControlInstance.PushCommand(command, vin, body, &apiResponse, autoWakeup)

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

	control.BleControlInstance.PushCommand(command, vin, body, nil, autoWakeup)
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

	if !checkBleControl(&response) {
		return
	}

	cacheTime := time.Duration(config.AppConfig.VehicleDataCacheTime) * time.Second

	// Check cache for each endpoint
	vehicleDataCacheMux.RLock()
	cachedData := make(map[string]json.RawMessage) // endpoint -> cached data
	missingEndpoints := []string{}

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
				cachedData[endpoint] = cachedEntry.data
				missingEndpoints = append(missingEndpoints, endpoint)
			}
		} else {
			// Cache miss for this endpoint
			logging.Debug("VehicleData endpoint cache miss", "VIN", vin, "Endpoint", endpoint)
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
	var apiResponse models.ApiResponse
	wg := sync.WaitGroup{}
	apiResponse.Wait = &wg
	apiResponse.Ctx = r.Context()

	wg.Add(1)
	autoWakeup := r.URL.Query().Get("wakeup") == "true" || vehicleOutOfRange
	control.BleControlInstance.PushCommand(command, vin, map[string]interface{}{"endpoints": endpoints}, &apiResponse, autoWakeup)

	wg.Wait()

	if !apiResponse.Result && strings.Contains(apiResponse.Error, "not in range") && !vehicleOutOfRange {
		logging.Debug("Vehicle out of range. Setting bool vehicleOutOfRange", "VIN", vin)
		fmt.Printf("Vehicle out of range. Setting bool vehicleOutOfRange")
		vehicleOutOfRange = true
	}

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

		vehicleOutOfRange = false
		response.Result = true
		response.Reason = "The request was successfully processed."
		response.Response = responseJson
	} else {
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

	if !checkBleControl(&response) {
		return
	}

	var apiResponse models.ApiResponse

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	apiResponse.Ctx = ctx
	defer cancel()
	cmd := &commands.Command{
		Command:  "body-controller-state",
		Domain:   commands.Domain.VCSEC,
		Vin:      vin,
		Response: &apiResponse,
	}
	conn, car, _, err := control.BleControlInstance.TryConnectToVehicle(ctx, cmd)
	if err == nil {
		//Successful
		defer conn.Close()
		//defer log.Debug("close connection (A)")
		defer car.Disconnect()
		//defer log.Debug("disconnect vehicle (A)")

		_, err, _ := control.BleControlInstance.ExecuteCommand(car, cmd, context.Background())
		if err != nil {
			response.Result = false
			response.Reason = err.Error()
			return
		}

		SetCacheControl(w, config.AppConfig.CacheMaxAge)

		if apiResponse.Result {
			response.Result = true
			response.Reason = "The request was successfully processed."
			response.Response = apiResponse.Response
		} else {
			response.Result = false
			response.Reason = apiResponse.Error
		}
	} else {
		response.Result = false
		response.Reason = err.Error()
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
