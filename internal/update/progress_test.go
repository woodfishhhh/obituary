package update

import (
	"testing"
	"time"
)

func TestProgressRoundTrip(t *testing.T) {
	home := t.TempDir()
	if p, err := ReadProgress(home); err != nil || p != (Progress{}) {
		t.Fatalf("no update yet: %+v %v", p, err)
	}
	tr := &tracker{home: home}
	tr.set(Progress{Phase: PhaseFailed, Error: "no network"})
	if p, err := ReadProgress(home); err != nil || p.Phase != PhaseFailed || p.Error != "no network" {
		t.Fatalf("%+v %v", p, err)
	}
	// A new run starts from a clean record, never the previous run's.
	tr.phase(PhaseStarting)
	if p, _ := ReadProgress(home); p != (Progress{Phase: PhaseStarting}) {
		t.Fatalf("%+v", p)
	}
}

func TestDownloadProgressIsThrottledButEndsExact(t *testing.T) {
	home := t.TempDir()
	tr := &tracker{home: home}
	tr.bytes(10, 100)
	tr.bytes(20, 100) // within 250 ms of the first: not written
	if p, _ := ReadProgress(home); p != (Progress{Phase: PhaseDownloading, Done: 10, Total: 100}) {
		t.Fatalf("%+v", p)
	}
	tr.bytes(100, 100) // the last chunk is always written
	if p, _ := ReadProgress(home); p.Done != 100 || p.Total != 100 {
		t.Fatalf("%+v", p)
	}
	tr.last = time.Now().Add(-time.Second)
	tr.bytes(150, -1) // an unknown total still reports once due
	if p, _ := ReadProgress(home); p.Done != 150 {
		t.Fatalf("%+v", p)
	}
}

func TestProgressIsOptional(t *testing.T) {
	var tr *tracker
	tr.phase(PhaseReady) // no tracker: nothing happens, nothing panics
	tr.bytes(1, 2)
	(&tracker{}).phase(PhaseReady)
}
