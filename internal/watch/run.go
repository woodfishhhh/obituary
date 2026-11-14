package watch

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/woodfishhhh/obituary/internal/core"
)

// Run is the resident/tray entry point. The caller owns cancellation and
// logging (report may be nil). It never sweeps: app.Tick owns expiration.
// The interval provides full reconciliation after missed events or outages.
// Consume both channels to handle changes and watcher errors; close the watcher
// on exit. Windows uses fsnotify's ReadDirectoryChangesW backend.
func Run(ctx context.Context, svc *core.Service, interval time.Duration, report func(error)) error {
	if interval <= 0 {
		return fmt.Errorf("watch reconciliation interval must be positive")
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	return run(ctx, svc, interval, report, watchSource{w.Add, w.Remove, w.Close, w.Events, w.Errors})
}

type watchSource struct {
	Add    func(string) error
	Remove func(string) error
	Close  func() error
	Events <-chan fsnotify.Event
	Errors <-chan error
}

func run(ctx context.Context, svc *core.Service, interval time.Duration, report func(error), w watchSource) error {
	svc = svc.WithContext(ctx)
	r := &resident{svc: svc, watcher: w, policies: make(map[int64]core.WatchPolicy), report: report}
	ledger, folders, failures, drained := r.drain()
	defer func() { _ = w.Close(); <-drained }()
	if _, err := svc.Watches(); err != nil {
		return err
	}
	if err := w.Add(svc.Store.Home); err != nil {
		return err
	}
	r.refresh(true)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	// Bursts settle into one refresh. A ledger commit only re-checks the
	// policies; a folder event (or lost events) rescans the folders.
	var quiet, latest <-chan time.Time
	all := false
	settle := func(folder bool) {
		all = all || folder
		quiet = time.After(settleQuiet)
		if latest == nil {
			latest = time.After(settleMax)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.refresh(true)
		case <-drained:
			return nil
		case err := <-failures:
			r.record(err)
			settle(true)
		case <-folders:
			settle(true)
		case <-ledger:
			settle(false)
		case <-quiet:
			r.refresh(all)
			quiet, latest, all = nil, nil, false
		case <-latest:
			r.refresh(all)
			quiet, latest, all = nil, nil, false
		}
	}
}

// settleQuiet and settleMax bound how long a burst of events is held back.
const settleQuiet, settleMax = 300 * time.Millisecond, 2 * time.Second

type resident struct {
	svc      *core.Service
	watcher  watchSource
	policies map[int64]core.WatchPolicy
	report   func(error)
}

func (r *resident) record(err error) {
	if err != nil && !errors.Is(err, context.Canceled) && r.report != nil {
		r.report(err)
	}
}

func (r *resident) refresh(all bool) {
	policies, err := r.svc.Watches()
	if err != nil {
		r.record(err)
		return
	}
	next := make(map[int64]core.WatchPolicy)
	for _, p := range policies {
		if p.Consent != core.ConsentApproved {
			continue
		}
		path, err := r.svc.WatchPath(p)
		if err != nil {
			r.record(err)
			continue
		}
		key := p.ID
		old, exists := r.policies[key]
		if !exists || old.Path != path {
			if err := r.watcher.Add(path); err != nil {
				r.record(err)
				continue
			}
		}
		next[key] = p
		if all || !exists || old.ID != p.ID || old.Rev != p.Rev {
			r.record(reconcileFolder(r.svc, p))
		}
	}
	for key, p := range r.policies {
		if _, keep := next[key]; !keep {
			err := r.watcher.Remove(p.Path)
			if !errors.Is(err, fsnotify.ErrNonExistentWatch) {
				r.record(err)
			}
		}
	}
	r.policies = next
}

// fsnotify/backend_windows.go sends events while servicing Add/Remove/Close.
// Keep draining independently through Close; one pending signal of each kind
// covers all coalesced events without blocking the backend or growing a queue.
func (r *resident) drain() (ledger, folders <-chan struct{}, failures <-chan error, done <-chan struct{}) {
	ledgerC, foldersC := make(chan struct{}, 1), make(chan struct{}, 1)
	failuresC, doneC := make(chan error, 1), make(chan struct{})
	signal := func(c chan struct{}) {
		select {
		case c <- struct{}{}:
		default:
		}
	}
	go func() {
		defer close(doneC)
		events, errs := r.watcher.Events, r.watcher.Errors
		for events != nil || errs != nil {
			select {
			case event, ok := <-events:
				switch {
				case !ok:
					events = nil
				case filepath.Dir(event.Name) != r.svc.Store.Home:
					signal(foldersC)
				case filepath.Base(event.Name) == "store.json":
					signal(ledgerC)
				}
			case err, ok := <-errs:
				if !ok {
					errs = nil
					continue
				}
				select {
				case failuresC <- err:
				default:
				}
			}
		}
	}()
	return ledgerC, foldersC, failuresC, doneC
}
