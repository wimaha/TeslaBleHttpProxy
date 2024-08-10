package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
	"github.com/wimaha/TeslaBleHttpProxy/control"
	"github.com/wimaha/TeslaBleHttpProxy/html"

	"github.com/gorilla/mux"
)

type Ret struct {
	Response Response `json:"response"`
}

type Response struct {
	Result  bool   `json:"result"`
	Reason  string `json:"reason"`
	Vin     string `json:"vin"`
	Command string `json:"command"`
}

var exceptedCommands = []string{"charge_port_door_open", "charge_port_door_close", "flash_lights", "wake_up", "set_charging_amps", "set_charge_limit", "charge_start", "charge_stop", "session_info"}

//go:embed static/*
var static embed.FS

func main() {
	log.Info("TeslaBleHttpProxy 1.2.2 is loading ...")

	envLogLevel := os.Getenv("logLevel")
	if envLogLevel == "debug" {
		log.SetLevel(log.DebugLevel)
		log.Debug("LogLevel set to debug")
		ble.SetDebugLog()
	}

	control.SetupBleControl()

	router := mux.NewRouter()

	// Define the endpoints
	///api/1/vehicles/{vehicle_tag}/command/set_charging_amps
	router.HandleFunc("/api/1/vehicles/{vin}/command/{command}", receiveCommand).Methods("POST")
	router.HandleFunc("/dashboard", html.ShowDashboard).Methods("GET")
	router.HandleFunc("/gen_keys", html.GenKeys).Methods("GET")
	router.HandleFunc("/remove_keys", html.RemoveKeys).Methods("GET")
	router.HandleFunc("/send_key", html.SendKey).Methods("POST")
	router.PathPrefix("/static/").Handler(http.FileServer(http.FS(static)))

	log.Info("TeslaBleHttpProxy is running!")
	log.Fatal(http.ListenAndServe(":8080", router))
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

	control.BleControlInstance.PushCommand(command, vin, body)

	response.Result = true
	response.Reason = "The command was successfully received and will be processed shortly."
}

/*func pushCommand(command string, vin string, body map[string]interface{}) error {
	if bleControl == nil {
		return fmt.Errorf("BleControl is not initialized. Maybe private.pem is missing.")
	}

	bleControl.PushCommand(command, vin, body)

	return nil
}*/
