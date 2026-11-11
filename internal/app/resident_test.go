//go:build windows

package app

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/windows"
)

func TestPendingFolderDecisionHasZeroCommitsAndWakeups(t *testing.T) {
	home := t.TempDir()
	var notifications atomic.Int32
	svc := &core.Service{Store: core.Store{Home: home}, FS: winfs.FS{}, AfterChange: func(*core.Service) { notifications.Add(1) }}
	id := core.Identity{Serial: 42}
	if err := svc.Store.Update(func(st *core.State) error {
		st.Items = []core.Item{{ID: 1, Rev: 1, Identity: &id, Source: "watch", WatchID: 3, IsDir: true, Path: filepath.Join(home, "pending"), ExpiresAt: time.Now().Add(-time.Hour)}}
		st.Proposals = []core.Proposal{{ID: 2, Kind: core.ProposalWatchBurial, WatchID: 3, WatchBurial: &core.WatchBurialReview{ItemID: 1, ItemRev: 1, Identity: id}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	r, err := NewResident(svc, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	before := svc.Store.ModTime()
	if _, _, err := r.step(Interval); err != nil {
		t.Fatal(err)
	}
	// The first tick may label the ledger change made above, once; the idle
	// window below must not notify again.
	if n := notifications.Load(); n > 1 {
		t.Fatalf("first tick notified %d times", n)
	}
	notifications.Store(0)
	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	wakeups := 0
	for ctx.Err() == nil {
		if err := r.wait(ctx, time.Hour); errors.Is(err, context.DeadlineExceeded) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		wakeups++
		if _, _, err := r.step(Interval); err != nil {
			t.Fatal(err)
		}
	}
	commits := 0
	if !svc.Store.ModTime().Equal(before) {
		commits++
	}
	if commits != 0 || wakeups != 0 || notifications.Load() != 0 {
		t.Fatalf("250ms idle: commits=%d wakeups=%d AfterChange=%d", commits, wakeups, notifications.Load())
	}
	t.Logf("250ms idle: commits=%d wakeups=%d AfterChange=%d", commits, wakeups, notifications.Load())
}

func TestIdleResidentSleepsUntilMaintenance(t *testing.T) {
	svc := &core.Service{Store: core.Store{Home: t.TempDir()}, FS: winfs.FS{}}
	r := resident(svc, log.New(io.Discard, "", 0))
	// The delay counts down from when the step began, so allow for the time
	// the step itself took on a busy machine.
	start := time.Now()
	delay, stop, err := r.step(Interval)
	if err != nil || stop || delay < Interval-time.Since(start)-time.Second {
		t.Fatalf("idle delay %v after %v, stop %v, err %v", delay, time.Since(start), stop, err)
	}
}

func TestResidentWakesOnLedgerReplacement(t *testing.T) {
	svc := &core.Service{Store: core.Store{Home: t.TempDir()}, FS: winfs.FS{}}
	r, err := NewResident(svc, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := svc.Store.Update(func(st *core.State) error { st.NextID = 40; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := r.wait(ctx, time.Hour); err != nil {
		t.Fatalf("missed ledger replacement: %v", err)
	}
}

func TestResidentSeparatesHomesAndCanonicalizesAliases(t *testing.T) {
	home := t.TempDir()
	r, err := NewResident(&core.Service{Store: core.Store{Home: home}}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !Running(filepath.Join(home, ".")) {
		t.Fatal("canonical alias missed instance")
	}
	other, err := NewResident(&core.Service{Store: core.Store{Home: t.TempDir()}}, log.New(io.Discard, "", 0))
	if err != nil || other == nil {
		t.Fatalf("other home blocked: %v", err)
	}
	other.Close()
}

func TestResidentReapsAnExpiryAddedWhileIdle(t *testing.T) {
	root := t.TempDir()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "data")}, FS: winfs.FS{}}
	r, err := NewResident(svc, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
		r.Close()
	})
	path := filepath.Join(root, "due.txt")
	if err := os.WriteFile(path, []byte("new expiry"), 0600); err != nil {
		t.Fatal(err)
	}
	// A separate service is the CLI writer; the resident must notice the commit.
	writer := &core.Service{Store: svc.Store, FS: winfs.FS{}}
	if rows, err := writer.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(250 * time.Millisecond)}); err != nil || core.Failed(rows) {
		t.Fatalf("add %v %v", rows, err)
	}
	if err := waitFor(func() bool {
		state, err := svc.Store.Read()
		return err == nil && len(state.Burials) == 1 && len(state.Items) == 0
	}, 5*time.Second, "resident missed a new expiry"); err != nil {
		t.Fatal(err)
	}
}

func TestResidentAcceptsStopRequestBeforeWriterCloses(t *testing.T) {
	home := t.TempDir()
	name, err := windows.UTF16PtrFromString(stopPath(home))
	if err != nil {
		t.Fatal(err)
	}
	// A directory create event arrives before os.WriteFile closes its handle.
	writer, err := windows.CreateFile(name, windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.CREATE_NEW, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(writer)
	r := resident(&core.Service{Store: core.Store{Home: home}}, log.New(io.Discard, "", 0))
	if _, stop, err := r.step(Interval); !stop || err != nil {
		t.Fatalf("stop request rejected while writer closes: stop=%v err=%v", stop, err)
	}
}
