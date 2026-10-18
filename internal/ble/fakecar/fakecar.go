// Package fakecar simulates a Tesla for tests, so the proxy can be exercised without BLE.
//
// A Car keeps a small vehicle state (sleep, charging, climate, locks) that the commands change,
// records every call, and can be told to fail or hang on the next calls of a method. Like the
// real car it rejects infotainment commands while asleep, so a test notices when the proxy
// forgets to wake it.
package fakecar

import (
	"context"
	"crypto/ecdh"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/keys"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

// ErrAsleep is returned for commands that need the infotainment system while the car sleeps.
var ErrAsleep = errors.New("vehicle is asleep")

// ErrNotPermitted makes Dial fail without retry, like a BLE adapter lacking permissions.
var ErrNotPermitted = errors.New("operation not permitted")

// PrivateKey returns a fixed valid key. With a key the proxy starts authenticated sessions, as
// in production; nothing is read from disk.
func PrivateKey() protocol.ECDHPrivateKey {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i + 1)
	}
	return protocol.UnmarshalECDHPrivateKey(b)
}

// Call is one recorded method call.
type Call struct {
	Method string
	Args   []any
}

// Car is a simulated vehicle. The exported fields are its state; set them before use and read
// them only after the proxy is done with the car, or inside Do.
type Car struct {
	mu sync.Mutex

	Vin            string
	Asleep         bool
	Locked         bool
	SentryMode     bool
	Charging       bool
	ChargingAmps   int32
	ChargeLimit    int32
	BatteryLevel   int32
	ClimateRunning bool
	InsideTemp     float32
	DriverTemp     float32
	PassTemp       float32
	FrunkOpen      bool
	TrunkOpen      bool
	Schedules      map[uint64]*vehicle.ChargeSchedule

	calls   []Call
	fail    map[string][]error
	hang    map[string]int
	nextID  uint64
	session []universal.Domain
}

// New returns an awake, locked car with plausible defaults.
func New(vin string) *Car {
	return &Car{
		Vin:          vin,
		Locked:       true,
		ChargingAmps: 16,
		ChargeLimit:  80,
		BatteryLevel: 55,
		InsideTemp:   19.5,
		DriverTemp:   21,
		PassTemp:     21,
		Schedules:    map[uint64]*vehicle.ChargeSchedule{},
		fail:         map[string][]error{},
		hang:         map[string]int{},
	}
}

// FailNext makes the next len(errs) calls of method return errs in order; a nil entry lets that
// call succeed. Method names are those of the Car methods, plus "Dial".
func (c *Car) FailNext(method string, errs ...error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fail[method] = append(c.fail[method], errs...)
}

// HangNext makes the next call of method block until its context ends.
func (c *Car) HangNext(method string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hang[method]++
}

// Calls returns all recorded calls in order.
func (c *Car) Calls() []Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.calls)
}

// Methods returns the names of all recorded calls in order.
func (c *Car) Methods() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	names := make([]string, len(c.calls))
	for i, call := range c.calls {
		names[i] = call.Method
	}
	return names
}

// Count returns how often method was called.
func (c *Car) Count(method string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, call := range c.calls {
		if call.Method == method {
			n++
		}
	}
	return n
}

// Do runs f with the state locked, for reading or changing it while the proxy may use the car.
func (c *Car) Do(f func(c *Car)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f(c)
}

// Dial simulates opening a BLE connection. Its signature matches control.Dialer apart from
// returning *Car; a test wraps it in one line.
func (c *Car) Dial(ctx context.Context, vin string) (*Car, bool, error) {
	if err := c.enter(ctx, "Dial", vin); err != nil {
		return nil, !errors.Is(err, ErrNotPermitted), err
	}
	if vin != c.Vin {
		return nil, true, fmt.Errorf("vehicle %s is not in range", vin)
	}
	return c, false, nil
}

// enter records the call, then applies a queued hang or failure. Called without the lock held.
func (c *Car) enter(ctx context.Context, method string, args ...any) error {
	c.mu.Lock()
	c.calls = append(c.calls, Call{Method: method, Args: args})
	hang := c.hang[method] > 0
	if hang {
		c.hang[method]--
	}
	var err error
	if queued := c.fail[method]; len(queued) > 0 {
		err, c.fail[method] = queued[0], queued[1:]
	}
	c.mu.Unlock()

	if hang {
		<-ctx.Done()
		return ctx.Err()
	}
	if err == nil {
		err = ctx.Err()
	}
	return err
}

// awakeCommand is enter for commands that the car only accepts while awake.
func (c *Car) awakeCommand(ctx context.Context, method string, args ...any) error {
	if err := c.enter(ctx, method, args...); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Asleep {
		return ErrAsleep
	}
	return nil
}

func (c *Car) set(f func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f()
}

func (c *Car) VIN() string { return c.Vin }

func (c *Car) StartSession(ctx context.Context, domains []universal.Domain) error {
	if err := c.enter(ctx, "StartSession", slices.Clone(domains)); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Asleep && slices.Contains(domains, protocol.DomainInfotainment) {
		return fmt.Errorf("infotainment handshake: %w", ErrAsleep)
	}
	c.session = slices.Clone(domains)
	return nil
}

func (c *Car) Disconnect() {
	_ = c.enter(context.Background(), "Disconnect")
	c.set(func() { c.session = nil })
}

func (c *Car) Wakeup(ctx context.Context) error {
	if err := c.enter(ctx, "Wakeup"); err != nil {
		return err
	}
	c.set(func() { c.Asleep = false })
	return nil
}

func (c *Car) BodyControllerState(ctx context.Context) (*vcsec.VehicleStatus, error) {
	if err := c.enter(ctx, "BodyControllerState"); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	vs := &vcsec.VehicleStatus{
		VehicleSleepStatus: vcsec.VehicleSleepStatus_E_VEHICLE_SLEEP_STATUS_AWAKE,
		VehicleLockState:   vcsec.VehicleLockState_E_VEHICLELOCKSTATE_UNLOCKED,
		ClosureStatuses:    &vcsec.ClosureStatuses{},
	}
	if c.Asleep {
		vs.VehicleSleepStatus = vcsec.VehicleSleepStatus_E_VEHICLE_SLEEP_STATUS_ASLEEP
	}
	if c.Locked {
		vs.VehicleLockState = vcsec.VehicleLockState_E_VEHICLELOCKSTATE_LOCKED
	}
	if c.FrunkOpen {
		vs.ClosureStatuses.FrontTrunk = vcsec.ClosureState_E_CLOSURESTATE_OPEN
	}
	if c.TrunkOpen {
		vs.ClosureStatuses.RearTrunk = vcsec.ClosureState_E_CLOSURESTATE_OPEN
	}
	return vs, nil
}

// Lock, Unlock and the trunks go through VCSEC and work while the car sleeps.

func (c *Car) Lock(ctx context.Context) error {
	if err := c.enter(ctx, "Lock"); err != nil {
		return err
	}
	c.set(func() { c.Locked = true })
	return nil
}

func (c *Car) Unlock(ctx context.Context) error {
	if err := c.enter(ctx, "Unlock"); err != nil {
		return err
	}
	c.set(func() { c.Locked = false })
	return nil
}

func (c *Car) OpenFrunk(ctx context.Context) error {
	if err := c.enter(ctx, "OpenFrunk"); err != nil {
		return err
	}
	c.set(func() { c.FrunkOpen = true })
	return nil
}

func (c *Car) ActuateTrunk(ctx context.Context) error {
	if err := c.enter(ctx, "ActuateTrunk"); err != nil {
		return err
	}
	c.set(func() { c.TrunkOpen = !c.TrunkOpen })
	return nil
}

func (c *Car) ClimateOn(ctx context.Context) error {
	if err := c.awakeCommand(ctx, "ClimateOn"); err != nil {
		return err
	}
	c.set(func() { c.ClimateRunning = true })
	return nil
}

func (c *Car) ClimateOff(ctx context.Context) error {
	if err := c.awakeCommand(ctx, "ClimateOff"); err != nil {
		return err
	}
	c.set(func() { c.ClimateRunning = false })
	return nil
}

func (c *Car) ChangeClimateTemp(ctx context.Context, driverCelsius float32, passengerCelsius float32) error {
	if err := c.awakeCommand(ctx, "ChangeClimateTemp", driverCelsius, passengerCelsius); err != nil {
		return err
	}
	c.set(func() { c.DriverTemp, c.PassTemp = driverCelsius, passengerCelsius })
	return nil
}

func (c *Car) ChargePortOpen(ctx context.Context) error {
	return c.awakeCommand(ctx, "ChargePortOpen")
}

func (c *Car) ChargePortClose(ctx context.Context) error {
	return c.awakeCommand(ctx, "ChargePortClose")
}

func (c *Car) FlashLights(ctx context.Context) error {
	return c.awakeCommand(ctx, "FlashLights")
}

func (c *Car) HonkHorn(ctx context.Context) error {
	return c.awakeCommand(ctx, "HonkHorn")
}

func (c *Car) SetSentryMode(ctx context.Context, state bool) error {
	if err := c.awakeCommand(ctx, "SetSentryMode", state); err != nil {
		return err
	}
	c.set(func() { c.SentryMode = state })
	return nil
}

// ChargeStart and ChargeStop answer like the car when there is nothing to do.
func (c *Car) ChargeStart(ctx context.Context) error {
	if err := c.awakeCommand(ctx, "ChargeStart"); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Charging {
		return errors.New("car could not execute command: is_charging")
	}
	c.Charging = true
	return nil
}

func (c *Car) ChargeStop(ctx context.Context) error {
	if err := c.awakeCommand(ctx, "ChargeStop"); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.Charging {
		return errors.New("car could not execute command: not_charging")
	}
	c.Charging = false
	return nil
}

func (c *Car) SetChargingAmps(ctx context.Context, amps int32) error {
	if err := c.awakeCommand(ctx, "SetChargingAmps", amps); err != nil {
		return err
	}
	c.set(func() { c.ChargingAmps = amps })
	return nil
}

func (c *Car) ChangeChargeLimit(ctx context.Context, chargeLimitPercent int32) error {
	if err := c.awakeCommand(ctx, "ChangeChargeLimit", chargeLimitPercent); err != nil {
		return err
	}
	c.set(func() { c.ChargeLimit = chargeLimitPercent })
	return nil
}

func (c *Car) AddChargeSchedule(ctx context.Context, schedule *vehicle.ChargeSchedule) error {
	if err := c.awakeCommand(ctx, "AddChargeSchedule", schedule); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	id := schedule.Id
	if id == 0 {
		c.nextID++
		id = c.nextID
	}
	c.Schedules[id] = schedule
	return nil
}

func (c *Car) RemoveChargeSchedule(ctx context.Context, id uint64) error {
	if err := c.awakeCommand(ctx, "RemoveChargeSchedule", id); err != nil {
		return err
	}
	c.set(func() { delete(c.Schedules, id) })
	return nil
}

func (c *Car) SessionInfo(ctx context.Context, publicKey *ecdh.PublicKey, domain universal.Domain) (*signatures.SessionInfo, error) {
	if err := c.enter(ctx, "SessionInfo", domain); err != nil {
		return nil, err
	}
	return &signatures.SessionInfo{Counter: 1}, nil
}

func (c *Car) SendAddKeyRequestWithRole(ctx context.Context, publicKey *ecdh.PublicKey, role keys.Role, formFactor vcsec.KeyFormFactor) error {
	return c.enter(ctx, "SendAddKeyRequestWithRole", role, formFactor)
}

// GetState returns the simulated state; the category is recorded but every section is filled.
func (c *Car) GetState(ctx context.Context, category vehicle.StateCategory) (*carserver.VehicleData, error) {
	if err := c.awakeCommand(ctx, "GetState", category); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	chargingState := &carserver.ChargeState_ChargingState{Type: &carserver.ChargeState_ChargingState_Stopped{Stopped: &carserver.Void{}}}
	if c.Charging {
		chargingState = &carserver.ChargeState_ChargingState{Type: &carserver.ChargeState_ChargingState_Charging{Charging: &carserver.Void{}}}
	}
	return &carserver.VehicleData{
		ChargeState: &carserver.ChargeState{
			ChargingState:          chargingState,
			OptionalBatteryLevel:   &carserver.ChargeState_BatteryLevel{BatteryLevel: c.BatteryLevel},
			OptionalChargeLimitSoc: &carserver.ChargeState_ChargeLimitSoc{ChargeLimitSoc: c.ChargeLimit},
			OptionalChargingAmps:   &carserver.ChargeState_ChargingAmps{ChargingAmps: c.ChargingAmps},
		},
		ClimateState: &carserver.ClimateState{
			OptionalInsideTempCelsius:    &carserver.ClimateState_InsideTempCelsius{InsideTempCelsius: c.InsideTemp},
			OptionalDriverTempSetting:    &carserver.ClimateState_DriverTempSetting{DriverTempSetting: c.DriverTemp},
			OptionalPassengerTempSetting: &carserver.ClimateState_PassengerTempSetting{PassengerTempSetting: c.PassTemp},
			OptionalIsClimateOn:          &carserver.ClimateState_IsClimateOn{IsClimateOn: c.ClimateRunning},
		},
		DriveState: &carserver.DriveState{},
	}, nil
}

// Session returns the domains of the current session, nil when disconnected.
func (c *Car) Session() []universal.Domain {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.session)
}
