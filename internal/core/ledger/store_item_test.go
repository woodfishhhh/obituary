package ledger

import (
	"os"
	"testing"
)

func TestItemRevisionUpdateDoesNotLoadResidentState(t *testing.T) {
	s := snapshotTestStore(t).WithPersistentSQLite()
	t.Cleanup(s.ClosePersistentSQLite)
	st, err := s.ReadHot()
	if err != nil {
		t.Fatal(err)
	}
	it := st.Items[0]
	fence, err := os.Stat(s.statePath())
	if err != nil {
		t.Fatal(err)
	}
	start := sqliteSessionDecodeCount()
	change := func(item *Item) { item.Rev++; item.Note = "retry" }
	for range 2 {
		if err := s.UpdateItem(it.ID, it.Rev, change); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.ReadHot()
	if err != nil || got.Items[0].Rev != it.Rev+1 || got.Items[0].Note != "retry" {
		t.Fatalf("stale retry changed item: %+v %v", got.Items, err)
	}
	if n := sqliteSessionDecodeCount() - start; n != 0 {
		t.Fatalf("single-row update materialized %d full sessions", n)
	}
	after, err := os.Stat(s.statePath())
	if err != nil || !os.SameFile(fence, after) {
		t.Fatal("resident replaced an unchanged fence", err)
	}
}
