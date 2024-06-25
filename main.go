package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/gorilla/mux"
	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
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

var privateKeyFile = "key/private.pem"

var exceptedCommands = []string{"flash_lights", "wake_up", "set_charging_amps", "charge_start", "charge_stop", "session_info"}

type Stack []Command

var currentCommands Stack

// IsEmpty: check if stack is empty
func (s *Stack) IsEmpty() bool {
	return len(*s) == 0
}

// Push a new value onto the stack
func (s *Stack) Push(str Command) {
	*s = append(*s, str) // Simply append the new value to the end of the stack
}

// Prepend to Stack
func (s *Stack) Prepend(str Command) {
	*s = append([]Command{str}, *s...)
}

// Remove and return top element of stack. Return true if stack is empty.
func (s *Stack) Pop() (Command, bool) {
	if s.IsEmpty() {
		return Command{}, true
	} else {
		index := 0             // Get the index of the top most element.
		element := (*s)[index] // Index into the slice and obtain the element.
		*s = (*s)[index+1:]    // Remove it from the stack by slicing it off.
		return element, false
	}
}

func main() {
	log.Info("TeslaBleHttpProxy is loading ...")

	envLogLevel := os.Getenv("logLevel")
	if envLogLevel == "debug" {
		log.SetLevel(log.DebugLevel)
		log.Debug("LogLevel set to debug")
	}

	router := mux.NewRouter()

	go loop()

	// Define the endpoints
	///api/1/vehicles/{vehicle_tag}/command/set_charging_amps
	//router.NotFoundHandler = router.NewRoute().HandlerFunc(testR).GetHandler()
	router.HandleFunc("/api/1/vehicles/{vin}/command/{command}", receiveCommand).Methods("POST")
	//router.HandleFunc("/api/1/vehicles/{vin}/command/{command}", receiveCommand).Methods("GET")

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
		w.WriteHeader(http.StatusCreated)
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

	var newCommand Command
	newCommand.Vin = vin
	newCommand.Command = command
	newCommand.Body = body

	currentCommands.Push(newCommand)

	response.Result = true
	response.Reason = "The command was successfully received and will be processed shortly."
}

func loop() {
	for {
		time.Sleep(1 * time.Second)
		command, empty := currentCommands.Pop()
		if !empty {
			handleCommand(command)
		}
	}
}

func handleCommand(command Command) {
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

	log.Debug("Connecting to bluetooth adapter ...")
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

	log.Debug("Connecting to vehicle...")
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
			return true, fmt.Errorf("failed to start charge: %s", err)
		}
	case "charge_stop":
		if err := car.ChargeStop(ctx); err != nil {
			return true, fmt.Errorf("failed to stop charge: %s", err)
		}
	case "set_charging_amps":
		if chargingAmpsString, ok := body["charging_amps"].(string); ok {
			if chargingAmps, err := strconv.ParseInt(chargingAmpsString, 10, 32); err == nil {
				if err := car.SetChargingAmps(ctx, int32(chargingAmps)); err != nil {
					return true, fmt.Errorf("failed to set charging Amps to %d: %s", chargingAmps, err)
				}
			} else {
				return false, fmt.Errorf("charing Amps parsing error: %s", err)
			}
		} else {
			return false, fmt.Errorf("charing Amps missing in body")
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
}
