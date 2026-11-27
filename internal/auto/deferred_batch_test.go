package auto

import (
	"fmt"
	"path/filepath"
	"testing"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func approveProject(t *testing.T, r *repo) {
	t.Helper()
	if err := r.engine.Svc.Store.Update(func(st *ledger.State) error {
		st.Projects = map[string]ledger.ProjectState{ledger.ProjectKey(r.root): {Consent: ledger.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func residentRepo(t *testing.T) *repo {
	t.Helper()
	if testing.Short() {
		t.Skip("uses a real Git repository")
	}
	r := newRepo(t)
	r.engine.Svc.Store = r.engine.Svc.Store.WithResidentReplay()
	approveProject(t, r)
	return r
}

func TestResidentBatchRegistersEveryRequestOnceAcrossBatches(t *testing.T) {
	r := residentRepo(t)
	const total = 2*replayBatchRows + 6
	for i := range total {
		path := r.write(fmt.Sprintf("debug-%02d.log", i), "artifact")
		rows, err := r.engine.Svc.CaptureObservations([]string{path}, fmt.Sprintf("session-%d", i))
		if err != nil || len(rows) != 1 {
			t.Fatal(rows, err)
		}
		if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 { // the second round is a replay of an empty queue and of nothing else
		if err := r.engine.ReplayObservations(); err != nil {
			t.Fatal(err)
		}
	}
	st, err := r.engine.Svc.Store.ReadSnapshot()
	if err != nil || len(st.Items) != total || len(st.ObservationReceipts) != total || r.engine.Svc.Store.HasDeferredObservations() {
		t.Fatalf("items=%d receipts=%d queued=%t err=%v", len(st.Items), len(st.ObservationReceipts), r.engine.Svc.Store.HasDeferredObservations(), err)
	}
	for _, it := range st.Items {
		if it.Rev != 1 {
			t.Fatalf("%s registered %d times", it.Path, it.Rev)
		}
	}
}

// lockProbeFS reports whether store.lock is free each time replay inspects the
// observed file: Git and identity checks must not hold up other processes.
type lockProbeFS struct {
	corefs.FS
	path  string
	probe func()
}

func (f *lockProbeFS) Identify(path string) (corefs.Identity, error) {
	if path == f.path {
		f.probe()
	}
	return f.FS.Identify(path)
}

func TestResidentBatchInspectsRequestsWithoutHoldingStoreLock(t *testing.T) {
	r := residentRepo(t)
	path := r.write("debug.log", "artifact")
	rows, err := r.engine.Svc.CaptureObservations([]string{path}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	probes := 0
	r.engine.Svc.FS = &lockProbeFS{FS: r.engine.Svc.FS, path: path, probe: func() {
		probes++
		checkOwnerLedgerLock(t, r.engine.Svc, false)
	}}
	if err := r.engine.ReplayObservations(); err != nil {
		t.Fatal(err)
	}
	if probes == 0 {
		t.Fatal("the lock was never probed")
	}
	if st, err := r.engine.Svc.Store.ReadSnapshot(); err != nil || len(st.Items) != 1 {
		t.Fatal(st.Items, err)
	}
}

func TestResidentBatchRetryDoesNotHoldBackHealthyRequests(t *testing.T) {
	r := residentRepo(t)
	good := r.write("good.log", "artifact")
	stale := r.write("stale.log", "artifact")
	for i, path := range []string{good, stale} {
		rows, err := r.engine.Svc.CaptureObservations([]string{path}, fmt.Sprintf("s%d", i))
		if err != nil {
			t.Fatal(err)
		}
		if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
			t.Fatal(err)
		}
	}
	healthy := r.engine.Svc.FS
	r.engine.Svc.FS = &unavailableObservationFS{FS: healthy, path: stale, volumeError: "volume offline"}
	if err := r.engine.ReplayObservations(); err == nil {
		t.Fatal("an unavailable volume must be reported for retry")
	}
	st, err := r.engine.Svc.Store.ReadSnapshot()
	pending, pendingErr := r.engine.Svc.Store.DeferredObservations()
	if err != nil || pendingErr != nil || len(st.Items) != 1 || filepath.Base(st.Items[0].Path) != "good.log" ||
		len(pending) != 1 || pending[0].Path != stale || pending[0].Attempts != 1 {
		t.Fatalf("items=%v pending=%+v %v %v", st.Items, pending, err, pendingErr)
	}
	r.engine.Svc.FS = healthy
	if err := r.engine.ReplayObservations(); err != nil {
		t.Fatal(err)
	}
	if st, err := r.engine.Svc.Store.ReadSnapshot(); err != nil || len(st.Items) != 2 || r.engine.Svc.Store.HasDeferredObservations() {
		t.Fatalf("the retried request was not recovered: %v %v", st.Items, err)
	}
}

// A resident killed between its ledger commit and the removal of the request
// (and the audit flush) leaves the request queued with its effect already
// committed. The next consumer must find the receipt, register nothing again
// and finish the acknowledgement and the audit delivery.
func TestResidentBatchCrashAfterCommitBeforeAcknowledgement(t *testing.T) {
	r := residentRepo(t)
	path := r.write("debug.log", "artifact")
	rows, err := r.engine.Svc.CaptureObservations([]string{path}, "s")
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	// The commit step of commitBatch, then "death": no audit flush, no acknowledgement.
	err = r.engine.Svc.Store.ReplayBatch(func(tx ledger.Store) error {
		svc := *r.engine.Svc
		svc.Store, svc.AfterChange = tx, nil
		engine := *r.engine
		engine.Svc = &svc
		return engine.applyObservation(rows[0])
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := r.engine.Svc.Store.ReadSnapshot()
	if err != nil || len(st.Items) != 1 || !r.engine.Svc.Store.HasDeferredObservations() {
		t.Fatalf("setup: items=%d queued=%t err=%v", len(st.Items), r.engine.Svc.Store.HasDeferredObservations(), err)
	}
	// A new resident process: no cache.
	reopened := *r.engine
	svc := *r.engine.Svc
	svc.Store = ledger.Store{Home: r.engine.Svc.Store.Home}.WithResidentReplay()
	reopened.Svc = &svc
	for range 2 {
		if err := reopened.ReplayObservations(); err != nil {
			t.Fatal(err)
		}
	}
	st, err = svc.Store.ReadSnapshot()
	if err != nil || len(st.Items) != 1 || st.Items[0].Rev != 1 || len(st.PendingAudit) != 0 || svc.Store.HasDeferredObservations() {
		t.Fatalf("items=%+v pendingAudit=%d queued=%t err=%v", st.Items, len(st.PendingAudit), svc.Store.HasDeferredObservations(), err)
	}
}
