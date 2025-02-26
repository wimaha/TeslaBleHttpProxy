package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
)

func (command *Command) Send(ctx context.Context, car *vehicle.Vehicle) (shouldRetry bool, err error) {
	log.Debug("sending command", "command", command.Command, "source", command.Source, "vin", command.Vin)
	if command.Source == CommandSource.TeslaBleHttpProxy {
		switch command.Command {
		case "session_info":
			publicKey, err := protocol.LoadPublicKey(config.PublicKeyFile)
			if err != nil {
				return false, fmt.Errorf("failed to load public key: %s", err)
			}

			info, err := car.SessionInfo(ctx, publicKey, protocol.DomainVCSEC)
			if err != nil {
				return true, fmt.Errorf("failed session_info: %s", err)
			}
			fmt.Printf("%s\n", info)
		case "add-key-request":
			publicKey, err := protocol.LoadPublicKey(config.PublicKeyFile)
			if err != nil {
				return false, fmt.Errorf("failed to load public key: %s", err)
			}

			if err := car.SendAddKeyRequest(ctx, publicKey, true, vcsec.KeyFormFactor_KEY_FORM_FACTOR_CLOUD_KEY); err != nil {
				return true, fmt.Errorf("failed to add key: %s", err)
			} else {
				log.Info(fmt.Sprintf("Sent add-key request to %s. Confirm by tapping NFC card on center console.", car.VIN()))
			}
		case "body_controller_state":
			info, err := car.BodyControllerState(ctx)
			if err != nil {
				return true, fmt.Errorf("failed to get body controller state: %s", err)
			}
			converted := models.BodyControllerStateFromBle(info)
			d, err := json.Marshal(converted)
			if err != nil {
				return true, fmt.Errorf("failed to marshal body-controller-state: %s", err)
			}
			command.Response.Response = d
		default:
			return false, fmt.Errorf("unrecognized proxy command: %s", command.Command)
		}
	} else if command.Source == CommandSource.FleetVehicleEndpoint {
		switch command.Command {
		case "vehicle_data":
			if command.Body == nil {
				return false, fmt.Errorf("request body is nil")
			}

			endpoints, ok := command.Body["endpoints"].([]string)
			if !ok {
				return false, fmt.Errorf("missing or invalid 'endpoints' in request body")
			}

			response := make(map[string]json.RawMessage)
			for _, endpoint := range endpoints {
				log.Debugf("get: %s", endpoint)
				category, err := GetCategory(endpoint)
				if err != nil {
					return false, fmt.Errorf("unrecognized state category charge")
				}
				data, err := car.GetState(ctx, category)
				if err != nil {
					return true, fmt.Errorf("failed to get vehicle data: %s", err)
				}

				var converted interface{}
				switch endpoint {
				case "charge_state":
					converted = models.ChargeStateFromBle(data)
				case "climate_state":
					converted = models.ClimateStateFromBle(data)
				case "drive_state":
					converted = models.DriveStateFromBle(data)
				case "location_data":
					converted = models.LocationDataFromBle(data)
				case "closures_state":
					converted = models.ClosuresStateFromBle(data)
				case "charge_schedule_data":
					converted = models.ChargeScheduleDataFromBle(data)
				case "preconditioning_schedule_data":
					converted = models.PreconditioningScheduleDataFromBle(data)
				case "tire_pressure":
					converted = models.TirePressureFromBle(data)
				case "media":
					converted = models.MediaFromBle(data)
				case "media_detail":
					converted = models.MediaDetailFromBle(data)
				case "software_update":
					converted = models.SoftwareUpdateFromBle(data)
				case "parental_controls":
					converted = models.ParentalControlsFromBle(data)
				case "gui_settings":
					fallthrough
				case "vehicle_config":
					fallthrough
				case "vehicle_state":
					fallthrough
				case "vehicle_data_combo":
					return false, fmt.Errorf("not supported via BLE %s", endpoint)
				default:
					return false, fmt.Errorf("unrecognized state category: %s", endpoint)
				}
				d, err := json.Marshal(converted)
				if err != nil {
					return true, fmt.Errorf("failed to marshal vehicle data: %s", err)
				}

				response[endpoint] = d
			}

			responseJson, err := json.Marshal(response)
			if err != nil {
				return false, fmt.Errorf("failed to marshal vehicle data: %s", err)
			}
			command.Response.Response = responseJson
		case "wake_up":
			// car.WakeUp and waiting for the car was already handled by the operated
			// connection, because this is marked as an infotainment domain command
			// Nothing to do here :^)
		default:
			return false, fmt.Errorf("unrecognized vehicle endpoint command: %s", command.Command)
		}
	} else if command.Source == CommandSource.FleetVehicleCommands {
		handler, ok := fleetVehicleCommands[command.Command]
		if !ok {
			return false, fmt.Errorf("unrecognized vehicle command: %s", command.Command)
		}

		// It should already be validated by the time it gets here
		if handler.execute != nil {
			if err := handler.execute(ctx, car, command.Body); err != nil {
				if handler.checkError != nil {
					err = handler.checkError(err)
				}

				if err != nil {
					return true, fmt.Errorf("failed to execute command: %s", err)
				}
			}
		} else {
			log.Warn("command not implemented: %s", command.Command)
		}
	} else {
		log.Warn("unrecognized command source: %s", command.Source)
	}

	// everything fine
	return false, nil
}
