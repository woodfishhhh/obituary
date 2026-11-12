package lifecycle_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestWatchConsentMigrationSnapshot(t *testing.T) {
	f := newFixture(t)
	dir := filepath.Dir(f.file("downloads/keep.txt"))
	p, err := f.svc.SetWatch(dir, "1d", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	p.Consent = core.ConsentApproved
	st := core.State{Version: 10, NextID: 2, Watches: []core.WatchPolicy{p}}
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "store.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	st, err = f.svc.Store.ReadSnapshot()
	if err != nil || st.Watches[0].Consent != "pending" || len(st.Proposals) != 1 {
		t.Fatalf("snapshot granted legacy consent: %+v %v", st, err)
	}
	st, err = f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 {
		t.Fatalf("migration: %+v %v", st, err)
	}
	again, err := f.svc.Store.Read()
	if err != nil || len(again.Proposals) != 1 || again.Proposals[0].ID != st.Proposals[0].ID {
		t.Fatalf("migration changed pending decision: %+v %v", again, err)
	}
}
