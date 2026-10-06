package commands

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/vehicle"
	"google.golang.org/protobuf/proto"
)

// Bodies exactly as energy-brain builds them (TeslaCommandService.addChargeSchedule): the
// existing caller must keep working whatever validation is added later.
const (
	bodyEndTime = `{"lat": 50.1,"lon": 14.4,"days_of_week": "FRIDAY","start_enabled": true,"start_time": 600,` +
		`"end_enabled": true,"end_time": 720,"one_time": false,"enabled": true}`
	bodyNoEnd = `{"lat": 50.1,"lon": 14.4,"days_of_week": "Monday,Tuesday","start_enabled": true,"start_time": 0,` +
		`"end_enabled": false,"one_time": false,"enabled": false}`
	bodyUpdate = `{"lat": 50.1,"lon": 14.4,"id": 3,"days_of_week": "All","start_enabled": true,"start_time": 1439,` +
		`"end_enabled": false,"one_time": false,"enabled": true}`
)

func decode(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestChargeScheduleFromEnergyBrainBodies(t *testing.T) {
	cases := []struct {
		name string
		body string
		want *vehicle.ChargeSchedule
	}{
		{"with end time", bodyEndTime, &vehicle.ChargeSchedule{
			StartEnabled: true, StartTime: 600, EndEnabled: true, EndTime: 720,
			Enabled: true, DaysOfWeek: 32, Latitude: 50.1, Longitude: 14.4}},
		{"no end time, midnight start", bodyNoEnd, &vehicle.ChargeSchedule{
			StartEnabled: true, StartTime: 0, Enabled: false, DaysOfWeek: 2 | 4,
			Latitude: 50.1, Longitude: 14.4}},
		{"update by id", bodyUpdate, &vehicle.ChargeSchedule{
			Id: 3, StartEnabled: true, StartTime: 1439, Enabled: true, DaysOfWeek: 127,
			Latitude: 50.1, Longitude: 14.4}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := chargeScheduleFromBody(decode(t, c.body))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !proto.Equal(got, c.want) {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestParseDaysOfWeekNames(t *testing.T) {
	cases := map[string]int32{
		"All": 127, "all": 127, "Weekdays": 62,
		"SUNDAY": 1, "monday": 2, "Tuesday": 4, "Wednesday": 8, "Thursday": 16, "Friday": 32, "Saturday": 64,
		"Monday, Friday": 2 | 32,
	}
	for in, want := range cases {
		got, err := parseDaysOfWeek(in)
		if err != nil || got != want {
			t.Errorf("parseDaysOfWeek(%q) = %d, %v, want %d", in, got, err, want)
		}
	}
}

func TestDaysOfWeekAsFleetBitmask(t *testing.T) {
	got, err := chargeScheduleFromBody(decode(t,
		`{"lat": 50.1, "lon": 14.4, "days_of_week": 34, "start_enabled": true, "start_time": 600}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.DaysOfWeek != 34 { // Monday + Friday
		t.Errorf("DaysOfWeek = %d, want 34", got.DaysOfWeek)
	}
}

func TestChargeScheduleRejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"typo in day name":      `{"days_of_week": "Mondey"}`,
		"empty days text":       `{"days_of_week": ""}`,
		"days list not a list":  `{"days_of_week": ["Monday"]}`,
		"bitmask too large":     `{"days_of_week": 128}`,
		"negative bitmask":      `{"days_of_week": -1}`,
		"fractional bitmask":    `{"days_of_week": 1.5}`,
		"start missing":         `{"start_enabled": true}`,
		"end missing":           `{"start_enabled": true, "start_time": 1, "end_enabled": true}`,
		"start after midnight":  `{"start_enabled": true, "start_time": 1440}`,
		"negative end":          `{"end_enabled": true, "end_time": -5}`,
		"time as text":          `{"start_enabled": true, "start_time": "08:00"}`,
		"enabled as text":       `{"enabled": "yes"}`,
		"negative id":           `{"id": -1}`,
		"id as text":            `{"id": "3"}`,
		"latitude out of range": `{"lat": 91}`,
		"longitude as text":     `{"lon": "14.4"}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			// on top of a valid body, so each case fails for its own field only
			body := decode(t, `{"lat": 50.1, "lon": 14.4, "days_of_week": "All"}`)
			for k, v := range decode(t, raw) {
				body[k] = v
			}
			got, err := chargeScheduleFromBody(body)
			if err == nil {
				t.Fatalf("expected an error, got schedule %+v", got)
			}
			var bad *InvalidInputError
			if !errors.As(err, &bad) {
				t.Errorf("error %v is not an InvalidInputError, so the API would report success", err)
			}
		})
	}
}

func TestChargeScheduleRequiresLocationAndDays(t *testing.T) {
	for _, key := range []string{"lat", "lon", "days_of_week"} {
		t.Run(key, func(t *testing.T) {
			body := decode(t, bodyEndTime)
			delete(body, key)
			_, err := chargeScheduleFromBody(body)
			var bad *InvalidInputError
			if !errors.As(err, &bad) {
				t.Fatalf("missing %s: got %v, want an InvalidInputError", key, err)
			}
		})
	}
}

func TestChargeScheduleIDFromBody(t *testing.T) {
	id, err := chargeScheduleIDFromBody(decode(t, `{"id": 7}`))
	if err != nil || id != 7 {
		t.Errorf("got %d, %v, want 7", id, err)
	}
	for _, raw := range []string{`{}`, `{"id": "7"}`, `{"id": -2}`, `{"id": 1.5}`} {
		if _, err := chargeScheduleIDFromBody(decode(t, raw)); err == nil {
			t.Errorf("%s: expected an error", raw)
		}
	}
}
