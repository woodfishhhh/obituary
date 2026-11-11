package hook

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func deepseekFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "deepseek-"+name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var in map[string]any
	if err := json.Unmarshal(data, &in); err != nil {
		t.Fatal(err)
	}
	return in
}

func decodeDeepSeek(t *testing.T, input any) (Event, error) {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return Decode(deepseekAdapter{}, data, Invocation{Agent: "deepseek-harness"})
}

func TestDeepSeekNativeShellGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Setenv("OBIT_DEEPSEEK_LOCAL", "1")
	for _, tc := range []struct {
		fixture string
		shell   guardrm.Shell
	}{{"bash-pre", guardrm.Bash}, {"pwsh-pre", guardrm.PowerShell}} {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Setenv("OBIT_DEEPSEEK_SHELL", string(tc.shell))
			e := newEnv(t)
			path := e.write("scratch.log")
			in := deepseekFixture(t, tc.fixture)
			in["cwd"] = e.root
			ev, err := decodeDeepSeek(t, in)
			if err != nil || ev.Shell != tc.shell || ev.SkipReason != "" {
				t.Fatalf("%+v %v", ev, err)
			}
			// Broken Git/ledger/config and an empty PATH cannot affect this path.
			write := func(path, content string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(e.engine.Svc.Store.Home, 0o700); err != nil {
				t.Fatal(err)
			}
			ledger := filepath.Join(e.engine.Svc.Store.Home, "store.json")
			write(ledger, "broken ledger")
			t.Setenv("PATH", "")
			text, err := Handle(e.engine, ev, nil)
			if err != nil || !strings.Contains(text, "restorable deletion") {
				t.Fatalf("%q %v", text, err)
			}
			data, err := (deepseekAdapter{}).Encode(ev, Result{Deny: true, Reason: text})
			if err != nil || !bytes.Contains(data, []byte(`"permissionDecision":"deny"`)) {
				t.Fatalf("%s %v", data, err)
			}
			if got, _ := os.ReadFile(path); string(got) != "scratch.log" {
				t.Fatalf("file changed: %s", got)
			}
			if got, _ := os.ReadFile(ledger); string(got) != "broken ledger" {
				t.Fatal("ledger changed")
			}
			if _, err := os.Stat(filepath.Join(e.engine.Svc.Store.Home, "store.lock")); !os.IsNotExist(err) {
				t.Fatalf("opened ledger: %v", err)
			}
		})
	}
}

func TestDeepSeekRemoteAndUnconfirmedShellsPassThrough(t *testing.T) {
	for _, tc := range []struct{ name, local, shell, cwd, tool, workdir string }{
		{"unattested", "", "powershell", "C:/repo", "pwsh", ""},
		{"wrong-shell", "1", "bash", "C:/repo", "pwsh", ""},
		{"unknown-tool", "1", "powershell", "C:/repo", "Shell", ""},
		{"ssh-posix", "1", "bash", "/workspace/repo", "bash", ""},
		{"ssh-mounted-name", "1", "bash", "/mnt/c/repo", "bash", ""},
		{"ssh-uri", "1", "powershell", "ssh://host/C:/repo", "pwsh", ""},
		{"unc", "1", "powershell", `\\server\share`, "pwsh", ""},
		{"remote-workdir", "1", "powershell", "C:/repo", "pwsh", "/mnt/c/repo"},
		{"drive-relative", "1", "powershell", "C:/repo", "pwsh", "C:repo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OBIT_DEEPSEEK_LOCAL", tc.local)
			t.Setenv("OBIT_DEEPSEEK_SHELL", tc.shell)
			in := deepseekFixture(t, "pwsh-pre")
			in["cwd"], in["tool_name"] = tc.cwd, tc.tool
			if tc.workdir != "" {
				in["tool_input"].(map[string]any)["workdir"] = tc.workdir
			}
			ev, err := decodeDeepSeek(t, in)
			if err != nil || ev.SkipReason == "" {
				t.Fatalf("unconfirmed accepted: %+v %v", ev, err)
			}
		})
	}
}

func TestDeepSeekWorkingDirectoryOverrides(t *testing.T) {
	t.Setenv("OBIT_DEEPSEEK_LOCAL", "1")
	t.Setenv("OBIT_DEEPSEEK_SHELL", "powershell")
	for _, tc := range []struct{ workdir, want string }{
		{"sub", filepath.Join(`C:\repo`, "sub")},
		{`C:\other`, `C:\other`},
	} {
		in := deepseekFixture(t, "pwsh-pre")
		in["tool_input"].(map[string]any)["workdir"] = tc.workdir
		ev, err := decodeDeepSeek(t, in)
		if err != nil || ev.SkipReason != "" || ev.Cwd != tc.want {
			t.Fatalf("workdir: %+v %v", ev, err)
		}
	}
	if _, err := Decode(deepseekAdapter{}, bytes.Repeat([]byte(" "), MaxInput+1), Invocation{}); err == nil {
		t.Fatal("accepted oversized input")
	}
}

func TestDeepSeekOfficialPostOnlyDrainsContext(t *testing.T) {
	t.Setenv("OBIT_DEEPSEEK_LOCAL", "1")
	for _, fixture := range []string{"write-post", "edit-post"} {
		for _, response := range []any{"success", "Error: denied", nil, map[string]any{"isError": false}} {
			in := deepseekFixture(t, fixture)
			in["tool_response"] = response
			ev, err := decodeDeepSeek(t, in)
			if err != nil || ev.Name != "Context" || len(ev.Paths) != 0 {
				t.Fatalf("text inferred success: %+v %v", ev, err)
			}
		}
	}
}

func TestDeepSeekObserverDefersUntilOfficialContext(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Setenv("OBIT_DEEPSEEK_LOCAL", "1")
	e := newEnv(t)
	path := e.write("debug.log")
	in := BridgeRequest{Version: 1, Agent: "deepseek-harness", Event: "PostToolUse", HostEvent: "tools/result", Tool: "write", Session: "s1", Cwd: e.root, Local: true, Paths: []string{path}, Success: new(true)}
	ev, err := decodeDeepSeek(t, in)
	if err != nil || !ev.DeferContext || len(ev.Paths) != 1 {
		t.Fatalf("%+v %v", ev, err)
	}
	agent, _ := Lookup("deepseek-harness")
	var out bytes.Buffer
	RunEvent(agent, e.engine, ev, &out, nil)
	st := e.state()
	if len(st.Proposals) != 1 || len(st.Proposals[0].Notified) != 0 || out.String() != "{\"version\":1}\n" {
		t.Fatalf("%+v %q", st, out.String())
	}
	post := deepseekFixture(t, "write-post")
	post["cwd"], post["session_id"] = e.root, "s1"
	ev, err = decodeDeepSeek(t, post)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	RunEvent(agent, e.engine, ev, &out, nil)
	if !strings.Contains(out.String(), "obit approve") || !strings.Contains(out.String(), `"hookEventName":"PostToolUse"`) {
		t.Fatalf("notice lost: %s", out.String())
	}
	if len(e.state().Proposals[0].Notified) != 1 {
		t.Fatal("delivery not acknowledged")
	}
}

func TestDeepSeekObserverRejectsFailureAndRemotePaths(t *testing.T) {
	t.Setenv("OBIT_DEEPSEEK_LOCAL", "1")
	for _, change := range []func(*BridgeRequest){
		func(in *BridgeRequest) { in.Local = false },
		func(in *BridgeRequest) { in.Success = new(false) },
		func(in *BridgeRequest) { in.Success = nil },
		func(in *BridgeRequest) { in.Tool = "bash" },
		func(in *BridgeRequest) { in.HostEvent = "tools/post-execute" },
		func(in *BridgeRequest) { in.Cwd = "/mnt/c/repo" },
		func(in *BridgeRequest) { in.Paths = []string{"/mnt/c/repo/debug.log"} },
		func(in *BridgeRequest) { in.Paths = []string{"ssh://host/C:/repo/debug.log"} },
		func(in *BridgeRequest) { in.Paths = []string{"debug.log"} },
		func(in *BridgeRequest) { in.Agent = "claude" },
	} {
		in := BridgeRequest{Version: 1, Agent: "deepseek-harness", Event: "PostToolUse", HostEvent: "tools/result", Tool: "edit", Session: "s", Cwd: "C:/repo", Local: true, Success: new(true), Paths: []string{"C:/repo/debug.log"}}
		change(&in)
		if ev, err := decodeDeepSeek(t, in); err == nil || len(ev.Paths) != 0 {
			t.Fatalf("accepted: %+v %v", ev, err)
		}
	}
	in := deepseekFixture(t, "ssh")
	if ev, err := decodeDeepSeek(t, in); !errors.Is(err, ErrSkip) || len(ev.Paths) != 0 {
		t.Fatalf("SSH mapped locally: %+v %v", ev, err)
	}
}

func TestDeepSeekLifecycleAndResponses(t *testing.T) {
	t.Setenv("OBIT_DEEPSEEK_LOCAL", "1")
	for _, fixture := range []string{"start", "stop"} {
		in := deepseekFixture(t, fixture)
		id := strings.Repeat("中", 5000)
		in["session_id"] = id
		ev, err := decodeDeepSeek(t, in)
		if err != nil || ev.Session != core.NormalizeSession("deepseek-harness:"+id) {
			t.Fatalf("%+v %v", ev, err)
		}
		data, err := (deepseekAdapter{}).Encode(ev, Result{Context: "continue", Deny: true, Reason: "continue"})
		if err != nil || len(data) != 0 {
			t.Fatalf("detached/end emitted: %s %v", data, err)
		}
	}
	for _, raw := range []string{"null", "[]", "{", "{}", `{"hook_event_name":"SessionEnd"}`, `{"hook_event_name":"SubagentStop"}`} {
		if _, err := Decode(deepseekAdapter{}, []byte(raw), Invocation{}); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	raw, _ := json.Marshal(deepseekFixture(t, "start"))
	if _, err := Decode(deepseekAdapter{}, raw, Invocation{Event: "Stop"}); err == nil {
		t.Fatal("event conflict")
	}
	if _, err := Decode(deepseekAdapter{}, append([]byte("\xef\xbb\xbf"), raw...), Invocation{}); err != nil {
		t.Fatal(err)
	}
	ev := Event{Name: "Context", HostEvent: "UserPromptSubmit"}
	data, _ := (deepseekAdapter{}).Encode(ev, Result{Context: strings.Repeat("中", 3000)})
	var out claudeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.HookSpecificOutput.HookEventName != "UserPromptSubmit" || len(out.HookSpecificOutput.AdditionalContext) > maxContext || !utf8.ValidString(out.HookSpecificOutput.AdditionalContext) {
		t.Fatalf("bad response %s", data)
	}
	neutral, _ := (deepseekAdapter{}).Encode(Event{Name: "PreToolUse"}, Result{Context: "ignore", Reason: "ignore"})
	if len(neutral) != 0 {
		t.Fatalf("neutral pre: %s", neutral)
	}
}
