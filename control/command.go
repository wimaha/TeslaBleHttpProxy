package control

import (
	"fmt"
	"strings"

	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

type ApiResponse struct {
	Finished bool
	Result   bool
	Error    string
	Response interface{}
}

type Command struct {
	Command  string
	Vin      string
	Body     map[string]interface{}
	Response *ApiResponse
}

var categoriesByName = map[string]vehicle.StateCategory{
	"charge":                vehicle.StateCategoryCharge,
	"climate":               vehicle.StateCategoryClimate,
	"drive":                 vehicle.StateCategoryDrive,
	"closures":              vehicle.StateCategoryClosures,
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
