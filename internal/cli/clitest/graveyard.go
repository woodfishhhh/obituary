//go:build windows

package clitest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func CommittedRetryFixture(t *testing.T) (*core.Service, core.Burial, core.Burial) {
	return BurialRetryFixture(t, "committed")
}

func BurialRetryFixture(t *testing.T, mode string) (*core.Service, core.Burial, core.Burial) {
	t.Helper()
	IsolateRunner(t)
	repo, _ := CommandDir(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Now()
	svc.Now = func() time.Time { return clock }
	path := filepath.Join(repo, "payload.txt")
	var burials []core.Burial
	for _, content := range []string{"older", "newer"} {
		WriteFixture(t, path, content)
		rows, err := svc.Bury([]string{path}, false, "")
		if err != nil || core.GraveFailed(rows) {
			t.Fatal(rows, err)
		}
		burials = append(burials, *rows[0].Burial)
		clock = clock.Add(time.Minute)
	}
	if mode == "scan" {
		svc.FS = &onceRootFailureFS{FS: svc.FS, root: filepath.Join(svc.Store.Home, "graveyard")}
		return svc, burials[0], burials[1]
	}
	original := svc.FS
	var held windows.Handle
	t.Cleanup(func() {
		if held != 0 {
			windows.CloseHandle(held)
		}
	})
	svc.FS = restoreReceiptFS{FS: original, after: func() {
		p, _ := windows.UTF16PtrFromString(filepath.Dir(burials[1].Grave) + ".json")
		h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		held = h
	}}
	rows, err := svc.Resurrect([]string{fmt.Sprint(burials[1].ID)})
	if err != nil || core.GraveFailed(rows) || rows[0].Warning == "" {
		t.Fatal(rows, err)
	}
	svc.FS = original
	if mode == "manifest" {
		windows.CloseHandle(held)
		held = 0
		p, _ := windows.UTF16PtrFromString(filepath.Dir(burials[1].Grave) + ".json")
		var err error
		held, err = windows.CreateFile(p, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	return svc, burials[0], burials[1]
}

func AssertRetryPreservesBurials(t *testing.T, older, newer core.Burial, mode, message string, failed bool) {
	t.Helper()
	if !failed || !strings.Contains(message, "retry") {
		t.Fatalf("unverifiable retry selected an older burial: %s", message)
	}
	if mode == "manifest" && !strings.Contains(message, fmt.Sprintf("#%d", newer.ID)) {
		t.Fatalf("unverifiable newest receipt lost its ID: %s", message)
	}
	for _, b := range []core.Burial{older, newer} {
		if mode == "manifest" && b.ID == newer.ID {
			continue
		}
		if _, err := os.Stat(b.Grave); err != nil {
			t.Fatalf("retry changed an unselected grave #%d: %v", b.ID, err)
		}
	}
}

func AssertCommittedRetry(t *testing.T, svc *core.Service, older, newer core.Burial, rows []core.GraveResult) {
	t.Helper()
	st, err := svc.Store.ReadSnapshot()
	if err != nil || len(rows) != 1 || core.GraveFailed(rows) || rows[0].Burial == nil || rows[0].Burial.ID != newer.ID || rows[0].Action != core.ActionResurrected || rows[0].Path != newer.Path || len(st.Burials) != 1 || st.Burials[0].ID != older.ID {
		t.Fatalf("committed retry selected older active burial: %+v %v", rows, err)
	}
	data, err := os.ReadFile(older.Grave)
	if err != nil || string(data) != "older" {
		t.Fatalf("older grave changed: %s %v", data, err)
	}
}

func RestoreReceiptFixture(t *testing.T) (*core.Service, []string, func()) {
	t.Helper()
	IsolateRunner(t)
	repo, _ := CommandDir(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(repo, "a.txt"), filepath.Join(repo, "b.txt"), filepath.Join(repo, "c.txt")}
	for _, path := range paths {
		WriteFixture(t, path, "buried")
		if got, err := svc.Bury([]string{path}, false, ""); err != nil || core.GraveFailed(got) {
			t.Fatal(got, err)
		}
	}
	WriteFixture(t, paths[1], "new work")
	var locked windows.Handle
	original := svc.FS
	count := 0
	svc.FS = restoreReceiptFS{FS: original, after: func() {
		count++
		if count != 2 {
			return
		}
		name, err := windows.UTF16PtrFromString(filepath.Join(svc.Store.Home, "store.json"))
		if err != nil {
			t.Fatal(err)
		}
		locked, err = windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
	}}
	unlock := func() {
		if locked != 0 {
			windows.CloseHandle(locked)
			locked = 0
		}
		svc.FS = original
	}
	t.Cleanup(unlock)
	return svc, paths, unlock
}

func PurgingCommandFixture(t *testing.T) (*core.Service, core.Burial) {
	t.Helper()
	IsolateRunner(t)
	repo, _ := CommandDir(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "payload.txt")
	WriteFixture(t, path, "keep until purge can finish")
	rows, err := svc.Bury([]string{path}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatal(rows, err)
	}
	b := *rows[0].Burial
	p, _ := windows.UTF16PtrFromString(b.Grave)
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { windows.CloseHandle(h) })
	rows, err = svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
	st, readErr := svc.Store.ReadSnapshot()
	if err != nil || readErr != nil || !core.GraveFailed(rows) || len(st.Burials) != 1 || st.Burials[0].PurgeIntent == nil || st.Burials[0].NextPurgeAttempt.IsZero() || st.Burials[0].LastPurgeError == "" {
		t.Fatalf("fixture did not persist interrupted purge: %+v %+v %v %v", rows, st, err, readErr)
	}
	return svc, st.Burials[0]
}

func PendingCommandBurial(t *testing.T, tracked bool) (*core.Service, string, core.PendingGrave) {
	t.Helper()
	IsolateRunner(t)
	repo, store := CommandDir(t)
	path := filepath.Join(repo, "keep.txt")
	WriteFixture(t, path, "keep")
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	if tracked {
		rows, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
		if err != nil || core.Failed(rows) {
			t.Fatalf("add: %+v, %v", rows, err)
		}
	}
	blocked := filepath.Join(store.Home, "store.json.tmp")
	svc.FS = &failBurialCommitFS{FS: svc.FS, blocked: blocked}
	if _, err := svc.Bury([]string{path}, false, ""); err == nil {
		t.Fatal("commit fault not injected")
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	pending, err := svc.PendingGraves()
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending: %+v, %v", pending, err)
	}
	return svc, path, pending[0]
}

// Attempt real manifest recovery after the list's ledger read, at its first
// filesystem scan. A consistent view holds recovery off until that scan ends.
func RecoverDuringGraveyardRead(t *testing.T, svc *core.Service) func() {
	t.Helper()
	recovering := *svc
	root := filepath.Join(svc.Store.Home, "graveyard")
	attempted := false
	done := make(chan error, 1)
	svc.FS = approvalFaultFS{FS: svc.FS, beforeRealPath: func(path string) {
		if path != root || attempted {
			return
		}
		attempted = true
		concurrent := recovering
		concurrent.Store = concurrent.Store.WithDeadline(time.Now().Add(100 * time.Millisecond))
		_, err := concurrent.Sweep(false)
		done <- err
	}}
	return func() {
		t.Helper()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("concurrent recovery failed: %v", err)
			}
		default:
			t.Fatal("recovery interleaving was not exercised")
		}
		if _, err := recovering.Sweep(false); err != nil {
			t.Fatalf("recovery after the view failed: %v", err)
		}
	}
}

type restoreReceiptFS struct {
	core.FS
	after func()
}

type onceRootFailureFS struct {
	core.FS
	root   string
	failed bool
}

func (f *onceRootFailureFS) RealPath(path string) (string, error) {
	if !f.failed && strings.EqualFold(path, f.root) {
		f.failed = true
		return "", os.ErrPermission
	}
	return f.FS.RealPath(path)
}

func (fs restoreReceiptFS) MovePinnedInto(path string, want core.Identity, parent core.RestoreParent, name string) (string, error) {
	actual, err := fs.FS.MovePinnedInto(path, want, parent, name)
	if err == nil {
		fs.after()
	}
	return actual, err
}

type approvalFaultFS struct {
	core.FS
	beforeRealPath func(string)
	beforeIdentify func(string)
}

func (fs approvalFaultFS) RealPath(path string) (string, error) {
	if fs.beforeRealPath != nil {
		fs.beforeRealPath(path)
	}
	return fs.FS.RealPath(path)
}

func (fs approvalFaultFS) Identify(path string) (core.Identity, error) {
	if fs.beforeIdentify != nil {
		fs.beforeIdentify(path)
	}
	return fs.FS.Identify(path)
}

type failBurialCommitFS struct {
	core.FS
	blocked string
}

func (f *failBurialCommitFS) MovePinned(path string, want core.Identity, to string) error {
	if err := f.FS.MovePinned(path, want, to); err != nil {
		return err
	}
	if f.blocked != "" {
		blocked := f.blocked
		f.blocked = ""
		return os.Mkdir(blocked, 0o700) // only the following store commit fails
	}
	return nil
}
