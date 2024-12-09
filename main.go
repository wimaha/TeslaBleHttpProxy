package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/wimaha/TeslaBleHttpProxy/control"
	"github.com/wimaha/TeslaBleHttpProxy/html"

	"github.com/gorilla/mux"
)

type Ret struct {
	Response Response `json:"response"`
}

type Response struct {
	Result   bool            `json:"result"`
	Reason   string          `json:"reason"`
	Vin      string          `json:"vin"`
	Command  string          `json:"command"`
	Response json.RawMessage `json:"response,omitempty"`
}

var exceptedCommands = []string{"vehicle_data", "auto_conditioning_start", "auto_conditioning_stop", "charge_port_door_open", "charge_port_door_close", "flash_lights", "wake_up", "set_charging_amps", "set_charge_limit", "charge_start", "charge_stop", "session_info"}

//go:embed static/*
var static embed.FS

func main() {
	log.Info("TeslaBleHttpProxy 1.3.0 is loading ...")

	envLogLevel := os.Getenv("logLevel")
	if envLogLevel == "debug" {
		log.SetLevel(log.DebugLevel)
		log.Debug("LogLevel set to debug")
		//ble.SetDebugLog()
	}

	addr := os.Getenv("httpListenAddress")
	if addr == "" {
		addr = ":8080"
	}
	log.Info("TeslaBleHttpProxy", "httpListenAddress", addr)

	control.SetupBleControl()

	router := mux.NewRouter()

	// Define the endpoints
	///api/1/vehicles/{vehicle_tag}/command/set_charging_amps
	router.HandleFunc("/api/1/vehicles/{vin}/command/{command}", receiveCommand).Methods("POST")
	router.HandleFunc("/api/1/vehicles/{vin}/vehicle_data", receiveVehicleData).Methods("GET")
	router.HandleFunc("/dashboard", html.ShowDashboard).Methods("GET")
	router.HandleFunc("/gen_keys", html.GenKeys).Methods("GET")
	router.HandleFunc("/remove_keys", html.RemoveKeys).Methods("GET")
	router.HandleFunc("/send_key", html.SendKey).Methods("POST")
	router.PathPrefix("/static/").Handler(http.FileServer(http.FS(static)))

	log.Info("TeslaBleHttpProxy is running!")
	log.Fatal(http.ListenAndServe(addr, router))
}

/*func testR(w http.ResponseWriter, r *http.Request) {
	log.Printf("[%s]%s\n", r.Method, r.URL)
}*/

func receiveCommand(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	vin := params["vin"]
	command := params["command"]

	var response Response
	response.Vin = vin
	response.Command = command

	defer func() {
		var ret Ret
		ret.Response = response

		w.Header().Set("Content-Type", "application/json")
		if response.Result {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		if err := json.NewEncoder(w).Encode(ret); err != nil {
			log.Fatal("failed to send response", "error", err)
		}
	}()

	if control.BleControlInstance == nil {
		response.Reason = "BleControl is not initialized. Maybe private.pem is missing."
		response.Result = false
		return
	}

	//Body
	var body map[string]interface{} = nil
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" && !strings.Contains(err.Error(), "cannot unmarshal bool") {
		log.Error("decoding body", "err", err)
	}

	log.Info("received", "command", command, "body", body)

	if !slices.Contains(exceptedCommands, command) {
		log.Error("not supported", "command", command)
		response.Reason = fmt.Sprintf("The command \"%s\" is not supported.", command)
		response.Result = false
		return
	}

	control.BleControlInstance.PushCommand(command, vin, body, nil)

	response.Result = true
	response.Reason = "The command was successfully received and will be processed shortly."
}

func receiveVehicleData(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	vin := params["vin"]
	command := "vehicle_data"

	var endpoints []string
	entpointsString := r.URL.Query().Get("endpoints")
	if entpointsString != "" {
		endpoints = strings.Split(entpointsString, ";")
	} else {
		endpoints = []string{"charge_state", "climate_state", "closures_state"} //'charge_state', 'climate_state', 'closures_state', 'drive_state', 'gui_settings', 'location_data', 'charge_schedule_data', 'preconditioning_schedule_data', 'vehicle_config', 'vehicle_state', 'vehicle_data_combo'
	}

	var apiResponse control.ApiResponse
	wg := sync.WaitGroup{}
	apiResponse.Wait = &wg

	wg.Add(1)
	control.BleControlInstance.PushCommand(command, vin, map[string]interface{}{"endpoints": endpoints}, &apiResponse)

	var response Response
	response.Vin = vin
	response.Command = command

	defer func() {
		//var ret Ret
		//ret.Response = response

		w.Header().Set("Content-Type", "application/json")
		if response.Result {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Fatal("failed to send response", "error", err)
		}
	}()

	if control.BleControlInstance == nil {
		response.Reason = "BleControl is not initialized. Maybe private.pem is missing."
		response.Result = false
		return
	}

	wg.Wait()

	if apiResponse.Result {
		response.Result = true
		response.Reason = "The command was successfully processed."
		response.Response = apiResponse.Response
	} else {
		response.Result = false
		response.Reason = apiResponse.Error
	}
}
