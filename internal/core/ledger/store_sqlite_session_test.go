package ledger

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestSQLiteSessionFailureKeepsCommittedRevision(t *testing.T) {
	for _, stage := range []string{"callback", "before-commit", "after-commit", "hot-write", "hot-flush", "hot-rename", "hot-confirm"} {
		t.Run(stage, func(t *testing.T) {
			store := snapshotTestStore(t).WithPersistentSQLite()
			t.Cleanup(store.ClosePersistentSQLite)
			before, err := store.ReadHot()
			if err != nil {
				t.Fatal(err)
			}
			injected := errors.New("injected transaction failure")
			previous := commitSQLite
			defer func() { commitSQLite = previous }()
			commitSQLite = func(ctx context.Context, tx *sql.Tx) error {
				if stage == "before-commit" {
					return injected
				}
				if err := previous(ctx, tx); err != nil {
					return err
				}
				if stage == "after-commit" {
					return injected
				}
				return nil
			}
			reset := injectSnapshotFault(t, stage)
			err = store.UpdateHot(func(st *State) error {
				st.Items[0].Rev++
				if stage == "callback" {
					return injected
				}
				return nil
			})
			reset()
			commitSQLite = previous
			if err == nil {
				t.Fatal("fault did not interrupt write")
			}
			want := before.Items[0].Rev
			if stage == "after-commit" || stage == "hot-confirm" {
				want++
			}
			got, err := store.Inspect()
			if err != nil || len(got.Items) != 1 || got.Items[0].Rev != want {
				t.Fatalf("inspect after fault: %+v, want rev %d, err %v", got.Items, want, err)
			}
			if err := store.UpdateHot(func(st *State) error {
				if got := st.Items[0].Rev; got != want {
					t.Fatalf("next transaction saw rev %d, want %d", got, want)
				}
				st.Items[0].Rev++
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			got, err = store.ReadHot()
			if err != nil || len(got.Items) != 1 || got.Items[0].Rev != want+1 {
				t.Fatalf("reopened: %+v, want rev %d, err %v", got.Items, want+1, err)
			}
		})
	}
}
