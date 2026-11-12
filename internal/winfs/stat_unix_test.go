//go:build linux || darwin

package winfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestPOSIXMissingBirthTimeRefusesRegistration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload")
	makePOSIXPayload(t, path, false, "keep")
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: FS{creationID: func(uint64) uint64 { return 0 }}}
	rows, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Error, "creation time") || !strings.Contains(rows[0].Error, "obit bury") {
		t.Fatalf("missing birth time registration accepted: %+v %v", rows, err)
	}
	st, err := svc.Store.ReadSnapshot()
	if err != nil || len(st.Items) != 0 || len(st.Burials) != 0 {
		t.Fatalf("unsupported registration changed ledger: %+v %v", st, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Fatalf("unsupported registration changed file: %q %v", data, err)
	}
	if _, err := svc.SetWatch(dir, "1h", "first-seen"); err == nil || !strings.Contains(err.Error(), "creation time") {
		t.Fatalf("missing birth time watch accepted: %v", err)
	}
	policy := core.WatchPolicy{ID: 1, Path: dir, Lifespan: "1h", Age: "first-seen"}
	if err := svc.RegisterWatchChild(policy, path); err == nil || !strings.Contains(err.Error(), "creation time") {
		t.Fatalf("missing birth time watch child accepted: %v", err)
	}
}

func TestPOSIXLegacyLifespanIsRetained(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload")
	makePOSIXPayload(t, path, false, "keep")
	now := time.Now()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: FS{}, Now: func() time.Time { return now }}
	rows, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: now.Add(time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	if err := svc.Store.UpdateHot(func(st *core.State) error {
		st.Items[0].Identity.FileID = "0000000000000000" + st.Items[0].Identity.FileID[16:]
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	svc.FS = FS{creationID: func(uint64) uint64 { return 0 }}
	now = now.Add(2 * time.Hour)
	out, err := svc.Sweep(false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry || !strings.Contains(out[0].Detail, "obit mark") {
		t.Fatalf("legacy identity moved: %+v %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(svc.Store.Home, "graveyard")); !os.IsNotExist(err) {
		t.Fatalf("legacy identity reached grave preparation: %v", err)
	}
	st, err := svc.Store.ReadSnapshot()
	if err != nil || len(st.Items) != 1 || len(st.Burials) != 0 {
		t.Fatalf("legacy entry lost: %+v %v", st, err)
	}
	events, err := svc.RegistrationLocationDiagnostics()
	if err != nil || len(events) != 1 || !strings.Contains(events[0].Detail, "obit mark") {
		t.Fatalf("legacy diagnostic missing: %+v %v", events, err)
	}
	svc.FS = FS{}
	rows, err = svc.Add([]string{path}, core.AddOptions{ExpiresAt: now.Add(time.Hour)})
	if err != nil || core.Failed(rows) || rows[0].Item.ID != st.Items[0].ID || rows[0].Item.Identity.RequirePersistent() != nil {
		t.Fatalf("explicit re-mark did not replace legacy identity: %+v %v", rows, err)
	}
	now = now.Add(2 * time.Hour)
	out, err = svc.Sweep(false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionBuried {
		t.Fatalf("re-marked object did not bury: %+v %v", out, err)
	}
}
