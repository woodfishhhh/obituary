package auto

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestReplayKeepsCurrentRegistrationGates(t *testing.T) {
	if testing.Short() {
		t.Skip("uses real Git and current project rules")
	}
	for _, gate := range []string{"tracked", "declined", "changed-rules", "protected-rules", "kept", "identity", "conflict"} {
		t.Run(gate, func(t *testing.T) {
			r := newRepo(t)
			path := r.write("debug.log", "artifact")
			if err := r.engine.Svc.Store.Update(func(st *core.State) error {
				st.Projects = map[string]core.ProjectState{core.ProjectKey(r.root): {Consent: core.ConsentApproved}}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			rows, err := r.engine.Svc.CaptureObservations([]string{path}, "s")
			if err != nil {
				t.Fatal(err)
			}
			if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
				t.Fatal(err)
			}
			prepareReplayGate(t, r, path, gate)
			_ = r.engine.ReplayObservations() // changed rules remain retryable
			st, err := r.engine.Svc.Store.Read()
			want := 0
			if gate == "conflict" {
				want = 1
			}
			if err != nil || len(st.Items) != want || len(st.Proposals) != 0 {
				t.Fatalf("replay bypassed %s: %+v %v", gate, st, err)
			}
			if want == 1 && st.Items[0].Source != "cli" {
				t.Fatal("replay changed manual lifespan")
			}
		})
	}
}

func TestReplayExhaustedRequestRetriesDeadLetterOnly(t *testing.T) {
	r := newEngineFixture(t)
	rows, err := r.engine.Svc.CaptureObservations([]string{r.write("debug.log", "keep")}, "s")
	if err != nil {
		t.Fatal(err)
	}
	rows[0].Attempts = core.ObservationMaxAttempts
	if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(r.engine.Svc.Store.Home, "audit.jsonl")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := r.engine.ReplayObservations(); err == nil {
		t.Fatal("dead-letter audit error hidden")
	}
	if !r.engine.Svc.Store.HasDeferredObservations() {
		t.Fatal("failed dead-letter transition lost request")
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	if err := r.engine.ReplayObservations(); err == nil {
		t.Fatal("exhausted request was processed again")
	}
	status, err := r.engine.Svc.Store.InspectObservations()
	if err != nil || status.Pending != 0 || status.Dead != 1 {
		t.Fatal(status, err)
	}
}

func prepareReplayGate(t *testing.T, r *repo, path, gate string) {
	t.Helper()
	switch gate {
	case "tracked":
		r.git("add", "-f", "--", path)
	case "declined":
		if err := r.engine.Svc.Store.Update(func(st *core.State) error {
			st.Projects[core.ProjectKey(r.root)] = core.ProjectState{Consent: core.ConsentDeclined}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	case "changed-rules", "protected-rules":
		file := r.write(".obitrules", "*.log off\n")
		if _, err := r.engine.Trust.Trust(file, ""); err != nil {
			t.Fatal(err)
		}
		if gate == "changed-rules" {
			r.write(".obitrules", "*.log 1d\n")
		}
	case "identity":
		if err := os.Rename(path, path+".old"); err != nil {
			t.Fatal(err)
		}
		r.write("debug.log", "replacement")
	case "kept", "conflict":
		items, err := r.engine.Svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(30 * 24 * time.Hour)})
		if err != nil || len(items) != 1 || items[0].Item == nil {
			t.Fatal(items, err)
		}
		if gate == "kept" {
			if _, err := r.engine.Svc.Cancel([]string{path}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestReplayReceiptSurvivesLostAcknowledgement(t *testing.T) {
	if testing.Short() {
		t.Skip("uses a real Git repository")
	}
	r := newRepo(t)
	path := r.write("debug.log", "artifact")
	rows, err := r.engine.Svc.CaptureObservations([]string{path}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	decisions, _, err := r.engine.ObserveCaptured(rows)
	if err != nil || len(decisions) != 1 || decisions[0].Action != Proposed {
		t.Fatal(decisions, err)
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 {
		t.Fatal(st, err)
	}
	before, _ := json.Marshal(st.Proposals)
	if err := r.engine.ReplayObservations(); err != nil {
		t.Fatal(err)
	}
	st, err = r.engine.Svc.Store.Read()
	after, _ := json.Marshal(st.Proposals)
	if err != nil || string(before) != string(after) {
		t.Fatal("replay refreshed a committed proposal", string(after), err)
	}
	if _, err := r.engine.Svc.Reject([]string{fmt.Sprint(st.Proposals[0].ID)}); err != nil {
		t.Fatal(err)
	}
	if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	if err := r.engine.ReplayObservations(); err != nil {
		t.Fatal(err)
	}
	st, err = r.engine.Svc.Store.Read()
	if err != nil || len(st.Proposals) != 0 || r.engine.Svc.Store.HasDeferredObservations() {
		t.Fatal("replay revived a rejected request", st, err)
	}
}

func TestReplayRetainsObservationOnUnavailableVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, reason := range []string{"volume offline", "volume serial changed"} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			path := r.write("debug.log", "keep")
			rows, err := r.engine.Svc.CaptureObservations([]string{path}, "s")
			if err != nil {
				t.Fatal(err)
			}
			if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
				t.Fatal(err)
			}
			fs := r.engine.Svc.FS
			r.engine.Svc.FS = &unavailableObservationFS{FS: fs, path: path, volumeError: reason}
			if err := r.engine.ReplayObservations(); err == nil {
				t.Fatal("unavailable volume was not retryable")
			}
			pending, err := r.engine.Svc.Store.DeferredObservations()
			if err != nil || len(pending) != 1 || pending[0].Attempts != 1 {
				t.Fatalf("offline observation lost: %+v %v", pending, err)
			}
			r.engine.Svc.FS = fs
			if err := r.engine.ReplayObservations(); err != nil {
				t.Fatal(err)
			}
			st, err := r.engine.Svc.Store.Read()
			if err != nil || len(st.Proposals) != 1 || r.engine.Svc.Store.HasDeferredObservations() {
				t.Fatalf("online observation not recovered: %+v %v", st.Proposals, err)
			}
		})
	}
}

func TestReplayDropsMissingObservationOnSameOnlineVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	path := r.write("debug.log", "gone")
	rows, err := r.engine.Svc.CaptureObservations([]string{path}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := r.engine.ReplayObservations(); err != nil {
		t.Fatal(err)
	}
	if r.engine.Svc.Store.HasDeferredObservations() {
		t.Fatal("confirmed missing observation kept")
	}
}

func TestOfflineObservationStillHasRetryBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	path := r.write("debug.log", "keep")
	rows, err := r.engine.Svc.CaptureObservations([]string{path}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	r.engine.Svc.FS = &unavailableObservationFS{FS: r.engine.Svc.FS, path: path, volumeError: "offline"}
	r.engine.Svc.Now = func() time.Time { return rows[0].ObservedAt.Add(7 * 24 * time.Hour) }
	if err := r.engine.ReplayObservations(); err == nil {
		t.Fatal("missing budget failure")
	}
	if r.engine.Svc.Store.HasDeferredObservations() {
		t.Fatal("expired observation kept")
	}
}

func TestReplayRetainsObservationWhenVolumeDisconnectsAfterPreflight(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	path := r.write("debug.log", "keep")
	rows, err := r.engine.Svc.CaptureObservations([]string{path}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	r.engine.Svc.FS = &unavailableObservationFS{FS: r.engine.Svc.FS, path: path, volumeError: "offline", onlineOnce: true}
	if err := r.engine.ReplayObservations(); err == nil {
		t.Fatal("volume loss during observation was treated as disappearance")
	}
	pending, err := r.engine.Svc.Store.DeferredObservations()
	if err != nil || len(pending) != 1 || pending[0].Attempts != 1 {
		t.Fatalf("observation lost: %+v %v", pending, err)
	}
}

func TestReplayDistinguishesOtherVolumeFromReplacement(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, ending := range []string{"volume-returns", "twenty-attempts", "seven-days", "same-volume-replacement"} {
		t.Run(ending, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			now := time.Now()
			r.engine.Svc.Now = func() time.Time { return now }
			path := r.write("debug.log", "keep")
			rows, err := r.engine.Svc.CaptureObservations([]string{path}, "s")
			if err != nil {
				t.Fatal(err)
			}
			if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
				t.Fatal(err)
			}
			fs := r.engine.Svc.FS
			replaced := ending == "same-volume-replacement"
			content := "keep"
			if replaced {
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				content = "replacement"
				r.write("debug.log", content)
			} else {
				r.engine.Svc.FS = changedObservationFS{FS: fs, path: path}
			}
			err = r.engine.ReplayObservations()
			pending, readErr := r.engine.Svc.Store.DeferredObservations()
			if replaced {
				if err != nil || readErr != nil || len(pending) != 0 {
					t.Fatalf("same-volume replacement was not skipped: %+v %v %v", pending, err, readErr)
				}
			} else if err == nil || readErr != nil || len(pending) != 1 || pending[0].Attempts != 1 || pending[0].LastError == "" || pending[0].Token != rows[0].Token {
				t.Fatalf("different volume discarded observation: %+v %v %v", pending, err, readErr)
			}
			st, err := r.engine.Svc.Store.Read()
			if err != nil || len(st.Items) != 0 || len(st.Proposals) != 0 {
				t.Fatalf("replacement was registered or proposed: %+v %v", st, err)
			}
			switch ending {
			case "volume-returns":
				r.engine.Svc.FS = fs
				if err := r.engine.ReplayObservations(); err != nil {
					t.Fatal(err)
				}
				st, err = r.engine.Svc.Store.Read()
				if err != nil || len(st.Proposals) != 1 {
					t.Fatalf("returned volume was not replayed: %+v %v", st.Proposals, err)
				}
			case "twenty-attempts", "seven-days":
				attempts := 20
				if ending == "seven-days" {
					now = now.Add(7 * 24 * time.Hour)
					attempts = 2
				}
				for attempt := 2; attempt <= attempts; attempt++ {
					if err := r.engine.ReplayObservations(); err == nil {
						t.Fatal("missing retry error")
					}
					pending, err := r.engine.Svc.Store.DeferredObservations()
					if err != nil || attempt < attempts && (len(pending) != 1 || pending[0].Attempts != attempt) {
						t.Fatalf("retry budget not respected: %+v %v", pending, err)
					}
				}
			}
			if r.engine.Svc.Store.HasDeferredObservations() {
				t.Fatal("completed or expired observation retained")
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != content {
				t.Fatalf("same-name file changed: %q %v", data, err)
			}
		})
	}
}

func TestReplayRetryLimitAndAge(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, age := range []bool{false, true} {
		t.Run(map[bool]string{false: "twenty-attempts", true: "seven-days"}[age], func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			now := time.Now()
			r.engine.Svc.Now = func() time.Time { return now }
			bad := r.write("bad.log", "retry")
			rows, err := r.engine.Svc.CaptureObservations([]string{bad}, "s")
			if err != nil {
				t.Fatal(err)
			}
			if err := r.engine.Svc.Store.DeferObservations(rows); err != nil {
				t.Fatal(err)
			}
			r.engine.Svc.FS = &unreadableObservationFS{FS: r.engine.Svc.FS, path: bad}
			attempts := 20
			if age {
				now = now.Add(7 * 24 * time.Hour)
				attempts = 1
			}
			for i := 1; i <= attempts; i++ {
				if err := r.engine.ReplayObservations(); err == nil {
					t.Fatal("retry error lost")
				}
				pending, err := r.engine.Svc.Store.DeferredObservations()
				if err != nil {
					t.Fatal(err)
				}
				if i < attempts && (len(pending) != 1 || pending[0].Attempts != i || pending[0].LastError == "") {
					t.Fatalf("retry not persisted: %+v", pending)
				}
				if i == attempts && len(pending) != 0 {
					t.Fatalf("unbounded retry: %+v", pending)
				}
			}
			status, err := r.engine.Svc.Store.InspectObservations()
			if err != nil || status.Dead != 1 {
				t.Fatalf("failed request not retained in dead queue: %+v %v", status, err)
			}
			events, err := r.engine.Svc.Store.History(100)
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 1 || events[0].Action != "observation dropped" {
				t.Fatalf("drop not logged: %+v", events)
			}
		})
	}
}

func TestReplayFailureDoesNotBlockOtherObservationsOrLeftovers(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	r.ask() // exercises the proposal itself, as "ask" mode keeps it
	bad := r.write("bad.log", "retry")
	if err := r.engine.Snapshot(r.root, "s"); err != nil {
		t.Fatal(err)
	}
	good := r.write("good.log", "register")
	r.leftover("node_modules")
	if err := r.engine.Svc.Store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(r.root): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	queued, err := r.engine.Svc.CaptureObservations([]string{bad, good}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.engine.Svc.Store.DeferObservations(queued); err != nil {
		t.Fatal(err)
	}
	r.engine.Svc.FS = &unreadableObservationFS{FS: r.engine.Svc.FS, path: bad}
	notices, err := r.engine.Finish(r.root, "s")
	if err != nil || len(notices) != 1 || notices[0].Proposal.Kind != core.ProposalLeftovers {
		t.Fatalf("failed observation blocked leftovers: %+v %v", notices, err)
	}
	rows, err := r.engine.Svc.Store.DeferredObservations()
	st, readErr := r.engine.Svc.Store.Read()
	if err != nil || readErr != nil || len(rows) != 1 || rows[0].Path != bad || len(st.Items) != 1 || st.Items[0].Path != good {
		t.Fatalf("retry lost or blocked healthy entry: queue=%+v items=%+v err=%v/%v", rows, st.Items, err, readErr)
	}
}

func TestFinishStillComputesLeftoversWhenQueueUnreadable(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	r.ask() // exercises the proposal itself, as "ask" mode keeps it
	if err := r.engine.Snapshot(r.root, "s"); err != nil {
		t.Fatal(err)
	}
	r.leftover("node_modules")
	if err := os.WriteFile(filepath.Join(r.engine.Svc.Store.Home, "observations.jsonl"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	notices, err := r.engine.Finish(r.root, "s")
	if err != nil || len(notices) != 1 || notices[0].Proposal.Kind != core.ProposalLeftovers {
		t.Fatalf("queue error prevented leftovers: %+v %v", notices, err)
	}
}
