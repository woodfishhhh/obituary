//go:build windows

package core_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func BenchmarkTickExpired(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				svc, st := benchmarkGraveyard(b, n)
				st.Items = nil
				for i := range st.Burials {
					st.Burials[i].PurgeAt = time.Now().Add(-time.Hour)
				}
				if err := svc.Store.Update(func(current *core.State) error { *current = st; return nil }); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
				_, rows, err := svc.Tick()
				b.StopTimer()
				if err != nil || len(rows) != n || core.GraveFailed(rows) {
					b.Fatalf("tick %d: %d outcomes, %v", n, len(rows), err)
				}
				st, err = svc.Store.ReadSnapshot()
				if err != nil || len(st.Burials) != 0 || st.Stats.Purged != int64(n) {
					b.Fatalf("incomplete purge: %v", err)
				}
			}
		})
	}
}

func TestTickPurgeBatchRequiresConfirmedIntents(t *testing.T) {
	core.RequireFsync(t)
	for _, mode := range []string{"write", "unconfirmed"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := dueBatch(t, 3)
			if _, err := f.svc.Sweep(false); err != nil {
				t.Fatal(err)
			}
			graves := f.burials()
			f.clock = f.clock.Add(8 * 24 * time.Hour)
			blocked := filepath.Join(f.svc.Store.Home, "store.json.tmp")
			if mode == "write" {
				if err := os.Mkdir(blocked, 0700); err != nil {
					t.Fatal(err)
				}
			}
			restore := core.SetReplacementSync(func(f *os.File) error { return errors.New("intent flush interrupted") })
			defer restore()
			_, rows, err := f.svc.Tick()
			if mode == "unconfirmed" {
				st, readErr := f.svc.Store.ReadSnapshot()
				if readErr != nil || len(st.Items) != 0 || len(st.PendingAudit) != 0 || len(st.Burials) != 3 {
					t.Fatalf("retry fixture contains unrelated writes: %+v %v", st, readErr)
				}
				for _, b := range st.Burials {
					if b.PurgeIntent == nil {
						t.Fatal("first unconfirmed commit did not publish intent")
					}
				}
				_, rows, err = f.svc.Tick()
			}
			restore()
			if mode == "write" {
				if err := os.Remove(blocked); err != nil {
					t.Fatal(err)
				}
			}
			if err == nil {
				t.Fatalf("unconfirmed purge batch reported success: %+v", rows)
			}
			for _, b := range graves {
				if !exists(b.Grave) {
					t.Fatalf("unconfirmed purge batch deleted content: %+v", rows)
				}
			}
			if _, rows, err := f.svc.Tick(); err != nil || core.GraveFailed(rows) || len(f.burials()) != 0 {
				t.Fatalf("confirmed intent retry: %+v %v", rows, err)
			}
		})
	}
}

func TestTickPurgeBatchRejectsReplacement(t *testing.T) {
	f, _ := dueBatch(t, 3)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	graves := f.burials()
	protected := graves[1].Grave
	changed := false
	restore := core.SetBeforePurgeDelete(f.svc, func() {
		if changed {
			return
		}
		changed = true
		if err := os.Rename(protected, protected+".saved"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(protected, []byte("replacement"), 0600); err != nil {
			t.Fatal(err)
		}
	})
	defer restore()
	f.clock = f.clock.Add(8 * 24 * time.Hour)
	_, rows, err := f.svc.Tick()
	data, readErr := os.ReadFile(protected)
	if err != nil || readErr != nil || len(rows) != 3 || len(f.burials()) != 1 || !exists(protected+".saved") || string(data) != "replacement" || exists(graves[0].Grave) || exists(graves[2].Grave) {
		t.Fatalf("purge batch deleted a replacement or blocked siblings: %+v %v", rows, err)
	}
}

func TestTickPurgeBatchAuditCapacity(t *testing.T) {
	f, _ := dueBatch(t, 3)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(8 * 24 * time.Hour)
	if err := f.svc.Store.Update(func(st *core.State) error {
		for range 999 {
			st.PendingAudit = append(st.PendingAudit, core.Event{Time: f.clock, Action: "fixture"})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	name, _ := windows.UTF16PtrFromString(filepath.Join(f.svc.Store.Home, "audit.jsonl"))
	h, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	_, rows, err := f.svc.Tick()
	st, readErr := f.svc.Store.ReadSnapshot()
	if err == nil || readErr != nil || len(st.PendingAudit) != 1000 || len(st.Burials) != 2 || st.Stats.Purged != 1 {
		t.Fatalf("purge batch exceeded audit capacity: %+v %v %v", rows, err, readErr)
	}
}

func TestTickPurgeBatchCommitFailure(t *testing.T) {
	f, _ := dueBatch(t, 3)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(8 * 24 * time.Hour)
	blocked := filepath.Join(f.svc.Store.Home, "store.json.tmp")
	deletes := 0
	restore := core.SetBeforePurgeDelete(f.svc, func() {
		deletes++
		if deletes == 3 {
			if err := os.Mkdir(blocked, 0700); err != nil {
				t.Fatal(err)
			}
		}
	})
	defer restore()
	_, rows, err := f.svc.Tick()
	restore()
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	st, readErr := f.svc.Store.ReadSnapshot()
	if err == nil || readErr != nil || deletes != 3 || len(st.Burials) != 3 || st.Stats.Purged != 0 {
		t.Fatalf("purge batch did not retain all deletion intents: %+v %v %v", rows, err, readErr)
	}
	for _, b := range st.Burials {
		if b.PurgeIntent == nil || exists(b.Grave) {
			t.Fatal("batch lost durable deletion intent")
		}
	}
	if _, _, err := f.svc.Tick(); err != nil {
		t.Fatal(err)
	}
	st, err = f.svc.Store.ReadSnapshot()
	if err != nil || len(st.Burials) != 0 || st.Stats.Purged != 3 || st.Stats.PurgedBytes != st.Stats.BuriedBytes {
		t.Fatalf("purge batch recovery lost accounting: %+v %v", st, err)
	}
}
