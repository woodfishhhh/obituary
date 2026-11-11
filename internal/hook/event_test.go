package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestHooksNeverExecuteDocker(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository and fake Docker executable")
	}
	e := newEnv(t)
	executable, state, log := testenv.FakeDocker(t)
	e.engine.Svc.Docker = docker.CLI{Executable: executable}
	testenv.WriteDocker(t, state)
	for _, event := range []string{"PreToolUse", "SessionStart", "PostToolUse", "Stop", "SessionEnd"} {
		e.event(map[string]any{"hook_event_name": event, "tool_name": "Bash", "tool_input": map[string]string{"command": "docker run alpine"}})
		if calls := testenv.DockerCalls(t, log); len(calls) != 0 {
			t.Fatalf("hook executed Docker: %s %v", event, calls)
		}
	}
}

func TestFirstWriteProposesInsteadOfRegistering(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	if ctx := e.event(map[string]any{"hook_event_name": "SessionStart", "source": "startup"}); ctx != "" {
		t.Fatalf("nothing to say yet, got %q", ctx)
	}
	ctx := e.wrote(e.write("debug.log"))
	st := e.state()
	if len(st.Items) != 0 || len(st.Proposals) != 1 || st.Proposals[0].Kind != core.ProposalOnboard {
		t.Fatalf("want one onboard proposal and nothing registered, got %+v", st)
	}
	id := st.Proposals[0].ID
	if !strings.Contains(ctx, fmt.Sprintf("obit approve %d", id)) || !strings.Contains(ctx, "debug.log") {
		t.Fatalf("the agent must hear about the proposal, got %q", ctx)
	}
	if again := e.wrote(e.write("second.log")); again != "" {
		t.Fatalf("the same session must not be told twice, got %q", again)
	}
	if n := len(e.state().Proposals[0].Candidates); n != 2 {
		t.Fatalf("the second artefact joins the open proposal, got %d candidates", n)
	}

	if _, err := e.engine.Approve([]string{fmt.Sprint(id)}); err != nil {
		t.Fatal(err)
	}
	if ctx := e.wrote(e.write("third.log")); ctx != "" {
		t.Fatalf("after approval writes are registered silently, got %q", ctx)
	}
	st = e.state()
	if len(st.Items) != 3 || len(st.Proposals) != 0 || st.Projects[core.ProjectKey(e.root)].Consent != core.ConsentApproved {
		t.Fatalf("want three registered items and consent, got items=%d proposals=%d projects=%+v", len(st.Items), len(st.Proposals), st.Projects)
	}
}

func TestRejectedProjectIsLeftAlone(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	e.wrote(e.write("debug.log"))
	if _, err := e.engine.Svc.Reject([]string{fmt.Sprint(e.state().Proposals[0].ID)}); err != nil {
		t.Fatal(err)
	}
	if ctx := e.wrote(e.write("again.log")); ctx != "" || len(e.state().Proposals) != 0 || len(e.state().Items) != 0 {
		t.Fatalf("a declined project gets no more proposals or registrations")
	}
}

func TestLeftoversAreProposedNotRegistered(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	e.write("before.txt") // untracked before the session: not its doing
	e.event(map[string]any{"hook_event_name": "SessionStart", "source": "startup"})
	e.write("new_feature.go")    // fresh work, not yet committed
	e.write("scratch/notes.txt") // a whole new directory
	e.finishEvent("Stop", "s1")
	st := e.state()
	if len(st.Items) != 0 || len(st.Proposals) != 1 || st.Proposals[0].Kind != core.ProposalLeftovers {
		t.Fatalf("want a leftovers proposal and nothing registered, got %+v", st)
	}
	var paths []string
	for _, c := range st.Proposals[0].Candidates {
		paths = append(paths, filepath.Base(c.Path))
	}
	if strings.Join(paths, ",") != "new_feature.go,scratch" {
		t.Fatalf("leftovers %v; before.txt predates the session", paths)
	}
	next := e.event(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s2"})
	if !strings.Contains(next, "new_feature.go") || !strings.Contains(next, "obit approve") {
		t.Fatalf("the next session must hear about the leftovers, got %q", next)
	}
}

func TestLeftoversSurviveInlineFinishDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("session difference uses a real Git repository")
	}
	t.Parallel()
	for _, next := range []string{"Stop", "SessionEnd"} {
		t.Run(next, func(t *testing.T) {
			e := newEnv(t)
			e.write("before.txt")
			if err := e.engine.Snapshot(e.root, "s1"); err != nil {
				t.Fatal(err)
			}
			path := e.write("new_feature.go")
			engine, svc := *e.engine, *e.engine.Svc
			// Model Git/filesystem work having exhausted the hook's shared
			// deadline, without a sleep or changing the production budget.
			svc.Store = svc.Store.WithDeadline(time.Now().Add(-time.Second))
			engine.Svc = &svc
			got, err := handle(&engine, Event{Name: "Stop", Cwd: e.root, Session: "s1"}, nil, true)
			if got.text() != "" || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("inline finish did not report its deadline: %q %v", got.text(), err)
			}
			if st := e.state(); len(st.Items) != 0 || len(st.Proposals) != 0 {
				t.Fatalf("expired finish changed the ledger: %+v", st)
			}
			e.finishEvent(next, "s1")
			st := e.state()
			if len(st.Items) != 0 || len(st.Proposals) != 1 || st.Proposals[0].Kind != core.ProposalLeftovers ||
				len(st.Proposals[0].Candidates) != 1 || st.Proposals[0].Candidates[0].Path != path {
				t.Fatalf("next ending event lost the original session difference: %+v", st)
			}
		})
	}
}

func TestEndingEventsDispatchWithExpiredStoreDeadline(t *testing.T) {
	t.Parallel()
	// Project discovery only needs the marker; dispatch must never invoke Git.
	root, home := t.TempDir(), filepath.Join(t.TempDir(), "unused-home")
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}.WithDeadline(time.Now().Add(-time.Second))}}
	for _, name := range []string{"Stop", "SessionEnd"} {
		calls, ticks := 0, 0
		runner := &Runner{
			Finish: func(gotRoot, session string, final bool) {
				if gotRoot != root || session != "s1" || final != (name == "SessionEnd") {
					t.Fatalf("wrong finish job: %q %q final=%v", gotRoot, session, final)
				}
				calls++
			},
			Tick: func() { ticks++ },
		}
		got, err := handle(e, Event{Name: name, Cwd: root, Session: "s1"}, runner, true)
		wantTicks := 0
		if name == "SessionEnd" {
			wantTicks = 1
		}
		if err != nil || got.text() != "" || calls != 1 || ticks != wantTicks {
			t.Fatalf("%s dispatch: context=%q err=%v jobs=%d ticks=%d", name, got.text(), err, calls, ticks)
		}
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("dispatch touched the ledger: %v", err)
	}
}

func TestHookFailsOpen(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	e := newEnv(t)
	for _, in := range []string{"", "not json", `{"hook_event_name":"PostToolUse"}`, `{"hook_event_name":"Unknown","cwd":"x","session_id":"s"}`} {
		var out bytes.Buffer
		Claude(e.engine, strings.NewReader(in), &out, nil)
		if out.Len() != 0 {
			t.Errorf("input %q produced output %q", in, out.String())
		}
	}
	t.Setenv("OBIT_DISABLED", "1")
	if ctx := e.wrote(e.write("debug.log")); ctx != "" || len(e.state().Proposals) != 0 {
		t.Fatal("OBIT_DISABLED must turn the hook off")
	}
}

func TestCommittedRuleItemIsNotBuried(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	e.engine.Svc.Owner = auto.Owner
	log := e.write("keep-this.log")
	if _, err := e.engine.Apply([]string{log}, auto.Options{}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-f", "keep-this.log"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "c"}} {
		if out, err := exec.Command("git", append([]string{"-C", e.root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	err := e.engine.Svc.Store.Update(func(st *core.State) error {
		st.Items[0].ExpiresAt = st.Items[0].CreatedAt // due now
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.engine.Svc.Sweep(false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionRefused {
		t.Fatalf("a rule item committed since must be refused, got %+v %v", out, err)
	}
	if _, err := os.Stat(log); err != nil {
		t.Fatal("the committed file must stay")
	}
}

func TestObservationIdentityCapturedBeforeLedgerWait(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	path := e.write("debug.log")
	if err := e.engine.Svc.Store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(e.root): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	replaced := false
	e.engine.Svc.FS = &replaceOnCaptureFS{FS: e.engine.Svc.FS, path: path, replace: func() {
		if err := os.Rename(path, path+".kept"); err != nil {
			t.Fatal(err)
		}
		e.write("debug.log")
		replaced = true
	}}
	release, err := core.LockFile(filepath.Join(e.engine.Svc.Store.Home, "store.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Handle(e.engine, Event{Name: "PostToolUse", Cwd: e.root, Session: "s", Paths: []string{path, path + ".missing"}}, nil)
	release()
	if !errors.Is(err, context.DeadlineExceeded) || !replaced {
		t.Fatalf("did not capture before waiting: %v %v", replaced, err)
	}
	rows, err := e.engine.Svc.Store.DeferredObservations()
	if err != nil || len(rows) != 1 {
		t.Fatalf("missing object queued: %+v %v", rows, err)
	}
	if err := e.engine.ReplayObservations(); err != nil {
		t.Fatal(err)
	}
	if st := e.state(); len(st.Items) != 0 || len(st.Proposals) != 0 {
		t.Fatalf("replacement registered: %+v", st)
	}
}

func TestDeferredObservationDoesNotRegisterReplacement(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	path := e.write("debug.log")
	if err := e.engine.Svc.Store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(e.root): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	release, err := core.LockFile(filepath.Join(e.engine.Svc.Store.Home, "store.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Handle(e.engine, Event{Name: "PostToolUse", Cwd: e.root, Session: "s", Paths: []string{path}}, nil)
	release()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".kept"); err != nil {
		t.Fatal(err)
	}
	e.write("debug.log")
	if err := e.engine.ReplayObservations(); err != nil {
		t.Fatal(err)
	}
	if st := e.state(); len(st.Items) != 0 || len(st.Proposals) != 0 || e.engine.Svc.Store.HasDeferredObservations() {
		t.Fatalf("replacement registered or event not retired: %+v", st)
	}
}

func TestReplayRejectsReplacementAfterInitialIdentityCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, approved := range []bool{false, true} {
		t.Run(map[bool]string{false: "proposal", true: "registration"}[approved], func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			path := e.write("debug.log")
			if approved {
				if err := e.engine.Svc.Store.Update(func(st *core.State) error {
					st.Projects = map[string]core.ProjectState{core.ProjectKey(e.root): {Consent: core.ConsentApproved}}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := e.engine.Svc.CaptureObservations([]string{path}, "s")
			if err != nil {
				t.Fatal(err)
			}
			if err := e.engine.Svc.Store.DeferObservations(rows); err != nil {
				t.Fatal(err)
			}
			e.engine.Svc.FS = &replaceOnCaptureFS{FS: e.engine.Svc.FS, path: path, replace: func() {
				if err := os.Rename(path, path+".kept"); err != nil {
					t.Fatal(err)
				}
				e.write("debug.log")
			}}
			if err := e.engine.ReplayObservations(); err != nil {
				t.Fatal(err)
			}
			if st := e.state(); len(st.Items) != 0 || len(st.Proposals) != 0 || e.engine.Svc.Store.HasDeferredObservations() {
				t.Fatalf("replay accepted replacement after initial check: %+v", st)
			}
		})
	}
}

func TestContendedObservationRejectsReplacementDuringReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, approved := range []bool{false, true} {
		t.Run(fmt.Sprint(approved), func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			path := e.write("debug.log")
			if err := e.engine.Svc.Store.Update(func(st *core.State) error {
				if approved {
					st.Projects = map[string]core.ProjectState{core.ProjectKey(e.root): {Consent: core.ConsentApproved}}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			release, err := core.LockFile(filepath.Join(e.engine.Svc.Store.Home, "store.lock"), true)
			if err != nil {
				t.Fatal(err)
			}
			unlock := sync.OnceFunc(release)
			t.Cleanup(unlock)
			replaced := false
			e.engine.Svc.FS = &replaceOnCaptureFS{FS: e.engine.Svc.FS, path: path, replace: func() {
				if err := os.Rename(path, path+".kept"); err != nil {
					t.Fatal(err)
				}
				e.write("debug.log")
				replaced = true
			}}
			_, err = Handle(e.engine, Event{Name: "PostToolUse", Cwd: e.root, Session: "s", Paths: []string{path}}, nil)
			if !errors.Is(err, context.DeadlineExceeded) || !replaced || !e.engine.Svc.Store.HasDeferredObservations() {
				t.Fatalf("busy ledger did not queue the captured object: %v", err)
			}
			unlock()
			if err := e.engine.ReplayObservations(); err != nil {
				t.Fatal(err)
			}
			if st := e.state(); len(st.Items) != 0 || len(st.Proposals) != 0 || e.engine.Svc.Store.HasDeferredObservations() {
				t.Fatalf("normal observation authorized replacement: items=%d proposals=%d", len(st.Items), len(st.Proposals))
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal("replacement moved", err)
			}
		})
	}
}

func TestTimedOutObservationQueuesAlternatePathSpellings(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, spelling := range []string{"slash", "backslash", "dotdot"} {
		t.Run(spelling, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			path := e.write("debug.log")
			input := path
			switch spelling {
			case "slash":
				input = filepath.ToSlash(path)
			case "dotdot":
				input = filepath.ToSlash(e.root) + "/unused/../debug.log"
			}
			if err := e.engine.Svc.Store.Update(func(*core.State) error { return nil }); err != nil {
				t.Fatal(err)
			}
			release, err := core.LockFile(filepath.Join(e.engine.Svc.Store.Home, "store.lock"), true)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Handle(e.engine, Event{Name: "PostToolUse", Cwd: e.root, Session: "s", Paths: []string{input}}, nil)
			release()
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
			rows, err := e.engine.Svc.Store.DeferredObservations()
			if err != nil || len(rows) != 1 || rows[0].Path != path {
				t.Fatalf("event dropped: %+v %v", rows, err)
			}
			if err := e.engine.ReplayObservations(); err != nil {
				t.Fatal(err)
			}
			if st := e.state(); len(st.Proposals) != 1 || e.engine.Svc.Store.HasDeferredObservations() {
				t.Fatal("event not replayed")
			}
		})
	}
}

func TestHookReturnsQuietlyWhenStoreLocked(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, event := range []string{"SessionStart", "PostToolUse"} {
		t.Run(event, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			path := e.write("debug.log")
			if err := os.MkdirAll(e.engine.Svc.Store.Home, 0o700); err != nil {
				t.Fatal(err)
			}
			release, err := core.LockFile(filepath.Join(e.engine.Svc.Store.Home, "store.lock"), true)
			if err != nil {
				t.Fatal(err)
			}
			payload, _ := json.Marshal(map[string]any{"hook_event_name": event, "session_id": "locked", "cwd": e.root,
				"tool_input": map[string]any{"file_path": path}})
			var out bytes.Buffer
			done := make(chan struct{})
			go func() {
				Claude(e.engine, bytes.NewReader(payload), &out, &Runner{})
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(1400 * time.Millisecond):
				t.Error("hook exceeded its bounded store wait")
			}
			release()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("hook did not finish after unlock")
			}
			data, err := os.ReadFile(filepath.Join(e.engine.Svc.Store.Home, "hook.log"))
			if out.Len() != 0 || err != nil || !strings.Contains(string(data), "deadline") {
				t.Fatalf("contended hook must be silent and log timeout: %q, %q, %v", out.String(), data, err)
			}
			if st := e.state(); len(st.Items) != 0 || len(st.Proposals) != 0 {
				t.Fatalf("timed out hook continued mutating: %+v", st)
			}
		})
	}
}

func TestNextHookReplaysQueuedWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("uses a real Git repository")
	}
	e := newEnv(t)
	path := e.write("debug.log")
	rows, err := e.engine.Svc.CaptureObservations([]string{path}, "earlier")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.engine.Svc.Store.DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	if _, err := Handle(e.engine, Event{Name: "Context", Cwd: e.root, Session: "later"}, &Runner{}); err != nil {
		t.Fatal(err)
	}
	st := e.state()
	if len(st.Proposals) != 1 || st.Proposals[0].Candidates[0].Path != path || e.engine.Svc.Store.HasDeferredObservations() {
		t.Fatalf("next hook did not replay: %+v", st)
	}
}

func TestContextNeverFloodsTheConversation(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("Obituary proposal #1: something rather long, ", 200)
	if got := capContext(long); len(got) > maxContext || !strings.Contains(got, "obit proposals") {
		t.Fatalf("capped context is %d bytes", len(got))
	}
	if got := capContext("short"); got != "short" {
		t.Fatalf("short context must pass through, got %q", got)
	}
}

func TestContextCapPreservesChineseUTF8(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"", "中", "中a", "中ab"} {
		got := capContext(prefix + strings.Repeat("中文遗留文件", 600))
		if len(got) > maxContext || !utf8.ValidString(got) || !strings.Contains(got, "obit proposals") {
			t.Errorf("invalid cap: %d bytes, valid UTF-8=%v", len(got), utf8.ValidString(got))
		}
	}
}

func TestNoticeRedeliveredAfterContextOrNotificationTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, write := range []bool{false, true} {
		t.Run(map[bool]string{false: "context", true: "notification"}[write], func(t *testing.T) {
			e := newEnv(t)
			ev := Event{Name: "PostToolUse", Cwd: e.root, Session: "s", Paths: []string{e.write("debug.log")}}
			previous := proposalConsent
			t.Cleanup(func() { proposalConsent = previous })
			var release func()
			proposalConsent = func(svc *core.Service, root string) (string, error) {
				if !write {
					return "", context.DeadlineExceeded
				}
				consent, err := previous(svc, root)
				if err == nil {
					release, err = core.LockFile(filepath.Join(svc.Store.Home, "store.lock"), false)
				}
				return consent, err
			}
			text, err := Handle(e.engine, ev, nil)
			if release != nil {
				release()
			}
			proposalConsent = previous
			if write && (text == "" || err != nil) || !write && (text != "" || !errors.Is(err, context.DeadlineExceeded)) {
				t.Fatalf("first failure: %q %v", text, err)
			}
			text, err = Handle(e.engine, ev, nil)
			if err != nil || text == "" {
				t.Fatalf("pending notice never delivered: %q %v", text, err)
			}
			text, err = Handle(e.engine, ev, nil)
			if err != nil || text != "" {
				t.Fatalf("notice repeated: %q %v", text, err)
			}
		})
	}
}

func TestRejectedLeftoversStayRejectedAcrossSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	e := newEnv(t)
	e.event(map[string]any{"hook_event_name": "SessionStart"})
	e.event(map[string]any{"hook_event_name": "SessionStart", "session_id": "s2"})
	e.write("work.txt")
	e.write("scratch/notes.txt")
	e.finishEvent("Stop", "s1")
	proposals := e.state().Proposals
	if len(proposals) == 0 {
		t.Fatal("Stop created no leftover proposal (a hook that skipped under load also lands here)")
	}
	p := proposals[0]
	// Ledger paths can differ in case from Git's spelling.
	if err := e.engine.Svc.Store.Update(func(st *core.State) error {
		for i := range st.Proposals[0].Candidates {
			st.Proposals[0].Candidates[i].Path = strings.ToUpper(st.Proposals[0].Candidates[i].Path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.engine.Svc.Reject([]string{fmt.Sprint(p.ID)}); err != nil {
		t.Fatal(err)
	}
	// A finish that read the old snapshot before Reject must not recreate it.
	if _, created, added, err := e.engine.Svc.Propose(p.Kind, p.Project, p.Session, p.Candidates); err != nil || created || added != 0 {
		t.Fatalf("concurrent stale finish recreated rejection: created=%v added=%d err=%v", created, added, err)
	}
	for _, event := range []string{"Stop", "SessionEnd"} {
		e.finishEvent(event, "s1")
		if st := e.state(); len(st.Proposals) != 0 || len(st.Items) != 0 {
			t.Fatalf("rejected leftovers reproposed: %+v", st)
		}
	}
	e.write("fresh.txt")
	e.finishEvent("Stop", "s1")
	st := e.state()
	if len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 1 || !strings.HasSuffix(st.Proposals[0].Candidates[0].Path, "fresh.txt") {
		t.Fatalf("new leftovers suppressed: %+v", st)
	}
	e.finishEvent("SessionEnd", "s2")
	st = e.state()
	if len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 1 || !strings.HasSuffix(st.Proposals[0].Candidates[0].Path, "fresh.txt") {
		t.Fatalf("another session repeated decided or pending leftovers: %+v", st)
	}
}

func TestContextReadTimeoutDoesNotMarkNotified(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, failAt := range []int{1, 2} {
		t.Run(fmt.Sprintf("read-%d", failAt), func(t *testing.T) {
			e, other := newEnv(t), newEnv(t)
			paths := []string{e.write("debug.log"), other.write("debug.log")}
			previous := proposalConsent
			t.Cleanup(func() { proposalConsent = previous })
			reads := 0
			proposalConsent = func(svc *core.Service, root string) (string, error) {
				reads++
				if reads == failAt {
					// Force the exact consent read to exhaust its shared lock budget.
					_, err := svc.Store.WithDeadline(time.Now()).Read()
					return "", err
				}
				return previous(svc, root)
			}
			text, err := Handle(e.engine, Event{Name: "PostToolUse", Cwd: e.root, Session: "retry", Paths: paths}, nil)
			if text != "" || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("timeout not reached: %q, %v", text, err)
			}
			st := e.state()
			if len(st.Proposals) != 2 {
				t.Fatalf("missing notices: %+v", st.Proposals)
			}
			for _, p := range st.Proposals {
				if len(p.Notified) != 0 {
					t.Fatalf("discarded context was acknowledged: %+v", p)
				}
			}
		})
	}
}

func TestNotificationWriteTimeoutDoesNotAcknowledgePreparedContext(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	e := newEnv(t)
	previous := proposalConsent
	t.Cleanup(func() { proposalConsent = previous })
	var release func()
	proposalConsent = func(svc *core.Service, root string) (string, error) {
		consent, err := previous(svc, root)
		if err != nil {
			return "", err
		}
		release, err = core.LockFile(filepath.Join(svc.Store.Home, "store.lock"), false)
		return consent, err
	}
	text, err := Handle(e.engine, Event{Name: "PostToolUse", Cwd: e.root, Session: "retry", Paths: []string{e.write("debug.log")}}, nil)
	if release != nil {
		release()
	}
	if text == "" || err != nil {
		t.Fatalf("notification write timeout: %q, %v", text, err)
	}
	st := e.state()
	if len(st.Proposals) != 1 || len(st.Proposals[0].Notified) != 0 {
		t.Fatalf("timed-out context acknowledged: %+v", st.Proposals)
	}
}
