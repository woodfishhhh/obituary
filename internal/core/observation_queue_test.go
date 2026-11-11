package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestObservationQueueDoesNotWaitForLedgerOrConsumer(t *testing.T) {
	f := newFixture(t)
	rows, err := f.svc.CaptureObservations([]string{f.file("debug.log")}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(f.svc.Store.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"store.lock", "observations.lock"} {
		release, err := core.LockFile(filepath.Join(f.svc.Store.Home, name), true)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if err := f.svc.Store.WithContext(ctx).WithDeadline(started.Add(-time.Second)).DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) >= 500*time.Millisecond {
		t.Fatal("queue waited for a lock")
	}
	pending, err := f.svc.Store.DeferredObservations()
	if err != nil || len(pending) != 1 || pending[0].Token != rows[0].Token {
		t.Fatal(pending, err)
	}
}

func TestObservationQueueOnlyPublishesAfterFlush(t *testing.T) {
	for _, stage := range []int{1, 2} {
		t.Run(fmt.Sprint(stage), func(t *testing.T) {
			f := newFixture(t)
			rows, err := f.svc.CaptureObservations([]string{f.file("debug.log")}, "s")
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			restore := core.SetFileSyncForTesting(func(file *os.File) error {
				calls++
				if calls == stage {
					return errors.New("injected flush interruption")
				}
				return file.Sync()
			})
			defer restore()
			if err := f.svc.Store.DeferObservations(rows); err == nil {
				t.Fatal("flush error hidden")
			}
			pending, err := f.svc.Store.DeferredObservations()
			if err != nil || len(pending) != stage-1 {
				t.Fatalf("partial publication: %v %v", pending, err)
			}
		})
	}
}

func TestObservationQueueCorruptAndPartialFilesDoNotBlock(t *testing.T) {
	f := newFixture(t)
	rows, err := f.svc.CaptureObservations([]string{f.file("debug.log")}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Store.DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(f.svc.Store.Home, "observations")
	for _, name := range []string{"broken.json", "interrupted.json.abc.tmp"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{"path":`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var replayed int
	err = f.svc.Store.WithObservationReplay(func() error {
		pending, err := f.svc.Store.DeferredObservations()
		if err != nil {
			return err
		}
		for _, row := range pending {
			replayed++
			if err := f.svc.Store.AcknowledgeObservation(row); err != nil {
				return err
			}
		}
		return nil
	})
	status, inspectErr := f.svc.Store.InspectObservations()
	if err == nil || inspectErr != nil || replayed != 1 || status.Pending != 0 || status.Dead != 1 || status.Partial != 1 {
		t.Fatalf("corruption blocked healthy request: replayed=%d status=%+v err=%v/%v", replayed, status, err, inspectErr)
	}
}

func TestObservationQueueReadFailureDoesNotQuarantine(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires Windows mandatory byte-range locks")
	}
	f := newFixture(t)
	rows, err := f.svc.CaptureObservations([]string{f.file("debug.log")}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Store.DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(f.svc.Store.Home, "observations", "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	release, err := core.LockFile(files[0], true)
	if err != nil {
		t.Fatal(err)
	}
	err = f.svc.Store.WithObservationReplay(func() error { _, err := f.svc.Store.DeferredObservations(); return err })
	release()
	if err == nil {
		t.Fatal("read error hidden")
	}
	pending, readErr := f.svc.Store.DeferredObservations()
	status, inspectErr := f.svc.Store.InspectObservations()
	events, auditErr := f.svc.Store.History(10)
	if readErr != nil || inspectErr != nil || auditErr != nil || len(pending) != 1 || status.Dead != 0 || len(events) != 0 {
		t.Fatalf("transient read failure discarded a request: %+v %+v %v %v %v %v", pending, status, events, readErr, inspectErr, auditErr)
	}
}

func TestObservationQueueKeepsNewWritesDuringReplay(t *testing.T) {
	store := core.Store{Home: t.TempDir()}
	path := filepath.Join(t.TempDir(), "debug.log")
	if err := deferTestObservations(t, store, []string{path}, "s1"); err != nil {
		t.Fatal(err)
	}
	old, err := store.DeferredObservations()
	if err != nil || len(old) != 1 {
		t.Fatalf("deduplication: %+v, %v", old, err)
	}
	if err := deferTestObservations(t, store, []string{strings.ToUpper(path)}, "s1"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeferObservations(old); err != nil {
		t.Fatal(err)
	}
	if err := store.AcknowledgeObservation(old[0]); err != nil {
		t.Fatal(err)
	}
	newer, err := store.DeferredObservations()
	if err != nil || len(newer) != 1 || newer[0].Token == old[0].Token {
		t.Fatalf("old replay consumed a newer write: %+v, %v", newer, err)
	}
	if err := deferTestObservations(t, store, []string{path}, "s2"); err != nil {
		t.Fatal(err)
	}
	if err := store.AcknowledgeObservation(newer[0]); err != nil {
		t.Fatal(err)
	}
	other, err := store.DeferredObservations()
	if err != nil || len(other) != 1 || other[0].Session != "s2" {
		t.Fatalf("another session was consumed: %+v, %v", other, err)
	}
	if err := store.AcknowledgeObservation(other[0]); err != nil {
		t.Fatal(err)
	}
	if store.HasDeferredObservations() {
		t.Fatal("drained queue still wakes background work")
	}
}

func TestObservationQueueOutgrowsLegacyCapacityWithoutEviction(t *testing.T) {
	if testing.Short() {
		t.Skip("writes more than the legacy 256-request capacity")
	}
	store := core.Store{Home: t.TempDir()}
	root := t.TempDir()
	var paths []string
	for i := 0; i < 256; i++ {
		paths = append(paths, filepath.Join(root, fmt.Sprintf("%d.log", i)))
	}
	if err := deferTestObservations(t, store, paths, "s1"); err != nil {
		t.Fatal(err)
	}
	if err := deferTestObservations(t, store, []string{filepath.Join(root, "overflow.log")}, "s1"); err != nil {
		t.Fatal("legacy shared-file capacity dropped a new request", err)
	}
	rows, err := store.DeferredObservations()
	if err != nil || len(rows) != 257 {
		t.Fatalf("overflow evicted accepted writes: %d, %v", len(rows), err)
	}
	for _, row := range rows {
		if err := store.AcknowledgeObservation(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := deferTestObservations(t, store, paths[:1], "s1"); err != nil {
		t.Fatal(err)
	}
	if err := deferTestObservations(t, store, paths[:1], strings.Repeat("x", 1<<20)); err == nil {
		t.Fatal("unbounded queue bytes")
	}
	rows, err = store.DeferredObservations()
	if err != nil || len(rows) != 1 || rows[0].Session != "s1" {
		t.Fatalf("byte overflow evicted accepted write: %+v, %v", rows, err)
	}
}

func TestObservationQueueNormalizesPathSpellings(t *testing.T) {
	f := newFixture(t)
	path := f.file("debug.log")
	rows, err := f.svc.CaptureObservations([]string{path}, "s")
	if err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{filepath.ToSlash(path), filepath.Dir(path) + `\unused\..\debug.log`, path} {
		row := rows[0]
		row.Path = spelling
		if err := f.svc.Store.DeferObservations([]core.Observation{row}); err != nil {
			t.Fatal(err)
		}
		pending, err := f.svc.Store.DeferredObservations()
		if err != nil || len(pending) != 1 || pending[0].Path != path {
			t.Fatalf("spelling not normalized: %+v %v", pending, err)
		}
	}
	pending, err := f.svc.Store.DeferredObservations()
	if err != nil || len(pending) != 1 || pending[0].Path != path {
		t.Fatalf("queue not normalized: %+v %v", pending, err)
	}
	rows[0].Path = filepath.ToSlash(filepath.Dir(path)) + "/unused/../debug.log"
	if err := f.svc.Store.AcknowledgeObservation(rows[0]); err != nil {
		t.Fatal(err)
	}
	if f.svc.Store.HasDeferredObservations() {
		t.Fatal("alternate spelling not acknowledged")
	}
}

func TestObservationRetryBoundsErrorAndPreservesFullQueue(t *testing.T) {
	for _, nearLimit := range []bool{false, true} {
		t.Run(fmt.Sprint(nearLimit), func(t *testing.T) {
			f := newFixture(t)
			rows, err := f.svc.CaptureObservations([]string{f.file("debug.log")}, "s")
			if err != nil {
				t.Fatal(err)
			}
			if nearLimit {
				data, err := json.Marshal(rows[0])
				if err != nil {
					t.Fatal(err)
				}
				rows[0].Session += strings.Repeat("x", (1<<20)-len(data)-32)
			}
			if err := f.svc.Store.DeferObservations(rows); err != nil {
				t.Fatal(err)
			}
			files, err := filepath.Glob(filepath.Join(f.svc.Store.Home, "observations", "*.json"))
			if err != nil || len(files) != 1 {
				t.Fatal(files, err)
			}
			queue := files[0]
			before, err := os.ReadFile(queue)
			if err != nil {
				t.Fatal(err)
			}
			_, retryErr := f.svc.Store.RetryObservation(rows[0], f.clock, errors.New(strings.Repeat("错误", 1<<18)))
			pending, err := f.svc.Store.DeferredObservations()
			if err != nil || len(pending) != 1 {
				t.Fatalf("retry corrupted queue: count=%d err=%v", len(pending), err)
			}
			if nearLimit {
				data, err := os.ReadFile(queue)
				if retryErr == nil || err != nil || !bytes.Equal(data, before) {
					t.Fatalf("oversized update changed accepted queue: %v %v", retryErr, err)
				}
			} else if retryErr != nil || pending[0].Attempts != 1 || len(pending[0].LastError) > 256 || !utf8.ValidString(pending[0].LastError) {
				t.Fatalf("retry error not bounded: bytes=%d err=%v", len(pending[0].LastError), retryErr)
			}
		})
	}
}
