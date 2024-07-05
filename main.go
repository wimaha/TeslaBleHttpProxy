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

type Command struct {
	Command string
	Vin     string
	Body    map[string]interface{}
}

//var privateKeyFile = "key/private.pem"

var exceptedCommands = []string{"flash_lights", "wake_up", "set_charging_amps", "set_charge_limit", "charge_start", "charge_stop", "session_info"}

//var currentCommands control.Stack

var bleControl *control.BleControl

//go:embed static/*
var static embed.FS

func main() {
	log.Info("TeslaBleHttpProxy 1.1 is loading ...")

	envLogLevel := os.Getenv("logLevel")
	if envLogLevel == "debug" {
		log.SetLevel(log.DebugLevel)
		log.Debug("LogLevel set to debug")
	}

	var err error
	if bleControl, err = control.NewBleControl(); err != nil {
		log.Fatal("BleControl could not be initialized!")
	}
	go bleControl.Loop()

	router := mux.NewRouter()

	// Define the endpoints
	///api/1/vehicles/{vehicle_tag}/command/set_charging_amps
	router.HandleFunc("/api/1/vehicles/{vin}/command/{command}", receiveCommand).Methods("POST")
	router.HandleFunc("/dashboard", html.ShowDashboard).Methods("GET")
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
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ret)
	}()

	//Body
	var body map[string]interface{} = nil
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" && !strings.Contains(err.Error(), "cannot unmarshal bool") {
		log.Error("decoding body", "err", err)
	}

	log.Info("received command", "command", command, "VIN", vin, "body", body)

	if !slices.Contains(exceptedCommands, command) {
		log.Error("The command is not supported.", "command", command)
		response.Reason = fmt.Sprintf("The command \"%s\" is not supported.", command)
		response.Result = false
		return
	}

	bleControl.PushCommand(command, vin, body)

	response.Result = true
	response.Reason = "The command was successfully received and will be processed shortly."
}

/*func handleCommand(command Command) {
	log.Info("handle command", "command", command.Command, "VIN", command.Vin)

	var response Response
	//response.Result = true
	//response.Reason = ""
	response.Vin = command.Vin
	response.Command = command.Command

	defer func() {
		if response.Result {
			log.Info("The command was successfully executed.", "command", command.Command)
		} else {
			log.Error("The command was canceled.", "command", command.Command, "err", response.Reason)
		}
	}()

	var err error
	var retry bool
	var privateKey protocol.ECDHPrivateKey
	if privateKeyFile != "" {
		if privateKey, err = protocol.LoadPrivateKey(privateKeyFile); err != nil {
			log.Error("Failed to load private key.", "err", err)
			response.Reason = fmt.Sprintf("Failed to load private key: %s", err)
			response.Result = false
			return
		}
	}
	log.Debug("PrivateKeyFile loaded")

	var sleep = 3 * time.Second
	var retryCount = 3
	if command.Command == "charge_start" || command.Command == "charge_stop" {
		retryCount = 6
	}

	//Retry max 3 Times
	for i := 0; i < retryCount; i++ {
		if i > 0 {
			log.Warn(err)
			log.Info(fmt.Sprintf("retrying in %d seconds", sleep/time.Second))
			time.Sleep(sleep)
			sleep *= 2
		}
		log.Debug("call executeCommand", "command", command.Command)
		retry, err = executeCommand(command.Body, command.Command, command.Vin, privateKey)
		if err == nil {
			//Successful
			response.Result = true
			response.Reason = ""
			return
		} else if !retry {
			//Failed but no retry possible
			response.Result = false
			response.Reason = err.Error()
			return
		}
	}
	response.Result = false
	response.Reason = err.Error()
	log.Error(fmt.Sprintf("stop retrying after %d attempts", retryCount))
}

// returns bool retry and error or nil if successful
func executeCommand(body map[string]interface{}, command string, vin string, privateKey protocol.ECDHPrivateKey) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Debug("Connecting to vehicle (A)...")
	conn, err := ble.NewConnection(ctx, vin)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			// The underlying BLE package calls HCIDEVDOWN on the BLE device, presumably as a
			// heavy-handed way of dealing with devices that are in a bad state.
			return false, fmt.Errorf("failed to connect to vehicle (A): %s\nTry again after granting this application CAP_NET_ADMIN:\nsudo setcap 'cap_net_admin=eip' \"$(which %s)\"", err, os.Args[0])
		} else {
			return true, fmt.Errorf("failed to connect to vehicle (A): %s", err)
		}
	}
	defer conn.Close()

	log.Debug("Create vehicle object ...")
	car, err := vehicle.NewVehicle(conn, privateKey, nil)
	if err != nil {
		return true, fmt.Errorf("failed to connect to vehicle (B): %s", err)
	}

	log.Debug("Connecting to vehicle (B)...")
	if err := car.Connect(ctx); err != nil {
		return true, fmt.Errorf("failed to connect to vehicle (C): %s", err)
	}
	defer car.Disconnect()

	// Bei wake_up nur die Domain VCSEC ansprechen
	var domains []universalmessage.Domain = nil
	if command == "wake_up" {
		domains = []universalmessage.Domain{protocol.DomainVCSEC}
	}
	// Most interactions with the car require an authenticated client.
	// StartSession() performs a handshake with the vehicle that allows
	// subsequent commands to be authenticated.
	log.Debug("start session...")
	if err := car.StartSession(ctx, domains); err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			//try wakeup vehicle
			log.Debug("try wakeup vehicle...")
			currentCommands.Prepend(Command{Command: command, Vin: vin, Body: body})
			currentCommands.Prepend(Command{Command: "wake_up", Vin: vin})
			return false, fmt.Errorf("vehicle sleeps! trying wakeup vehicle... command will be processed again")
		}
		return true, fmt.Errorf("failed to perform handshake with vehicle: %s", err)
	}

	log.Debug("sending command ...", "command", command)
	switch command {
	case "flash_lights":
		if err := car.FlashLights(ctx); err != nil {
			return true, fmt.Errorf("failed to flash lights: %s", err)
		}
	case "wake_up":
		if err := car.Wakeup(ctx); err != nil {
			return true, fmt.Errorf("failed to wake up car: %s", err)
		}
	case "charge_start":
		if err := car.ChargeStart(ctx); err != nil {
			if strings.Contains(err.Error(), "is_charging") {
				//The car is already charging, so the command is somehow successfully executed.
				log.Info("the car is already charging")
				return false, nil
			}
			return true, fmt.Errorf("failed to start charge: %s", err)
		}
	case "charge_stop":
		if err := car.ChargeStop(ctx); err != nil {
			if strings.Contains(err.Error(), "not_charging") {
				//The car has already stopped charging, so the command is somehow successfully executed.
				log.Info("the car has already stopped charging")
				return false, nil
			}
			return true, fmt.Errorf("failed to stop charge: %s", err)
		}
	case "set_charging_amps":
		var chargingAmps int32
		switch v := body["charging_amps"].(type) {
		case float64:
			chargingAmps = int32(v)
		case string:
			if chargingAmps64, err := strconv.ParseInt(v, 10, 32); err == nil {
				chargingAmps = int32(chargingAmps64)
			} else {
				return false, fmt.Errorf("charing Amps parsing error: %s", err)
			}
		default:
			return false, fmt.Errorf("charing Amps missing in body")
		}
		if err := car.SetChargingAmps(ctx, chargingAmps); err != nil {
			return true, fmt.Errorf("failed to set charging Amps to %d: %s", chargingAmps, err)
		}
	case "set_charge_limit":
		var chargeLimit int32
		switch v := body["percent"].(type) {
		case float64:
			chargeLimit = int32(v)
		case string:
			if chargeLimit64, err := strconv.ParseInt(v, 10, 32); err == nil {
				chargeLimit = int32(chargeLimit64)
			} else {
				return false, fmt.Errorf("charing Amps parsing error: %s", err)
			}
		default:
			return false, fmt.Errorf("charing Amps missing in body")
		}
		if err := car.ChangeChargeLimit(ctx, chargeLimit); err != nil {
			return true, fmt.Errorf("failed to set charge limit to %d %%: %s", chargeLimit, err)
		}
	case "session_info":
		publicKey, err := protocol.LoadPublicKey("key/public.pem")
		if err != nil {
			return false, fmt.Errorf("failed to load public key: %s", err)
		}

		info, err := car.SessionInfo(ctx, publicKey, protocol.DomainVCSEC)
		if err != nil {
			return true, fmt.Errorf("failed session_info: %s", err)
		}
		fmt.Printf("%s\n", info)
	}

	// everything fine
	return false, nil
}*/
