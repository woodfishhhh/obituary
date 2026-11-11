package hook

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func isolateLatencyRunner(t *testing.T) {
	t.Helper()
	clitest.IsolateRunner(t)
	oldSpawn, oldOnce := cmdutil.SpawnSelf, cmdutil.RunOnce
	t.Cleanup(func() { cmdutil.SpawnSelf, cmdutil.RunOnce = oldSpawn, oldOnce })
	cmdutil.SpawnSelf = func(...string) error { return nil }
	cmdutil.RunOnce = func(string) error { return nil }
}

func readHookLog(t *testing.T, store core.Store) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(store.Home, "hook.log"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

// Only a diagnosed deadline may defer this sample; empty output alone is never
// success. Both durable states must deliver on the very next callback.
func checkPendingLatency(t *testing.T, store core.Store, root, agent, out, previousLog string) string {
	t.Helper()
	log := readHookLog(t, store)
	if log == previousLog {
		checkLatencyResult(t, store, "PostToolUse/pending", out)
		checkDeliveredProposal(t, store, root, "latency")
		return "immediate"
	}
	checkDeadlineLog(t, agent, strings.TrimPrefix(log, previousLog))
	if out != "" {
		t.Fatalf("deadline did not return silently: %q", out)
	}
	stage := checkDeferredProposal(t, store, root, agent)
	input, err := json.Marshal(map[string]string{"hook_event_name": "PostToolUse", "session_id": "latency", "cwd": root})
	if err != nil {
		t.Fatal(err)
	}
	out, err = executeHookPayload(t, string(input), agent)
	if err != nil {
		t.Fatal(err)
	}
	checkLatencyResult(t, store, "PostToolUse/pending", out)
	checkDeliveredProposal(t, store, root, "latency")
	if out, err = executeHookPayload(t, string(input), agent); err != nil || out != "" {
		t.Fatalf("proposal repeated: %q %v", out, err)
	}
	checkDeliveredProposal(t, store, root, "latency")
	if got := readHookLog(t, store); got != log {
		t.Fatalf("recovery callback failed open: %s", strings.TrimPrefix(got, log))
	}
	return stage
}

func checkDeadlineLog(t *testing.T, agent, log string) {
	t.Helper()
	_, message, found := strings.Cut(log, agent+" PostToolUse: ")
	quoted, _, _ := strings.Cut(message, "; session=")
	cause, err := strconv.Unquote(quoted)
	deadline := context.DeadlineExceeded.Error()
	gitDeadline := strings.HasPrefix(cause, "could not ask Git whether it is tracked: git ") &&
		strings.HasSuffix(cause, ": "+deadline) && !strings.Contains(cause, "\n")
	if !found || err != nil || strings.Count(log, "\n") != 1 ||
		(cause != deadline && cause != "lock store.lock: "+deadline && !gitDeadline) {
		t.Fatalf("unexpected hook diagnostic: %s", log)
	}
}

func checkDeferredProposal(t *testing.T, store core.Store, root, agent string) string {
	t.Helper()
	st, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := store.DeferredObservations()
	if err != nil || len(st.Items) != 0 {
		t.Fatalf("deferred write registered or queue unreadable: %+v %+v %v", st, rows, err)
	}
	if len(st.Proposals) == 1 {
		checkProposalScope(t, st.Proposals[0], root, "latency")
		if len(rows) != 0 || len(st.Proposals[0].Notified) != 0 || len(st.ObservationReceipts) != 1 {
			t.Fatalf("committed proposal was lost or acknowledged: %+v queue=%+v", st, rows)
		}
		return "notice"
	}
	if len(st.Proposals) != 0 || len(rows) != 1 || len(st.ObservationReceipts) != 0 {
		t.Fatalf("uncommitted write must retain exactly one observation: %+v queue=%+v", st, rows)
	}
	row := rows[0]
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	id, err := svc.FS.Identify(filepath.Join(root, "debug.log"))
	if err != nil || row.Identity == nil || *row.Identity != id || row.Path != filepath.Join(root, "debug.log") ||
		row.Project != root || row.Session != "latency" || row.Host != agent || row.Event != "PostToolUse" ||
		row.Token == "" || row.ObservedAt.IsZero() || row.Attempts != 0 || row.LastError != "" {
		t.Fatalf("deferred observation lost its identity or scope: %+v %v", row, err)
	}
	return "observation"
}

func checkProposalScope(t *testing.T, p core.Proposal, root, session string) {
	t.Helper()
	if p.Kind != core.ProposalOnboard || p.Project != root || p.Session != session ||
		len(p.Candidates) != 1 || p.Candidates[0].Path != filepath.Join(root, "debug.log") {
		t.Fatalf("proposal scope changed: %+v", p)
	}
}

func checkDeliveredProposal(t *testing.T, store core.Store, root, session string) {
	t.Helper()
	st, err := store.Read()
	if err != nil || len(st.Items) != 0 || len(st.Proposals) != 1 || len(st.ObservationReceipts) != 1 || store.HasDeferredObservations() {
		t.Fatalf("proposal delivery lost or duplicated the write: %+v %v", st, err)
	}
	checkProposalScope(t, st.Proposals[0], root, session)
	if !slices.Equal(st.Proposals[0].Notified, []string{session}) {
		t.Fatalf("proposal acknowledgement: %v", st.Proposals[0].Notified)
	}
}

func injectDeliveryDeadline(t *testing.T, store core.Store, stage string) func() {
	t.Helper()
	if stage == "locked" {
		release, err := core.LockFile(filepath.Join(store.Home, "store.lock"), true)
		if err != nil {
			t.Fatal(err)
		}
		unlock := sync.OnceFunc(release)
		t.Cleanup(unlock)
		return unlock
	}
	if stage != "committed" {
		return func() {}
	}
	delayed := false
	restore := core.SetFileSyncForTesting(func(*os.File) error {
		st, err := store.Inspect()
		if err != nil {
			return err
		}
		if !delayed && len(st.Proposals) == 1 {
			delayed = true
			// An in-flight commit finishes after the shared hook deadline;
			// its next context read must leave the notice unacknowledged.
			time.Sleep(time.Second)
		}
		return nil
	})
	t.Cleanup(restore)
	return func() {
		restore()
		if !delayed {
			t.Fatal("proposal commit deadline was not exercised")
		}
	}
}
