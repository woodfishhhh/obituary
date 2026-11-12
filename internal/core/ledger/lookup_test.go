package ledger_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestPurgeIntentNextDueUsesRetryTime(t *testing.T) {
	now := time.Now().UTC()
	for _, retry := range []time.Time{now.Add(time.Minute), now.Add(-time.Minute), {}} {
		st := core.State{Burials: []core.Burial{{
			PurgeAt: now.Add(7 * 24 * time.Hour), NextPurgeAttempt: retry,
			PurgeIntent: &core.PurgeIntent{VerifiedAt: now},
		}}}
		want := retry
		if want.IsZero() {
			want = now
		}
		if got, ok := st.NextDue(); !ok || !got.Equal(want) {
			t.Fatalf("purge intent retry postponed until retention: got=%v want=%v", got, want)
		}
		st.Burials[0].PurgeIntent = nil
		if got, ok := st.NextDue(); !ok || !got.Equal(st.Burials[0].PurgeAt) {
			t.Fatalf("ordinary burial ignored retention: %v", got)
		}
	}
}

func TestFindPathPrefersExactAndRejectsAmbiguousASCII(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "k.txt"), filepath.Join(root, "\u212a.txt"), filepath.Join(root, "K.TXT")}
	st := core.State{}
	for i, path := range paths {
		st.Items = append(st.Items, core.Item{ID: int64(i + 1), Path: path})
	}
	for i, path := range paths {
		got, err := st.FindPath(path)
		if err != nil || got != i {
			t.Fatalf("exact ledger path conflated: %d %v", got, err)
		}
	}
	if _, err := st.FindPath(filepath.Join(root, "k.TXT")); err == nil || !strings.Contains(err.Error(), "use an ID") {
		t.Fatalf("ambiguous ASCII path accepted: %v", err)
	}
}
