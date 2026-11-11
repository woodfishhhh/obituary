//go:build windows

package core_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestAuditFailureRetainsTerminalRecoveryEvidence(t *testing.T) {
	for _, operation := range []string{"resurrect", "abandon", "recover-resurrect"} {
		t.Run(operation, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "payload.txt")
			actual := b.Path
			if operation != "resurrect" {
				_, release := interruptResurrection(t, f, b)
				release()
			}
			action := core.ActionResurrected
			if operation == "abandon" {
				action = core.ActionAbandoned
				actual = filepath.Join(f.dir, "elsewhere", "payload.txt")
				if err := os.Mkdir(filepath.Dir(actual), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(b.Path, actual); err != nil {
					t.Fatal(err)
				}
			}
			name, _ := windows.UTF16PtrFromString(filepath.Join(f.svc.Store.Home, "audit.jsonl"))
			h, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			release := func() {
				if h != 0 {
					windows.CloseHandle(h)
					h = 0
				}
			}
			t.Cleanup(release)
			switch operation {
			case "abandon":
				var rows []core.GraveResult
				rows, err = f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
				if len(rows) != 1 || rows[0].Action != action || core.GraveFailed(rows) {
					t.Fatalf("committed abandonment lost receipt: %+v %v", rows, err)
				}
			case "recover-resurrect":
				_, err = f.svc.ReadGraveyard()
			default:
				var rows []core.GraveResult
				rows, err = f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
				if len(rows) != 1 || rows[0].Action != action || core.GraveFailed(rows) {
					t.Fatalf("committed resurrection lost receipt: %+v %v", rows, err)
				}
			}
			if !core.OnlyAuditErrors(err) {
				t.Fatalf("fixture did not fail only audit: %v", err)
			}
			before, err := f.svc.Store.ReadSnapshot()
			manifest := filepath.Dir(b.Grave) + ".json"
			if err != nil || len(before.Burials) != 0 || len(before.RestoreCleanups) != 1 || !exists(manifest) || !exists(filepath.Dir(b.Grave)) {
				t.Fatalf("audit failure discarded terminal recovery evidence: cleanup=%d manifest=%v grave=%v err=%v", len(before.RestoreCleanups), exists(manifest), exists(filepath.Dir(b.Grave)), err)
			}
			release()
			// A previous interrupted append must not swallow the recovery event.
			if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "audit.jsonl"), []byte(`{"action":`), 0600); err != nil {
				t.Fatal(err)
			}
			view, err := f.svc.ReadGraveyard()
			after, readErr := f.svc.Store.ReadSnapshot()
			if err != nil || readErr != nil || len(view.Pending) != 0 || len(after.RestoreCleanups) != 0 || exists(manifest) || exists(filepath.Dir(b.Grave)) || before.Stats != after.Stats || read(t, actual) != "payload.txt" {
				t.Fatalf("audit retry failed to converge without changing outcome: %+v %v %v", view, err, readErr)
			}
			events, err := f.svc.Store.History(20)
			found := false
			for _, e := range events {
				found = found || e.ID == b.ID && e.Action == action && strings.Contains(e.Detail, "confirmed after an unconfirmed commit")
			}
			if err != nil || !found {
				t.Fatalf("audit failure lost eventual confirmation: %+v %v", events, err)
			}
		})
	}
}

func TestAuditInterruptedTail(t *testing.T) {
	for _, action := range []string{core.ActionBuried, core.ActionResurrected} {
		t.Run(action, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("payload.txt")
			var b core.Burial
			if action == core.ActionResurrected {
				b = buryOne(t, f, "payload.txt")
			}
			if err := os.MkdirAll(f.svc.Store.Home, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "audit.jsonl"), []byte(`{"action":`), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			if action == core.ActionBuried {
				_, err = f.svc.Bury([]string{path}, false, "")
			} else {
				_, err = f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			}
			if err != nil {
				t.Fatal(err)
			}
			events, err := f.svc.Store.History(100)
			if err != nil || len(events) != 1 || events[0].Action != action {
				t.Fatalf("interrupted tail swallowed committed audit: %+v %v", events, err)
			}
			if got := read(t, filepath.Join(f.svc.Store.Home, "audit.jsonl")); !strings.HasPrefix(got, "{\"action\":\n") {
				t.Fatalf("partial audit tail was not preserved: %q", got)
			}
		})
	}
}

func TestAuditBoundsSerializedLine(t *testing.T) {
	store := core.Store{Home: t.TempDir()}
	detail := strings.Repeat("\x00<&😀", 200000)
	if err := store.Record(core.Event{Action: core.ActionAbandoned, ID: 7, Path: "payload", Detail: detail}); err != nil {
		t.Fatal(err)
	}
	events, err := store.History(10)
	if err != nil || len(events) != 1 || events[0].ID != 7 || events[0].Path != "payload" || !strings.HasSuffix(events[0].Detail, " [truncated]") {
		t.Fatalf("audit writer produced an unreadable line: count=%d err=%v", len(events), err)
	}
	data, err := os.ReadFile(filepath.Join(store.Home, "audit.jsonl"))
	if err != nil || len(data) >= 1<<20 {
		t.Fatalf("audit serialized line exceeds reader limit: %d %v", len(data), err)
	}
	if err := store.Record(core.Event{Path: strings.Repeat("p", 1<<20)}); !core.OnlyAuditErrors(err) {
		t.Fatalf("oversized non-detail fields were written: %v", err)
	}
}

func TestConfirmedCommitRecoveryRetainsAudit(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	core.RequireFsync(t)
	for _, action := range []string{core.ActionBuried, core.ActionResurrected} {
		t.Run(action, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("payload.txt")
			var b core.Burial
			moved := false
			if action == core.ActionResurrected {
				b = buryOne(t, f, "payload.txt")
				f.svc.FS = restoreCommitFaultFS{FS: f.fs, after: func() { moved = true }}
			}
			step := core.SetBurialStep(f.svc, func(stage string) error { moved = moved || stage == "moved"; return nil })
			defer step()
			stop := core.SetReplacementSync(func(file *os.File) error {
				if moved && filepath.Base(file.Name()) == "store.json.tmp" {
					return errors.New("unconfirmed ledger flush")
				}
				return file.Sync()
			})
			defer stop()
			var err error
			if action == core.ActionBuried {
				_, err = f.svc.Bury([]string{path}, false, "")
				b = f.burials()[0]
			} else {
				_, err = f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			}
			stop()
			step()
			if err == nil || !strings.Contains(err.Error(), "commit unconfirmed") {
				t.Fatalf("fixture did not interrupt the commit: %v", err)
			}
			before, err := f.svc.Store.ReadSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if action == core.ActionBuried {
				_, err = f.svc.Bury([]string{path}, false, "")
			} else {
				_, err = f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			}
			if err != nil {
				t.Fatal(err)
			}
			events, err := f.svc.Store.History(100)
			found := false
			for _, event := range events {
				found = found || event.Action == action && event.ID == b.ID && samePath(event.Path, path) && strings.Contains(event.Detail, "confirmed after an unconfirmed commit")
			}
			if err != nil || !found {
				t.Fatalf("confirmed recovery lost the operation audit: %+v %v", events, err)
			}
			after, err := f.svc.Store.ReadSnapshot()
			if err != nil || after.Stats.Buried != before.Stats.Buried || after.Stats.Resurrected != before.Stats.Resurrected {
				t.Fatalf("recovery audit counted an operation twice: %+v %+v %v", before.Stats, after.Stats, err)
			}
		})
	}
}

func TestCommittedAuditPreservesHookDeadline(t *testing.T) {
	store := core.Store{Home: t.TempDir()}
	release, err := core.LockFile(filepath.Join(store.Home, "audit.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	done := make(chan error, 1)
	go func() {
		done <- store.WithContext(ctx).WithDeadline(time.Now().Add(80 * time.Millisecond)).RecordCommitted(core.Event{Action: core.ActionAdded})
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			t.Fatalf("hook audit deadline changed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("committed audit discarded hook lock deadline")
	}
}
