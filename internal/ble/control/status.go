package control

import (
	"sort"
	"sync"
	"time"
)

// Sleep states as the command loop last saw them.
const (
	SleepStatusAwake  = "awake"
	SleepStatusAsleep = "asleep"
)

// Event is something the command loop did with a vehicle, for the dashboard.
type Event struct {
	At      time.Time `json:"at"`
	Command string    `json:"command"`
	Message string    `json:"message,omitempty"`
}

// VehicleConnection is what the command loop knows about one vehicle. It is filled where the
// loop learns it anyway and never causes a BLE request of its own.
type VehicleConnection struct {
	Vin            string    `json:"vin"`
	Connected      bool      `json:"connected"`
	ConnectedSince time.Time `json:"connected_since,omitzero"`
	SleepStatus    string    `json:"sleep_status,omitempty"` // SleepStatusAwake, SleepStatusAsleep or empty if unknown
	SleepStatusAt  time.Time `json:"sleep_status_at,omitzero"`
	LastSuccess    *Event    `json:"last_success,omitempty"`
	LastError      *Event    `json:"last_error,omitempty"`
}

type statusTracker struct {
	mu       sync.Mutex
	vehicles map[string]*VehicleConnection
}

func (t *statusTracker) update(vin string, f func(v *VehicleConnection)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.vehicles == nil {
		t.vehicles = make(map[string]*VehicleConnection)
	}
	v, ok := t.vehicles[vin]
	if !ok {
		v = &VehicleConnection{Vin: vin}
		t.vehicles[vin] = v
	}
	f(v)
}

func (t *statusTracker) connected(vin string, connected bool) {
	t.update(vin, func(v *VehicleConnection) {
		v.Connected = connected
		if connected {
			v.ConnectedSince = time.Now()
		} else {
			v.ConnectedSince = time.Time{}
		}
	})
}

func (t *statusTracker) succeeded(vin, command string) {
	t.update(vin, func(v *VehicleConnection) {
		v.LastSuccess = &Event{At: time.Now(), Command: command}
	})
}

func (t *statusTracker) failed(vin, command string, err error) {
	t.update(vin, func(v *VehicleConnection) {
		v.LastError = &Event{At: time.Now(), Command: command, Message: err.Error()}
	})
}

func (t *statusTracker) sleepStatus(vin, status string) {
	t.update(vin, func(v *VehicleConnection) {
		v.SleepStatus = status
		v.SleepStatusAt = time.Now()
	})
}

func (t *statusTracker) snapshot() []VehicleConnection {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]VehicleConnection, 0, len(t.vehicles))
	for _, v := range t.vehicles {
		c := *v
		if v.LastSuccess != nil {
			e := *v.LastSuccess
			c.LastSuccess = &e
		}
		if v.LastError != nil {
			e := *v.LastError
			c.LastError = &e
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Vin < out[j].Vin })
	return out
}

// Vehicles returns what the command loop knows about each vehicle it talked to since it started.
func (bc *BleControl) Vehicles() []VehicleConnection {
	return bc.status.snapshot()
}

// QueueLength returns the number of commands waiting for the loop.
func (bc *BleControl) QueueLength() int {
	return len(bc.commandStack)
}
