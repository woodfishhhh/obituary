package hook

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestHermesBridge(t *testing.T) {
	checkPluginBridge(t, "hermes", true)
}

func TestHermesLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	checkPluginLifecycle(t, "hermes", map[string]string{
		"on_session_start": "SessionStart", "on_session_end": "Stop",
	})
	in := pluginBridgeFixture(t, "hermes", "post")
	in.Event, in.HostEvent = "SessionEnd", "on_session_end"
	agent, _ := Lookup("hermes")
	if ev, err := Decode(agent.Adapter, pluginBridgeJSON(t, in), Invocation{Agent: "hermes"}); err == nil {
		t.Fatalf("turn end accepted as final teardown: %+v", ev)
	}
	if agent.Capabilities.SessionEnd.Level != Unsupported {
		t.Fatal("Hermes advertised final teardown")
	}
}

func TestHermesObservationOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	checkPluginDeferredObservation(t, "hermes")
}

// Plugin tests share bridge assertions; each agent supplies its official callback fixtures.
func pluginBridgeFixture(t *testing.T, agent, kind string) BridgeRequest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", agent+"-"+kind+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var in BridgeRequest
	if err := json.Unmarshal([]byte(testenv.NativeFixture(string(raw))), &in); err != nil {
		t.Fatal(err)
	}
	return in
}

func pluginBridgeJSON(t *testing.T, in BridgeRequest) []byte {
	t.Helper()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func checkPluginBridge(t *testing.T, name string, deferred bool) {
	t.Helper()
	agent, ok := Lookup(name)
	if !ok || agent.Capabilities.DeleteGuard.Level != Limited || agent.Capabilities.DeleteGuard.Detail == "" {
		t.Fatalf("missing limited bridge registration: %+v", agent)
	}
	for _, kind := range []string{"pre", "post"} {
		t.Run(kind+"-fixture", func(t *testing.T) {
			in := pluginBridgeFixture(t, name, kind)
			raw, err := os.ReadFile(filepath.Join("testdata", name+"-"+kind+".json"))
			if err != nil {
				t.Fatal(err)
			}
			raw = []byte(testenv.NativeFixture(string(raw)))
			ev, err := Decode(agent.Adapter, raw, Invocation{Agent: name})
			if err != nil || ev.Name != in.Event || ev.HostEvent != in.HostEvent || ev.Tool != in.Tool || ev.Session != name+":"+in.Session || ev.Command != in.Command {
				t.Fatalf("fixture: %+v %v", ev, err)
			}
			if kind == "post" && (!reflect.DeepEqual(ev.Paths, in.Paths) || ev.DeferContext != deferred) {
				t.Fatalf("write evidence: %+v", ev)
			}
			bom, err := Decode(agent.Adapter, append([]byte("\xef\xbb\xbf"), raw...), Invocation{Agent: name})
			if err != nil || !reflect.DeepEqual(ev, bom) {
				t.Fatalf("BOM changed event: %+v %v", bom, err)
			}
		})
	}
	post := pluginBridgeFixture(t, name, "post")
	t.Run("invocation-conflict", func(t *testing.T) {
		for _, invocation := range []Invocation{{Agent: "other"}, {Agent: name, Event: "Stop"}} {
			if ev, err := Decode(agent.Adapter, pluginBridgeJSON(t, post), invocation); err == nil {
				t.Fatalf("conflicting invocation accepted: %+v", ev)
			}
		}
	})
	t.Run("response-fields", func(t *testing.T) {
		data, err := agent.Adapter.Encode(Event{Name: "PreToolUse"}, Result{Deny: true, Reason: "obit rm 'a&b'", Context: "ignored"})
		var got map[string]any
		if err != nil || json.Unmarshal(data, &got) != nil || len(got) != 3 || got["deny"] != true || got["reason"] != "obit rm 'a&b'" || bytes.Contains(data, []byte(`\u0026`)) {
			t.Fatalf("unexpected denial fields: %s %v", data, err)
		}
		data, err = agent.Adapter.Encode(Event{Name: "PostToolUse"}, Result{Deny: true, Reason: "ignored", Context: "proposal"})
		var response BridgeResponse
		if err != nil || json.Unmarshal(data, &response) != nil || response.Deny || response.Reason != "" || response.Context != "proposal" {
			t.Fatalf("unexpected context response: %s %v", data, err)
		}
	})
	for _, tc := range []struct {
		name string
		edit func(*BridgeRequest)
	}{
		{"version", func(in *BridgeRequest) { in.Version++ }},
		{"agent", func(in *BridgeRequest) { in.Agent = "other" }},
		{"unknown-event", func(in *BridgeRequest) { in.Event = "unknown" }},
		{"unknown-host", func(in *BridgeRequest) { in.HostEvent = "unknown" }},
		{"missing-host", func(in *BridgeRequest) { in.HostEvent = "" }},
		{"event-conflict", func(in *BridgeRequest) { in.Event = "Stop" }},
		{"unknown-write-tool", func(in *BridgeRequest) { in.Tool = "custom_write" }},
		{"shell-output-path", func(in *BridgeRequest) { in.Tool = "terminal" }},
		{"read-tool-path", func(in *BridgeRequest) { in.Tool = "read_file" }},
		{"no-session", func(in *BridgeRequest) { in.Session = "" }},
		{"failed-write", func(in *BridgeRequest) { b := false; in.Success = &b }},
		{"unconfirmed-write", func(in *BridgeRequest) { in.Success = nil }},
		{"remote", func(in *BridgeRequest) { in.Local = false }},
		{"remote-cwd", func(in *BridgeRequest) { in.Cwd = testenv.ForeignFixture("/home/remote") }},
		{"relative-cwd", func(in *BridgeRequest) { in.Cwd = "repo" }},
		{"code-mode", func(in *BridgeRequest) { in.SkipReason = "code-mode" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := post
			tc.edit(&in)
			if ev, err := Decode(agent.Adapter, pluginBridgeJSON(t, in), Invocation{Agent: name}); err == nil || len(ev.Paths) != 0 {
				t.Fatalf("unverified observation accepted: %+v %v", ev, err)
			}
		})
	}
	t.Run("normalized-session", func(t *testing.T) {
		in := post
		in.Session = strings.Repeat("会话", 3000)
		raw := pluginBridgeJSON(t, in)
		a, err := Decode(agent.Adapter, raw, Invocation{Agent: name})
		b, again := Decode(agent.Adapter, raw, Invocation{Agent: name})
		if err != nil || again != nil || a.Session != b.Session || a.Session != core.NormalizeSession(name+":"+in.Session) {
			t.Fatalf("unstable session: %+v %+v %v %v", a, b, err, again)
		}
	})
	t.Run("fallback", func(t *testing.T) {
		e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
		for _, raw := range []string{"", "{", "null", "[]", `{"version":1,"paths":"bad"}`} {
			var out bytes.Buffer
			Run(name, e, strings.NewReader(raw), &out, &Runner{})
			if out.String() != "{\"version\":1}\n" {
				t.Fatalf("invalid neutral response: %q", out.String())
			}
		}
		if string(agent.Adapter.Fallback(Event{})) != "{\"version\":1}\n" {
			t.Fatal("zero event fallback changed")
		}
	})
	pre := pluginBridgeFixture(t, name, "pre")
	for _, tc := range []struct {
		name string
		edit func(*BridgeRequest)
		deny bool
	}{
		{"local-shell", func(*BridgeRequest) {}, true},
		{"no-session-guard", func(in *BridgeRequest) { in.Session = "" }, true},
		{"unknown-shell", func(in *BridgeRequest) { in.Shell = "" }, false},
		{"unknown-shell-tool", func(in *BridgeRequest) { in.Tool = "custom_shell" }, false},
		{"unsupported-shell", func(in *BridgeRequest) { in.Shell = "cmd" }, false},
		{"remote-guard", func(in *BridgeRequest) { in.Local = false }, false},
		{"code-mode-guard", func(in *BridgeRequest) { in.SkipReason = "code-mode" }, false},
		{"long-command", func(in *BridgeRequest) { in.Command = "rm " + strings.Repeat("字", 2400); in.Shell = "bash" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := pre
			in.Cwd = t.TempDir()
			if err := os.Mkdir(filepath.Join(in.Cwd, "scratch dir"), 0700); err != nil {
				t.Fatal(err)
			}
			tc.edit(&in)
			if tc.deny && in.Shell == "powershell" {
				testenv.WindowsOnly(t, "PowerShell Remove-Item integration is Windows-only")
			}
			e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
			// A pre-hook has no Git, service FS or configuration dependency.
			t.Setenv("PATH", t.TempDir())
			var out bytes.Buffer
			Run(name, e, bytes.NewReader(pluginBridgeJSON(t, in)), &out, nil)
			var got BridgeResponse
			if err := json.Unmarshal(out.Bytes(), &got); err != nil || got.Version != BridgeVersion || got.Deny != tc.deny || got.Context != "" || len(got.Reason) > maxContext || !utf8.ValidString(got.Reason) {
				t.Fatalf("guard response: %s %v", out.Bytes(), err)
			}
			if tc.deny && !strings.Contains(got.Reason, "restorable deletion") {
				t.Fatalf("missing complete denial: %s", got.Reason)
			}
			if !tc.deny && got.Reason != "" {
				t.Fatalf("skip carried a denial: %+v", got)
			}
		})
	}
	for _, event := range []string{"Stop", "SessionEnd"} {
		data, err := agent.Adapter.Encode(Event{Name: event}, Result{Deny: true, Reason: "deny", Context: "continue"})
		if err != nil || string(data) != string(agent.Adapter.Fallback(Event{})) {
			t.Fatalf("%s produced continuation: %s %v", event, data, err)
		}
	}
}

func checkPluginLifecycle(t *testing.T, name string, events map[string]string) {
	t.Helper()
	agent, _ := Lookup(name)
	for host, neutral := range events {
		t.Run(host, func(t *testing.T) {
			e := newEnv(t)
			in := pluginBridgeFixture(t, name, "post")
			in.HostEvent, in.Event, in.Cwd = host, neutral, e.root
			ev, err := Decode(agent.Adapter, pluginBridgeJSON(t, in), Invocation{Agent: name})
			if err != nil || ev.Name != neutral || ev.Session != name+":"+in.Session {
				t.Fatalf("lifecycle: %+v %v", ev, err)
			}
			if neutral == "SessionStart" {
				return
			}
			called := 0
			runner := &Runner{Tick: func() {}, Finish: func(root, session string, final bool) {
				called++
				if root != e.root || session != name+":"+in.Session || final != (neutral == "SessionEnd") {
					t.Fatalf("incorrect finish: %s %s %v", root, session, final)
				}
			}}
			var out bytes.Buffer
			RunEvent(agent, e.engine, ev, &out, runner)
			if called != 1 || out.String() != string(agent.Adapter.Fallback(ev)) {
				t.Fatalf("finish not silent: calls=%d output=%s", called, out.String())
			}
		})
	}
}

func checkPluginDeferredObservation(t *testing.T, name string) {
	t.Helper()
	e := newEnv(t)
	path := e.write("debug.log")
	in := pluginBridgeFixture(t, name, "post")
	in.Cwd, in.Paths, in.ContextDelivery = e.root, []string{path}, true
	agent, _ := Lookup(name)
	ev, err := Decode(agent.Adapter, pluginBridgeJSON(t, in), Invocation{Agent: name})
	if err != nil || !ev.DeferContext {
		t.Fatalf("observation-only callback allowed acknowledgement: %+v %v", ev, err)
	}
	var out bytes.Buffer
	RunEvent(agent, e.engine, ev, &out, &Runner{})
	if out.String() != string(agent.Adapter.Fallback(ev)) {
		t.Fatal(out.String())
	}
	if st := e.state(); len(st.Proposals) != 1 || len(st.Proposals[0].Notified) != 0 {
		t.Fatalf("deferred observation lost or acknowledged: %+v", st.Proposals)
	}
}
