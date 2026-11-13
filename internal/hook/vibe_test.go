package hook

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVibeProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	agent, _ := Lookup("vibe")
	in := cGroupFixture(t, "vibe-post")
	ev := cGroupDecode(t, agent, in, "post_tool")
	if ev.Name != "PostToolUse" || ev.HostEvent != "post_tool" || ev.Tool != "file_system.write_file" || ev.Session != "vibe:vibe-session" || len(ev.Paths) != 1 || ev.DeferContext {
		t.Fatalf("post: %+v", ev)
	}
	cGroupInputErrors(t, agent, in, "", "hook_event_name", "session_id")
	cGroupGuardSkipped(t, agent, cGroupFixture(t, "vibe-pre"), "pre_tool")
	cGroupFinish(t, agent, cGroupFixture(t, "vibe-stop"), "post_agent", false)
	cGroupResponse(t, agent, true, "hook_specific_output", "additional_context")
	cGroupNeutralFailures(t, agent)
	cGroupExactResponse(t, agent, "PostToolUse", Result{Context: "proposal"}, `{"hook_specific_output":{"additional_context":"proposal"}}`)
	cGroupExactResponse(t, agent, "PreToolUse", Result{Deny: true, Reason: "reason"}, `{"decision":"deny","reason":"reason"}`)
	if agent.Capabilities.SessionStart.Level != Unsupported || agent.Capabilities.SessionEnd.Level != Unsupported || agent.Capabilities.DeleteGuard.Level != Unsupported {
		t.Fatalf("capabilities: %+v", agent.Capabilities)
	}
}

func TestVibeObservesOnlySuccessfulFileTools(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	agent, _ := Lookup("vibe")
	for _, tool := range []string{"write_file", "file_system.write_file", "edit", "file_system.edit"} {
		t.Run(tool, func(t *testing.T) {
			e := newEnv(t)
			path := e.write("debug.log")
			in := cGroupFixture(t, "vibe-post")
			in["cwd"], in["tool_name"] = e.root, tool
			in["tool_input"].(map[string]any)["file_path"] = path
			var out bytes.Buffer
			Run(agent.Name, e.engine, bytes.NewReader(cGroupRaw(t, in)), &out, nil)
			st := e.state()
			if len(st.Items) != 0 || len(st.Proposals) != 1 || !strings.Contains(out.String(), "obit approve") {
				t.Fatalf("proposal: %+v %s", st, out.String())
			}
			var response struct {
				Output struct {
					Context string `json:"additional_context"`
				} `json:"hook_specific_output"`
			}
			if err := json.Unmarshal(out.Bytes(), &response); err != nil || response.Output.Context == "" || len(response.Output.Context) > maxContext {
				t.Fatalf("response: %s %v", out.String(), err)
			}
		})
	}
	for _, tc := range []struct {
		name, key string
		value     any
	}{
		{"failure", "tool_status", "failure"}, {"missing-status", "tool_status", nil},
		{"shell-output", "tool_name", "file_system.bash"}, {"remote-tool", "tool_name", "mcp_remote.write_file"},
		{"code-mode", "tool_name", "code_execution"}, {"unknown-tool", "tool_name", "WriteFile"},
		{"missing-path", "tool_input", map[string]any{}},
		{"relative-path", "tool_input", map[string]any{"file_path": "debug.log"}},
		{"remote-path", "tool_input", map[string]any{"file_path": testenv.ForeignFixture("/home/remote/debug.log")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := cGroupFixture(t, "vibe-post")
			in[tc.key] = tc.value
			cGroupNoObservation(t, agent, in, "")
		})
	}
	for _, cwd := range []string{"relative", testenv.ForeignFixture("/home/remote"), "ssh://host/repo"} {
		in := cGroupFixture(t, "vibe-post")
		in["cwd"] = cwd
		if ev, err := Decode(agent.Adapter, cGroupRaw(t, in), Invocation{Agent: "vibe"}); err == nil || len(ev.Paths) != 0 {
			t.Fatalf("remote cwd: %+v %v", ev, err)
		}
	}
}
