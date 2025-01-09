package commands

import (
	"context"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

func TestAllExpectedCommandsImplemented(t *testing.T) {
	// Get all case statements from the Send method by creating a dummy command
	// and checking which commands return "unrecognized command"
	command := &Command{}

	for _, expectedCmd := range ExpectedCommands {
		command.Command = expectedCmd
		car := &vehicle.Vehicle{}
		ctx := context.Background()

		// Try to find if command is implemented by checking if it returns
		// "unrecognized command" error
		_, err := command.Send(ctx, car)

		if err != nil && err.Error() == "unrecognized command: "+expectedCmd {
			t.Errorf("Command %q is in ExpectedCommands but not implemented in Send method", expectedCmd)
		}
	}
}
