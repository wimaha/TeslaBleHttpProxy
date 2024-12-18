package control

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

type ApiResponse struct {
	Wait     *sync.WaitGroup
	Result   bool
	Error    string
	Response json.RawMessage
}

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

type Command struct {
	Command  string
	Domain   DomainType
	Vin      string
	Body     map[string]interface{}
	Response *ApiResponse
}

// 'charge_state', 'climate_state', 'closures_state', 'drive_state', 'gui_settings', 'location_data', 'charge_schedule_data', 'preconditioning_schedule_data', 'vehicle_config', 'vehicle_state', 'vehicle_data_combo'
var categoriesByName = map[string]vehicle.StateCategory{
	"charge_state":          vehicle.StateCategoryCharge,
	"climate_state":         vehicle.StateCategoryClimate,
	"drive":                 vehicle.StateCategoryDrive,
	"closures_state":        vehicle.StateCategoryClosures,
	"charge-schedule":       vehicle.StateCategoryChargeSchedule,
	"precondition-schedule": vehicle.StateCategoryPreconditioningSchedule,
	"tire-pressure":         vehicle.StateCategoryTirePressure,
	"media":                 vehicle.StateCategoryMedia,
	"media-detail":          vehicle.StateCategoryMediaDetail,
	"software-update":       vehicle.StateCategorySoftwareUpdate,
	"parental-controls":     vehicle.StateCategoryParentalControls,
}

func GetCategory(nameStr string) (vehicle.StateCategory, error) {
	if category, ok := categoriesByName[strings.ToLower(nameStr)]; ok {
		return category, nil
	}
	return 0, fmt.Errorf("unrecognized state category '%s'", nameStr)
}
