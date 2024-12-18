package control

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
)

// body-controller-state
func BodyControllerState(vin string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := &Command{
		Command: "body-controller-state",
		Domain:  Domain.VCSEC,
		Vin:     vin,
	}
	conn, car, _, err := BleControlInstance.tryConnectToVehicle(ctx, cmd)
	if err == nil {
		//Successful
		defer conn.Close()
		defer log.Debug("close connection (A)")
		defer car.Disconnect()
		defer log.Debug("disconnect vehicle (A)")

		_, err := BleControlInstance.executeCommand(car, cmd)
		if err != nil {
			return err
		}

		return nil
	} else {
		return err
	}
}
