package hook

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func TestCodeBuddyShellDeclaration(t *testing.T) {
	testenv.WindowsOnly(t, "Windows host shell declarations and PowerShell commandWindows registration")
	for _, name := range []string{"codebuddy", "codebuddy-ide"} {
		t.Run(name, func(t *testing.T) {
			testAgentShellDeclaration(t, name, name, []string{"Bash", "PowerShell"})
			// The shared hook codebuddy registration receives native IDE
			// execute_command payloads, which require the separate IDE key.
			testAgentShellDeclaration(t, name, "codebuddy-ide", []string{"execute_command"})
		})
	}
}

func TestCodeBuddyProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, name := range []string{"codebuddy", "codebuddy-ide"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			agent, ok := Lookup(name)
			if !ok || agent.Capabilities.DeleteGuard.Level != Limited || agent.Capabilities.WriteObservation.Level != Limited {
				t.Fatalf("inaccurate coverage: %+v", agent)
			}
			start := cGroupFixture(t, "codebuddy-start")
			ev := cGroupDecode(t, agent, start, "SessionStart")
			if ev.Name != "SessionStart" || ev.Source != "startup" || ev.Session != name+":codebuddy-session" {
				t.Fatalf("start: %+v", ev)
			}
			cGroupInputErrors(t, agent, start, "SessionStart", "hook_event_name", "session_id")
			for _, fixture := range []string{"codebuddy-cli-pre", "codebuddy-ide-pre"} {
				ev := cGroupDecode(t, agent, cGroupFixture(t, fixture), "")
				if runtime.GOOS != "windows" {
					if ev.Command == "" || ev.SkipReason == "" || ev.ShellDeclared {
						t.Fatalf("Windows shell declaration must remain unconfirmed: %+v", ev)
					}
					continue
				}
				if ev.Command == "" || ev.Shell != guardrm.Uncertain || ev.ShellDeclared {
					t.Fatalf("payload cannot declare a shell: %+v", ev)
				}
			}
			for _, fixture := range []string{"codebuddy-cli-post", "codebuddy-ide-post"} {
				post := cGroupFixture(t, fixture)
				ev := cGroupDecode(t, agent, post, "PostToolUse")
				if ev.Name != "PostToolUse" || ev.DeferContext || !reflect.DeepEqual(ev.Paths, []string{"debug.log"}) {
					t.Fatalf("write: %+v", ev)
				}
			}
			cGroupFinish(t, agent, cGroupFixture(t, "codebuddy-stop"), "", false)
			cGroupFinish(t, agent, cGroupFixture(t, "codebuddy-end"), "", true)
			cGroupResponse(t, agent, true, "hookSpecificOutput", "additionalContext")
			cGroupNeutralFailures(t, agent)
			cGroupExactResponse(t, agent, "PreToolUse", Result{Deny: true, Reason: "use obit rm x"}, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"use obit rm x"}}`)
			cGroupExactResponse(t, agent, "Context", Result{Context: "proposal"}, `{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"proposal"}}`)
		})
	}
}

func TestCodeBuddyWriteOutcomes(t *testing.T) {
	t.Parallel()
	agent, _ := Lookup("codebuddy")
	for _, tool := range []string{"Write", "Edit", "MultiEdit", "write_to_file", "replace_in_file"} {
		t.Run(tool, func(t *testing.T) {
			post := cGroupFixture(t, "codebuddy-cli-post")
			post["tool_name"] = tool
			key := "file_path"
			if tool == "write_to_file" || tool == "replace_in_file" {
				key = "filePath"
			}
			post["tool_input"] = map[string]any{key: "debug.log"}
			for _, response := range []any{nil, "success", map[string]any{}, map[string]any{"success": false}, map[string]any{"success": "true"}, map[string]any{"success": true, "error": "failed"}, map[string]any{"success": true, "is_error": true}} {
				post["tool_response"] = response
				if ev, err := Decode(agent.Adapter, cGroupRaw(t, post), Invocation{Agent: agent.Name}); !errors.Is(err, ErrSkip) || len(ev.Paths) != 0 {
					t.Fatalf("failed/unknown result observed: %+v %v", ev, err)
				}
			}
			post["tool_response"] = map[string]any{"success": true, "error": nil}
			for _, path := range []string{testenv.ForeignFixture("/home/remote/file"), "ssh://host/work/file", ""} {
				post["tool_input"] = map[string]any{key: path}
				if ev, err := Decode(agent.Adapter, cGroupRaw(t, post), Invocation{Agent: agent.Name}); err == nil || len(ev.Paths) != 0 {
					t.Fatalf("non-native write observed: %+v %v", ev, err)
				}
			}
		})
	}
	for _, fixture := range []string{"codebuddy-failed", "codebuddy-remote"} {
		if _, err := Decode(agent.Adapter, cGroupRaw(t, cGroupFixture(t, fixture)), Invocation{Agent: agent.Name}); !errors.Is(err, ErrSkip) {
			t.Fatal(fixture, err)
		}
	}
	post := cGroupFixture(t, "codebuddy-ide-post")
	post["tool_input"] = map[string]any{"filePath": "one.log", "file_path": "two.log"}
	if _, err := Decode(agent.Adapter, cGroupRaw(t, post), Invocation{}); err == nil {
		t.Fatal("conflicting aliases accepted")
	}
	post["tool_input"] = map[string]any{"file_path": "debug.log"}
	if ev := cGroupDecode(t, agent, post, ""); !reflect.DeepEqual(ev.Paths, []string{"debug.log"}) {
		t.Fatal(ev)
	}
	post["tool_name"] = "execute_command"
	if _, err := Decode(agent.Adapter, cGroupRaw(t, post), Invocation{}); !errors.Is(err, ErrSkip) {
		t.Fatal("shell response treated as write", err)
	}
}

func TestCodeBuddySuccessfulWriteProducesProposal(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, fixture := range []string{"codebuddy-cli-post", "codebuddy-ide-post"} {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t) // Actual Git worktree and Windows file identity.
			e.write("debug.log")
			post := cGroupFixture(t, fixture)
			post["cwd"] = e.root
			var out bytes.Buffer
			Run("codebuddy", e.engine, bytes.NewReader(cGroupRaw(t, post)), &out, nil)
			state := e.state()
			if len(state.Items) != 0 || len(state.Proposals) != 1 || !strings.Contains(out.String(), "obit approve") {
				t.Fatalf("no proposal or premature registration: %s %+v", out.String(), state)
			}
			out.Reset()
			Run("codebuddy", e.engine, bytes.NewReader(cGroupRaw(t, post)), &out, nil)
			if len(e.state().Proposals) != 1 || out.Len() != 0 {
				t.Fatalf("duplicate observation/notification: %s", out.String())
			}
		})
	}
}

func TestCodeBuddyFailedWriteDoesNotTouchLedger(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	agent, _ := Lookup("codebuddy-ide")
	post := cGroupFixture(t, "codebuddy-ide-post")
	post["tool_response"] = map[string]any{"success": false}
	cGroupNoObservation(t, agent, post, "")
}
