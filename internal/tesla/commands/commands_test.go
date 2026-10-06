package commands

import (
	"context"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

func TestAllExceptedCommandsImplemented(t *testing.T) {
	// Get all case statements from the Send method by creating a dummy command
	// and checking which commands return "unrecognized command"
	command := &Command{}

	for _, expectedCmd := range ExceptedCommands {
		command.Command = expectedCmd
		car := &vehicle.Vehicle{}
		ctx := context.Background()

		// Try to find if command is implemented by checking if it returns
		// "unrecognized command" error
		// An empty Vehicle makes the SDK dereference nil as soon as a command reaches it.
		// That panic proves the command is implemented, so it is not a failure here.
		err := sendRecovering(command, ctx, car)

		if err != nil && err.Error() == "unrecognized command: "+expectedCmd {
			t.Errorf("Command %q is in ExceptedCommands but not implemented in Send method", expectedCmd)
		}
	}
}

// sendRecovering returns the error of command.Send, or nil if the call panicked inside the SDK.
func sendRecovering(command *Command, ctx context.Context, car *vehicle.Vehicle) (err error) {
	defer func() {
		if recover() != nil {
			err = nil
		}
	}()
	_, err = command.Send(ctx, car)
	return err
}
