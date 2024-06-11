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

	//Body
	var body map[string]interface{} = nil
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
		log.Printf("Error decoding body: %s.\n", err)
	}
	log.Printf("%s\n", body)

	var response Response
	response.Result = true
	response.Reason = ""
	response.Vin = vin
	response.Command = command

	defer func() {
		var ret Ret
		ret.Response = response

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

	// For simplcity, allow 30 seconds to wake up the vehicle, connect to it,
	// and unlock. In practice you'd want a fresh timeout for each command.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	var privateKey protocol.ECDHPrivateKey
	if privateKeyFile != "" {
		if privateKey, err = protocol.LoadPrivateKey(privateKeyFile); err != nil {
			log.Printf("Failed to load private key: %s\n", err)
			response.Reason = fmt.Sprintf("Failed to load private key: %s", err)
			response.Result = false
			return
		}
	}

	conn, err := ble.NewConnection(ctx, vin)
	if err != nil {
		log.Printf("Failed to connect to vehicle (A): %s\n", err)
		response.Reason = fmt.Sprintf("Failed to connect to vehicle (A): %s", err)
		response.Result = false

		if strings.Contains(err.Error(), "operation not permitted") {
			// The underlying BLE package calls HCIDEVDOWN on the BLE device, presumably as a
			// heavy-handed way of dealing with devices that are in a bad state.
			log.Printf("Try again after granting this application CAP_NET_ADMIN:\n\n\tsudo setcap 'cap_net_admin=eip' \"$(which %s)\"\n", os.Args[0])
			response.Reason = fmt.Sprintf("Failed to connect to vehicle (A): %s\nTry again after granting this application CAP_NET_ADMIN:\nsudo setcap 'cap_net_admin=eip' \"$(which %s)\"", err, os.Args[0])
		}

		return
	}
	defer conn.Close()

	car, err := vehicle.NewVehicle(conn, privateKey, nil)
	if err != nil {
		log.Printf("Failed to connect to vehicle (B): %s\n", err)
		response.Reason = fmt.Sprintf("Failed to connect to vehicle (B): %s", err)
		response.Result = false
		return
	}

	if err := car.Connect(ctx); err != nil {
		log.Printf("Failed to connect to vehicle (C): %s\n", err)
		response.Reason = fmt.Sprintf("Failed to connect to vehicle (C): %s", err)
		response.Result = false
		return
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
		log.Printf("Failed to perform handshake with vehicle: %s\n", err)
		response.Reason = fmt.Sprintf("Failed to perform handshake with vehicle: %s", err)
		response.Result = false
		return
	}

	switch command {
	case "flash_lights":
		if err := car.FlashLights(ctx); err != nil {
			log.Printf("Failed to flash lights: %s:\n", err)
			response.Reason = fmt.Sprintf("Failed to flash lights: %s", err)
			response.Result = false
			return
		}
	case "wake_up":
		if err := car.Wakeup(ctx); err != nil {
			log.Printf("Failed to wake up car: %s:\n", err)
			response.Reason = fmt.Sprintf("Failed to wake up car: %s", err)
			response.Result = false
			return
		}
	case "charge_start":
		if err := car.ChargeStart(ctx); err != nil {
			log.Printf("Failed to start charge: %s:\n", err)
			response.Reason = fmt.Sprintf("Failed to start charge: %s", err)
			response.Result = false
			return
		}
	case "charge_stop":
		if err := car.ChargeStop(ctx); err != nil {
			log.Printf("Failed to stop charge: %s:\n", err)
			response.Reason = fmt.Sprintf("Failed to stop charge: %s", err)
			response.Result = false
			return
		}
	case "set_charging_amps":
		if chargingAmpsString, ok := body["charging_amps"].(string); ok {
			if chargingAmps, err := strconv.ParseInt(chargingAmpsString, 10, 32); err == nil {
				if err := car.SetChargingAmps(ctx, int32(chargingAmps)); err != nil {
					log.Printf("Failed to set charging Amps to %d: %s\n", chargingAmps, err)
					response.Reason = fmt.Sprintf("Failed to set charging Amps to %d: %s", chargingAmps, err)
					response.Result = false
					return
				}
			} else {
				log.Printf("Charing Amps parsing error: %s\n", err)
				response.Reason = fmt.Sprintf("Charing Amps parsing error: %s", err)
				response.Result = false
				return
			}
		} else {
			log.Printf("Charing Amps missing in body\n")
			response.Reason = "Charing Amps missing in body"
			response.Result = false
			return
		}
	case "session_info":
		publicKey, err := protocol.LoadPublicKey("key/public.pem")
		if err != nil {
			log.Printf("Failed to load public key: %s\n", err)
			response.Reason = fmt.Sprintf("Failed to load public key: %s", err)
			response.Result = false
			return
		}

		info, err := car.SessionInfo(ctx, publicKey, protocol.DomainVCSEC)
		if err != nil {
			log.Printf("Failed session_info: %s:\n", err)
			response.Reason = fmt.Sprintf("Failed session_info: %s", err)
			response.Result = false
			return
		}
		fmt.Printf("%s\n", info)
	}
}
