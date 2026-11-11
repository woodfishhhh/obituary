//go:build windows

package app

import (
	"bytes"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestServeIsDetectedOnceAndStopsOnRequest(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	svc := &core.Service{Store: core.Store{Home: home}, Guard: Guard(home), FS: winfs.FS{}}
	logger := log.New(io.Discard, "", 0)
	done := make(chan error, 1)
	go func() { done <- Serve(svc, logger) }()

	if err := waitFor(func() bool { return Running(home) }, 5*time.Second, "not running"); err != nil {
		t.Fatal(err)
	}
	if err := Serve(svc, logger); err != nil {
		t.Fatal("a second daemon should exit successfully without starting", err)
	}
	if !Running(home) {
		t.Fatal("probing must not disturb the running daemon")
	}
	if err := Stop(home); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after Stop")
	}
	if Running(home) {
		t.Fatal("still reported running after stop")
	}
	if _, err := os.Stat(stopPath(home)); !os.IsNotExist(err) {
		t.Fatalf("stop request left behind: %v", err)
	}
}

func TestStopClearsStaleRequest(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(stopPath(home), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Stop(home); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stopPath(home)); !os.IsNotExist(err) {
		t.Fatalf("stale request left behind: %v", err)
	}
}

func TestResidentDoesNotRetickBlockedBurialBeforeBackoff(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	fs := &unavailableRootFS{FS: winfs.FS{}, root: filepath.Join(home, "graveyard")}
	svc := &core.Service{Store: core.Store{Home: home}, FS: fs}
	if err := svc.Store.Update(func(st *core.State) error {
		st.Burials = []core.Burial{{ID: 1, Path: filepath.Join(home, "offline.txt"), Grave: filepath.Join(fs.root, "1-grave", "offline.txt"), PurgeAt: time.Now().Add(-time.Hour)}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	rounds := 0
	err := serveLoop(svc, log.New(&output, "", 0), 5*time.Minute, func(time.Duration) {
		rounds++
		if rounds == 3 {
			if err := os.WriteFile(stopPath(home), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err != nil || fs.scans != 1 || strings.Count(output.String(), "blocked") != 1 {
		t.Fatalf("blocked burial polled repeatedly: scans=%d log=%s err=%v", fs.scans, output.String(), err)
	}
	st, err := svc.Store.Read()
	if err != nil || st.Burials[0].PurgeAttempts != 1 || st.Burials[0].LastPurgeError == "" {
		t.Fatalf("burial retry not persisted: %+v %v", st.Burials, err)
	}
}

func TestBackgroundBusyTickIsNotAnError(t *testing.T) {
	t.Parallel()
	svc := &core.Service{Store: core.Store{Home: t.TempDir()}, FS: winfs.FS{}}
	release, ok, err := core.TryLockExclusive(filepath.Join(svc.Store.Home, "sweep.lock"))
	if err != nil || !ok {
		t.Fatalf("lock: %v %v", ok, err)
	}
	defer release()
	if _, _, err := Tick(svc); err != nil {
		t.Fatalf("busy worker logged as failure: %v", err)
	}
}

func TestResidentRecoversManifestsWithoutNextDue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "home")}, FS: winfs.FS{}}
	pending := func(name string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		blocked := filepath.Join(svc.Store.Home, "store.json.tmp")
		svc.FS = commitFailureFS{FS: winfs.FS{}, blocked: blocked}
		if _, err := svc.Bury([]string{path}, false, ""); err == nil {
			t.Fatal("commit failure not reached")
		}
		svc.FS = winfs.FS{}
		if err := os.Remove(blocked); err != nil {
			t.Fatal(err)
		}
	}
	pending("startup.txt")
	st, err := svc.Store.Read()
	if _, due := st.NextDue(); err != nil || due {
		t.Fatalf("fixture has a due time: %+v, %v", st, err)
	}
	rounds := 0
	err = serveLoop(svc, log.New(io.Discard, "", 0), time.Millisecond, func(time.Duration) {
		rounds++
		st, err := svc.Store.Read()
		if err != nil || len(st.Burials) != rounds || len(st.Items) != 0 {
			t.Errorf("loop round %d failed recovery without due items: %+v, %v", rounds, st, err)
		}
		if rounds == 1 {
			pending("periodic.txt")
			time.Sleep(5 * time.Millisecond)
		} else if err := os.WriteFile(stopPath(svc.Store.Home), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || rounds != 2 {
		t.Fatalf("resident loop: rounds=%d, %v", rounds, err)
	}
}

func TestResidentDoesNotRetickBlockedItemBeforeBackoff(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	fs := &unavailableRootFS{FS: winfs.FS{}, root: filepath.Join(home, "graveyard")}
	svc := &core.Service{Store: core.Store{Home: home}, FS: fs}
	if err := svc.Store.Update(func(st *core.State) error {
		st.Items = []core.Item{{ID: 1, Rev: 1, Path: filepath.Join(home, "offline.txt"), ExpiresAt: time.Now().Add(-time.Hour)}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	rounds := 0
	err := serveLoop(svc, log.New(&output, "", 0), 5*time.Minute, func(time.Duration) {
		rounds++
		if rounds == 3 {
			if err := os.WriteFile(stopPath(home), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err != nil || fs.scans != 1 || strings.Count(output.String(), "blocked") != 1 {
		t.Fatalf("blocked entry polled repeatedly: scans=%d log=%s err=%v", fs.scans, output.String(), err)
	}
}

func TestResidentDeduplicatesBlockedOutcomes(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	fs := &unavailableRootFS{FS: winfs.FS{}, root: filepath.Join(home, "graveyard")}
	svc := &core.Service{Store: core.Store{Home: home}, FS: fs}
	var output bytes.Buffer
	rounds := 0
	err := serveLoop(svc, log.New(&output, "", 0), time.Nanosecond, func(time.Duration) {
		rounds++
		if rounds == 3 {
			if err := os.WriteFile(stopPath(home), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err != nil || fs.scans != 3 || strings.Count(output.String(), "blocked") != 1 {
		t.Fatalf("unchanged recovery warning flooded log: scans=%d log=%s err=%v", fs.scans, output.String(), err)
	}
}

func TestBackgroundTickReplaysQueuedWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	root, home := t.TempDir(), t.TempDir()
	testenv.InitRepo(t, root)
	path := filepath.Join(root, "debug.log")
	if err := os.WriteFile(path, []byte("queued"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := &core.Service{Store: core.Store{Home: home}, FS: winfs.FS{}}
	if err := svc.Store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(root): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	queued, err := svc.CaptureObservations([]string{path}, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.DeferObservations(queued); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Tick(svc); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Items) != 1 || st.Items[0].Session != "s1" || svc.Store.HasDeferredObservations() {
		t.Fatalf("background did not drain queue: %+v, %v", st.Items, err)
	}
}
