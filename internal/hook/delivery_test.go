package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestDeferredObservationAndSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, next := range []string{"SessionStart", "PostToolUse", "Context"} {
		t.Run(next, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			t.Cleanup(func() {
				if t.Failed() {
					data, err := os.ReadFile(filepath.Join(e.engine.Svc.Store.Home, "hook.log"))
					t.Logf("hook.log: %s (%v)", data, err)
					rows, err := e.engine.Svc.Store.DeferredObservations()
					t.Logf("deferred observations: %+v (%v)", rows, err)
					paths, err := filepath.Glob(filepath.Join(e.engine.Svc.Store.Home, "sessions", "*.json"))
					t.Logf("snapshots: %v (%v)", paths, err)
					for _, path := range paths {
						data, err := os.ReadFile(path)
						t.Logf("snapshot: %s (%v)", data, err)
					}
				}
			})
			e.write("before.txt")
			agent := Agent{Name: "plugin", Adapter: BridgeAdapter{}}
			var out bytes.Buffer
			ev := Event{Name: "SessionStart", Cwd: e.root, Session: "plugin:s", DeferContext: true}
			RunEvent(agent, e.engine, ev, &out, &Runner{})
			ev.Name, ev.Paths = "PostToolUse", []string{e.write("debug.log")}
			RunEvent(agent, e.engine, ev, &out, &Runner{})
			// A busy hook can persist the write in the observation queue.
			// Replay it as the background runner does before checking delivery.
			pending, err := e.engine.Svc.Store.DeferredObservations()
			if err != nil {
				t.Fatal(err)
			}
			id, err := e.engine.Svc.FS.Identify(ev.Paths[0])
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range pending {
				if row.Path != ev.Paths[0] || row.Session != ev.Session || row.Identity == nil || *row.Identity != id {
					t.Fatalf("deferred write changed identity or scope: %+v", row)
				}
			}
			if st := e.state(); len(pending) > 1 || len(st.Proposals) == 0 && len(pending) != 1 || len(st.Items) != 0 || strings.Contains(out.String(), "context") {
				t.Fatalf("deferred write was not persisted silently: %+v queue=%+v output=%s", st, pending, out.String())
			}
			if err := e.engine.ReplayObservations(); err != nil {
				t.Fatal(err)
			}
			if st := e.state(); len(st.Proposals) != 1 || len(st.Proposals[0].Notified) != 0 || len(st.Items) != 0 || e.engine.Svc.Store.HasDeferredObservations() {
				t.Fatalf("deferred write lost or acknowledged: %+v %s", st, out.String())
			}
			e.write("fresh.txt")
			ev.Name, ev.Paths = "Stop", nil
			// An expired hook context makes the finish handoff independent of
			// machine speed. Background work uses the original engine below.
			ctx, cancel := context.WithDeadline(context.Background(), time.Time{})
			defer cancel()
			hookEngine, hookService := *e.engine, *e.engine.Svc
			hookService.Store = hookService.Store.WithContext(ctx)
			hookEngine.Svc = &hookService
			var finishRoot, finishSession string
			finishes := 0
			runner := &Runner{Finish: func(root, session string, final bool) {
				finishes++
				finishRoot, finishSession = root, session
				if root != e.root || session != ev.Session || final {
					t.Errorf("incorrect finish handoff: %s %s %v", root, session, final)
				}
			}}
			out.Reset()
			RunEvent(agent, &hookEngine, ev, &out, runner)
			if finishes != 1 || out.String() != string(agent.Adapter.Fallback(ev)) {
				t.Fatalf("finish handoff was not silent: calls=%d output=%s", finishes, out.String())
			}
			if st := e.state(); len(st.Proposals) != 1 {
				t.Fatalf("finish ran before background handoff: %+v", st.Proposals)
			}
			// The CLI runs Finish in a separate process without the hook's
			// one-second ledger deadline; run that work synchronously here.
			if _, err := e.engine.Finish(finishRoot, finishSession); err != nil {
				t.Fatal(err)
			}
			st := e.state()
			if len(st.Proposals) != 2 || strings.Contains(out.String(), "context") {
				t.Fatalf("deferred start lost snapshot: %+v %s", st.Proposals, out.String())
			}
			for _, p := range st.Proposals {
				if len(p.Notified) != 0 || strings.Contains(listCandidates(p), "before.txt") {
					t.Fatalf("snapshot or silent finish failed: %+v", p)
				}
			}
			out.Reset()
			ev.Name, ev.DeferContext = next, false
			RunEvent(agent, e.engine, ev, &out, &Runner{})
			if !strings.Contains(out.String(), "debug.log") || !strings.Contains(out.String(), "fresh.txt") {
				t.Fatalf("pending notice never delivered: %s", out.String())
			}
			for _, p := range e.state().Proposals {
				if !slices.Contains(p.Notified, ev.Session) {
					t.Fatalf("delivery not acknowledged: %+v", p)
				}
			}
			out.Reset()
			ev.Name = "PostToolUse"
			RunEvent(agent, e.engine, ev, &out, &Runner{})
			if strings.Contains(out.String(), "context") {
				t.Fatalf("notice repeated: %s", out.String())
			}
		})
	}
}

func TestDeferredWriteSurvivesLedgerTimeout(t *testing.T) {
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
			id, err := e.engine.Svc.FS.Identify(path)
			if err != nil {
				t.Fatal(err)
			}
			release, err := ledger.LockFile(filepath.Join(e.engine.Svc.Store.Home, "store.lock"), true)
			if err != nil {
				t.Fatal(err)
			}
			unlock := sync.OnceFunc(release)
			t.Cleanup(unlock)
			agent := Agent{Name: "plugin", Adapter: BridgeAdapter{}}
			ev := Event{Name: "PostToolUse", Cwd: e.root, Session: "plugin:s", Paths: []string{path}, DeferContext: true}
			var out bytes.Buffer
			RunEvent(agent, e.engine, ev, &out, &Runner{})
			unlock()
			if out.String() != string(agent.Adapter.Fallback(ev)) {
				t.Fatalf("timed-out observation produced context: %s", out.String())
			}
			pending, err := e.engine.Svc.Store.DeferredObservations()
			if err != nil || len(pending) != 1 || pending[0].Path != path || pending[0].Session != ev.Session || pending[0].Identity == nil || *pending[0].Identity != id {
				t.Fatalf("timed-out observation lost identity or scope: %+v %v", pending, err)
			}
			if st := e.state(); len(st.Items) != 0 || len(st.Proposals) != 0 {
				t.Fatalf("timed-out observation wrote the locked ledger: %+v", st)
			}
			for range 2 {
				if err := e.engine.ReplayObservations(); err != nil {
					t.Fatal(err)
				}
				st := e.state()
				if approved {
					if len(st.Items) != 1 || len(st.Proposals) != 0 || st.Items[0].Path != path || st.Items[0].Session != ev.Session || st.Items[0].Identity == nil || *st.Items[0].Identity != id {
						t.Fatalf("approved observation not registered exactly once: %+v", st)
					}
				} else if len(st.Items) != 0 || len(st.Proposals) != 1 || len(st.Proposals[0].Notified) != 0 || len(st.Proposals[0].Candidates) != 1 || st.Proposals[0].Candidates[0].Path != path {
					t.Fatalf("pending observation not proposed silently exactly once: %+v", st)
				}
				if e.engine.Svc.Store.HasDeferredObservations() {
					t.Fatal("replayed observation retained")
				}
			}
		})
	}
}

func TestDeferredWriteRegistersApprovedProject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	if err := e.engine.Svc.Store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(e.root): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	text, err := Handle(e.engine, Event{Name: "PostToolUse", Cwd: e.root, Session: "s", Paths: []string{e.write("debug.log")}, DeferContext: true}, nil)
	if err != nil || text != "" || len(e.state().Items) != 1 || len(e.state().Proposals) != 0 {
		t.Fatalf("deferred approved write was not registered: %q %v %+v", text, err, e.state())
	}
}

type deliveryWriter func([]byte) (int, error)

func (w deliveryWriter) Write(p []byte) (int, error) { return w(p) }

type encodingFailure struct{ BridgeAdapter }

func (encodingFailure) Encode(Event, Result) ([]byte, error) { return nil, errors.New("encode failed") }

func TestNotificationWaitsForOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, failure := range []string{"write", "short-write", "encode", "panic", "none"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			agent := Agent{Name: "plugin", Adapter: BridgeAdapter{}}
			ev := Event{Name: "PostToolUse", Cwd: e.root, Session: "plugin:s", Paths: []string{e.write("debug.log")}, DeferContext: true}
			RunEvent(agent, e.engine, ev, io.Discard, nil)
			ev.Name, ev.Paths, ev.DeferContext = "Context", nil, false
			if failure == "encode" {
				agent.Adapter = encodingFailure{}
			}
			writer := deliveryWriter(func(p []byte) (int, error) {
				if len(e.state().Proposals[0].Notified) != 0 {
					t.Fatal("notification acknowledged before output")
				}
				switch failure {
				case "write":
					return 0, errors.New("broken pipe")
				case "short-write":
					return len(p) / 2, nil
				case "panic":
					panic("output panic")
				}
				return len(p), nil
			})
			RunEvent(agent, e.engine, ev, writer, nil)
			if got := len(e.state().Proposals[0].Notified); (got == 1) != (failure == "none") {
				t.Fatalf("failed output acknowledged: %s notified=%d", failure, got)
			}
			agent.Adapter = BridgeAdapter{}
			var out bytes.Buffer
			RunEvent(agent, e.engine, ev, &out, nil)
			if strings.Contains(out.String(), "debug.log") != (failure != "none") {
				t.Fatalf("pending output not retried: %s", out.String())
			}
		})
	}
}

func TestPendingDeliveryBudgetAndScope(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e, other := newEnv(t), newEnv(t)
	for i := 0; i < 9; i++ {
		if _, _, _, err := e.engine.Svc.Propose(core.ProposalLeftovers, e.root, "seed", []core.Candidate{{Path: e.write(fmt.Sprintf("pending-%d.txt", i)), Lifespan: "1d"}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, _, err := e.engine.Svc.Propose(core.ProposalLeftovers, other.root, "other", []core.Candidate{{Path: other.write("other.txt"), Lifespan: "1d"}}); err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	for range 9 {
		text, err := Handle(e.engine, Event{Name: "Context", Cwd: e.root, Session: "s"}, nil)
		if err != nil || len(text) > maxContext || strings.Contains(text, "other.txt") {
			t.Fatalf("pending context scope/budget: %s %v", text, err)
		}
		for _, p := range e.state().Proposals {
			if !seen[p.ID] && slices.Contains(p.Notified, "s") {
				if !strings.Contains(text, describeProposal(p, "")) {
					t.Fatalf("truncated notice acknowledged: %+v %s", p, text)
				}
				seen[p.ID] = true
			}
		}
	}
	for _, p := range e.state().Proposals {
		if slices.Contains(p.Notified, "s") != (p.Project == e.root) {
			t.Fatalf("proposal starved or crossed scope: %+v", p)
		}
	}
}

func TestPluginDeferredContextChannels(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	// Host channels: Hermes pre_llm_call context; OpenClaw granted
	// before_prompt_build; Kilo experimental.chat.system.transform prompt array.
	for name, host := range map[string]string{"hermes": "pre_llm_call", "openclaw": "before_prompt_build", "kilocode": "experimental.chat.system.transform"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			agent, _ := Lookup(name)
			var input BridgeRequest
			if name == "openclaw" {
				success := true
				input = BridgeRequest{Version: 1, Agent: name, Event: "PostToolUse", HostEvent: "after_tool_call", Tool: "write", Session: "s", Local: true, Success: &success}
			} else {
				input = pluginBridgeFixture(t, name, "post")
			}
			input.Cwd, input.Paths = e.root, []string{e.write("debug.log")}
			run := func() string {
				t.Helper()
				raw, _ := json.Marshal(input)
				ev, err := Decode(agent.Adapter, raw, Invocation{Agent: name})
				if err != nil {
					t.Fatal(err)
				}
				var out bytes.Buffer
				RunEvent(agent, e.engine, ev, &out, &Runner{})
				return out.String()
			}
			if out := run(); strings.Contains(out, "context") || len(e.state().Proposals) != 1 {
				t.Fatalf("observation lost: %s %+v", out, e.state().Proposals)
			}
			input.Event, input.HostEvent, input.Paths = "Context", host, []string{e.write("not-observed.log")}
			input.Success = nil
			input.ContextDelivery = false
			if out := run(); strings.Contains(out, "context") || len(e.state().Proposals[0].Notified) != 0 {
				t.Fatalf("unconfirmed context delivered: %s", out)
			}
			input.ContextDelivery = true
			if out := run(); !strings.Contains(out, "debug.log") || len(e.state().Proposals[0].Notified) != 1 || len(e.state().Proposals[0].Candidates) != 1 {
				t.Fatalf("context channel failed: %s %+v", out, e.state().Proposals)
			}
			// A notification callback must not manufacture a session baseline.
			e.write("fresh.txt")
			if _, err := e.engine.Finish(e.root, name+":"+input.Session); err != nil || len(e.state().Proposals) != 1 {
				t.Fatalf("context callback took a snapshot: %+v %v", e.state().Proposals, err)
			}
		})
	}
}
