package commands

import (
	"context"
	"crypto/ecdh"
	"fmt"
	"strings"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/keys"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
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

// Vehicle is the part of *vehicle.Vehicle that Send uses. Tests pass a simulated car instead.
type Vehicle interface {
	VIN() string
	Wakeup(ctx context.Context) error
	ClimateOn(ctx context.Context) error
	ClimateOff(ctx context.Context) error
	ChangeClimateTemp(ctx context.Context, driverCelsius float32, passengerCelsius float32) error
	ChargePortOpen(ctx context.Context) error
	ChargePortClose(ctx context.Context) error
	FlashLights(ctx context.Context) error
	HonkHorn(ctx context.Context) error
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
	OpenFrunk(ctx context.Context) error
	ActuateTrunk(ctx context.Context) error
	SetSentryMode(ctx context.Context, state bool) error
	ChargeStart(ctx context.Context) error
	ChargeStop(ctx context.Context) error
	SetChargingAmps(ctx context.Context, amps int32) error
	ChangeChargeLimit(ctx context.Context, chargeLimitPercent int32) error
	AddChargeSchedule(ctx context.Context, schedule *vehicle.ChargeSchedule) error
	RemoveChargeSchedule(ctx context.Context, id uint64) error
	SessionInfo(ctx context.Context, publicKey *ecdh.PublicKey, domain universal.Domain) (*signatures.SessionInfo, error)
	SendAddKeyRequestWithRole(ctx context.Context, publicKey *ecdh.PublicKey, role keys.Role, formFactor vcsec.KeyFormFactor) error
	GetState(ctx context.Context, category vehicle.StateCategory) (*carserver.VehicleData, error)
	BodyControllerState(ctx context.Context) (*vcsec.VehicleStatus, error)
}

var _ Vehicle = (*vehicle.Vehicle)(nil)

type Command struct {
	Command    string
	Domain     DomainType
	Vin        string
	Body       map[string]interface{}
	Response   *models.ApiResponse
	AutoWakeup bool
}

// 'charge_state', 'climate_state', 'closures_state', 'drive_state', 'gui_settings', 'location_data', 'charge_schedule_data', 'preconditioning_schedule_data', 'vehicle_config', 'vehicle_state', 'vehicle_data_combo'
var categoriesByName = map[string]vehicle.StateCategory{
	"charge_state":          vehicle.StateCategoryCharge,
	"climate_state":         vehicle.StateCategoryClimate,
	"drive":                 vehicle.StateCategoryDrive,
	"drive_state":           vehicle.StateCategoryDrive,
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
