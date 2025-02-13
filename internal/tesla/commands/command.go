package commands

import (
	"fmt"
	"strings"

	"github.com/teslamotors/vehicle-command/pkg/vehicle"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
)

type DomainType string

var Domain = struct {
	None         DomainType
	VCSEC        DomainType
	Infotainment DomainType
}{
	None:         "",
	VCSEC:        "vcsec",
	Infotainment: "infotainment",
}

type CommandSourceType string

var CommandSource = struct {
	FleetVehicleCommands CommandSourceType
	FleetVehicleEndpoint CommandSourceType
	TeslaBleHttpProxy    CommandSourceType
}{
	FleetVehicleCommands: "cmd",
	FleetVehicleEndpoint: "end",
	TeslaBleHttpProxy:    "proxy",
}

type Command struct {
	Command      string
	Source       CommandSourceType
	Vin          string
	Body         map[string]interface{}
	TotalRetries int
	Response     *models.ApiResponse
}

var categoriesByName = map[string]vehicle.StateCategory{
	"charge_state":                  vehicle.StateCategoryCharge,
	"climate_state":                 vehicle.StateCategoryClimate,
	"drive_state":                   vehicle.StateCategoryDrive,
	"location_data":                 vehicle.StateCategoryLocation,
	"closures_state":                vehicle.StateCategoryClosures,
	"charge_schedule_data":          vehicle.StateCategoryChargeSchedule,
	"preconditioning_schedule_data": vehicle.StateCategoryPreconditioningSchedule,

	// Missing standard categories
	// "gui_settings"
	// "vehicle_config"
	// "vehicle_state"
	// "vehicle_data_combo"

	// Non-standard categories
	"tire_pressure":     vehicle.StateCategoryTirePressure,
	"media":             vehicle.StateCategoryMedia,
	"media_detail":      vehicle.StateCategoryMediaDetail,
	"software_update":   vehicle.StateCategorySoftwareUpdate,
	"parental_controls": vehicle.StateCategoryParentalControls,
}

func (command *Command) Domain() DomainType {
	switch command.Command {
	case "session_info":
		fallthrough
	case "add-key-request":
		fallthrough
	case "body_controller_state":
		return Domain.VCSEC
	default:
		return Domain.Infotainment
	}
}

func (command *Command) IsContextDone() bool {
	if command.Response == nil {
		return false
	}
	if command.Response.Ctx.Err() != nil {
		command.Response.Wait.Done()
		return true
	}
	return false
}

func GetCategory(nameStr string) (vehicle.StateCategory, error) {
	if category, ok := categoriesByName[strings.ToLower(nameStr)]; ok {
		return category, nil
	}
	return 0, fmt.Errorf("unrecognized state category '%s'", nameStr)
}
