package control

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/tesla/commands"
)

// An invalid body is rejected before the car is touched, so a nil vehicle is enough here.
func TestExecuteCommandReportsInvalidInput(t *testing.T) {
	wait := &sync.WaitGroup{}
	wait.Add(1)
	cmd := &commands.Command{
		Command:  "add_charge_schedule",
		Body:     map[string]interface{}{"days_of_week": "Mondey"},
		Response: &models.ApiResponse{Wait: wait, Ctx: context.Background()},
	}

	retry, err, _ := (&BleControl{}).ExecuteCommand(nil, cmd, context.Background())

	var bad *commands.InvalidInputError
	if !errors.As(err, &bad) {
		t.Fatalf("err = %v, want InvalidInputError", err)
	}
	if retry != nil {
		t.Errorf("a bad request must not be retried")
	}
	if cmd.Response.Result || cmd.Response.Error == "" {
		t.Errorf("response = %+v, want result false with a reason", cmd.Response)
	}
	wait.Wait() // returns only if the caller was released
}

// set_temps/actuate_trunk bad bodies take the same InvalidInputError path: reported to the
// caller, not retried, connection kept. Parsing fails before the car is touched.
func TestExecuteCommandReportsInvalidTempsAndTrunk(t *testing.T) {
	for _, tc := range []struct {
		command string
		body    map[string]interface{}
	}{
		{"set_temps", map[string]interface{}{"driver_temp": 40.0}},
		{"actuate_trunk", map[string]interface{}{"which_trunk": "side"}},
	} {
		wait := &sync.WaitGroup{}
		wait.Add(1)
		cmd := &commands.Command{
			Command:  tc.command,
			Body:     tc.body,
			Response: &models.ApiResponse{Wait: wait, Ctx: context.Background()},
		}
		retry, err, _ := (&BleControl{}).ExecuteCommand(nil, cmd, context.Background())
		var bad *commands.InvalidInputError
		if !errors.As(err, &bad) {
			t.Fatalf("%s: err = %v, want InvalidInputError", tc.command, err)
		}
		if retry != nil {
			t.Errorf("%s: a bad request must not be retried", tc.command)
		}
		if cmd.Response.Result || cmd.Response.Error == "" {
			t.Errorf("%s: response = %+v, want result false with a reason", tc.command, cmd.Response)
		}
		wait.Wait()
	}
}
