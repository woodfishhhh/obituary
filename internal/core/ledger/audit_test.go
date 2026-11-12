//go:build windows

package ledger_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/ledger"
)

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
