//go:build windows

package core_test

import (
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

func TestTerminalAuditEventuallyRecorded(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, operation := range []string{"bury", "sweep", "purge", "empty-purge"} {
		for _, recover := range []string{"sweep", "tick", "graveyard"} {
			t.Run(operation+"/"+recover, func(t *testing.T) {
				f := newFixture(t)
				path := f.file("payload.txt")
				var b core.Burial
				action := core.ActionBuried
				if operation == "purge" || operation == "empty-purge" {
					b = buryOne(t, f, "payload.txt")
					action = core.ActionPurged
					if operation == "empty-purge" {
						if err := os.Remove(b.Grave); err != nil {
							t.Fatal(err)
						}
						action = core.ActionLost
					}
				} else {
					f.add(path, time.Hour)
					if operation == "sweep" {
						f.clock = f.clock.Add(time.Hour)
					}
				}
				p, _ := windows.UTF16PtrFromString(filepath.Join(f.svc.Store.Home, "audit.jsonl"))
				h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
				if err != nil {
					t.Fatal(err)
				}
				release := func() {
					if h != 0 {
						windows.CloseHandle(h)
						h = 0
					}
				}
				defer release()
				switch operation {
				case "bury":
					_, err = f.svc.Bury([]string{path}, false, "")
				case "sweep":
					_, err = f.svc.Sweep(false)
				default:
					_, err = f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
				}
				if err != nil && !core.OnlyAuditErrors(err) {
					t.Fatalf("expected only audit failure after commit: %v", err)
				}
				before, err := f.svc.Store.ReadSnapshot()
				if err != nil {
					t.Fatal(err)
				}
				if action == core.ActionBuried && (len(before.Burials) != 1 || exists(path)) || action != core.ActionBuried && (len(before.Burials) != 0 || exists(b.Grave)) {
					t.Fatal("business operation did not commit")
				}
				release()
				for range 2 {
					switch recover {
					case "sweep":
						_, err = f.svc.Sweep(false)
					case "tick":
						_, _, err = f.svc.Tick()
					default:
						_, err = f.svc.ReadGraveyard()
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				events, err := f.svc.Store.History(100)
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, e := range events {
					if e.Action == action {
						count++
					}
				}
				if count != 1 {
					t.Fatalf("terminal audit was lost or replayed without a crash: action=%s count=%d", action, count)
				}
				after, err := f.svc.Store.ReadSnapshot()
				if err != nil || after.Stats != before.Stats {
					t.Fatalf("audit delivery repeated accounting: %+v %+v %v", before.Stats, after.Stats, err)
				}
			})
		}
	}
}

func TestAuditBacklogRefusesNewOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, operation := range []string{"bury", "purge", "sweep", "resurrect"} {
		t.Run(operation, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "buried.txt")
			path := f.file("new.txt")
			if operation == "sweep" {
				f.add(path, time.Minute)
				f.clock = f.clock.Add(time.Minute)
			}
			if err := f.svc.Store.Update(func(st *core.State) error {
				for range 1000 {
					st.PendingAudit = append(st.PendingAudit, core.Event{Time: f.clock, Action: "fixture", ID: 1, Path: path})
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			p, _ := windows.UTF16PtrFromString(filepath.Join(f.svc.Store.Home, "audit.jsonl"))
			h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			unlock := func() {
				if h != 0 {
					windows.CloseHandle(h)
					h = 0
				}
			}
			defer unlock()
			run := func() error {
				var err error
				switch operation {
				case "bury":
					_, err = f.svc.Bury([]string{path}, false, "")
				case "purge":
					_, err = f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
				case "resurrect":
					_, err = f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
				case "sweep":
					_, err = f.svc.Sweep(false)
				}
				return err
			}
			err = run()
			if err == nil || !strings.Contains(err.Error(), "audit backlog is full") || !exists(path) || !exists(b.Grave) {
				t.Fatalf("full audit backlog permitted a new move/delete: %v new=%v grave=%v", err, exists(path), exists(b.Grave))
			}
			st, err := f.svc.Store.ReadSnapshot()
			if err != nil || len(st.PendingAudit) != 1000 {
				t.Fatalf("backlog dropped or grew events: %d %v", len(st.PendingAudit), err)
			}
			unlock()
			if _, err := f.svc.ReadGraveyard(); err != nil {
				t.Fatal(err)
			}
			if err := run(); err != nil {
				t.Fatalf("operation did not resume after audit delivery: %v", err)
			}
		})
	}
}

func TestEmptyAuditBatchDoesNotWaitForOutbox(t *testing.T) {
	f := newFixture(t)
	if err := f.svc.Store.Update(func(*core.State) error { return nil }); err != nil {
		t.Fatal(err)
	}
	release, err := core.LockFile(filepath.Join(f.svc.Store.Home, "audit-outbox.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	f.svc.Store = f.svc.Store.WithDeadline(time.Now().Add(150 * time.Millisecond))
	if _, _, err := f.svc.Tick(); err != nil {
		t.Fatalf("empty outbox waited for delivery lock: %v", err)
	}
}

func TestAuditBatchFlushesOnEarlyReturn(t *testing.T) {
	f := newFixture(t)
	first, second := f.file("first.txt"), f.file("second.txt")
	started := 0
	stop := core.SetBurialStep(f.svc, func(stage string) error {
		if stage != "reserved" {
			return nil
		}
		started++
		if started != 2 {
			return nil
		}
		events, err := f.svc.Store.History(10)
		if err != nil || len(events) != 0 {
			t.Fatalf("batch delivered audit before operation returned: %+v %v", events, err)
		}
		return errors.New("second burial interrupted")
	})
	defer stop()
	rows, err := f.svc.Bury([]string{first, second}, false, "")
	if err == nil || len(rows) != 2 || rows[0].Action != core.ActionBuried || !exists(second) {
		t.Fatalf("early return fixture: %+v %v", rows, err)
	}
	events, err := f.svc.Store.History(10)
	if err != nil || len(events) != 1 || events[0].Path != first {
		t.Fatalf("early return lost committed batch audit: %+v %v", events, err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.PendingAudit) != 0 {
		t.Fatalf("batch did not finish delivery: %+v %v", st.PendingAudit, err)
	}
}

func BenchmarkAuditBatch(b *testing.B) {
	for _, operation := range []string{"bury", "purge", "sweep"} {
		b.Run(operation, func(b *testing.B) {
			for range b.N {
				b.StopTimer()
				root := b.TempDir()
				now := time.Now()
				svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "home")}, FS: &testFS{}, Now: func() time.Time { return now }}
				paths := make([]string, 100)
				for i := range paths {
					paths[i] = filepath.Join(root, fmt.Sprintf("payload-%03d.txt", i))
					if err := os.WriteFile(paths[i], []byte("payload"), 0600); err != nil {
						b.Fatal(err)
					}
				}
				switch operation {
				case "purge":
					rows, err := svc.Bury(paths, false, "")
					if err != nil || core.GraveFailed(rows) {
						b.Fatalf("setup bury: %+v %v", rows, err)
					}
				case "sweep":
					if _, err := svc.Add(paths, core.AddOptions{ExpiresAt: now.Add(time.Second)}); err != nil {
						b.Fatal(err)
					}
					now = now.Add(time.Second)
				}
				b.StartTimer()
				var err error
				switch operation {
				case "bury":
					var rows []core.GraveResult
					rows, err = svc.Bury(paths, false, "")
					if len(rows) != 100 || core.GraveFailed(rows) {
						b.Fatal("bury results", rows)
					}
				case "purge":
					var rows []core.GraveResult
					rows, err = svc.Purge(true, false)
					if len(rows) != 100 || core.GraveFailed(rows) {
						b.Fatal("purge results", rows)
					}
				case "sweep":
					var rows []core.Outcome
					rows, err = svc.Sweep(false)
					if len(rows) != 100 {
						b.Fatal("sweep count", len(rows))
					}
					for _, row := range rows {
						if row.Action != core.ActionBuried {
							b.Fatal("sweep result", row)
						}
					}
				}
				if err != nil {
					b.Fatal(err)
				}
				b.StopTimer()
			}
		})
	}
}
