package commands

import (
	"context"
	"testing"

	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/fakecar"
)

func TestAllExceptedCommandsImplemented(t *testing.T) {
	for _, expectedCmd := range ExceptedCommands {
		command := &Command{Command: expectedCmd, Response: &models.ApiResponse{}}
		_, err := command.Send(context.Background(), fakecar.New(testVin))
		if err != nil && err.Error() == "unrecognized command: "+expectedCmd {
			t.Errorf("Command %q is in ExceptedCommands but not implemented in Send method", expectedCmd)
		}
	}
}
