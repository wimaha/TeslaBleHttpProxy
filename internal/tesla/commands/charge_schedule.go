package commands

import (
	"fmt"
	"math"
	"strings"

	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

const maxMinuteOfDay = 1439 // 23:59

// InvalidInputError marks a request the caller got wrong. Unlike a failed send it must reach the
// caller as an error: the retry loop otherwise drops the error of a command that is not retried.
type InvalidInputError struct{ msg string }

func (e *InvalidInputError) Error() string { return e.msg }

func invalidInput(format string, args ...interface{}) error {
	return &InvalidInputError{msg: fmt.Sprintf(format, args...)}
}

// chargeScheduleFromBody builds the schedule from the JSON body of add_charge_schedule.
// Fields that are absent stay at their zero value; fields that are present but of the wrong
// type or out of range are an error, so a typo never turns into a schedule on the wrong days.
//
// days_of_week is either the Fleet API integer bitmask (Sunday=1 ... Saturday=64) or text:
// "All", "Weekdays" or comma-separated full day names.
func chargeScheduleFromBody(body map[string]interface{}) (*vehicle.ChargeSchedule, error) {
	schedule := &vehicle.ChargeSchedule{}
	var err error

	if id, ok, err := intField(body, "id", 0, math.MaxInt64); err != nil {
		return nil, err
	} else if ok {
		schedule.Id = uint64(id)
	}

	if schedule.StartEnabled, err = boolField(body, "start_enabled"); err != nil {
		return nil, err
	}
	if schedule.EndEnabled, err = boolField(body, "end_enabled"); err != nil {
		return nil, err
	}
	if schedule.OneTime, err = boolField(body, "one_time"); err != nil {
		return nil, err
	}
	if schedule.Enabled, err = boolField(body, "enabled"); err != nil {
		return nil, err
	}

	start, startGiven, err := intField(body, "start_time", 0, maxMinuteOfDay)
	if err != nil {
		return nil, err
	}
	if schedule.StartEnabled && !startGiven {
		return nil, invalidInput("start_time is required when start_enabled is true")
	}
	schedule.StartTime = int32(start)

	end, endGiven, err := intField(body, "end_time", 0, maxMinuteOfDay)
	if err != nil {
		return nil, err
	}
	if schedule.EndEnabled && !endGiven {
		return nil, invalidInput("end_time is required when end_enabled is true")
	}
	schedule.EndTime = int32(end)

	lat, err := floatField(body, "lat", -90, 90)
	if err != nil {
		return nil, err
	}
	schedule.Latitude = float32(lat)
	lon, err := floatField(body, "lon", -180, 180)
	if err != nil {
		return nil, err
	}
	schedule.Longitude = float32(lon)

	if raw, present := body["days_of_week"]; present {
		if schedule.DaysOfWeek, err = parseDaysOfWeekValue(raw); err != nil {
			return nil, err
		}
	}
	return schedule, nil
}

// chargeScheduleIDFromBody reads the id of remove_charge_schedule.
func chargeScheduleIDFromBody(body map[string]interface{}) (uint64, error) {
	id, ok, err := intField(body, "id", 0, math.MaxInt64)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, invalidInput("id missing in body")
	}
	return uint64(id), nil
}

func boolField(body map[string]interface{}, key string) (bool, error) {
	raw, present := body[key]
	if !present {
		return false, nil
	}
	v, ok := raw.(bool)
	if !ok {
		return false, invalidInput("%s must be true or false", key)
	}
	return v, nil
}

// intField reads an optional whole number within [min, max].
func intField(body map[string]interface{}, key string, min, max int64) (int64, bool, error) {
	raw, present := body[key]
	if !present {
		return 0, false, nil
	}
	v, ok := raw.(float64)
	if !ok || v != math.Trunc(v) || v < float64(min) || v > float64(max) {
		return 0, true, invalidInput("%s must be a whole number between %d and %d", key, min, max)
	}
	return int64(v), true, nil
}

// floatField reads an optional number within [min, max].
func floatField(body map[string]interface{}, key string, min, max float64) (float64, error) {
	raw, present := body[key]
	if !present {
		return 0, nil
	}
	v, ok := raw.(float64)
	if !ok || v < min || v > max {
		return 0, invalidInput("%s must be a number between %v and %v", key, min, max)
	}
	return v, nil
}

func parseDaysOfWeekValue(raw interface{}) (int32, error) {
	switch v := raw.(type) {
	case float64:
		if v != math.Trunc(v) || v < 0 || v > 127 {
			return 0, invalidInput("days_of_week as a number must be a bitmask between 0 and 127")
		}
		return int32(v), nil
	case string:
		return parseDaysOfWeek(v)
	default:
		return 0, invalidInput("days_of_week must be a bitmask number or text such as \"Monday,Friday\"")
	}
}

func parseDaysOfWeek(days string) (int32, error) {
	dayMap := map[string]int32{
		"sunday": 1, "monday": 2, "tuesday": 4, "wednesday": 8,
		"thursday": 16, "friday": 32, "saturday": 64,
	}
	if strings.EqualFold(days, "all") {
		return 127, nil
	}
	if strings.EqualFold(days, "weekdays") {
		return 62, nil
	}
	var mask int32
	for _, d := range strings.Split(days, ",") {
		name := strings.ToLower(strings.TrimSpace(d))
		bit, ok := dayMap[name]
		if !ok {
			return 0, invalidInput("unknown day %q in days_of_week", strings.TrimSpace(d))
		}
		mask |= bit
	}
	return mask, nil
}
