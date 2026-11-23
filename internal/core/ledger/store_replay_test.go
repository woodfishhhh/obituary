package ledger

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// replayRow is one request whose effect on the ledger is observable: the first
// replay bumps the item's Rev and stores the request's receipt, a repeat finds
// the receipt and changes nothing.
func replayRow() Observation { return Observation{Token: "one", ObservedAt: time.Now()} }

func replayMutation(row Observation) func(Store) error {
	return func(tx Store) error {
		return tx.UpdateHot(func(st *State) error {
			if !st.ObservationCompleted(&row) {
				st.Items[0].Rev++
				st.RememberObservation(&row, row.ObservedAt)
			}
			return nil
		})
	}
}

func TestResidentReplaySurvivesCommitFaults(t *testing.T) {
	RequireFsync(t)
	for _, stage := range []string{"callback", "hot-write", "hot-flush", "hot-rename", "hot-confirm"} {
		t.Run(stage, func(t *testing.T) {
			s := snapshotTestStore(t).WithResidentReplay()
			row, mutate := replayRow(), replayMutation(replayRow())
			before, err := s.ReadHot()
			if err != nil {
				t.Fatal(err)
			}
			reset := injectSnapshotFault(t, stage)
			err = s.ReplayBatch(func(tx Store) error {
				if err := mutate(tx); err != nil {
					return err
				}
				if stage == "callback" {
					return errors.New(stage)
				}
				return nil
			})
			if err == nil {
				t.Fatal("fault acknowledged")
			}
			reset()
			// A new process: no cache, same files. Replaying twice must not repeat the effect.
			reopened := Store{Home: s.Home}.WithResidentReplay()
			for range 2 {
				if err := reopened.ReplayBatch(mutate); err != nil {
					t.Fatal(err)
				}
			}
			st, err := reopened.ReadSnapshot()
			if err != nil || !st.ObservationCompleted(&row) || st.Items[0].Rev != before.Items[0].Rev+1 {
				t.Fatalf("loss or duplicate: %v %v", st.Items, err)
			}
		})
	}
}

func TestResidentReplayPreservesOtherWriters(t *testing.T) {
	s := snapshotTestStore(t).WithResidentReplay()
	direct := Store{Home: s.Home}
	note := func(f func(old string) string) func(Store) error {
		return func(tx Store) error {
			return tx.UpdateHot(func(st *State) error { st.Items[0].Note = f(st.Items[0].Note); return nil })
		}
	}
	if err := s.ReplayBatch(note(func(string) string { return "first" })); err != nil {
		t.Fatal(err)
	}
	if err := direct.UpdateHot(func(st *State) error { st.Items[0].Note = "direct"; return nil }); err != nil {
		t.Fatal(err)
	}
	// The cache holds "first"; the file now differs, so it must not be used.
	if err := s.ReplayBatch(note(func(old string) string { return old + "+second" })); err != nil {
		t.Fatal(err)
	}
	if st, err := direct.ReadSnapshot(); err != nil || st.Items[0].Note != "direct+second" {
		t.Fatal(st.Items, err)
	}
}

func TestResidentReplayRunsCallbackWithoutStoreLock(t *testing.T) {
	s := snapshotTestStore(t).WithResidentReplay()
	first := true
	err := s.ReplayBatch(func(tx Store) error {
		if first {
			// What Git and identity checks do for seconds must not queue other
			// processes: a writer gets the lock while the callback runs.
			if err := writeWithin(Store{Home: s.Home}, func(st *State) { st.Items[0].Note = "writer" }); err != nil {
				t.Errorf("writer queued behind the callback: %v", err)
			}
			first = false
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// writeWithin is another process's ordinary hot write, bounded so that a
// callback holding the lock fails the test instead of hanging it.
func writeWithin(s Store, change func(*State)) error {
	return s.WithDeadline(time.Now().Add(2 * time.Second)).UpdateHot(func(st *State) error { change(st); return nil })
}

func TestResidentReplayRedoesBatchAfterConflict(t *testing.T) {
	s := snapshotTestStore(t).WithResidentReplay()
	direct := Store{Home: s.Home}
	runs := 0
	err := s.ReplayBatch(func(tx Store) error {
		runs++
		if runs == 1 {
			if err := writeWithin(direct, func(st *State) { st.Items[0].Note = "direct" }); err != nil {
				return err
			}
		}
		return tx.UpdateHot(func(st *State) error { st.Items[0].Rev += 10; st.Items[0].Note += "+batch"; return nil })
	})
	if err != nil || runs != 2 {
		t.Fatalf("runs=%d err=%v", runs, err)
	}
	st, err := direct.ReadSnapshot()
	if err != nil || st.Items[0].Note != "direct+batch" {
		t.Fatalf("a writer's commit was overwritten or the batch lost: %+v %v", st.Items, err)
	}
}

func TestResidentReplayLocksAfterRepeatedConflicts(t *testing.T) {
	s := snapshotTestStore(t).WithResidentReplay()
	direct := Store{Home: s.Home}
	runs := 0
	err := s.ReplayBatch(func(tx Store) error {
		runs++
		if runs <= replayOptimisticTries { // a competing writer wins every unlocked attempt
			if err := writeWithin(direct, func(st *State) { st.Items[0].Rev++ }); err != nil {
				return err
			}
		}
		return tx.UpdateHot(func(st *State) error { st.Items[0].Note = "batch"; return nil })
	})
	if err != nil || runs != replayOptimisticTries+1 {
		t.Fatalf("runs=%d err=%v", runs, err)
	}
	if st, err := direct.ReadSnapshot(); err != nil || st.Items[0].Note != "batch" {
		t.Fatal(st.Items, err)
	}
}

func acceptedRow(t *testing.T, token string) Observation {
	t.Helper()
	return Observation{
		Path: filepath.Join(t.TempDir(), token+".log"), Session: "s", Token: token,
		ObservedAt: time.Now(), Identity: &Identity{FileID: token},
	}
}

func TestAcceptObservationsIsOneDurableWriteTheConsumerImports(t *testing.T) {
	s := Store{Home: t.TempDir()}
	rows := []Observation{acceptedRow(t, "a"), acceptedRow(t, "b"), acceptedRow(t, "c")}
	if err := s.AcceptObservations(rows); err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptObservations(rows[:1]); err != nil { // the same token again
		t.Fatal(err)
	}
	var queued []Observation
	err := s.WithObservationReplay(func() error {
		var err error
		queued, err = s.DeferredObservations()
		return err
	})
	if err != nil || len(queued) != 3 {
		t.Fatalf("queued %d, want 3 (a repeated token is one request): %v", len(queued), err)
	}
}

func TestAcceptObservationsWhileConsumerOwnsTheLock(t *testing.T) {
	s := Store{Home: t.TempDir()}
	release, err := LockFile(filepath.Join(s.Home, "observations.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := s.AcceptObservations([]Observation{acceptedRow(t, "a")}); err != nil {
		t.Fatal(err)
	}
	if queued, err := s.DeferredObservations(); err != nil || len(queued) != 1 {
		t.Fatalf("a busy consumer must not stop acceptance: %d %v", len(queued), err)
	}
}

func TestAcceptObservationsRefusesWhatReplayCannotUse(t *testing.T) {
	s := Store{Home: t.TempDir()}
	noIdentity := acceptedRow(t, "a")
	noIdentity.Identity = nil
	relative := acceptedRow(t, "b")
	relative.Path = "relative.log"
	for name, row := range map[string]Observation{"no identity": noIdentity, "relative path": relative} {
		if err := s.AcceptObservations([]Observation{row}); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if s.HasDeferredObservations() {
		t.Fatal("a refused request left a queue behind")
	}
}

func TestAcceptObservationsOverflowsToRequestFiles(t *testing.T) {
	s := Store{Home: t.TempDir()}
	big := acceptedRow(t, "big")
	big.LastError = strings.Repeat("x", observationBytes*2/5)
	for _, token := range []string{"one", "two", "three"} { // the third no longer fits the shared file
		row := big
		row.Token = token
		if err := s.AcceptObservations([]Observation{row}); err != nil {
			t.Fatal(err)
		}
	}
	var queued []Observation
	err := s.WithObservationReplay(func() error {
		var err error
		queued, err = s.DeferredObservations()
		return err
	})
	if err != nil || len(queued) != 3 {
		t.Fatalf("queued %d, want 3: %v", len(queued), err)
	}
}

// The hook falls back to its own per-request file whenever acceptance fails, so
// a failed or unconfirmed write must leave the shared file whole and the same
// request must still be imported exactly once.
func TestAcceptObservationsSurvivesWriteFaults(t *testing.T) {
	RequireFsync(t)
	for _, stage := range []string{"backup-write", "backup-flush", "backup-confirm"} { // any file but the ledger
		t.Run(stage, func(t *testing.T) {
			s := Store{Home: t.TempDir()}
			first, second := acceptedRow(t, "first"), acceptedRow(t, "second")
			if err := s.AcceptObservations([]Observation{first}); err != nil {
				t.Fatal(err)
			}
			reset := injectSnapshotFault(t, stage)
			err := s.AcceptObservations([]Observation{second})
			reset()
			if err == nil {
				t.Fatal("fault acknowledged")
			}
			if err := s.DeferObservations([]Observation{second}); err != nil { // the hook's fallback
				t.Fatal(err)
			}
			var queued []Observation
			err = s.WithObservationReplay(func() error {
				var err error
				queued, err = s.DeferredObservations()
				return err
			})
			if err != nil || len(queued) != 2 {
				t.Fatalf("queued %d, want exactly the two requests: %v", len(queued), err)
			}
		})
	}
}

func TestNoticesQuietOnlyForTheLedgerTheResidentCommitted(t *testing.T) {
	s := snapshotTestStore(t).WithResidentReplay()
	if s.NoticesQuiet() {
		t.Fatal("nothing has been committed yet")
	}
	touch := func(change func(*State)) func(Store) error {
		return func(tx Store) error { return tx.UpdateHot(func(st *State) error { change(st); return nil }) }
	}
	if err := s.ReplayBatch(touch(func(st *State) { st.Items[0].Note = "a" })); err != nil {
		t.Fatal(err)
	}
	if !s.NoticesQuiet() {
		t.Fatal("a ledger with no proposal and no untold receipt is quiet")
	}
	// Another process commits: the resident cannot vouch for that file.
	direct := Store{Home: s.Home}
	if err := direct.UpdateHot(func(st *State) error { st.Items[0].Note = "b"; return nil }); err != nil {
		t.Fatal(err)
	}
	if s.NoticesQuiet() {
		t.Fatal("quiet survived another process's commit")
	}
	if err := s.ReplayBatch(touch(func(st *State) { st.AutoReceipts = []AutoReceipt{{ID: st.Items[0].ID}} })); err != nil {
		t.Fatal(err)
	}
	if s.NoticesQuiet() {
		t.Fatal("an automatic registration nobody was told about is news")
	}
	if err := s.ReplayBatch(touch(func(st *State) { st.AutoReceipts[0].Told = true; st.Proposals = []Proposal{{ID: 99}} })); err != nil {
		t.Fatal(err)
	}
	if s.NoticesQuiet() {
		t.Fatal("a proposal is news for any session that has not heard of it")
	}
	if (Store{Home: s.Home}).NoticesQuiet() {
		t.Fatal("only the resident's store may claim quiet")
	}
}
