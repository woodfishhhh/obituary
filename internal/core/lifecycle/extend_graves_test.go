package lifecycle_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
)

func TestExtendGravesPushesPurgeAtAndClearsRetry(t *testing.T) {
	f := newFixture(t)
	a := writePayload(t, f.dir, "a.log", "alpha")
	b := writePayload(t, f.dir, "b.log", "beta")
	buried, err := f.svc.Bury([]string{a, b}, false, "")
	if err != nil || len(buried) != 2 {
		t.Fatalf("seed bury: %+v %v", buried, err)
	}
	idA, idB := buried[0].Burial.ID, buried[1].Burial.ID
	origPurgeA := buried[0].Burial.PurgeAt

	// Make B overdue with prior purge retry state.
	if err := f.svc.Store.UpdateGraveyard(func(st *core.State) error {
		idx := st.BurialIndex(idB)
		st.Burials[idx].PurgeAt = f.clock.Add(-2 * time.Hour)
		st.Burials[idx].PurgeAttempts = 3
		st.Burials[idx].NextPurgeAttempt = f.clock.Add(time.Hour)
		st.Burials[idx].LastPurgeError = "busy"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rows, err := f.svc.ExtendGraves([]string{fmt.Sprintf("#%d", idA), fmt.Sprintf("#%d", idB)}, 48*time.Hour)
	if err != nil {
		t.Fatalf("ExtendGraves: %v", err)
	}
	if len(rows) != 2 || rows[0].Action != core.ActionExtended || rows[1].Action != core.ActionExtended {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if !rows[0].Burial.PurgeAt.Equal(origPurgeA.Add(48 * time.Hour)) {
		t.Fatalf("A purgeAt = %v, want %v", rows[0].Burial.PurgeAt, origPurgeA.Add(48*time.Hour))
	}
	if !rows[1].Burial.PurgeAt.Equal(f.clock.Add(48 * time.Hour)) {
		t.Fatalf("overdue B should extend from now: got %v, want %v", rows[1].Burial.PurgeAt, f.clock.Add(48*time.Hour))
	}
	if rows[1].Burial.PurgeAttempts != 0 || !rows[1].Burial.NextPurgeAttempt.IsZero() || rows[1].Burial.LastPurgeError != "" {
		t.Fatalf("B retry state not cleared: %+v", rows[1].Burial)
	}
}

func TestExtendGravesRejectsActiveItemsPurgingAndBadDuration(t *testing.T) {
	f := newFixture(t)
	active := f.add(writePayload(t, f.dir, "active.txt", "still-alive"), 24*time.Hour)
	purgingPath := writePayload(t, f.dir, "purging.txt", "mid-purge")
	buried, err := f.svc.Bury([]string{purgingPath}, false, "")
	if err != nil || len(buried) != 1 {
		t.Fatalf("seed bury: %+v %v", buried, err)
	}
	purgingID := buried[0].Burial.ID
	if err := f.svc.Store.UpdateGraveyard(func(st *core.State) error {
		idx := st.BurialIndex(purgingID)
		st.Burials[idx].PurgeIntent = &core.PurgeIntent{VerifiedAt: f.clock, Basis: "test"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.ExtendGraves([]string{fmt.Sprintf("#%d", purgingID)}, 0); err == nil {
		t.Fatal("zero duration accepted")
	}

	rows, err := f.svc.ExtendGraves([]string{fmt.Sprintf("#%d", active.ID), fmt.Sprintf("#%d", purgingID), "#999999"}, 24*time.Hour)
	if err != nil {
		t.Fatalf("ExtendGraves errors should live in result slots: %v", err)
	}
	for i, r := range rows {
		if r.Error == "" || r.Action == core.ActionExtended {
			t.Fatalf("slot %d should fail: %+v", i, r)
		}
	}
}

func TestExtendGravesCleansRecordedV1ManifestBeforeExtending(t *testing.T) {
	f := newFixture(t)
	p := writePayload(t, f.dir, "recorded.log", "payload")
	buried, err := f.svc.Bury([]string{p}, false, "")
	if err != nil || len(buried) != 1 || buried[0].Burial == nil {
		t.Fatalf("seed bury: %+v %v", buried, err)
	}
	b := *buried[0].Burial
	manifest := filepath.Dir(b.Grave) + ".json"
	mb := b
	mb.Size = 0 // manifests record burial metadata before post-commit size accounting
	raw, err := json.Marshal(struct {
		Version int `json:"version"`
		core.Burial
	}{Version: 1, Burial: mb})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	rows, err := f.svc.ExtendGraves([]string{fmt.Sprintf("#%d", b.ID)}, 24*time.Hour)
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionExtended || rows[0].Error != "" {
		t.Fatalf("ExtendGraves with recorded v1 manifest: %+v %v", rows, err)
	}
	if exists(manifest) {
		t.Fatal("v1 recorded manifest was not cleaned before extending")
	}
	if !rows[0].Burial.PurgeAt.Equal(b.PurgeAt.Add(24 * time.Hour)) {
		t.Fatalf("PurgeAt = %v, want %v", rows[0].Burial.PurgeAt, b.PurgeAt.Add(24*time.Hour))
	}
	// Extended grave must remain cleanly resurrectable (not unverifiable).
	res, err := f.svc.Resurrect([]string{fmt.Sprintf("#%d", b.ID)})
	if err != nil || len(res) != 1 || res[0].Action != core.ActionResurrected {
		t.Fatalf("Resurrect after ExtendGraves: %+v %v", res, err)
	}
}

func TestExtendGravesRejectsRestoreIntentUnverifiableMismatchAndContainerIntent(t *testing.T) {
	f := newFixture(t)
	restorePath := writePayload(t, f.dir, "restore-intent.log", "v2")
	corruptPath := writePayload(t, f.dir, "unverifiable.log", "bad")
	mismatchPath := writePayload(t, f.dir, "mismatch.log", "mm")
	buried, err := f.svc.Bury([]string{restorePath, corruptPath, mismatchPath}, false, "")
	if err != nil || len(buried) != 3 {
		t.Fatalf("seed bury: %+v %v", buried, err)
	}
	bRestore, bCorrupt, bMismatch := *buried[0].Burial, *buried[1].Burial, *buried[2].Burial

	parent, err := f.fs.OpenRestoreParent(filepath.Dir(bRestore.Path))
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	if err := core.WriteRestoreIntent(bRestore, parent, filepath.Base(bRestore.Path)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Dir(bCorrupt.Grave)+".json", []byte("{not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cid := docker.Identity{
		Target:  &docker.Target{Endpoint: "stub", DaemonID: "stub"},
		ID:      strings.Repeat("c", 64),
		ShortID: strings.Repeat("c", 12),
		Name:    "pending-container",
		Created: f.clock.Add(-time.Hour),
	}
	cBurial := core.Burial{
		ID:        901,
		Kind:      core.KindContainer,
		Container: &cid,
		Path:      cid.Display(),
		BuriedAt:  f.clock,
		PurgeAt:   f.clock.Add(24 * time.Hour),
	}
	if err := f.svc.Store.UpdateGraveyard(func(st *core.State) error {
		st.Burials[st.BurialIndex(bMismatch.ID)].IdentityMismatch = "replaced during move"
		st.Burials = append(st.Burials, cBurial)
		st.ContainerIntents = append(st.ContainerIntents, core.ContainerIntent{
			Token:     "tok-1",
			Operation: "rm",
			Burial:    &cBurial,
			At:        f.clock,
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	refs := []string{
		fmt.Sprintf("#%d", bRestore.ID),
		fmt.Sprintf("#%d", bCorrupt.ID),
		fmt.Sprintf("#%d", bMismatch.ID),
		fmt.Sprintf("#%d", cBurial.ID),
	}
	rows, err := f.svc.ExtendGraves(refs, 48*time.Hour)
	if err != nil {
		t.Fatalf("ExtendGraves returned top-level error: %v", err)
	}
	for i, r := range rows {
		if r.Error == "" || r.Action == core.ActionExtended {
			t.Fatalf("slot %d (%s) should be rejected: %+v", i, refs[i], r)
		}
	}
	if !strings.Contains(rows[3].Error, "container operation pending; run obit reap") {
		t.Fatalf("container intent error = %q", rows[3].Error)
	}

	// Verify PurgeAt was not mutated on any rejected burial.
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	wantPurge := map[int64]time.Time{
		bRestore.ID:  bRestore.PurgeAt,
		bCorrupt.ID:  bCorrupt.PurgeAt,
		bMismatch.ID: bMismatch.PurgeAt,
		cBurial.ID:   cBurial.PurgeAt,
	}
	for id, want := range wantPurge {
		idx := st.BurialIndex(id)
		if idx < 0 || !st.Burials[idx].PurgeAt.Equal(want) {
			t.Fatalf("burial #%d PurgeAt mutated: got %v, want %v", id, st.Burials[idx].PurgeAt, want)
		}
	}
}
