//go:build windows

package lifecycle_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestLastBurialScopeAndBatch(t *testing.T) {
	f := newFixture(t)
	bury := func(session, project string, names ...string) []string {
		t.Helper()
		f.clock = f.clock.Add(time.Second)
		svc, err := f.svc.BeginBurial(session, project)
		if err != nil {
			t.Fatal(err)
		}
		var paths []string
		for _, name := range names {
			paths = append(paths, f.file(name))
		}
		rows, err := svc.Bury(paths, false, "")
		if err != nil || core.GraveFailed(rows) {
			t.Fatal(rows, err)
		}
		var refs []string
		for _, r := range rows {
			refs = append(refs, fmt.Sprint(r.Burial.ID))
		}
		return refs
	}
	older := bury("mine", f.dir, "older.txt")
	mine := bury("mine", f.dir, "a.txt", "sub/b.txt")
	project := bury("", f.dir, "project.txt")
	other := bury("another-session", f.dir, "other.txt")
	bury("", filepath.Dir(f.dir), "elsewhere.txt")
	bury("", "", "mcp.txt")
	// A registered item's origin is not evidence that its later deletion was
	// performed by this session. Nor is the background sweep an explicit batch.
	expired := f.file("expired.txt")
	if rows, err := f.svc.Add([]string{expired}, core.AddOptions{ExpiresAt: f.clock.Add(time.Second), Session: "mine", Project: f.dir}); err != nil || core.Failed(rows) {
		t.Fatal(rows, err)
	}
	f.clock = f.clock.Add(2 * time.Second)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, session, project string
		want                   []string
	}{
		{"session first", "mine", f.dir, mine},
		{"session outside project", "mine", "", mine},
		{"project fallback", "new-session", f.dir, project},
		{"project without session", "", f.dir, other},
		{"no global fallback", "unknown", "", nil},
		{"unknown project", "", filepath.Join(f.dir, "unknown"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := f.svc.LastBurialRefs(tc.session, tc.project)
			if tc.want == nil {
				if err == nil || !strings.Contains(err.Error(), "obit graveyard") {
					t.Fatalf("unrelated deletion selected: %v %v", got, err)
				}
			} else if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("wrong deletion batch: %v %v; want %v", got, err, tc.want)
			}
		})
	}
	if rows, err := f.svc.Resurrect(mine); err != nil || core.GraveFailed(rows) || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	if refs, _, err := f.svc.LastBurialRefs("mine", f.dir); err != nil || !reflect.DeepEqual(refs, older) {
		t.Fatalf("completed batch still selected: %v %v", refs, err)
	}
}

func TestLastBurialKeepsRecoveredLatestBatch(t *testing.T) {
	for _, remaining := range []bool{false, true} {
		t.Run(fmt.Sprint(remaining), func(t *testing.T) {
			f := newFixture(t)
			bury := func(names ...string) []core.GraveResult {
				t.Helper()
				f.clock = f.clock.Add(time.Second)
				svc, err := f.svc.BeginBurial("mine", f.dir)
				if err != nil {
					t.Fatal(err)
				}
				var paths []string
				for _, name := range names {
					paths = append(paths, f.file(name))
				}
				rows, err := svc.Bury(paths, false, "")
				if err != nil || core.GraveFailed(rows) {
					t.Fatal(rows, err)
				}
				return rows
			}
			older := bury("older")[0].Burial
			names := []string{"first", "second"}
			if remaining {
				names = append(names, "third")
			}
			latest := bury(names...)
			if rows, err := f.svc.Resurrect([]string{fmt.Sprint(latest[0].Burial.ID)}); err != nil || core.GraveFailed(rows) {
				t.Fatal(rows, err)
			}
			_, unlock := interruptResurrection(t, f, *latest[1].Burial)
			unlock()
			refs, restored, err := f.svc.LastBurialRefs("mine", f.dir)
			if err != nil {
				t.Fatal(err)
			}
			rows, err := f.svc.Resurrect(refs)
			if err != nil || core.GraveFailed(rows) {
				t.Fatal(rows, err)
			}
			if exists(older.Path) || !exists(older.Grave) {
				t.Fatalf("retry restored unrelated older batch after reconciling latest: %v", refs)
			}
			if len(restored) != 1 || restored[0].Burial.ID != latest[1].Burial.ID || restored[0].Action != core.ActionResurrected || restored[0].Path != latest[1].Burial.Path {
				t.Fatalf("reconciled latest member lost its receipt: %+v", restored)
			}
			if remaining {
				if !reflect.DeepEqual(refs, []string{fmt.Sprint(latest[2].Burial.ID)}) {
					t.Fatalf("remaining latest member not selected: %v", refs)
				}
			} else if len(refs) != 0 {
				t.Fatalf("completed latest batch still has references: %v", refs)
			}
			for _, row := range latest {
				if !exists(row.Burial.Path) || exists(row.Burial.Grave) {
					t.Fatalf("latest batch member not restored: %+v", row)
				}
			}
		})
	}
}

func TestLastBurialRejectsOtherSessionLegacyAndTies(t *testing.T) {
	for _, mode := range []string{"different-session", "legacy", "tie"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			svc, err := f.svc.BeginBurial("other", f.dir)
			if err != nil {
				t.Fatal(err)
			}
			rows, err := svc.Bury([]string{f.file("a")}, false, "")
			if err != nil || core.GraveFailed(rows) {
				t.Fatal(rows, err)
			}
			if mode == "legacy" {
				if err := f.svc.Store.Update(func(st *core.State) error {
					st.Burials[0].Operation = nil
					st.Burials[0].Session, st.Burials[0].Project = "mine", f.dir
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			session := "mine"
			if mode == "tie" {
				session = "other"
				svc, err = f.svc.BeginBurial("other", f.dir)
				if err != nil {
					t.Fatal(err)
				}
				rows, err = svc.Bury([]string{f.file("b")}, false, "")
				if err != nil || core.GraveFailed(rows) {
					t.Fatal(rows, err)
				}
			}
			if refs, _, err := f.svc.LastBurialRefs(session, f.dir); err == nil || !strings.Contains(err.Error(), "obit graveyard") {
				t.Fatalf("ambiguous or foreign batch selected: %v %v", refs, err)
			}
		})
	}
}

func TestBurialBatchSurvivesInterruptedCommit(t *testing.T) {
	f := newFixture(t)
	svc, err := f.svc.BeginBurial("mine", f.dir)
	if err != nil {
		t.Fatal(err)
	}
	first, second := f.file("first"), f.file("second")
	moves := 0
	restore := core.SetBurialStep(svc, func(step string) error {
		if step == "moved" {
			moves++
			if moves == 2 {
				return errors.New("interrupted")
			}
		}
		return nil
	})
	rows, err := svc.Bury([]string{first, second}, false, "")
	restore()
	if err == nil || len(rows) != 2 || rows[0].Action != core.ActionRecoveryPending || rows[1].Action != core.ActionRecoveryPending {
		t.Fatal(rows, err)
	}
	refs, _, err := f.svc.LastBurialRefs("mine", f.dir)
	if err == nil || !strings.Contains(err.Error(), "obit reap") {
		t.Fatalf("incomplete pending batch selected: %v %v", refs, err)
	}
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	refs, _, err = f.svc.LastBurialRefs("mine", f.dir)
	if err != nil || len(refs) != 2 {
		t.Fatalf("recovered batch member omitted: %v %v", refs, err)
	}
	rows, err = f.svc.Resurrect(refs)
	if err != nil || core.GraveFailed(rows) || len(rows) != 2 || read(t, first) != "first" || read(t, second) != "second" {
		t.Fatalf("batch recovery failed: %+v %v", rows, err)
	}
}
