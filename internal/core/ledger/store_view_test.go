package ledger

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func countSnapshotReads(t *testing.T) *int {
	t.Helper()
	reads, old := 0, readGraveyardSnapshot
	readGraveyardSnapshot = func(path string) ([]byte, error) { reads++; return old(path) }
	t.Cleanup(func() { readGraveyardSnapshot = old })
	return &reads
}

func TestReadActiveViewParsesTheSnapshotOncePerFile(t *testing.T) {
	s := snapshotTestStore(t).WithResidentReplay()
	reads := countSnapshotReads(t)
	first, err := s.ReadActiveView()
	if err != nil {
		t.Fatal(err)
	}
	if want, err := s.ReadActive(); err != nil || !reflect.DeepEqual(want.Items, first.Items) {
		t.Fatalf("the view differs from ReadActive: %v", err)
	}
	// Another process commits the hot ledger: the next read is that commit, never an old copy,
	// and the snapshot it still references is not parsed again.
	if err := (Store{Home: s.Home}).UpdateHot(func(st *State) error { st.Items[0].Rev += 5; return nil }); err != nil {
		t.Fatal(err)
	}
	*reads = 0
	second, err := s.ReadActiveView()
	if err != nil || second.Items[0].Rev != first.Items[0].Rev+5 {
		t.Fatalf("stale view after another process committed: %+v %v", second.Items[0], err)
	}
	if _, err := s.ReadActiveView(); err != nil || *reads != 0 {
		t.Fatalf("the snapshot was read %d times for a snapshot that did not change: %v", *reads, err)
	}
}

func TestReadActiveViewIsReadActiveOutsideTheResident(t *testing.T) {
	s := snapshotTestStore(t)
	reads := countSnapshotReads(t)
	for range 2 {
		if _, err := s.ReadActiveView(); err != nil {
			t.Fatal(err)
		}
	}
	if *reads != 2 {
		t.Fatalf("a process that is not the resident keeps nothing between reads: %d snapshot reads", *reads)
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
