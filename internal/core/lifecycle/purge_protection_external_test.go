package lifecycle_test

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

type observingPurger struct {
	core.FS
	observe func(corefs.PurgePlan) error
}

func (o observingPurger) PurgeGrave(_ string, _ core.Identity, plan corefs.PurgePlan) error {
	return o.observe(plan)
}

func TestPurgeGuardsNewRegistrations(t *testing.T) {
	for _, entry := range []string{"PurgeRefs", "Tick"} {
		t.Run(entry, func(t *testing.T) {
			f := newFixture(t)
			member := f.file("tree/member")
			rows, err := f.svc.Bury([]string{filepath.Dir(member)}, true, "")
			if err != nil || len(rows) != 1 || rows[0].Burial == nil {
				t.Fatal(rows, err)
			}
			b := rows[0].Burial
			f.clock = b.PurgeAt.Add(time.Hour)
			first, second := f.file("first"), f.file("second")
			var before core.Item
			defer core.SetBeforePurgeDelete(f.svc, func() { before = f.add(first, 24*time.Hour) })()
			called := false
			f.svc.FS = observingPurger{f.fs, func(plan corefs.PurgePlan) error {
				called = true
				if before.Identity == nil || plan.WithObject(before.Identity.FileID, false, nil) == nil {
					t.Fatal("new_id_present=false before backend")
				}
				late := f.add(second, 24*time.Hour)
				err := plan.WithObject(late.Identity.FileID, false, func() error { t.Fatal("guard authorized a registered object"); return nil })
				if err == nil {
					t.Fatal("new_id_present=false after pin")
				}
				t.Log("new_id_present=true before backend and after cache refill")
				return fmt.Errorf("protected: %w", err)
			}}
			if entry == "Tick" {
				_, rows, err = f.svc.Tick()
			} else {
				rows, err = f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
			}
			if err != nil || !called || len(rows) != 1 || !strings.Contains(rows[0].Error, "protected") || read(t, first) != "first" || read(t, second) != "second" || read(t, filepath.Join(b.Grave, "member")) != "tree/member" {
				t.Fatal(rows, err)
			}
		})
	}
}

func TestPurgeVerifiedStageHandling(t *testing.T) {
	recorded := ".obit-purge-" + rand.Text()
	stray := ".obit-purge-" + rand.Text()
	cases := []struct {
		name      string
		intent    *core.PurgeIntent
		wantStage string
	}{
		{name: "nil-intent", intent: nil},
		{name: "legacy-empty-stage", intent: &core.PurgeIntent{Basis: "explicit selection"}},
		{name: "recorded-stage", intent: &core.PurgeIntent{Basis: "explicit selection", Stage: recorded}, wantStage: recorded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			rows, err := f.svc.Bury([]string{f.file("payload.txt")}, true, "")
			if err != nil || len(rows) != 1 || rows[0].Burial == nil {
				t.Fatal(rows, err)
			}
			b := *rows[0].Burial
			b.PurgeIntent = tc.intent
			if tc.intent == nil {
				if err := os.Mkdir(filepath.Join(filepath.Dir(b.Grave), stray), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			var got corefs.PurgePlan
			called := false
			f.svc.FS = observingPurger{f.fs, func(plan corefs.PurgePlan) error {
				called = true
				got = plan
				return nil
			}}
			if err := core.PurgeVerified(f.svc, b); err != nil {
				t.Fatalf("PurgeVerified(%s): %v", tc.name, err)
			}
			if !called || got.Name != filepath.Base(b.Grave) || got.Object != *b.Identity || !got.ValidStage() {
				t.Fatalf("unexpected plan: called=%v plan=%+v", called, got)
			}
			if tc.wantStage != "" && got.Stage != tc.wantStage {
				t.Fatalf("stage=%q, want %q", got.Stage, tc.wantStage)
			}
			if tc.intent == nil && (got.Stage == stray || len(got.LegacyMembers) != 0) {
				t.Fatalf("nil intent adopted an unowned stage directory: %+v", got)
			}
		})
	}
}
