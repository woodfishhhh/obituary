package hook

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestQoderProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, name := range []string{"qoder", "qoder-cn"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			agent, ok := Lookup(name)
			if !ok || agent.Capabilities.DeleteGuard.Level != Limited {
				t.Fatalf("registry: %+v", agent)
			}
			start := cGroupFixture(t, name+"-start")
			ev := cGroupDecode(t, agent, start, "SessionStart")
			if ev.Session != name+":"+start["session_id"].(string) || ev.Source != "startup" {
				t.Fatalf("session: %+v", ev)
			}
			cGroupInputErrors(t, agent, start, "", "hook_event_name", "session_id")
			pre := name + "-pre"
			if name == "qoder" {
				pre = "qoder-cli-pre"
				cGroupGuardSkipped(t, agent, cGroupFixture(t, "qoder-ide-pre"), "")
			}
			cGroupGuardSkipped(t, agent, cGroupFixture(t, pre), "")
			post := name + "-post"
			if name == "qoder" {
				post = "qoder-post-feedback"
			}
			in := cGroupFixture(t, post)
			for _, tool := range []string{"Write", "Edit", "create_file", "search_replace", "edit_file"} {
				in["tool_name"] = tool
				ev := cGroupDecode(t, agent, in, "PostToolUse")
				if len(ev.Paths) != 1 || filepath.ToSlash(ev.Paths[0]) != testenv.NativeFixture("C:/repo/debug.log") || !ev.DeferContext || ev.Tool != tool {
					t.Fatalf("write %s: %+v", tool, ev)
				}
			}
			for _, event := range []string{"PostToolUseFailure", "SubagentStop", "Unknown"} {
				in["hook_event_name"] = event
				if _, err := Decode(agent.Adapter, cGroupRaw(t, in), Invocation{Agent: name}); !errors.Is(err, ErrSkip) {
					t.Fatalf("accepted %s: %v", event, err)
				}
			}
			in["hook_event_name"] = "PostToolUse"
			invalid := []string{"", "relative.log", testenv.ForeignFixture("/home/remote/debug.log"), "ssh://host/debug.log"}
			if runtime.GOOS == "windows" {
				invalid = append(invalid, "C:/repo/file:stream", `\\?\C:\repo.\file`)
			}
			for _, path := range invalid {
				in["tool_input"] = map[string]any{"file_path": path}
				if _, err := Decode(agent.Adapter, cGroupRaw(t, in), Invocation{Agent: name}); err == nil {
					t.Fatalf("accepted remote/missing path %q", path)
				}
			}
			cGroupFinish(t, agent, cGroupFixture(t, name+"-end"), "", true)
			cGroupFinish(t, agent, cGroupFixture(t, "qoder-stop"), "", false)
			cGroupResponse(t, agent, true, "hookSpecificOutput", "additionalContext")
			cGroupNeutralFailures(t, agent)
			cGroupExactResponse(t, agent, "PreToolUse", Result{Deny: true, Reason: "obit rm scratch"}, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"obit rm scratch"}}`)
		})
	}
}

func TestQoderDeferredProposalDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, name := range []string{"qoder", "qoder-cn", "lingma"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			agent, _ := Lookup(name)
			in := cGroupFixture(t, "qoder-post-feedback")
			in["cwd"] = e.root
			in["tool_input"] = map[string]any{"file_path": e.write("debug.log")}
			var out bytes.Buffer
			Run(name, e.engine, bytes.NewReader(cGroupRaw(t, in)), &out, &Runner{})
			st := e.state()
			if out.Len() != 0 || len(st.Proposals) != 1 || len(st.Items) != 0 || len(st.Proposals[0].Notified) != 0 {
				t.Fatalf("post lost/acknowledged: %s %+v", out.String(), st)
			}
			in["hook_event_name"] = "UserPromptSubmit"
			Run(name, e.engine, bytes.NewReader(cGroupRaw(t, in)), &out, &Runner{})
			if name == "lingma" {
				if out.Len() != 0 || len(e.state().Proposals[0].Notified) != 0 {
					t.Fatal("unverified Lingma channel acknowledged notice")
				}
				return
			}
			if !strings.Contains(out.String(), "UserPromptSubmit") || !strings.Contains(out.String(), "obit approve") || len(e.state().Proposals[0].Notified) != 1 {
				t.Fatalf("prompt delivery: %s %+v", out.String(), e.state())
			}
			out.Reset()
			Run(name, e.engine, bytes.NewReader(cGroupRaw(t, in)), &out, &Runner{})
			if out.Len() != 0 {
				t.Fatal("notice repeated", out.String())
			}
			// Even a caller-supplied context must not turn Post into feedback.
			got, err := agent.Adapter.Encode(Event{Name: "PostToolUse"}, Result{Context: "notice"})
			if err != nil || len(got) != 0 {
				t.Fatal("post encoded context", string(got), err)
			}
		})
	}
}

func TestQoderFailedWritesAndForeignPathsDoNotObserve(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, name := range []string{"qoder", "qoder-cn", "lingma"} {
		e := newEnv(t)
		in := cGroupFixture(t, "qoder-post-feedback")
		in["cwd"], in["hook_event_name"] = e.root, "PostToolUseFailure"
		in["tool_input"] = map[string]any{"file_path": e.write("debug.log")}
		Run(name, e.engine, bytes.NewReader(cGroupRaw(t, in)), new(bytes.Buffer), &Runner{})
		in["hook_event_name"], in["cwd"] = "PostToolUse", "ssh://host/repo"
		Run(name, e.engine, bytes.NewReader(cGroupRaw(t, in)), new(bytes.Buffer), &Runner{})
		if _, err := os.Stat(filepath.Join(e.engine.Svc.Store.Home, "store.json")); !os.IsNotExist(err) {
			t.Fatal("failed/foreign write opened ledger", name, err)
		}
	}
}
