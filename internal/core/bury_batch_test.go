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
)

func TestBuryBatchInterruptedMovesRecoverOnce(t *testing.T) {
	f := newFixture(t)
	paths := []string{f.file("first"), f.file("second"), f.file("untouched")}
	stop := errors.New("interrupted second move")
	moves := 0
	restore := core.SetBurialStep(f.svc, func(step string) error {
		if step == "moved" {
			moves++
			if moves == 2 {
				return stop
			}
		}
		return nil
	})
	rows, err := f.svc.BuryDeletion(paths, false, "rm")
	restore()
	if !errors.Is(err, stop) || len(rows) != 3 || !exists(paths[2]) {
		t.Fatalf("interruption lost receipts or untouched input: %+v %v", rows, err)
	}
	for i := range 2 {
		if rows[i].Action != core.ActionRecoveryPending || rows[i].Burial != nil || exists(paths[i]) || !exists(filepath.Dir(rows[i].Grave)+".json") {
			t.Fatalf("uncommitted batch lost independent recovery evidence: %+v", rows)
		}
		if read(t, rows[i].Grave) != filepath.Base(paths[i]) {
			t.Fatal("moved content changed")
		}
	}
	if len(f.burials()) != 0 {
		t.Fatal("interrupted transaction partially committed")
	}
	for range 2 {
		if _, err := f.svc.Sweep(false); err != nil {
			t.Fatal(err)
		}
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.Burials) != 2 || st.Stats.Buried != 2 || st.Stats.DeleteBuried != 2 || !exists(paths[2]) {
		t.Fatalf("recovery lost or duplicated content: %+v %v", st, err)
	}
}

func TestBuryBatchPinsPreparedIdentity(t *testing.T) {
	f := newFixture(t)
	paths := []string{f.file("first"), f.file("replace"), f.file("last")}
	backup := paths[1] + ".old"
	f.fs.onRecycle = func(string) {
		f.fs.onRecycle = nil
		if err := os.Rename(paths[1], backup); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths[1], []byte("replacement"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := f.svc.Bury(paths, false, "")
	if err != nil || len(rows) != 3 || rows[0].Action != core.ActionBuried || rows[2].Action != core.ActionBuried || rows[1].Error == "" {
		t.Fatalf("changed identity affected siblings: %+v %v", rows, err)
	}
	if read(t, paths[1]) != "replacement" || read(t, backup) != "replace" || len(f.burials()) != 2 {
		t.Fatal("prepared identity moved replacement")
	}
}

func TestBuryBatchAllowsBudgetedWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("hundreds of moves with concurrent writer")
	}
	f := newFixture(t)
	paths := make([]string, 256)
	for i := range paths {
		paths[i] = f.file(fmt.Sprintf("file-%03d", i))
	}
	written := make(chan error, 1)
	started := false
	f.fs.onRecycle = func(string) {
		if !started {
			started = true
			go func() {
				store := f.svc.Store.WithDeadline(time.Now().Add(time.Second))
				written <- store.Update(func(st *core.State) error {
					if len(st.Burials) == len(paths) {
						return errors.New("writer only ran after burial completed")
					}
					st.NextID += 10000
					return nil
				})
			}()
		}
		time.Sleep(5 * time.Millisecond)
	}
	rows, err := f.svc.BuryDeletion(paths, false, "rm")
	if writerErr := <-written; writerErr != nil {
		t.Fatalf("one-second writer starved: %v", writerErr)
	}
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury: %+v %v", rows, err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || st.NextID < 10000 || len(st.Burials) != len(paths) {
		t.Fatalf("writer or burial lost: %+v %v", st, err)
	}
}

func TestBuryBatchSharesLedgerTransactions(t *testing.T) {
	f := newFixture(t)
	paths := []string{f.file("a"), f.file("b"), f.file("c"), f.file("d")}
	writes := 0
	t.Cleanup(core.SetReplacementSync(func(file *os.File) error {
		if strings.HasSuffix(file.Name(), "store.json.tmp") {
			writes++
		}
		return core.SyncFileForTesting(file)
	}))
	rows, err := f.svc.BuryDeletion(paths, false, "rm")
	// Reservation, sizes and acknowledgement write once; slow moves may each use a transaction.
	if err != nil || core.GraveFailed(rows) || writes > len(paths)+3 {
		t.Fatalf("per-item ledger rewrite returned: writes=%d rows=%+v err=%v", writes, rows, err)
	}
}

func TestBuryBatchPreparationAllowsWriter(t *testing.T) {
	f := newFixture(t)
	paths := []string{f.file("first"), f.file("second")}
	checked := 0
	t.Cleanup(core.SetBurialStep(f.svc, func(step string) error {
		if step != "manifest" {
			return nil
		}
		if !ledgerUnlocked(t, f) {
			t.Fatal("manifest preparation held store.lock")
		}
		checked++
		return f.svc.Store.WithDeadline(time.Now().Add(time.Second)).Update(func(st *core.State) error { st.NextID++; return nil })
	}))
	rows, err := f.svc.Bury(paths, false, "")
	if err != nil || core.GraveFailed(rows) || checked != 2 {
		t.Fatalf("preparation writer: %+v %v checked=%d", rows, err, checked)
	}
}

func TestBuryBatchRechecksEachRevision(t *testing.T) {
	for _, change := range []string{"extend", "cancel"} {
		t.Run(change, func(t *testing.T) {
			f, paths := dueBatch(t, 3)
			f.svc.FS = &prepareHookFS{testFS: f.fs, before: func() {
				var err error
				if change == "extend" {
					_, err = f.svc.Extend([]string{paths[1]}, time.Hour)
				} else {
					_, err = f.svc.Cancel([]string{paths[1]})
				}
				if err != nil {
					t.Fatal(err)
				}
			}}
			rows, err := f.svc.Bury(paths, false, "")
			if err != nil || len(rows) != 3 || rows[0].Action != core.ActionBuried || rows[1].Action != core.ActionSkipped || rows[2].Action != core.ActionBuried || !exists(paths[1]) {
				t.Fatalf("batch moved changed registration or lost sibling: %+v %v", rows, err)
			}
		})
	}
}

func TestBuryBatchCommitFailuresKeepEveryManifest(t *testing.T) {
	for _, failure := range []string{"write", "unconfirmed"} {
		t.Run(failure, func(t *testing.T) {
			f := newFixture(t)
			paths := []string{f.file("first"), f.file("second"), f.file("last")}
			moves := 0
			blocked := filepath.Join(f.svc.Store.Home, "store.json.tmp")
			restoreStep := core.SetBurialStep(f.svc, func(step string) error {
				if step == "moved" {
					moves++
					if moves == 3 && failure == "write" {
						return os.Mkdir(blocked, 0700)
					}
				}
				return nil
			})
			defer restoreStep()
			restoreSync := core.SetReplacementSync(func(file *os.File) error {
				if moves == 3 && failure == "unconfirmed" && strings.HasSuffix(file.Name(), "store.json.tmp") {
					return errors.New("unconfirmed batch")
				}
				return core.SyncFileForTesting(file)
			})
			defer restoreSync()
			rows, err := f.svc.BuryDeletion(paths, false, "rm")
			restoreStep()
			restoreSync()
			if failure == "write" {
				if err := os.Remove(blocked); err != nil {
					t.Fatal(err)
				}
			}
			if err == nil || len(rows) != 3 {
				t.Fatalf("missing commit failure: %+v %v", rows, err)
			}
			for _, row := range rows {
				if row.Action != core.ActionRecoveryPending || row.Burial != nil || !exists(filepath.Dir(row.Grave)+".json") {
					t.Fatalf("manifest lost: %+v", row)
				}
			}
			for range 2 {
				if _, err := f.svc.Sweep(false); err != nil {
					t.Fatal(err)
				}
			}
			st, err := f.svc.Store.ReadSnapshot()
			if err != nil || len(st.Burials) != 3 || st.Stats.DeleteBuried != 3 || st.Stats.Buried != 3 {
				t.Fatalf("recovery duplicated records: %+v %v", st, err)
			}
			for _, burial := range st.Burials {
				if read(t, burial.Grave) != filepath.Base(burial.Path) {
					t.Fatal("payload lost")
				}
			}
		})
	}
}

func BenchmarkBuryThousand(b *testing.B) {
	for _, individual := range []bool{true, false} {
		name := "batch"
		if individual {
			name = "individual"
		}
		b.Run(name, func(b *testing.B) {
			for range b.N {
				benchmarkBuryThousand(b, individual)
			}
		})
	}
}

func benchmarkBuryThousand(b *testing.B, individual bool) {
	b.StopTimer()
	root := b.TempDir()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "home")}, FS: &testFS{}}
	paths := make([]string, 1000)
	for i := range paths {
		paths[i] = filepath.Join(root, fmt.Sprintf("payload-%04d", i))
		if err := os.WriteFile(paths[i], []byte("payload"), 0600); err != nil {
			b.Fatal(err)
		}
	}
	// Keep roughly a megabyte of unrelated metadata present for every rewrite.
	if err := svc.Store.Update(func(st *core.State) error {
		for i := range 256 {
			st.Items = append(st.Items, core.Item{ID: int64(i + 1), Path: filepath.Join(root, fmt.Sprint("unrelated-", i)), Note: strings.Repeat("x", 4096)})
		}
		st.NextID = 257
		return nil
	}); err != nil {
		b.Fatal(err)
	}
	var longest time.Duration
	core.ObserveStoreLocks(&svc.Store, func(d time.Duration) { longest = max(longest, d) })
	writes := 0
	restore := core.SetReplacementSync(func(file *os.File) error {
		if strings.HasSuffix(file.Name(), "store.json.tmp") {
			writes++
		}
		return core.SyncFileForTesting(file)
	})
	defer restore()
	b.StartTimer()
	if individual {
		for _, path := range paths {
			rows, err := svc.BuryDeletion([]string{path}, false, "rm")
			if err != nil || core.GraveFailed(rows) {
				b.Fatalf("bury: %+v %v", rows, err)
			}
		}
	} else {
		rows, err := svc.BuryDeletion(paths, false, "rm")
		if err != nil || core.GraveFailed(rows) {
			b.Fatalf("bury: %+v %v", rows, err)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(longest.Microseconds())/1000, "max-lock-ms")
	b.ReportMetric(float64(writes), "ledger-writes/op")
}
