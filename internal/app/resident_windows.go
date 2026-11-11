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

// Watch the directory so ledger replacement commits remain observable.
// The OS blocks while there are no events.
func (r *Resident) wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		case err := <-r.watcher.Errors:
			return fmt.Errorf("watch data directory: %w", err)
		case event, ok := <-r.watcher.Events:
			if !ok {
				return errors.New("data directory watcher closed")
			}
			switch filepath.Base(event.Name) {
			case "store.json", "obitd.stop", "obitd.paused", "observations.jsonl":
				return nil
			}
		}
	}
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
	state, err := r.Service.Store.Read()
	if err != nil {
		r.errs.report("read store", err)
		return interval, false, nil
	}
	next, has := state.NextDue()
	now := time.Now()
	if r.lastTick.IsZero() || now.Sub(r.lastTick) >= interval || has && !next.After(now) || r.Service.Store.HasDeferredObservations() {
		r.tick()
		r.lastTick = now
		state, err = r.Service.Store.Read()
		if err != nil {
			r.errs.report("read store", err)
			return interval, false, nil
		}
		next, has = state.NextDue()
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

func (r *Resident) tick() {
	outcomes, purged, err := Tick(r.Service)
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
