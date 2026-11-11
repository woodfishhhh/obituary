//go:build windows

package core_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestSnapshotMissingBlocksRecovery(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.svc.Store.Home, st.Graveyard.File)
	if err := os.Rename(path, path+".held"); err != nil {
		t.Fatal(err)
	}
	for _, run := range []func() error{
		func() error { _, err := f.svc.ReadGraveyard(); return err },
		func() error { _, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)}); return err },
		func() error { _, err := f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false); return err },
		func() error { _, err := f.svc.Sweep(false); return err },
	} {
		if err := run(); err == nil {
			t.Fatal("missing snapshot was treated as empty graveyard")
		}
		if !exists(b.Grave) {
			t.Fatal("missing snapshot authorized deletion")
		}
	}
	rows, err := f.svc.Add([]string{f.file("unrelated.txt")}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatal("unrelated hot registration read missing snapshot", rows, err)
	}
}

func TestSnapshotSummaryCannotAuthorizePurge(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	path := filepath.Join(f.svc.Store.Home, "store.json")
	var hot map[string]json.RawMessage
	if err := json.Unmarshal([]byte(read(t, path)), &hot); err != nil {
		t.Fatal(err)
	}
	hot["graveyardIndex"] = json.RawMessage(`{"maxId":0,"reservedAudit":0}`)
	data, _ := json.Marshal(hot)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
	if err == nil || !exists(b.Grave) {
		t.Fatal("unverified hot summary authorized purge", rows, err)
	}
}

func TestSnapshotConcurrentHotAndTerminalUpdates(t *testing.T) {
	f := newFixture(t)
	first := buryOne(t, f, "first.txt")
	second := buryOne(t, f, "second.txt")
	live := f.add(f.file("live.txt"), time.Hour)
	var wg sync.WaitGroup
	wg.Add(1)
	errors := make(chan error, 1)
	go func() {
		defer wg.Done()
		for range 12 {
			if err := f.svc.Store.UpdateHot(func(st *core.State) error { st.Items[st.Index(live.ID)].Rev++; return nil }); err != nil {
				errors <- err
				return
			}
		}
	}()
	rows, restoreErr := f.svc.Resurrect([]string{fmt.Sprint(first.ID)})
	purged, purgeErr := f.svc.PurgeRefs([]string{fmt.Sprint(second.ID)}, false)
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	if restoreErr != nil || purgeErr != nil || core.GraveFailed(rows) || core.GraveFailed(purged) {
		t.Fatal(rows, restoreErr, purged, purgeErr)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.Burials) != 0 || st.Items[st.Index(live.ID)].Rev != live.Rev+12 || st.Stats.Resurrected != 1 || st.Stats.Purged != 1 {
		t.Fatal("concurrent hot and terminal updates lost state or double counted", st, err)
	}
}
