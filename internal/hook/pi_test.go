package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func TestPiBridge(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testPluginBridge(t, "pi")
}

// Shared protocol assertions also exercise OMP's reused Pi decoder and the
// other TS bridge adapters. Host-to-bridge conversion belongs to plugin tests.
func testPluginBridge(t *testing.T, name string) {
	t.Helper()
	agent, ok := Lookup(name)
	if !ok {
		t.Fatalf("agent %s is not registered", name)
	}
	var fixture struct {
		Sources []string `json:"sources"`
		Note    string   `json:"note"`
		Cases   []struct {
			Name   string          `json:"name"`
			Host   json.RawMessage `json:"host"`
			Bridge BridgeRequest   `json:"bridge"`
		} `json:"cases"`
	}
	data, err := os.ReadFile(filepath.Join("testdata", name+"-protocol.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil || len(fixture.Sources) == 0 || fixture.Note == "" || len(fixture.Cases) == 0 {
		t.Fatalf("invalid sourced fixture: %v", err)
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			if !json.Valid(tc.Host) {
				t.Fatal("invalid host example")
			}
			raw := pluginJSON(t, tc.Bridge)
			ev, err := Decode(agent.Adapter, raw, Invocation{Agent: name})
			if err != nil || ev.Name != tc.Bridge.Event || ev.HostEvent != tc.Bridge.HostEvent || ev.Tool != tc.Bridge.Tool || ev.Session != name+":session-1" {
				t.Fatalf("bridge decode: %+v %v", ev, err)
			}
			if ev.Command != tc.Bridge.Command || ev.Shell != tc.Bridge.Shell {
				t.Fatalf("shell evidence changed: %+v", ev)
			}
			if ev.Name == "PostToolUse" && len(ev.Paths) != len(tc.Bridge.Paths) {
				t.Fatalf("successful write paths lost: %+v", ev)
			}
			for i, path := range ev.Paths {
				want, _ := NativePath(tc.Bridge.Paths[i])
				if path != want {
					t.Fatalf("path changed: %q != %q", path, want)
				}
			}
			bom, err := Decode(agent.Adapter, append([]byte("\xef\xbb\xbf"), raw...), Invocation{Agent: name})
			if err != nil || !reflect.DeepEqual(ev, bom) {
				t.Fatalf("BOM changed event: %+v %v", bom, err)
			}
		})
	}
	success := true
	base := BridgeRequest{Version: 1, Agent: name, Event: "PostToolUse", HostEvent: fixture.Cases[len(fixture.Cases)-1].Bridge.HostEvent,
		Tool: "write", Session: "session-1", Cwd: `C:\repo`, Local: true, Success: &success, Paths: []string{"debug.log"}, ContextDelivery: true}
	t.Run("reject-unconfirmed-input", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			edit func(*BridgeRequest)
		}{
			{"version", func(r *BridgeRequest) { r.Version = 2 }},
			{"agent", func(r *BridgeRequest) { r.Agent = "other" }},
			{"unknown-event", func(r *BridgeRequest) { r.Event = "unknown" }},
			{"no-session", func(r *BridgeRequest) { r.Session = "" }},
			{"failed-write", func(r *BridgeRequest) { failed := false; r.Success = &failed }},
			{"missing-success", func(r *BridgeRequest) { r.Success = nil }},
			{"remote", func(r *BridgeRequest) { r.Local = false }},
			{"remote-cwd", func(r *BridgeRequest) { r.Cwd = "/home/remote" }},
			{"relative-cwd", func(r *BridgeRequest) { r.Cwd = "repo" }},
			{"code-mode", func(r *BridgeRequest) { r.SkipReason = "code-mode" }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				input := base
				tc.edit(&input)
				raw := pluginJSON(t, input)
				ev, err := Decode(agent.Adapter, raw, Invocation{Agent: name})
				if err == nil || len(ev.Paths) != 0 {
					t.Fatalf("unconfirmed observation accepted: %+v %v", ev, err)
				}
				testPluginFallback(t, name, raw)
			})
		}
		for _, raw := range []string{"", "{", "null", "[]", `{"version":1,"paths":"debug.log"}`, "{} {}"} {
			if _, err := Decode(agent.Adapter, []byte(raw), Invocation{Agent: name}); err == nil {
				t.Fatalf("malformed payload accepted: %s", raw)
			}
			testPluginFallback(t, name, []byte(raw))
		}
		if _, err := Decode(agent.Adapter, pluginJSON(t, base), Invocation{Agent: name, Event: "PreToolUse"}); err == nil {
			t.Fatal("conflicting invocation event accepted")
		}
	})
	t.Run("shell-output-is-not-path-evidence", func(t *testing.T) {
		raw := bytes.TrimSuffix(pluginJSON(t, func() BridgeRequest { r := base; r.Paths = nil; return r }()), []byte("}"))
		raw = append(raw, []byte(`,"output":"Wrote C:/repo/forged.log","result":{"path":"C:/repo/forged.log"}}`)...)
		ev, err := Decode(agent.Adapter, raw, Invocation{Agent: name})
		if err != nil || len(ev.Paths) != 0 {
			t.Fatalf("output became paths: %+v %v", ev, err)
		}
	})
	t.Run("session-and-wsl-normalization", func(t *testing.T) {
		input := base
		input.Session, input.Cwd = strings.Repeat("s", 4097), "/mnt/c/repo"
		input.Paths = []string{"/mnt/c/repo/debug.log", "/home/remote/debug.log"}
		ev, err := Decode(agent.Adapter, pluginJSON(t, input), Invocation{Agent: name})
		if err != nil || ev.Cwd != `C:\repo` || !reflect.DeepEqual(ev.Paths, []string{`C:\repo\debug.log`}) || ev.Session != core.NormalizeSession(name+":"+input.Session) {
			t.Fatalf("normalization: %+v %v", ev, err)
		}
		again, _ := Decode(agent.Adapter, pluginJSON(t, input), Invocation{Agent: name})
		if again.Session != ev.Session {
			t.Fatal("session identity is unstable")
		}
	})
	t.Run("guard-evidence", func(t *testing.T) { testPluginGuard(t, agent, fixture.Cases[0].Bridge) })
	t.Run("response-schema", func(t *testing.T) {
		for _, event := range []string{"PreToolUse", "PostToolUse", "SessionStart", "Stop", "SessionEnd"} {
			for _, deferred := range []bool{false, true} {
				ev := Event{Name: event, DeferContext: deferred}
				data, err := agent.Adapter.Encode(ev, Result{Deny: true, Reason: "obit rm x && echo <done>", Context: "proposal"})
				if err != nil {
					t.Fatal(err)
				}
				got := pluginResponse(t, data)
				want := BridgeResponse{Version: 1}
				if event == "PreToolUse" {
					want.Result = Result{Deny: true, Reason: "obit rm x && echo <done>"}
				} else if (event == "PostToolUse" || event == "SessionStart") && !deferred {
					want.Context = "proposal"
				}
				if got != want || bytes.Contains(data, []byte(`\u003c`)) {
					t.Fatalf("response for %+v: %s", ev, data)
				}
			}
		}
		if got := pluginResponse(t, agent.Adapter.Fallback(Event{})); got != (BridgeResponse{Version: 1}) {
			t.Fatalf("fallback: %+v", got)
		}
	})
	t.Run("real-repository", func(t *testing.T) { testPluginRepository(t, agent, base) })
	for _, support := range []Support{agent.Capabilities.DeleteGuard, agent.Capabilities.WriteObservation, agent.Capabilities.SessionStart,
		agent.Capabilities.Stop, agent.Capabilities.SessionEnd, agent.Capabilities.Context, agent.Capabilities.MCP, agent.Capabilities.Skill} {
		if support.Level == Supported || support.Detail == "" {
			t.Fatalf("plugin installation overstated or limit unexplained: %+v", support)
		}
	}
}

func pluginJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func pluginResponse(t *testing.T, data []byte) BridgeResponse {
	t.Helper()
	var response BridgeResponse
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil || response.Version != 1 {
		t.Fatalf("invalid bridge response: %q %v", data, err)
	}
	return response
}

func testPluginFallback(t *testing.T, name string, raw []byte) {
	t.Helper()
	home := t.TempDir()
	engine := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}}}
	var out bytes.Buffer
	Run(name, engine, bytes.NewReader(raw), &out, nil)
	if got := pluginResponse(t, out.Bytes()); got != (BridgeResponse{Version: 1}) {
		t.Fatalf("fallback changed host behavior: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(home, "store.json")); !os.IsNotExist(err) {
		t.Fatalf("invalid payload opened ledger: %v", err)
	}
}

func testPluginGuard(t *testing.T, agent Agent, base BridgeRequest) {
	t.Helper()
	// The pre-tool fast path works with no filesystem service or git executable.
	t.Setenv("PATH", "")
	for _, tc := range []struct {
		name string
		edit func(*BridgeRequest)
		deny bool
	}{
		{"local-bash", func(r *BridgeRequest) {}, true},
		{"local-powershell", func(r *BridgeRequest) { r.Shell = guardrm.PowerShell; r.Command = "Remove-Item debug.log" }, true},
		{"without-session", func(r *BridgeRequest) { r.Session = "" }, true},
		{"unknown-shell", func(r *BridgeRequest) { r.Shell = "" }, false},
		{"ambiguous-shell", func(r *BridgeRequest) { r.Shell = guardrm.Uncertain }, false},
		{"remote", func(r *BridgeRequest) { r.Local = false }, false},
		{"remote-posix", func(r *BridgeRequest) { r.Cwd = "/workspace/repo" }, false},
		{"code-mode", func(r *BridgeRequest) { r.SkipReason = "code-mode runtime" }, false},
		{"long-command", func(r *BridgeRequest) { r.Command = "rm " + strings.Repeat("x", 5000) }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			input.Cwd = t.TempDir()
			tc.edit(&input)
			raw := pluginJSON(t, input)
			home := t.TempDir()
			engine := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}}}
			var out bytes.Buffer
			Run(agent.Name, engine, bytes.NewReader(raw), &out, nil)
			got := pluginResponse(t, out.Bytes())
			if got.Deny != tc.deny || got.Context != "" || len(got.Reason) > maxContext || !utf8.ValidString(got.Reason) {
				t.Fatalf("guard result: %+v", got)
			}
			if got.Deny && !strings.Contains(got.Reason, "restorable deletion") {
				t.Fatalf("missing suggestion: %+v", got)
			}
			if !bytes.Equal(raw, pluginJSON(t, input)) {
				t.Fatal("original command was mutated")
			}
			if _, err := os.Stat(filepath.Join(home, "store.json")); !os.IsNotExist(err) {
				t.Fatalf("pre tool opened ledger: %v", err)
			}
		})
	}
}

func lockPluginObservationStore(t *testing.T, e *env) func() {
	t.Helper()
	if _, err := e.engine.Svc.Store.Read(); err != nil {
		t.Fatal(err)
	}
	release, err := core.LockFile(filepath.Join(e.engine.Svc.Store.Home, "store.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	unlock := sync.OnceFunc(release)
	t.Cleanup(unlock)
	return unlock
}

func replayPluginObservation(t *testing.T, e *env, agent Agent, input BridgeRequest) {
	t.Helper()
	rows, err := e.engine.Svc.Store.DeferredObservations()
	if err != nil || len(rows) != 1 || rows[0].Path != input.Paths[0] || rows[0].Session != agent.Name+":"+input.Session {
		t.Fatalf("observation-only callback did not queue its write: %+v %v", rows, err)
	}
	id, err := e.engine.Svc.FS.Identify(input.Paths[0])
	if err != nil || rows[0].Identity == nil || *rows[0].Identity != id {
		t.Fatalf("queued write lost its captured identity: %+v %v", rows, err)
	}
	if st := e.state(); len(st.Proposals) != 0 || len(st.Items) != 0 {
		t.Fatalf("callback changed the locked ledger: %+v", st)
	}
	// Replay is background work and gets its own budget; the hook remains silent.
	if err := e.engine.ReplayObservations(); err != nil {
		t.Fatal(err)
	}
	st := e.state()
	if e.engine.Svc.Store.HasDeferredObservations() || !st.ObservationCompleted(&rows[0]) {
		t.Fatal("background replay did not acknowledge the queued write")
	}
}

func checkPluginPendingDelivery(t *testing.T, e *env, agent Agent, input BridgeRequest) {
	t.Helper()
	input.Event, input.Paths, input.ContextDelivery = "PostToolUse", nil, true
	if agent.Name == "openclaw" {
		input.Event, input.HostEvent, input.Success = "Context", "before_prompt_build", nil
	}
	var out bytes.Buffer
	Run(agent.Name, e.engine, bytes.NewReader(pluginJSON(t, input)), &out, &Runner{})
	response := pluginResponse(t, out.Bytes())
	st := e.state()
	if !strings.Contains(response.Context, "obit approve") || len(st.Proposals) != 1 || len(st.Proposals[0].Notified) != 1 || st.Proposals[0].Notified[0] != agent.Name+":"+input.Session {
		t.Fatalf("replayed plugin notice was not delivered: %s %+v", &out, st.Proposals)
	}
	out.Reset()
	Run(agent.Name, e.engine, bytes.NewReader(pluginJSON(t, input)), &out, &Runner{})
	if response := pluginResponse(t, out.Bytes()); response != (BridgeResponse{Version: 1}) {
		t.Fatalf("replayed plugin notice repeated: %+v", response)
	}
}

func testPluginRepository(t *testing.T, agent Agent, base BridgeRequest) {
	t.Helper()
	for _, deferred := range []bool{false, true} {
		t.Run(map[bool]string{false: "delivery", true: "deferred"}[deferred], func(t *testing.T) {
			e := newEnv(t)
			input := base
			input.Cwd, input.ContextDelivery = e.root, !deferred
			input.Paths = []string{e.write(strings.Repeat("测", 20) + ".log")}
			unlock := func() {}
			if deferred {
				unlock = lockPluginObservationStore(t, e)
			}
			var out bytes.Buffer
			Run(agent.Name, e.engine, bytes.NewReader(pluginJSON(t, input)), &out, &Runner{})
			unlock()
			response := pluginResponse(t, out.Bytes())
			if deferred {
				replayPluginObservation(t, e, agent, input)
			}
			if deferred || agent.Name == "openclaw" {
				state := e.state()
				if response != (BridgeResponse{Version: 1}) || len(state.Proposals) != 1 || len(state.Proposals[0].Notified) != 0 {
					t.Fatalf("observation-only callback lost or acknowledged notices: %s %+v", out.String(), state.Proposals)
				}
				if deferred {
					checkPluginPendingDelivery(t, e, agent, input)
				}
				return
			}
			if len(response.Context) == 0 || len(response.Context) > maxContext || !utf8.ValidString(response.Context) || !strings.Contains(response.Context, "obit approve") {
				log, _ := os.ReadFile(filepath.Join(e.engine.Svc.Store.Home, "hook.log"))
				t.Fatalf("context budget/delivery: %s; log: %s", out.String(), log)
			}
			state := e.state()
			if len(state.Proposals) != 1 || len(state.Proposals[0].Notified) != 1 || state.Proposals[0].Notified[0] != agent.Name+":session-1" {
				t.Fatalf("missing namespaced notice: %+v", state.Proposals)
			}
		})
	}
	t.Run("context-budget", func(t *testing.T) {
		e := newEnv(t)
		var paths []string
		for i := 0; i < 25; i++ {
			paths = append(paths, e.write(strings.Repeat("测", 20)+string(rune('a'+i))+".log"))
		}
		// Prepare the pending notice outside the hook's one-second lock budget.
		// This tests output delivery independently of a bulk Observe workload.
		if _, _, err := e.engine.Observe(paths, "seed"); err != nil {
			t.Fatal(err)
		}
		input := base
		input.Event, input.Cwd, input.Paths, input.Success = "SessionStart", e.root, nil, nil
		var out bytes.Buffer
		Run(agent.Name, e.engine, bytes.NewReader(pluginJSON(t, input)), &out, &Runner{})
		response := pluginResponse(t, out.Bytes())
		if agent.Name == "openclaw" {
			if response.Context != "" {
				t.Fatalf("observation-only callback emitted context: %s", out.String())
			}
			return
		}
		if len(response.Context) == 0 || len(response.Context) > maxContext || !utf8.ValidString(response.Context) || !strings.Contains(response.Context, "obit approve") {
			t.Fatalf("context budget: %s", out.String())
		}
	})
	for _, event := range []string{"SessionStart", "Stop", "SessionEnd"} {
		t.Run(event, func(t *testing.T) {
			e := newEnv(t)
			input := base
			input.Event, input.Cwd = event, e.root
			input.Paths, input.Success = nil, nil
			finishes, ticks := 0, 0
			runner := &Runner{Tick: func() { ticks++ }, Finish: func(root, session string, final bool) {
				finishes++
				if root != e.root || session != agent.Name+":session-1" || final != (event == "SessionEnd") {
					t.Fatalf("wrong handoff: %q %q %v", root, session, final)
				}
			}}
			var out bytes.Buffer
			Run(agent.Name, e.engine, bytes.NewReader(pluginJSON(t, input)), &out, runner)
			if response := pluginResponse(t, out.Bytes()); response != (BridgeResponse{Version: 1}) {
				t.Fatalf("lifecycle event emitted text: %+v", response)
			}
			if event == "SessionStart" {
				if ticks != 1 || finishes != 0 {
					t.Fatalf("start handoff: %d %d", ticks, finishes)
				}
			} else if finishes != 1 || ticks != map[bool]int{false: 0, true: 1}[event == "SessionEnd"] {
				t.Fatalf("ending handoff: %d %d", finishes, ticks)
			}
		})
	}
}
