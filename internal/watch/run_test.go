package watch_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/watch"
)

func TestRunDrainsEventsDuringAddAndClose(t *testing.T) {
	svc, _ := fixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	events := make(chan fsnotify.Event)
	errs := make(chan error)
	entered := make(chan struct{})
	release := make(chan struct{})
	add := func(string) error {
		close(entered)
		select {
		case events <- fsnotify.Event{Name: filepath.Join(svc.Store.Home, "store.json")}:
		case <-release:
		}
		return nil
	}
	closeWatcher := func() error {
		select {
		case events <- fsnotify.Event{}:
		case <-release:
		}
		close(events)
		close(errs)
		return nil
	}
	done := make(chan error, 1)
	go func() {
		done <- watch.RunWithSource(ctx, svc, add, func(string) error { return nil }, closeWatcher, events, errs)
	}()
	<-entered
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(250 * time.Millisecond):
		close(release)
		<-done
		t.Fatal("event reader and Add/Close wait on each other during cancellation")
	}
}

func TestRunDiscoversEventsAndPolicyChangesWithoutTick(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	svc, dir := fixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	done := make(chan error, 1)
	errs := make(chan error, 32)
	go func() {
		done <- watch.Run(ctx, svc, time.Hour, func(err error) {
			select {
			case errs <- err:
			default:
			}
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("watch did not stop after cancellation")
		}
	})
	if _, err := svc.SetWatch(dir, "30d", core.WatchFirstSeen); err != nil {
		t.Fatal(err)
	}
	approvePolicies(t, svc)
	write(t, filepath.Join(dir, "ready.txt"))
	waitItems(t, ctx, svc, 1)
	write(t, filepath.Join(dir, "event.txt"))
	waitItems(t, ctx, svc, 2)
	// ReadDirectoryChangesW and policy changes, not the hour-long scan timer,
	// must discover this directory and then its next file.
	second := filepath.Join(filepath.Dir(dir), "second")
	write(t, filepath.Join(second, "existing.txt"))
	if _, err := svc.SetWatch(second, "1d", core.WatchFirstSeen); err != nil {
		t.Fatal(err)
	}
	approvePolicies(t, svc)
	waitItems(t, ctx, svc, 3)
	write(t, filepath.Join(second, "event.txt"))
	waitItems(t, ctx, svc, 4)
	if err := svc.RemoveWatch(dir); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "after-removal.txt"))
	waitItems(t, ctx, svc, 2)
	select {
	case err := <-errs:
		t.Fatal(err)
	default:
	}
}

func waitItems(t *testing.T, ctx context.Context, svc *core.Service, count int) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		st, err := svc.Store.Read()
		if err != nil {
			t.Fatal(err)
		}
		if len(st.Items) == count {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("expected %d registrations, got %d: %v", count, len(st.Items), ctx.Err())
		case <-ticker.C:
		}
	}
}
