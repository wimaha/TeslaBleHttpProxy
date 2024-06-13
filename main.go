package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

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

var privateKeyFile = "key/private.pem"

var exceptedCommands = []string{"flash_lights", "wake_up", "set_charging_amps", "charge_start", "charge_stop", "session_info"}

func main() {
	log.Println("TeslaBleHttpProxy is loading ...")
	//log.Println("Config loading ...")

	router := mux.NewRouter()

	// Define the endpoints
	///api/1/vehicles/{vehicle_tag}/command/set_charging_amps
	router.HandleFunc("/api/1/vehicles/{vin}/command/{command}", handleCommand).Methods("POST")

	log.Println("TeslaBleHttpProxy is running!")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func handleCommand(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	vin := params["vin"]
	command := params["command"]

	log.Printf("Command: %s (VIN: %s)", command, vin)

	var response Response
	//response.Result = true
	//response.Reason = ""
	response.Vin = vin
	response.Command = command

	defer func() {
		var ret Ret
		ret.Response = response

		if response.Result {
			log.Printf("The command \"%s\" was successfully executed.\n", command)
		} else {
			log.Printf("The command \"%s\" was canceled:\n", command)
			log.Printf("[Error]%s\n", response.Reason)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ret)
	}()

	if !slices.Contains(exceptedCommands, command) {
		log.Printf("The command \"%s\" is not supported.\n", command)
		response.Reason = fmt.Sprintf("The command \"%s\" is not supported.", command)
		response.Result = false
		return
	}

	//Body
	var body map[string]interface{} = nil
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" && !strings.Contains(err.Error(), "cannot unmarshal bool") {
		log.Printf("Error decoding body: %s.\n", err)
	}
	log.Printf("%s\n", body)

	var err error
	var retry bool
	var privateKey protocol.ECDHPrivateKey
	if privateKeyFile != "" {
		if privateKey, err = protocol.LoadPrivateKey(privateKeyFile); err != nil {
			log.Printf("Failed to load private key: %s\n", err)
			response.Reason = fmt.Sprintf("Failed to load private key: %s", err)
			response.Result = false
			return
		}
	}

	var sleep = 3 * time.Second
	//Retry max 3 Times
	for i := 0; i < 3; i++ {
		if i > 0 {
			log.Printf("[Error]%s\n", err)
			log.Printf("retrying in %d seconds", sleep/time.Second)
			time.Sleep(sleep)
			sleep *= 2
		}
		retry, err = executeCommand(body, command, vin, privateKey)
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
	log.Printf("stop retrying after 3 attempts")
}

// returns bool retry and error or nil if successful
func executeCommand(body map[string]interface{}, command string, vin string, privateKey protocol.ECDHPrivateKey) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("Connecting to vehicle...\n")
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

	car, err := vehicle.NewVehicle(conn, privateKey, nil)
	if err != nil {
		return true, fmt.Errorf("failed to connect to vehicle (B): %s", err)
	}

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
	if err := car.StartSession(ctx, domains); err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			//try wakeup vehicle
			log.Printf("try wakeup vehicle...\n")
			if _, err = executeCommand(body, "wake_up", vin, privateKey); err == nil {
				//vehicle wakeup successful
				log.Printf("wakeup successful!\n")
				if err := car.StartSession(ctx, domains); err != nil {
					return true, fmt.Errorf("failed to perform handshake with vehicle: %s", err)
				}
			}
		}
		return true, fmt.Errorf("failed to perform handshake with vehicle: %s", err)
	}

	log.Printf("sending command \"%s\"...", command)
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

	// should never go here
	return false, fmt.Errorf("should never go here")
}
