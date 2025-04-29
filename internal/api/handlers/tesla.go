package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/gorilla/mux"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/control"
	"github.com/wimaha/TeslaBleHttpProxy/internal/tesla/commands"
)

func writeResponseWithStatus(w http.ResponseWriter, response *models.Response) {
	var ret models.Ret
	ret.Response = *response

	w.Header().Set("Content-Type", "application/json")
	status := http.StatusOK
	if !response.Result {
		status = http.StatusServiceUnavailable
	}
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(ret); err != nil {
		log.Fatal("failed to send response", "error", err)
	}
	log.Debug("response", "command", response.Command, "status", status, "result", response.Result, "reason", response.Reason)
}

func checkBleControl(response *models.Response) bool {
	if control.BleControlInstance == nil {
		response.Reason = "BleControl is not initialized. Maybe private.pem is missing."
		response.Result = false
		return false
	}
	return true
}

func processCommand(w http.ResponseWriter, r *http.Request, vin string, command_name string, src commands.CommandSourceType, body map[string]interface{}, wait bool) (models.Response, bool) {
	var response models.Response
	response.Vin = vin
	response.Command = command_name

	if !checkBleControl(&response) {
		return response, true
	}

	var apiResponse models.ApiResponse
	command := commands.Command{
		Command: command_name,
		Source:  src,
		Vin:     vin,
		Body:    body,
	}

	if wait {
		wg := sync.WaitGroup{}
		command.Response = &apiResponse
		apiResponse.Wait = &wg
		apiResponse.Ctx = r.Context()

		wg.Add(1)
		control.BleControlInstance.PushCommand(command)

		wgDone := make(chan struct{})
		go func() {
			defer close(wgDone)
			wg.Wait()
		}()
		select {
		case <-wgDone:
		case <-r.Context().Done():
			log.Info("Request cancelled", "vin", vin, "command", command_name)
			return response, false
		}

		SetCacheControl(w, config.AppConfig.CacheMaxAge)

		if apiResponse.Result {
			response.Result = true
			response.Reason = "The command was successfully processed."
			response.Response = apiResponse.Response
		} else {
			response.Result = false
			response.Reason = apiResponse.Error
		}
	} else {
		control.BleControlInstance.PushCommand(command)
		response.Result = true
		response.Reason = "The command was successfully received and will be processed shortly."
	}

	return response, true
}

func VehicleCommand(w http.ResponseWriter, r *http.Request) {
	ShowRequest(r, "Command")
	params := mux.Vars(r)
	vin := params["vin"]
	command := params["command"]

	wait := r.URL.Query().Get("wait") == "true"

	var body map[string]interface{} = nil

	// Check if the body is empty
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Debug("decoding body", "err", err)
		// If json fails we try to validate anyways
	}

	if err := commands.ValidateFleetVehicleCommand(command, body); err != nil {
		writeResponseWithStatus(w, &models.Response{Vin: vin, Command: command, Result: false, Reason: err.Error()})
		return
	}

	log.Info("received", "command", command, "body", body)

	if resp, ok := processCommand(w, r, vin, command, commands.CommandSource.FleetVehicleCommands, body, wait); ok {
		writeResponseWithStatus(w, &resp)
	}
}

func VehicleEndpoint(w http.ResponseWriter, r *http.Request) {
	ShowRequest(r, "VehicleEndpoint")
	params := mux.Vars(r)
	vin := params["vin"]
	command := params["command"]

	var body map[string]interface{} = nil

	src := commands.CommandSource.FleetVehicleEndpoint
	wait := true

	checkMethod := func(expected string) bool {
		if r.Method != expected {
			writeResponseWithStatus(w, &models.Response{Vin: vin, Command: command, Result: false, Reason: "Invalid method: " + r.Method})
			return false
		}
		return true
	}

	switch command {
	case "wake_up":
		if !checkMethod("POST") {
			return
		}
		wait = r.URL.Query().Get("wait") == "true"
	case "vehicle_data":
		if !checkMethod("GET") {
			return
		}
		var endpoints []string
		endpointsString := r.URL.Query().Get("endpoints")
		if endpointsString != "" {
			endpoints = strings.Split(endpointsString, ";")
		} else {
			// 'charge_state', 'climate_state', 'closures_state',
			// 'drive_state', 'gui_settings', 'location_data',
			// 'charge_schedule_data', 'preconditioning_schedule_data',
			// 'vehicle_config', 'vehicle_state', 'vehicle_data_combo'
			endpoints = []string{"charge_state", "climate_state"}
		}

		// Ensure that the endpoints are valid
		for _, endpoint := range endpoints {
			if _, err := commands.GetCategory(endpoint); err != nil {
				writeResponseWithStatus(w, &models.Response{Vin: vin, Command: command, Result: false, Reason: err.Error()})
				return
			}
		}

		body = map[string]interface{}{"endpoints": endpoints}
	default:
		writeResponseWithStatus(w, &models.Response{Vin: vin, Command: command, Result: false, Reason: "Unrecognized command: " + command})
		return
	}

	log.Info("received", "command", command, "body", body)
	if resp, ok := processCommand(w, r, vin, command, src, body, wait); ok {
		writeResponseWithStatus(w, &resp)
	}
}

func ProxyCommand(w http.ResponseWriter, r *http.Request) {
	ShowRequest(r, "ProxyCommand")
	params := mux.Vars(r)
	vin := params["vin"]
	command := params["command"]

	switch command {
	case "connection_status":
	case "body_controller_state":
	case "session_info":
	default:
		writeResponseWithStatus(w, &models.Response{Vin: vin, Command: command, Result: false, Reason: "Unrecognized command: " + command})
		return
	}

	log.Info("received", "command", command)
	if resp, ok := processCommand(w, r, vin, command, commands.CommandSource.TeslaBleHttpProxy, nil, true); ok {
		writeResponseWithStatus(w, &resp)
	}
}

func ShowRequest(r *http.Request, handler string) {
	log.Debug("received", "handler", handler, "method", r.Method, "url", r.URL, "from", r.RemoteAddr)
}

func SetCacheControl(w http.ResponseWriter, maxAge int) {
	if maxAge > 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, must-revalidate", maxAge))
	} else {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	}
}
