package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

// Resident owns the long-lived Service. Menu actions and scheduled work share
// this object; CLI writers continue to use the existing ledger/sweep locks.
type Resident struct {
	Service  *core.Service
	Changed  func()
	mu       sync.Mutex
	watcher  *fsnotify.Watcher
	mutex    windows.Handle
	logger   *log.Logger
	errs     *errorLog
	lastTick time.Time
	// read is the ledger's modification time when the last step read it;
	// hot and graves are what it read, which the tray shows instead of
	// reading again.
	read   time.Time
	hot    *core.State
	graves *graves
	// stopFolderWatch stops the live obit watch policies.
	stopFolderWatch func()
	runErr          error
}

func daemonMutexName(home string) (*uint16, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	if real, err := filepath.EvalSymlinks(home); err == nil {
		home = real
	}
	sum := sha256.Sum256([]byte(strings.ToUpper(filepath.Clean(home))))
	return windows.UTF16PtrFromString(fmt.Sprintf(`Local\Obituary-obitd-%s-%x`, user.User.Sid.String(), sum[:16]))
}

// NewResident returns nil without error when this user's store is served already.
func NewResident(svc *core.Service, logger *log.Logger) (*Resident, error) {
	if err := os.MkdirAll(svc.Store.Home, 0o700); err != nil {
		return nil, err
	}
	name, err := daemonMutexName(svc.Store.Home)
	if err != nil {
		return nil, err
	}
	mutex, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(mutex)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	w, err := fsnotify.NewWatcher()
	if err == nil {
		err = w.Add(svc.Store.Home)
	}
	if err == nil {
		// A hook that cannot take store.lock queues its write here and leaves
		// the replay to this resident, so a new file must wake it.
		dir := svc.Store.ObservationDir()
		if err = os.MkdirAll(dir, 0o700); err == nil {
			err = w.Add(dir)
		}
	}
	if err != nil {
		if w != nil {
			w.Close()
		}
		windows.CloseHandle(mutex)
		return nil, err
	}
	if err := writeDaemonProcess(svc.Store.Home, false, nil); err != nil {
		w.Close()
		windows.CloseHandle(mutex)
		return nil, err
	}
	logger.Printf("obitd started (pid %d, home %s)", os.Getpid(), svc.Store.Home)
	r := resident(svc, logger)
	r.watcher, r.mutex = w, mutex
	r.stopFolderWatch = startFolderWatch(svc, logger)
	return r, nil
}

func resident(svc *core.Service, logger *log.Logger) *Resident {
	return &Resident{Service: svc, logger: logger,
		errs: &errorLog{logger: logger, last: map[string]string{}, at: map[string]time.Time{}}}
}

func (r *Resident) Close() {
	if r.stopFolderWatch != nil {
		r.stopFolderWatch()
	}
	err := r.watcher.Close()
	if writeErr := writeDaemonProcess(r.Service.Store.Home, true, errors.Join(r.runErr, err)); writeErr != nil {
		r.logger.Printf("shutdown receipt: %v", writeErr)
	}
	windows.CloseHandle(r.mutex)
}

// Serve is the headless resident entry, also used by process-level tests.
func Serve(svc *core.Service, logger *log.Logger) error {
	r, err := NewResident(svc, logger)
	if err != nil || r == nil {
		return err
	}
	defer r.Close()
	return r.Run(context.Background())
}

func (r *Resident) Run(ctx context.Context) (runErr error) {
	stopUpdates := startUpdateChecks(ctx, r.Service.Store.Home)
	defer stopUpdates()
	defer func() { r.runErr = runErr }()
	for {
		delay, stop, err := r.step(Interval)
		if err != nil || stop {
			return err
		}
		if r.Changed != nil {
			r.Changed()
		}
		if err := r.wait(ctx, delay); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
	}
}

// A burst of commits (an agent writing many files) becomes one step: wait for
// a quiet gap, but never hold a change back longer than settleMax.
var settleQuiet, settleMax = 300 * time.Millisecond, 2 * time.Second

// Watch the directory so ledger replacement commits remain observable.
// The OS blocks while there are no events.
func (r *Resident) wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	var quiet, latest <-chan time.Time
	settle := func() {
		quiet = time.After(settleQuiet)
		if latest == nil {
			latest = time.After(settleMax)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		case <-quiet:
			return nil
		case <-latest:
			return nil
		case err := <-r.watcher.Errors:
			// Lost events are no reason to stop: the step reads the ledger anyway.
			if !errors.Is(err, fsnotify.ErrEventOverflow) {
				return fmt.Errorf("watch data directory: %w", err)
			}
			settle()
		case event, ok := <-r.watcher.Events:
			if !ok {
				return errors.New("data directory watcher closed")
			}
			if r.wakes(event) {
				settle()
			}
		}
	}
}

// wakes ignores events for a ledger this resident has already read, such as
// its own commits during a tick, and its own removal of replayed writes.
func (r *Resident) wakes(event fsnotify.Event) bool {
	if filepath.Dir(event.Name) == r.Service.Store.ObservationDir() {
		return event.Has(fsnotify.Create)
	}
	switch filepath.Base(event.Name) {
	case "store.json":
		return !r.Service.Store.ModTime().Equal(r.read)
	case "obitd.stop", "obitd.paused", "update.json":
		return true
	}
	return false
}

func (r *Resident) step(interval time.Duration) (time.Duration, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	home := r.Service.Store.Home
	if _, err := os.Stat(stopPath(home)); err == nil {
		return 0, true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, false, err
	}
	paused, err := Paused(home)
	if err != nil {
		return 0, false, err
	}
	if paused {
		return interval, false, nil
	}
	state, ok := r.readState()
	if !ok {
		return interval, false, nil
	}
	next, has := r.nextDue(state)
	now := time.Now()
	worked := true
	switch {
	case r.lastTick.IsZero() || now.Sub(r.lastTick) >= interval || has && !next.After(now):
		r.tick()
		r.lastTick = now
	case r.Service.Store.HasDeferredObservations():
		// Hooks queue what they could not commit; absorb only that.
		if err := ReplayObservations(r.Service); err != nil {
			r.errs.report("replay", err)
		} else {
			r.errs.clear("replay")
		}
	default:
		worked = false
	}
	if worked {
		if state, ok = r.readState(); !ok {
			return interval, false, nil
		}
		next, has = r.nextDue(state)
	}
	delay := time.Until(r.lastTick.Add(interval))
	if has {
		delay = min(delay, time.Until(next))
	}
	// A competing sweep or a failed ledger read can leave a past due entry.
	// Its owner will emit a store event; avoid spinning while waiting for it.
	if delay <= 0 {
		delay = interval
		if has && next.After(now) {
			delay = time.Millisecond
		}
	}
	return delay, false, nil
}

// readState reads the hot ledger, and the graveyard snapshot only when the
// hot ledger names a new one: most commits never change the graveyard. It
// notes the modification time first, so a commit that lands during the read
// still wakes the next wait.
func (r *Resident) readState() (core.State, bool) {
	r.read = r.Service.Store.ModTime()
	state, err := r.Service.Store.ReadHot()
	if err == nil && (state.Graveyard == nil || r.graves == nil || *state.Graveyard != r.graves.ref) {
		if state, err = r.Service.Store.Read(); err == nil {
			r.graves = summarize(state)
		}
	}
	if err != nil {
		r.hot, r.graves = nil, nil
		r.errs.report("read store", err)
		return state, false
	}
	r.hot = &state
	return state, true
}

// nextDue adds the graveyard's next purge to what the hot ledger has due.
func (r *Resident) nextDue(hot core.State) (time.Time, bool) {
	next, has := hot.NextDue()
	if g := r.graves; g != nil && g.has && (!has || g.next.Before(next)) {
		return g.next, true
	}
	return next, has
}

func (r *Resident) tick() {
	outcomes, purged, err := Tick(r.Service)
	err = ProposalLogErrors(r.Service.Store.Home, r.logger, err)
	if err != nil {
		r.errs.report("tick", err)
	} else {
		r.errs.clear("tick")
	}
	for _, o := range outcomes {
		key := fmt.Sprintf("#%d", o.Item.ID)
		if o.Action == core.ActionBlocked || o.Action == core.ActionRetry {
			r.errs.report(key, fmt.Errorf("%s %s %s", o.Action, o.Path, o.Detail))
		} else {
			r.errs.clear(key)
			r.logger.Printf("#%d %s %s %s", o.Item.ID, o.Action, o.Path, o.Detail)
		}
	}
	for _, result := range purged {
		r.logger.Printf("%s %s %s", result.Ref, result.Action, result.Error)
	}
}

func serveLoop(svc *core.Service, logger *log.Logger, interval time.Duration, wait func(time.Duration)) error {
	r := resident(svc, logger)
	for {
		delay, stop, err := r.step(interval)
		if err != nil || stop {
			return err
		}
		wait(delay)
	}
}
