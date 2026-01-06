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

	"github.com/charmbracelet/log"
	"github.com/gorilla/mux"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/control"
	"github.com/wimaha/TeslaBleHttpProxy/internal/tesla/commands"
)

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
		log.Fatal("failed to send response", "error", err)
	}
	log.Debug("Response", "Command", response.Command, "Status", status, "Result", response.Result, "Reason", response.Reason)
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
	autoWakeup := r.URL.Query().Get("wakeup") == "true"

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
		log.Error("Decoding body", "Error", err)
	}

	logRequestWithBody(r, "Command", body)

	if !slices.Contains(commands.ExceptedCommands, command) {
		log.Error("Command not supported", "Command", command)
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
			log.Error("Endpoint not supported", "Endpoint", endpoint)
			response.Reason = fmt.Sprintf("The endpoint \"%s\" is not supported.", endpoint)
			response.Result = false
			return
		}
	}

	defer commonDefer(w, &response)

	if !checkBleControl(&response) {
		return
	}

	var apiResponse models.ApiResponse
	wg := sync.WaitGroup{}
	apiResponse.Wait = &wg
	apiResponse.Ctx = r.Context()

	wg.Add(1)
	autoWakeup := r.URL.Query().Get("wakeup") == "true"
	control.BleControlInstance.PushCommand(command, vin, map[string]interface{}{"endpoints": endpoints}, &apiResponse, autoWakeup)

	wg.Wait()

	SetCacheControl(w, config.AppConfig.CacheMaxAge)

	if apiResponse.Result {
		response.Result = true
		response.Reason = "The request was successfully processed."
		response.Response = apiResponse.Response
	} else {
		response.Result = false
		response.Reason = apiResponse.Error
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
	log.Debug("Received HTTP request", "Handler", handler, "Method", r.Method, "Endpoint", r.URL, "Client", r.RemoteAddr)
}

func logRequestWithBody(r *http.Request, handler string, body map[string]interface{}) {
	log.Debug("Received HTTP request", "Handler", handler, "Method", r.Method, "Endpoint", r.URL, "Client", r.RemoteAddr, "Body", body)
}

func SetCacheControl(w http.ResponseWriter, maxAge int) {
	if maxAge > 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, must-revalidate", maxAge))
	} else {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	}
}
