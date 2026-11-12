//go:build windows

package lifecycle_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"golang.org/x/sys/windows"
)

func TestRestoreBatchStopsOnLedgerFailure(t *testing.T) {
	for _, phase := range []string{"read", "commit", "recover", "confirm"} {
		t.Run(phase, func(t *testing.T) {
			f := newFixture(t)
			first := buryOne(t, f, "first.txt")
			second := buryOne(t, f, "second.txt")
			store := filepath.Join(f.svc.Store.Home, "store.json")
			switch phase {
			case "read":
				if err := os.WriteFile(store, []byte("{broken"), 0600); err != nil {
					t.Fatal(err)
				}
			case "commit":
				f.svc.FS = restoreCommitFaultFS{FS: f.fs, after: func() { holdManifest(t, store) }}
			case "recover", "confirm":
				_, release := interruptResurrection(t, f, first)
				release()
				if phase == "confirm" {
					holdManifest(t, filepath.Dir(first.Grave)+".json")
					if _, err := f.svc.ReadGraveyard(); err != nil {
						t.Fatal(err)
					}
				}
				holdManifest(t, store)
			}
			refs := []string{fmt.Sprint(first.ID), fmt.Sprint(second.ID)}
			rows, err := f.svc.Resurrect(refs)
			if err == nil || len(rows) != 2 || rows[0].Ref != refs[0] || rows[0].Error == "" || rows[1].Ref != refs[1] || rows[1].Action != core.ActionNotExecuted || rows[1].Error == "" || !exists(second.Grave) || exists(second.Path) {
				t.Fatalf("ledger failure lost batch boundary: %+v %v", rows, err)
			}
			if phase == "commit" || phase == "recover" {
				if rows[0].Action != core.ActionResurrected || rows[0].Path != first.Path || read(t, rows[0].Path) != "first.txt" || !exists(filepath.Dir(first.Grave)+".json") {
					t.Fatalf("failed commit lost moved-object receipt: %+v %v", rows, err)
				}
			}
			if phase == "read" && (!exists(first.Grave) || rows[0].Action != "") {
				t.Fatalf("failed read changed first burial: %+v", rows)
			}
		})
	}
}

type restoreIdentifyFS struct {
	core.FS
	path  string
	after func() error
}

func (f restoreIdentifyFS) Identify(path string) (core.Identity, error) {
	id, err := f.FS.Identify(path)
	if err == nil && samePath(path, f.path) {
		err = f.after()
	}
	return id, err
}

func TestRestoreBatchContinuesAfterItemFailure(t *testing.T) {
	for _, failure := range []string{"invalid-ref", "missing-ref", "verify-missing", "protected", "recorded-manifest", "intent-write", "move"} {
		t.Run(failure, func(t *testing.T) {
			f := newFixture(t)
			first := buryOne(t, f, "first.txt")
			second := buryOne(t, f, "second.txt")
			ref := fmt.Sprint(first.ID)
			switch failure {
			case "invalid-ref":
				ref = "#99999999999999999999"
			case "missing-ref":
				ref = "#999"
			case "verify-missing":
				f.svc.FS = restoreIdentifyFS{FS: f.fs, path: first.Grave, after: func() error { return core.ErrNotFound }}
			case "protected":
				f.svc.Guard.Keep = append(f.svc.Guard.Keep, first.Path)
			case "recorded-manifest":
				if err := os.WriteFile(filepath.Dir(first.Grave)+".json", []byte("{broken"), 0600); err != nil {
					t.Fatal(err)
				}
			case "intent-write":
				if err := os.Mkdir(filepath.Dir(first.Grave)+".json.tmp", 0700); err != nil {
					t.Fatal(err)
				}
			case "move":
				f.fs.onRecycle = func(path string) {
					f.fs.fail = nil
					if samePath(path, first.Grave) {
						f.fs.fail = errors.New("restore move busy")
					}
				}
			}
			rows, err := f.svc.Resurrect([]string{ref, fmt.Sprint(second.ID)})
			if err != nil || len(rows) != 2 || rows[0].Ref != ref || rows[0].Error == "" || rows[1].Action != core.ActionResurrected || rows[1].Error != "" || !exists(first.Grave) || exists(first.Path) || read(t, second.Path) != "second.txt" {
				t.Fatalf("item failure lost error or blocked later reference: %+v %v", rows, err)
			}
			if failure == "verify-missing" && !strings.Contains(rows[0].Error, "empty (removed outside Obituary)") {
				t.Fatalf("missing grave lost diagnostic: %+v", rows[0])
			}
		})
	}
}

func TestRestoreCancellationBoundaries(t *testing.T) {
	for _, phase := range []string{"before-write", "after-move"} {
		t.Run(phase, func(t *testing.T) {
			f := newFixture(t)
			first := buryOne(t, f, "first.txt")
			second := buryOne(t, f, "second.txt")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if phase == "before-write" {
				f.svc.FS = restoreIdentifyFS{FS: f.fs, path: first.Grave, after: func() error { cancel(); return nil }}
			} else {
				f.svc.FS = restoreCommitFaultFS{FS: f.fs, after: cancel}
			}
			rows, err := f.svc.WithContext(ctx).Resurrect([]string{fmt.Sprint(first.ID), fmt.Sprint(second.ID), "#999"})
			if !errors.Is(err, context.Canceled) || len(rows) != 3 || rows[2].Action != core.ActionNotExecuted || rows[2].Error == "" || !exists(second.Grave) {
				t.Fatalf("cancellation lost batch boundary: %+v %v", rows, err)
			}
			if phase == "before-write" {
				if rows[0].Error == "" || rows[1].Action != core.ActionNotExecuted || !exists(first.Grave) || exists(first.Path) || exists(filepath.Dir(first.Grave)+".json") {
					t.Fatalf("canceled resurrection performed a write: %+v %v", rows, err)
				}
				return
			}
			st, readErr := f.svc.Store.ReadSnapshot()
			if readErr != nil || rows[0].Action != core.ActionResurrected || rows[0].Error != "" || rows[1].Error == "" || rows[1].Action != "" || st.BurialIndex(first.ID) >= 0 || st.Stats.Resurrected != 1 || read(t, first.Path) != "first.txt" {
				t.Fatalf("cancellation abandoned a moved-object commit: %+v %v %v", rows, err, readErr)
			}
		})
	}
}

func TestRestoreRecoveryReceiptsContinueBatch(t *testing.T) {
	for _, phase := range []string{"ready", "unverifiable", "committed"} {
		t.Run(phase, func(t *testing.T) {
			f := newFixture(t)
			first := buryOne(t, f, "output/first.txt")
			second := buryOne(t, f, "second.txt")
			if phase == "ready" {
				f.fs.fail = errors.New("busy")
				f.svc.Resurrect([]string{fmt.Sprint(first.ID)})
				f.fs.fail = nil
			} else {
				_, release := interruptResurrection(t, f, first)
				release()
			}
			if phase == "unverifiable" {
				if err := os.Rename(first.Path, filepath.Join(f.dir, "elsewhere.txt")); err != nil {
					t.Fatal(err)
				}
			} else {
				holdManifest(t, filepath.Dir(first.Grave)+".json")
			}
			if phase == "committed" {
				if _, err := f.svc.ReadGraveyard(); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := f.svc.Resurrect([]string{fmt.Sprint(first.ID), fmt.Sprint(second.ID)})
			if err != nil || len(rows) != 2 || rows[0].Burial == nil || rows[0].Burial.ID != first.ID || rows[1].Action != core.ActionResurrected || rows[1].Error != "" || read(t, second.Path) != "second.txt" {
				t.Fatalf("recovery lost identity or aborted unrelated reference: %+v %v", rows, err)
			}
			switch phase {
			case "ready":
				if rows[0].Action != core.ActionBlocked || rows[0].Error == "" || rows[0].Warning != rows[0].Error || !exists(first.Grave) {
					t.Fatalf("restore-ready lost blocked warning receipt: %+v", rows)
				}
			case "unverifiable":
				if rows[0].Error == "" || rows[0].Action != "" || read(t, filepath.Join(f.dir, "elsewhere.txt")) != "output/first.txt" {
					t.Fatalf("unverifiable intent lost error receipt: %+v", rows)
				}
			case "committed":
				if rows[0].Action != core.ActionResurrected || rows[0].Error != "" || rows[0].Warning == "" || rows[0].Path != first.Path {
					t.Fatalf("committed restore lost warning receipt: %+v", rows)
				}
			}
		})
	}
}

func TestResurrectRetainsIntentAfterPostRenameSyncFailure(t *testing.T) {
	core.RequireFsync(t)
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	moved := false
	f.svc.FS = restoreCommitFaultFS{FS: f.fs, after: func() { moved = true }}
	stop := core.SetReplacementSync(func(file *os.File) error {
		if moved && filepath.Base(file.Name()) == "store.json.tmp" {
			return errors.New("injected post-rename sync failure")
		}
		return file.Sync()
	})
	defer stop()
	for range 2 {
		rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
		if err == nil || !core.GraveFailed(rows) || !exists(filepath.Dir(b.Grave)+".json") {
			t.Fatalf("restore discarded unconfirmed commit evidence: %+v %v", rows, err)
		}
	}
	stop()
	rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
	if err != nil || core.GraveFailed(rows) || read(t, b.Path) != "payload.txt" {
		t.Fatalf("restore did not confirm after flush recovered: %+v %v", rows, err)
	}
}

type restoreCommitFaultFS struct {
	core.FS
	after func()
}

func (fs restoreCommitFaultFS) MovePinnedInto(path string, want core.Identity, parent core.RestoreParent, name string) (string, error) {
	actual, err := fs.FS.MovePinnedInto(path, want, parent, name)
	if err == nil {
		fs.after()
	}
	return actual, err
}

func interruptResurrection(t *testing.T, f *fixture, b core.Burial) ([]core.GraveResult, func()) {
	t.Helper()
	var h windows.Handle
	f.svc.FS = restoreCommitFaultFS{FS: f.fs, after: func() {
		p, err := windows.UTF16PtrFromString(filepath.Join(f.svc.Store.Home, "store.json"))
		if err != nil {
			t.Fatal(err)
		}
		h, err = windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
	}}
	unlock := func() {
		if h != 0 {
			windows.CloseHandle(h)
			h = 0
		}
		f.svc.FS = f.fs
	}
	t.Cleanup(unlock)
	got, err := f.svc.Resurrect([]string{b.Path})
	if err == nil || !strings.Contains(err.Error(), "replace store") || len(got) != 1 || got[0].Path == "" || read(t, got[0].Path) != filepath.ToSlash(strings.TrimPrefix(b.Path, f.dir+string(os.PathSeparator))) {
		t.Fatalf("restored path lost after ledger failure: %+v %v", got, err)
	}
	return got, unlock
}

func TestResurrectRecoversFailedCommit(t *testing.T) {
	for _, retry := range []string{"resurrect", "sweep", "graveyard"} {
		t.Run(retry, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "output/payload.txt")
			first, unlock := interruptResurrection(t, f, b)
			if len(f.burials()) != 1 {
				t.Fatal("failed commit lost burial")
			}
			unlock()
			var actual string
			switch retry {
			case "resurrect":
				got, err := f.svc.Resurrect([]string{b.Path})
				if err != nil || core.GraveFailed(got) || len(got) != 1 {
					t.Fatalf("resurrection intent did not recover: %+v %v", got, err)
				}
				actual = got[0].Path
			case "sweep":
				got, err := f.svc.Sweep(false)
				if err != nil || len(got) != 1 || got[0].Action != core.ActionResurrected {
					t.Fatalf("resurrection intent did not recover: %+v %v", got, err)
				}
				actual = got[0].Path
			case "graveyard":
				view, err := f.svc.ReadGraveyard()
				if err != nil || view == nil || len(view.Burials) != 0 || len(view.Pending) != 0 {
					t.Fatalf("resurrection intent did not recover: %+v %v", view, err)
				}
				actual = first[0].Path
			}
			st, err := f.svc.Store.Read()
			if err != nil || len(st.Burials) != 0 || st.Stats.Resurrected != 1 || st.Stats.ResurrectedBytes != b.Size || actual != first[0].Path {
				t.Fatalf("resurrection commit not reconciled: %+v %s %v", st, actual, err)
			}
			pending, err := f.svc.PendingGraves()
			if err != nil || len(pending) != 0 {
				t.Fatalf("intent not cleaned: %+v %v", pending, err)
			}
			events, err := f.svc.Store.History(20)
			count := 0
			for _, event := range events {
				if event.Action == core.ActionResurrected {
					count++
					if event.Path != actual {
						t.Fatal("recovery audit lost actual path")
					}
				}
			}
			if err != nil || count != 1 {
				t.Fatalf("recovery audit count=%d: %v", count, err)
			}
			f.add(actual, time.Hour)
			if _, err := f.svc.Sweep(false); err != nil {
				t.Fatal(err)
			}
			st, _ = f.svc.Store.Read()
			if st.Stats.Resurrected != 1 {
				t.Fatal("recovery counted resurrection twice")
			}
		})
	}
}

func TestResurrectRecoveryRequiresIdentity(t *testing.T) {
	for _, mode := range []string{"moved", "replacement", "renamed", "parent-replaced"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "output/payload.txt")
			first, unlock := interruptResurrection(t, f, b)
			unlock()
			actual := first[0].Path
			if mode == "parent-replaced" {
				parent := filepath.Dir(actual)
				if err := os.Rename(parent, parent+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(parent, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(parent+"-old", "payload.txt"), actual); err != nil {
					t.Fatal(err)
				}
			} else {
				to := filepath.Join(f.dir, "moved.txt")
				if mode == "renamed" {
					to = filepath.Join(filepath.Dir(actual), "renamed.txt")
				}
				if err := os.Rename(actual, to); err != nil {
					t.Fatal(err)
				}
				if mode == "replacement" {
					if err := os.WriteFile(actual, []byte("replacement"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "renamed" {
					actual = to
				}
			}
			got, err := f.svc.Resurrect([]string{b.Path})
			if mode == "renamed" {
				if err != nil || core.GraveFailed(got) || got[0].Path != actual || len(f.burials()) != 0 {
					t.Fatalf("same-parent renamed identity not recovered: %+v %v", got, err)
				}
				return
			}
			if err != nil || !core.GraveFailed(got) || len(f.burials()) != 1 {
				t.Fatalf("unverified resurrection was committed: %+v %v", got, err)
			}
			view, err := f.svc.ReadGraveyard()
			if err != nil || len(view.Pending) != 1 || view.Pending[0].Status != "unverifiable" {
				t.Fatalf("unverified resurrection intent disappeared: %+v %v", view, err)
			}
			if _, err := f.svc.Sweep(false); err != nil {
				t.Fatal(err)
			}
			st, _ := f.svc.Store.Read()
			if len(st.Burials) != 1 || st.Stats.Resurrected != 0 {
				t.Fatal("unverified resurrection was committed")
			}
		})
	}
}

func TestResurrectRefusesAGraveOutsideTheGraveyard(t *testing.T) {
	f := newFixture(t)
	buryOne(t, f, "a.log")
	precious := f.file("elsewhere/a.log")
	id, err := f.svc.FS.Identify(precious)
	if err != nil {
		t.Fatal(err)
	}
	err = f.svc.Store.Update(func(st *core.State) error {
		st.Burials[0].Grave, st.Burials[0].Identity = precious, &id // a tampered ledger
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := f.svc.Resurrect([]string{"#1"})
	if err != nil || results[0].Error == "" || !exists(precious) {
		t.Fatalf("resurrect must refuse a grave outside the graveyard, got %+v %v", results, err)
	}
}

func assertRestoreAdviceDoesNotLockLedger(t *testing.T, run func(*fixture, core.Burial) string) {
	t.Helper()
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	_, unlock := interruptResurrection(t, f, b)
	unlock()
	fs := pausedRestoreFS{FS: f.fs, entered: make(chan struct{}), release: make(chan struct{})}
	f.svc.FS = fs
	done := make(chan string, 1)
	go func() { done <- run(f, b) }()
	var advice string
	select {
	case <-fs.entered:
		_, err := f.svc.Store.WithDeadline(time.Now().Add(100 * time.Millisecond)).ReadSnapshot()
		close(fs.release)
		advice = <-done
		if err != nil {
			t.Fatalf("restore advice held ledger lock during identity search: %v", err)
		}
	case advice = <-done:
		close(fs.release)
	case <-time.After(3 * time.Second):
		close(fs.release)
		<-done
		t.Fatal("restore advice did not complete")
	}
	if !strings.Contains(advice, "obit resurrect ") || !strings.Contains(advice, "obit reap") || !strings.Contains(advice, "obit purge ") {
		t.Fatalf("unfinished restore lacks conservative advice: %s", advice)
	}
}
