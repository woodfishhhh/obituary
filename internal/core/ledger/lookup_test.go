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

func TestNextDueForSkipsPendingQuotaDecision(t *testing.T) {
	now := time.Now().UTC()
	idA := core.Identity{Serial: 1, FileID: "1"}
	idB := core.Identity{Serial: 1, FileID: "2"}
	dueA := now.Add(-time.Hour)
	dueB := now.Add(time.Minute)
	st := core.State{
		Items: []core.Item{
			{ID: 1, Rev: 1, Identity: &idA, Path: "a.bin", ExpiresAt: dueA},
			{ID: 2, Rev: 1, Identity: &idB, Path: "b.bin", ExpiresAt: dueB},
		},
		Proposals: []core.Proposal{{
			ID: 3, Kind: core.ProposalGraveyardQuota,
			QuotaReview: &core.QuotaBurialReview{ItemID: 1, ItemRev: 1, Identity: idA, ItemBytes: 8000, MaxBytes: 5000},
		}},
	}
	if got, ok := st.NextDueFor(5000); !ok || !got.Equal(dueB) {
		t.Fatalf("NextDueFor(5000) = (%v, %v), want %v (must skip pending quota item)", got, ok, dueB)
	}
	// Raising limit unblocks A so it becomes due again immediately.
	if got, ok := st.NextDueFor(10000); !ok || !got.Equal(dueA) {
		t.Fatalf("NextDueFor(10000) = (%v, %v), want %v", got, ok, dueA)
	}
	// Unlimited (0) keeps NextDue() backward-compatible and unblocks A.
	if got, ok := st.NextDue(); !ok || !got.Equal(dueA) {
		t.Fatalf("NextDue() = (%v, %v), want %v", got, ok, dueA)
	}
}
