//go:build windows

package core_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestHotRegistrationNormalizesSnapshotIdentity(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	if err := f.svc.Store.UpdateGraveyard(func(st *core.State) error {
		st.Burials[0].Identity.Volume = `\\?\` + st.Burials[0].Identity.Volume
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(b.Grave, b.Path); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Add([]string{b.Path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err == nil && !core.Failed(rows) {
		t.Fatal("snapshot identity spelling bypassed buried registration check")
	}
}

func TestSnapshotIndexChecksInterruptedRestore(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	_, release := interruptResurrection(t, f, b)
	release()
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.svc.Store.Home, st.Graveyard.File)
	if err := os.Rename(path, path+".held"); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Add([]string{b.Path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err == nil && !core.Failed(rows) {
		t.Fatal("safety index skipped interrupted resurrection")
	}
	if !exists(b.Path) {
		t.Fatal("registration moved the pending restore")
	}
}

func TestSnapshotValidationFailureDoesNotAuthorizeNextCandidate(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	if err := f.svc.Store.UpdateGraveyard(func(st *core.State) error {
		st.Burials[0].Identity.Volume = `\\?\` + st.Burials[0].Identity.Volume
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(b.Grave, b.Path); err != nil {
		t.Fatal(err)
	}
	hot, err := f.svc.Store.ReadHot()
	if err != nil {
		t.Fatal(err)
	}
	hot.GraveyardIndex.ReservedAudit++
	data, err := json.Marshal(hot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "store.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	rows, _ := f.svc.Add([]string{b.Path, b.Path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	for _, row := range rows {
		if row.Action != "" {
			t.Fatal("failed snapshot validation authorized a later candidate", rows)
		}
	}
	st, err := f.svc.Store.ReadHot()
	if err != nil || len(st.Items) != 0 {
		t.Fatal("failed snapshot validation persisted a registration", st.Items, err)
	}
}
