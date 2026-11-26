package lifecycle_test

import (
	"fmt"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
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
