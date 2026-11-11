package hook

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func TestMiniMaxShellDeclaration(t *testing.T) {
	testAgentShellDeclaration(t, "minimax-code", "minimax-code", []string{"bash", "Bash", "PowerShell"})
}

func minimaxFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "minimax-"+name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	return input
}

func minimaxDecode(t *testing.T, input map[string]any) (Event, error) {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return Decode(minimaxAdapter{}, raw, Invocation{Agent: "minimax-code"})
}

func TestMiniMaxProtocolFixtures(t *testing.T) {
	t.Parallel()
	agent, ok := Lookup("minimax-code")
	if !ok || agent.Capabilities.DeleteGuard.Level != Limited {
		t.Fatalf("inaccurate coverage: %+v", agent)
	}
	for _, name := range []string{"native-pre", "claude-pre", "post", "start", "stop", "end"} {
		t.Run(name, func(t *testing.T) {
			input := minimaxFixture(t, name)
			ev, err := minimaxDecode(t, input)
			if err != nil || ev.Session != "minimax-code:minimax-session" || ev.HostEvent != input["hook_event_name"] {
				t.Fatalf("%+v %v", ev, err)
			}
			if name == "post" && (len(ev.Paths) != 1 || ev.Paths[0] != filepath.Join(ev.Cwd, "debug.log")) {
				t.Fatalf("paths: %+v", ev)
			}
			if strings.HasSuffix(name, "pre") && (ev.Shell != guardrm.Uncertain || ev.ShellDeclared || ev.Command == "") {
				t.Fatalf("unattested shell: %+v", ev)
			}
			raw, _ := json.Marshal(input)
			if _, err := Decode(minimaxAdapter{}, append([]byte("\xef\xbb\xbf"), raw...), Invocation{}); err != nil {
				t.Fatal(err)
			}
			if _, err := Decode(minimaxAdapter{}, raw, Invocation{Event: "different"}); err == nil {
				t.Fatal("event conflict accepted")
			}
		})
	}
	if _, err := minimaxDecode(t, minimaxFixture(t, "failed")); !errors.Is(err, ErrSkip) {
		t.Fatalf("failed write: %v", err)
	}
}

func TestMiniMaxObservationRequiresCapturedSuccess(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, tool, key string
		response        any
		accept          bool
	}{
		{"native-write", "write", "path", map[string]any{"content": []any{map[string]any{"type": "text", "text": "ok"}}, "details": map[string]any{"created": false, "bytes_written": 0}}, true},
		{"native-edit", "edit", "file_path", map[string]any{"content": []any{map[string]any{"type": "text"}}, "details": map[string]any{"diff": ""}}, true},
		{"compatible-write", "Write", "file_path", map[string]any{"type": "create", "filePath": `C:\repo\debug.log`, "structuredPatch": []any{}}, true},
		{"compatible-edit", "Edit", "file_path", map[string]any{"filePath": `C:\repo\debug.log`, "structuredPatch": []any{}}, true},
		{"text-only", "write", "path", map[string]any{"content": []any{map[string]any{"type": "text", "text": "Successfully wrote"}}}, false},
		{"missing", "write", "path", nil, false},
		{"error", "write", "path", map[string]any{"isError": true, "details": map[string]any{"created": true, "bytes_written": 10}}, false},
		{"negative-bytes", "write", "path", map[string]any{"content": []any{map[string]any{}}, "details": map[string]any{"created": true, "bytes_written": -1}}, false},
		{"read", "read", "path", map[string]any{"content": []any{map[string]any{}}, "details": map[string]any{"created": true, "bytes_written": 10}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := minimaxFixture(t, "post")
			input["tool_name"], input["tool_input"], input["tool_response"] = tc.tool, map[string]any{tc.key: "debug.log"}, tc.response
			ev, err := minimaxDecode(t, input)
			if (err == nil && len(ev.Paths) == 1) != tc.accept {
				t.Fatalf("%+v %v", ev, err)
			}
		})
	}
	for _, path := range []string{"/work/debug.log", "/mnt/c/repo/debug.log", "~/debug.log", "ssh://host/debug.log", ""} {
		input := minimaxFixture(t, "post")
		input["tool_input"] = map[string]any{"path": path}
		if ev, err := minimaxDecode(t, input); err == nil {
			t.Fatalf("unsupported path accepted: %+v", ev)
		}
	}
	for _, cwd := range []string{"/workspace", "/mnt/c/repo", "ssh://host/repo", "relative", ""} {
		input := minimaxFixture(t, "post")
		input["cwd"] = cwd
		if ev, err := minimaxDecode(t, input); err == nil {
			t.Fatalf("remote cwd accepted: %+v", ev)
		}
	}
}

func TestMiniMaxWriteObservedAndFinishSilent(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	input := minimaxFixture(t, "start")
	input["cwd"] = e.root
	run := func(input map[string]any, runner *Runner) string {
		raw, _ := json.Marshal(input)
		var out bytes.Buffer
		Run("minimax-code", e.engine, bytes.NewReader(raw), &out, runner)
		return out.String()
	}
	run(input, &Runner{})
	path := e.write("debug.log")
	input = minimaxFixture(t, "post")
	input["cwd"] = e.root
	input["tool_input"] = map[string]any{"path": path}
	out := run(input, &Runner{})
	if !strings.Contains(out, "debug.log") || len(e.state().Proposals) != 1 || len(e.state().Proposals[0].Notified) != 1 {
		t.Fatalf("observation %s %+v", out, e.state())
	}
	input = minimaxFixture(t, "failed")
	input["cwd"] = e.root
	input["tool_input"] = map[string]any{"path": e.write("failed.log")}
	if out := run(input, &Runner{}); out != "" || len(e.state().Proposals) != 1 {
		t.Fatalf("failed observed: %s", out)
	}
	for _, name := range []string{"stop", "end"} {
		input = minimaxFixture(t, name)
		input["cwd"] = e.root
		called := false
		out := run(input, &Runner{Finish: func(root, session string, final bool) {
			called = root == e.root && session == "minimax-code:minimax-session" && final == (name == "end")
		}})
		if !called || out != "" {
			t.Fatalf("finish %s: called=%v %q", name, called, out)
		}
	}
}

func TestMiniMaxEncodingAndNeutralFailures(t *testing.T) {
	t.Parallel()
	a := minimaxAdapter{}
	for _, name := range []string{"Stop", "SessionEnd", "unknown"} {
		data, err := a.Encode(Event{Name: name}, Result{Deny: true, Reason: "deny", Context: "continue"})
		if err != nil || len(data) != 0 {
			t.Fatalf("ending response: %s %v", data, err)
		}
	}
	for _, name := range []string{"SessionStart", "PostToolUse"} {
		data, err := a.Encode(Event{Name: name}, Result{Context: strings.Repeat("中文", 2000)})
		var out claudeOutput
		if err != nil || json.Unmarshal(data, &out) != nil || len(out.HookSpecificOutput.AdditionalContext) > maxContext {
			t.Fatalf("budget: %s %v", data, err)
		}
		data, err = a.Encode(Event{Name: name, DeferContext: true}, Result{Context: "later"})
		if err != nil || len(data) != 0 {
			t.Fatalf("deferred: %s %v", data, err)
		}
	}
	data, err := a.Encode(Event{Name: "PreToolUse"}, Result{Deny: true, Reason: "reason", Context: "ignored"})
	if err != nil || string(data) != "{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"reason\"}}\n" {
		t.Fatalf("deny: %s %v", data, err)
	}
	for _, raw := range []string{"{", "null", `{"hook_event_name":"Unknown"}`, strings.Repeat(" ", MaxInput+1)} {
		if _, err := Decode(a, []byte(raw), Invocation{}); err == nil {
			t.Fatalf("accepted malformed input")
		}
	}
	if len(a.Fallback(Event{})) != 0 {
		t.Fatal("fallback must be silent")
	}
}

func TestMiniMaxNativePathNormalizationKeepsUntouchedNeighbor(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, tool := range []string{"write", "edit", "Write", "Edit"} {
		for _, names := range [][2]string{{"@debug.log", "debug.log"}, {"debug\u00a0copy.log", "debug copy.log"}} {
			t.Run(tool+"/"+names[0], func(t *testing.T) {
				t.Parallel()
				e := newEnv(t)
				neighbor, actual := e.write(names[0]), e.write(names[1])
				e.engine.Svc.Owner, e.engine.Svc.Recheck = auto.Owner, e.engine.RecheckRules
				if err := e.engine.Svc.Store.Update(func(st *core.State) error {
					st.Projects = map[string]core.ProjectState{core.ProjectKey(e.root): {Consent: core.ConsentApproved}}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				input := minimaxFixture(t, "post")
				key := "file_path"
				if tool == "write" {
					key = "path"
				}
				input["cwd"], input["tool_name"], input["tool_input"] = e.root, tool, map[string]any{key: names[0]}
				switch tool {
				case "edit":
					input["tool_response"] = map[string]any{"content": []any{map[string]any{"type": "text", "text": "ok"}}, "details": map[string]any{"diff": "changed"}}
				case "Write", "Edit":
					input["tool_response"] = map[string]any{"type": "update", "filePath": actual, "structuredPatch": []any{}}
				}
				raw, _ := json.Marshal(input)
				Run("minimax-code", e.engine, bytes.NewReader(raw), &bytes.Buffer{}, &Runner{})
				e.engine.Svc.Now = func() time.Time { return time.Now().Add(30 * 24 * time.Hour) }
				if _, err := e.engine.Svc.Sweep(false); err != nil {
					t.Fatal(err)
				}
				if data, err := os.ReadFile(neighbor); err != nil || string(data) != names[0] {
					t.Fatalf("untouched neighbor was moved: %v", err)
				}
				if tool == "Write" || tool == "Edit" {
					if _, err := os.Stat(actual); !os.IsNotExist(err) {
						t.Fatalf("confirmed result path was not observed: %v", err)
					}
				}
			})
		}
	}
}

func TestMiniMaxResultPathMustCorrespondToInput(t *testing.T) {
	t.Parallel()
	for _, path := range []string{`C:\repo\neighbor.log`, "debug.log", "/mnt/c/repo/debug.log"} {
		input := minimaxFixture(t, "post")
		input["tool_name"] = "Write"
		input["tool_input"] = map[string]any{"file_path": "debug.log"}
		input["tool_response"] = map[string]any{"type": "update", "filePath": path, "structuredPatch": []any{}}
		if ev, err := minimaxDecode(t, input); err == nil {
			t.Fatalf("unconfirmed result path observed: %+v", ev)
		}
	}
}
