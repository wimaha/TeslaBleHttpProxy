package control

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/fakecar"
	"github.com/wimaha/TeslaBleHttpProxy/internal/tesla/commands"
)

// Stopping, replacing and sharing the BLE connection: what a key change, body-controller-state
// and releaseAdapterWhenIdle rely on.

// newControl returns a BleControl for the cars without starting its loop.
func newControl(t *testing.T, connectionTimeout time.Duration, cars ...*fakecar.Car) *BleControl {
	t.Helper()
	byVin := map[string]*fakecar.Car{}
	for _, c := range cars {
		byVin[c.Vin] = c
	}
	bc := New(fakecar.PrivateKey(), func(ctx context.Context, vin string) (Car, bool, error) {
		c, ok := byVin[vin]
		if !ok {
			return nil, true, errors.New("vehicle is not in range")
		}
		car, retry, err := c.Dial(ctx, vin)
		if err != nil {
			return nil, retry, err
		}
		return car, false, nil
	})
	bc.Timing = Timing{LoopDelay: time.Millisecond, RetryDelay: time.Millisecond, ConnectionTimeout: connectionTimeout}
	bc.releaseAdapter = func() error { return nil }
	t.Cleanup(bc.Stop)
	return bc
}

// pushWait queues the command like a handler with wait=true and returns a channel that is
// closed once the command is answered.
func pushWait(bc *BleControl, command commands.Command) (*models.ApiResponse, <-chan struct{}) {
	wg := &sync.WaitGroup{}
	resp := &models.ApiResponse{Wait: wg, Ctx: context.Background()}
	command.Response = resp
	wg.Add(1)
	bc.Push(command)
	answered := make(chan struct{})
	go func() { wg.Wait(); close(answered) }()
	return resp, answered
}

func waitAnswered(t *testing.T, what string, answered <-chan struct{}) {
	t.Helper()
	select {
	case <-answered:
	case <-time.After(3 * time.Second):
		t.Fatalf("%s: no answer within 3s", what)
	}
}

func bodyControllerState(vin string) commands.Command {
	return commands.Command{Command: "body-controller-state", Domain: commands.Domain.VCSEC, Vin: vin}
}

func TestStopAnswersQueuedAndLaterCommands(t *testing.T) {
	car := fakecar.New(vin1)
	bc := newControl(t, time.Second, car)

	queued, queuedAnswered := pushWait(bc, commands.Command{Command: "flash_lights", Vin: vin1})
	bc.Stop()
	later, laterAnswered := pushWait(bc, commands.Command{Command: "flash_lights", Vin: vin1})
	waitAnswered(t, "push after Stop", laterAnswered)
	mustFail(t, later, "restarted")

	// The loop of a stopped control answers what was queued before and exits.
	go bc.Loop()
	waitAnswered(t, "queued before Stop", queuedAnswered)
	mustFail(t, queued, "restarted")
	if n := car.Count("Dial"); n != 0 {
		t.Errorf("Dial count = %d, want 0", n)
	}
}

func TestStopClosesOpenConnection(t *testing.T) {
	car := fakecar.New(vin1)
	bc := newControl(t, time.Minute, car)
	go bc.Loop()

	resp, answered := pushWait(bc, commands.Command{Command: "flash_lights", Vin: vin1})
	waitAnswered(t, "flash_lights", answered)
	mustSucceed(t, resp)

	bc.Stop()
	eventually(t, "connection closed after Stop", func() bool { return car.Count("Disconnect") == 1 })
}

// countedCar tracks how many connections are open at the same time.
type countedCar struct {
	*fakecar.Car
	open, maxOpen *atomic.Int32
}

func (c countedCar) Disconnect() {
	c.open.Add(-1)
	c.Car.Disconnect()
}

// countingDialer dials car and records the highest number of simultaneously open connections.
func countingDialer(car *fakecar.Car, open, maxOpen *atomic.Int32) Dialer {
	return func(ctx context.Context, vin string) (Car, bool, error) {
		c, retry, err := car.Dial(ctx, vin)
		if err != nil {
			return nil, retry, err
		}
		n := open.Add(1)
		for m := maxOpen.Load(); n > m && !maxOpen.CompareAndSwap(m, n); m = maxOpen.Load() {
		}
		return countedCar{Car: c, open: open, maxOpen: maxOpen}, false, nil
	}
}

// After a key change the new loop takes over without a second connection being open at the
// same time: the old loop closes its connection when stopped, the new one waits for that.
func TestReplacedLoopHandsOverConnection(t *testing.T) {
	car := fakecar.New(vin1)
	var open, maxOpen atomic.Int32
	old := newControl(t, time.Minute, car)
	old.dial = countingDialer(car, &open, &maxOpen)
	go old.Loop()
	resp, answered := pushWait(old, commands.Command{Command: "flash_lights", Vin: vin1})
	waitAnswered(t, "old loop", answered)
	mustSucceed(t, resp)

	replacement := newControl(t, time.Minute, car)
	replacement.dial = countingDialer(car, &open, &maxOpen)
	go replacement.Loop()
	resp, answered = pushWait(replacement, commands.Command{Command: "flash_lights", Vin: vin1})
	time.Sleep(20 * time.Millisecond) // the new loop is now waiting for the old connection
	old.Stop()

	waitAnswered(t, "new loop", answered)
	mustSucceed(t, resp)
	if n := car.Count("Dial"); n != 2 {
		t.Errorf("Dial count = %d, want 2 (one per loop)", n)
	}
	if n := maxOpen.Load(); n != 1 {
		t.Errorf("%d connections were open at the same time, want 1", n)
	}
}

func TestInstanceSwapIsRaceFree(t *testing.T) {
	previous := Replace(nil)
	t.Cleanup(func() { Replace(previous) })

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				bc := New(nil, nil)
				if old := Replace(bc); old != nil {
					old.Stop()
				}
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				// Read once, like the handlers; a stopped instance answers right away.
				if bc := Instance(); bc != nil {
					bc.Stop()
					bc.Push(commands.Command{Command: "flash_lights", Vin: vin1})
				}
			}
		}()
	}
	wg.Wait()
}

// body-controller-state goes through the queue: it reuses an open connection instead of
// opening a second one.
func TestBodyControllerStateUsesOpenConnection(t *testing.T) {
	car := fakecar.New(vin1)
	bc := newControl(t, time.Minute, car)
	go bc.Loop()

	resp, answered := pushWait(bc, commands.Command{Command: "flash_lights", Vin: vin1})
	waitAnswered(t, "flash_lights", answered)
	mustSucceed(t, resp)

	resp, answered = pushWait(bc, bodyControllerState(vin1))
	waitAnswered(t, "body-controller-state", answered)
	mustSucceed(t, resp)
	if len(resp.Response) == 0 {
		t.Error("body-controller-state returned no data")
	}
	if n := car.Count("Dial"); n != 1 {
		t.Errorf("Dial count = %d, want 1: body-controller-state must use the open connection", n)
	}
}

// A VCSEC-only connection (opened for body-controller-state) must not serve a command that
// needs Infotainment; that command reconnects and wakes the car as usual.
func TestInfotainmentCommandReconnectsAfterBodyControllerState(t *testing.T) {
	car := fakecar.New(vin1)
	car.Asleep = true
	bc := newControl(t, time.Minute, car)
	go bc.Loop()

	resp, answered := pushWait(bc, bodyControllerState(vin1))
	waitAnswered(t, "body-controller-state", answered)
	mustSucceed(t, resp)
	if n := car.Count("Wakeup"); n != 0 {
		t.Fatalf("Wakeup count = %d, want 0: body-controller-state must not wake the car", n)
	}

	resp, answered = pushWait(bc, commands.Command{Command: "charge_start", Vin: vin1, AutoWakeup: true})
	waitAnswered(t, "charge_start", answered)
	mustSucceed(t, resp)

	if n := car.Count("Dial"); n != 2 {
		t.Errorf("Dial count = %d, want 2 (reconnect for Infotainment)", n)
	}
	if n := car.Count("Wakeup"); n != 1 {
		t.Errorf("Wakeup count = %d, want 1", n)
	}
	if got := sessions(car); !reflect.DeepEqual(got[len(got)-1], withInfotainment) {
		t.Errorf("last session = %v, want %v", got[len(got)-1], withInfotainment)
	}
}

func TestAdapterReleasedOnlyWhileIdle(t *testing.T) {
	car := fakecar.New(vin1)
	bc := newControl(t, 50*time.Millisecond, car)
	bc.ReleaseAdapterWhenIdle = true
	var releases, whileConnected atomic.Int32
	bc.releaseAdapter = func() error {
		releases.Add(1)
		if car.Count("Dial") != car.Count("Disconnect") {
			whileConnected.Add(1)
		}
		return nil
	}
	go bc.Loop()

	eventually(t, "release before the first command", func() bool { return releases.Load() >= 1 })
	before := releases.Load()
	resp, answered := pushWait(bc, commands.Command{Command: "flash_lights", Vin: vin1})
	waitAnswered(t, "flash_lights", answered)
	mustSucceed(t, resp)
	eventually(t, "release after the connection closed", func() bool { return releases.Load() > before })

	if n := whileConnected.Load(); n != 0 {
		t.Errorf("adapter released %d times while a connection was open", n)
	}
}

func TestAdapterKeptByDefault(t *testing.T) {
	car := fakecar.New(vin1)
	bc := newControl(t, 20*time.Millisecond, car)
	var releases atomic.Int32
	bc.releaseAdapter = func() error { releases.Add(1); return nil }
	go bc.Loop()

	resp, answered := pushWait(bc, commands.Command{Command: "flash_lights", Vin: vin1})
	waitAnswered(t, "flash_lights", answered)
	mustSucceed(t, resp)
	eventually(t, "connection closed", func() bool { return car.Count("Disconnect") == 1 })
	time.Sleep(20 * time.Millisecond)

	if n := releases.Load(); n != 0 {
		t.Errorf("adapter released %d times without releaseAdapterWhenIdle", n)
	}
}

// An error that is not worth retrying must reach the caller instead of being reported as success.
func TestNonRetriedErrorIsReported(t *testing.T) {
	wait := &sync.WaitGroup{}
	wait.Add(1)
	cmd := &commands.Command{
		Command:  "no_such_command",
		Response: &models.ApiResponse{Wait: wait, Ctx: context.Background()},
	}

	retry, err, _ := (&BleControl{}).ExecuteCommand(nil, cmd, context.Background())

	if err == nil || cmd.Response.Result {
		t.Fatalf("err = %v, result = %v, want the error reported", err, cmd.Response.Result)
	}
	if retry != nil {
		t.Errorf("a non-retried error must not be re-run")
	}
	wait.Wait()
}

// After a key change while the car is out of range, the old loop must not keep Bluetooth busy
// with its connection attempts (SendKeysToVehicle would wait for it).
func TestStopEndsConnectionAttempt(t *testing.T) {
	bc := newControl(t, time.Second) // no car: every dial fails and is retried
	bc.Timing.RetryDelay = time.Minute
	go bc.Loop()

	resp, answered := pushWait(bc, commands.Command{Command: "flash_lights", Vin: vin1})
	time.Sleep(20 * time.Millisecond) // first attempt failed, now waiting to retry
	bc.Stop()

	waitAnswered(t, "command during connection attempt", answered)
	mustFail(t, resp, "restarted")
	if !lockBLE(time.Second) {
		t.Fatal("bleMu still held after Stop")
	}
	bleMu.Unlock()
}

// A full queue answers at once instead of blocking the caller, so Stop is never held up.
func TestFullQueueRejectsAndStopDoesNotHang(t *testing.T) {
	bc := newControl(t, time.Second, fakecar.New(vin1)) // loop not started: nothing drains the queue
	for i := 0; i < cap(bc.commandStack); i++ {
		bc.Push(commands.Command{Command: "flash_lights", Vin: vin1})
	}
	resp, answered := pushWait(bc, commands.Command{Command: "flash_lights", Vin: vin1})
	waitAnswered(t, "push into a full queue", answered)
	mustFail(t, resp, "queue is full")
	// Without a response (no wait=true) the caller learns it from the returned error.
	if err := bc.Push(commands.Command{Command: "flash_lights", Vin: vin1}); !errors.Is(err, errQueueFull) {
		t.Errorf("Push into a full queue = %v, want errQueueFull", err)
	}

	stopped := make(chan struct{})
	go func() { bc.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop blocked with a full queue")
	}
}

// A caller that gave up (body_controller_state after 15 s) does not reach the car later.
func TestCanceledRequestDoesNotReachCar(t *testing.T) {
	car := fakecar.New(vin1)
	bc := newControl(t, time.Second, car)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	resp := &models.ApiResponse{Wait: wg, Ctx: ctx}
	command := bodyControllerState(vin1)
	command.Response = resp
	bc.Push(command)
	go bc.Loop()

	answered := make(chan struct{})
	go func() { wg.Wait(); close(answered) }()
	waitAnswered(t, "canceled body-controller-state", answered)
	mustFail(t, resp, "canceled")
	if n := len(car.Calls()); n != 0 {
		t.Errorf("car got %v, want no contact", car.Methods())
	}
}

// RequestClose ends an idle connection right away instead of after the connection timeout.
func TestRequestCloseEndsIdleConnection(t *testing.T) {
	car := fakecar.New(vin1)
	bc := newControl(t, time.Minute, car)
	go bc.Loop()
	resp, answered := pushWait(bc, commands.Command{Command: "flash_lights", Vin: vin1})
	waitAnswered(t, "flash_lights", answered)
	mustSucceed(t, resp)

	bc.RequestClose()
	eventually(t, "connection closed on request", func() bool { return car.Count("Disconnect") == 1 })
	if !lockBLE(time.Second) {
		t.Fatal("bleMu still held after the connection closed")
	}
	bleMu.Unlock()
}

func TestLockBLEGivesUp(t *testing.T) {
	bleMu.Lock()
	defer bleMu.Unlock()
	start := time.Now()
	if lockBLE(100 * time.Millisecond) {
		t.Fatal("lockBLE succeeded while bleMu was held")
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("lockBLE took %s, want about 100ms", d)
	}
}
