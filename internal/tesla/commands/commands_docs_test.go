package commands

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/fakecar"
)

// Commands in ExceptedCommands that are deliberately not listed under "Vehicle Commands":
// vehicle_data has its own endpoint and README section, session_info is for debugging only.
var undocumentedCommands = []string{"vehicle_data", "session_info"}

// readmeSection returns the lines of the README section that starts with heading, up to the
// next heading of the same or a higher level.
func readmeSection(t *testing.T, heading string) []string {
	t.Helper()
	data, err := os.ReadFile("../../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	level := heading[:strings.Index(heading, " ")+1]
	var section []string
	in := false
	for _, line := range strings.Split(string(data), "\n") {
		if in && strings.HasPrefix(line, "#") {
			if l := line[:strings.Index(line+" ", " ")+1]; len(l) <= len(level) {
				break
			}
		}
		if strings.TrimSpace(line) == heading {
			in = true
			continue
		}
		if in {
			section = append(section, line)
		}
	}
	if !in {
		t.Fatalf("README has no section %q", heading)
	}
	return section
}

// The command list in the README and ExceptedCommands must match in both directions.
func TestReadmeListsAllCommands(t *testing.T) {
	// The command list is the first bullet list of the section; later lists are notes.
	var documented []string
	for _, line := range readmeSection(t, "### Vehicle Commands") {
		if name, ok := strings.CutPrefix(line, "- "); ok {
			documented = append(documented, strings.Trim(strings.TrimSpace(name), "`"))
		} else if len(documented) > 0 && strings.TrimSpace(line) != "" {
			break
		}
	}
	if len(documented) == 0 {
		t.Fatal("no command list found under \"Vehicle Commands\"")
	}

	for _, cmd := range ExceptedCommands {
		if !slices.Contains(documented, cmd) && !slices.Contains(undocumentedCommands, cmd) {
			t.Errorf("command %q is supported but missing from the README list under \"Vehicle Commands\"", cmd)
		}
	}
	for _, cmd := range documented {
		if !slices.Contains(ExceptedCommands, cmd) {
			t.Errorf("README lists %q under \"Vehicle Commands\", but it is not in ExceptedCommands", cmd)
		}
	}
}

// Each command that needs a body must have an example request with body in the README.
// A command needs one if it fails without a body on a simulated car.
func TestReadmeHasExampleForCommandsWithBody(t *testing.T) {
	section := strings.Join(readmeSection(t, "### Vehicle Commands"), "\n")
	for _, cmd := range ExceptedCommands {
		if slices.Contains(undocumentedCommands, cmd) {
			continue
		}
		command := &Command{Command: cmd, Response: &models.ApiResponse{}}
		if _, err := command.Send(context.Background(), fakecar.New(testVin)); err == nil {
			continue
		}
		if !strings.Contains(section, "/command/"+cmd+"` with body") {
			t.Errorf("%q needs a body, but the README has no example request with body for it", cmd)
		}
	}
}

func TestReadmeMentionsAllVehicleDataEndpoints(t *testing.T) {
	section := strings.Join(readmeSection(t, "### Vehicle Data"), "\n")
	for _, endpoint := range ExceptedEndpoints {
		if !strings.Contains(section, endpoint) {
			t.Errorf("vehicle_data endpoint %q is supported but not mentioned in the README", endpoint)
		}
	}
}
