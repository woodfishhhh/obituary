package ledger

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadActiveViewDecodesOncePerCommit(t *testing.T) {
	s := snapshotTestStore(t).WithResidentReplay()
	first, err := s.ReadActiveView()
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.ReadActiveView()
	if err != nil || &first.Items[0] != &second.Items[0] {
		t.Fatalf("store.json did not change, so the decoded items are reused: %v", err)
	}
	if want, err := s.ReadActive(); err != nil || !reflect.DeepEqual(want.Items, second.Items) {
		t.Fatalf("the view differs from ReadActive: %v", err)
	}
	// Another process commits: the next read is that commit, never the old copy.
	if err := (Store{Home: s.Home}).UpdateHot(func(st *State) error { st.Items[0].Rev += 5; return nil }); err != nil {
		t.Fatal(err)
	}
	third, err := s.ReadActiveView()
	if err != nil || third.Items[0].Rev != first.Items[0].Rev+5 {
		t.Fatalf("stale view after another process committed: %+v %v", third.Items[0], err)
	}
}

func TestReadActiveViewIsReadActiveOutsideTheResident(t *testing.T) {
	s := snapshotTestStore(t)
	a, err := s.ReadActiveView()
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.ReadActiveView()
	if err != nil || &a.Items[0] == &b.Items[0] {
		t.Fatalf("a process that is not the resident keeps no copy: %v", err)
	}
}

func TestReadActiveViewVerifiesTheGraveyardSnapshotAgain(t *testing.T) {
	s := snapshotTestStore(t).WithResidentReplay()
	st, err := s.ReadActiveView()
	if err != nil {
		t.Fatal(err)
	}
	// Same store.json, but the snapshot it vouches for was changed.
	f, err := os.OpenFile(filepath.Join(s.Home, st.Graveyard.File), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, werr := f.WriteString(" ")
	if err := errors.Join(werr, f.Close()); err != nil {
		t.Fatal(err)
	}
	_, want := s.ReadActive()
	if _, got := s.ReadActiveView(); want == nil || got == nil {
		t.Fatalf("a tampered snapshot must be refused as ReadActive refuses it: ReadActive %v, view %v", want, got)
	}
}
