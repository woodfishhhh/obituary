package hook

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"encoding/json"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func bridgePayload(t *testing.T, change func(*BridgeRequest)) []byte {
	t.Helper()
	success := true
	input := BridgeRequest{Version: BridgeVersion, Agent: "plugin", Event: "PostToolUse", HostEvent: "tool_result", Tool: "write",
		Session: "session", Cwd: testenv.NativeFixture(`C:\repo`), Local: true, Success: &success, Paths: []string{"debug.log"}, ContextDelivery: true}
	if change != nil {
		change(&input)
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestBridgeDecode(t *testing.T) {
	t.Parallel()
	adapter := BridgeAdapter{}
	raw := bridgePayload(t, nil)
	ev, err := adapter.Decode(raw, Invocation{Agent: "plugin"})
	if err != nil || ev.Name != "PostToolUse" || ev.HostEvent != "tool_result" || ev.Tool != "write" || ev.Session != "plugin:session" || ev.Cwd != testenv.NativeFixture(`C:\repo`) || !reflect.DeepEqual(ev.Paths, []string{"debug.log"}) || ev.DeferContext {
		t.Fatalf("decode: %+v %v", ev, err)
	}
	bom, err := Decode(adapter, append([]byte("\xef\xbb\xbf"), raw...), Invocation{Agent: "plugin"})
	if err != nil || !reflect.DeepEqual(ev, bom) {
		t.Fatalf("BOM changed event: %+v %v", bom, err)
	}
	raw = bridgePayload(t, func(in *BridgeRequest) {
		in.Cwd = "/mnt/c/repo"
		in.Paths = []string{"/mnt/c/repo/debug.log", "/home/remote.log", "other.log"}
		in.Session = strings.Repeat("s", 4097)
		in.ContextDelivery = false
	})
	ev, err = adapter.Decode(raw, Invocation{Agent: "plugin"})
	wantCwd, wantPaths := `C:\repo`, []string{`C:\repo\debug.log`, "other.log"}
	if runtime.GOOS != "windows" {
		wantCwd, wantPaths = "/mnt/c/repo", []string{"/mnt/c/repo/debug.log", "/home/remote.log", "other.log"}
	}
	if err != nil || ev.Cwd != wantCwd || !reflect.DeepEqual(ev.Paths, wantPaths) || !strings.HasPrefix(ev.Session, "sha256:") || len(ev.Session) != 71 || !ev.DeferContext {
		t.Fatalf("normalization: %+v %v", ev, err)
	}
	again, _ := adapter.Decode(raw, Invocation{Agent: "plugin"})
	if again.Session != ev.Session {
		t.Fatal("session identity changed")
	}
}

func TestBridgeRejectsUnconfirmedObservation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		edit func(*BridgeRequest)
	}{
		{"version", func(in *BridgeRequest) { in.Version++ }},
		{"agent", func(in *BridgeRequest) { in.Agent = "other" }},
		{"event", func(in *BridgeRequest) { in.Event = "unknown" }},
		{"session", func(in *BridgeRequest) { in.Session = "" }},
		{"local", func(in *BridgeRequest) { in.Local = false }},
		{"cwd", func(in *BridgeRequest) { in.Cwd = "relative" }},
		{"remote", func(in *BridgeRequest) {
			in.Cwd = "/home/user/repo"
			if runtime.GOOS != "windows" {
				in.Local = false
			}
		}},
		{"success-missing", func(in *BridgeRequest) { in.Success = nil }},
		{"failed", func(in *BridgeRequest) { b := false; in.Success = &b }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if ev, err := (BridgeAdapter{}).Decode(bridgePayload(t, tc.edit), Invocation{Agent: "plugin"}); err == nil || len(ev.Paths) != 0 {
				t.Fatalf("unsafe bridge accepted: %+v %v", ev, err)
			}
		})
	}
	for _, raw := range []string{"", "not-json", "null", `{"version":1,"paths":"output"}`} {
		if _, err := (BridgeAdapter{}).Decode([]byte(raw), Invocation{Agent: "plugin"}); err == nil {
			t.Fatalf("malformed input accepted: %s", raw)
		}
	}
	if _, err := (BridgeAdapter{}).Decode(bridgePayload(t, nil), Invocation{Agent: "plugin", Event: "PreToolUse"}); err == nil {
		t.Fatal("event conflict accepted")
	}
}

func TestBridgeGuardRequiresLocalShellEvidence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		local bool
		shell guardrm.Shell
		cwd   string
		want  bool
	}{
		{"local-bash", true, guardrm.Bash, testenv.NativeFixture(`C:\repo`), true},
		{"local-powershell", true, guardrm.PowerShell, testenv.NativeFixture(`C:\repo`), true},
		{"wsl-bash", true, guardrm.Bash, "/mnt/c/repo", true},
		{"unknown-shell", true, "", testenv.NativeFixture(`C:\repo`), false},
		{"uncertain-shell", true, guardrm.Uncertain, testenv.NativeFixture(`C:\repo`), false},
		{"remote", false, guardrm.Bash, testenv.NativeFixture(`C:\repo`), false},
		{"wsl-filesystem", true, guardrm.Bash, "/home/repo", runtime.GOOS != "windows"},
		{"relative-cwd", true, guardrm.Bash, "repo", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw := bridgePayload(t, func(in *BridgeRequest) {
				in.Event, in.Command, in.Session = "PreToolUse", "rm scratch.log", ""
				in.Cwd, in.Local, in.Shell = tc.cwd, tc.local, tc.shell
			})
			ev, err := (BridgeAdapter{}).Decode(raw, Invocation{Agent: "plugin"})
			if err != nil || ev.Name != "PreToolUse" || (ev.SkipReason == "") != tc.want || ev.Cwd != tc.cwd {
				t.Fatalf("guard evidence changed: %+v %v", ev, err)
			}
		})
	}
}

func TestBridgeResponse(t *testing.T) {
	t.Parallel()
	adapter := BridgeAdapter{}
	for _, tc := range []struct {
		event Event
		want  Result
	}{
		{Event{Name: "PreToolUse"}, Result{Deny: true, Reason: "obit rm x && echo <done>"}},
		{Event{Name: "PostToolUse"}, Result{Context: "proposal"}},
		{Event{Name: "PostToolUse", DeferContext: true}, Result{}},
		{Event{Name: "Stop"}, Result{}},
		{Event{Name: "SessionEnd"}, Result{}},
	} {
		data, err := adapter.Encode(tc.event, Result{Deny: true, Reason: "obit rm x && echo <done>", Context: "proposal"})
		var got BridgeResponse
		if err != nil || json.Unmarshal(data, &got) != nil || got.Version != BridgeVersion || got.Result != tc.want || bytes.Contains(data, []byte(`\u003c`)) {
			t.Fatalf("%+v: %s %v", tc.event, data, err)
		}
	}
	var neutral BridgeResponse
	if err := json.Unmarshal(adapter.Fallback(Event{}), &neutral); err != nil || neutral != (BridgeResponse{Version: BridgeVersion}) {
		t.Fatalf("invalid neutral response: %+v %v", neutral, err)
	}
}

func TestBridgeObservationOnlyDoesNotAcknowledgeNotice(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	path := e.write("debug.log")
	var out bytes.Buffer
	agent := Agent{Name: "plugin", Adapter: BridgeAdapter{}}
	RunEvent(agent, e.engine, Event{Name: "PostToolUse", Cwd: e.root, Session: "plugin:s", Paths: []string{path}, DeferContext: true}, &out, &Runner{})
	if out.String() != string(agent.Adapter.Fallback(Event{})) {
		t.Fatal(out.String())
	}
	if st := e.state(); len(st.Proposals) != 1 || len(st.Proposals[0].Notified) != 0 {
		t.Fatalf("deferred observation lost or acknowledged: %+v", st.Proposals)
	}
}
