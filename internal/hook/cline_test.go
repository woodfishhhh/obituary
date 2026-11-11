package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestClineBridge(t *testing.T) {
	checkPluginBridge(t, "cline", false)
}

func TestClineLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	checkPluginLifecycle(t, "cline", map[string]string{
		"beforeRun": "SessionStart", "afterRun": "Stop", "onDispose": "SessionEnd",
	})
}

func TestClineSuccessfulWriteProposal(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, success := range []bool{false, true} {
		t.Run(fmt.Sprint(success), func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			path := e.write("debug.log")
			in := pluginBridgeFixture(t, "cline", "post")
			in.Cwd, in.Paths, in.Success = e.root, []string{path}, &success
			var out bytes.Buffer
			Run("cline", e.engine, bytes.NewReader(pluginBridgeJSON(t, in)), &out, &Runner{})
			var got BridgeResponse
			if err := json.Unmarshal(out.Bytes(), &got); err != nil || got.Version != BridgeVersion || got.Deny || len(got.Context) > maxContext {
				t.Fatalf("write response: %s %v", out.Bytes(), err)
			}
			st := e.state()
			if success {
				if len(st.Proposals) != 1 || len(st.Items) != 0 || !strings.Contains(got.Context, "obit approve") {
					t.Fatalf("missing proposal: %+v %s", st, out.Bytes())
				}
				if st.Proposals[0].Session != "cline:"+in.Session || len(st.Proposals[0].Notified) != 1 || st.Proposals[0].Notified[0] != "cline:"+in.Session {
					t.Fatalf("missing session/delivery acknowledgement: %+v", st.Proposals)
				}
			} else if len(st.Proposals) != 0 || len(st.Items) != 0 || got.Context != "" {
				t.Fatalf("failed write was observed: %+v %s", st, out.Bytes())
			}
		})
	}
}

func TestClineEditorTool(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	path := e.write("editor.log")
	in := pluginBridgeFixture(t, "cline", "post")
	in.Cwd, in.Paths, in.Tool = e.root, []string{path}, "editor"
	var out bytes.Buffer
	Run("cline", e.engine, bytes.NewReader(pluginBridgeJSON(t, in)), &out, &Runner{})
	if len(e.state().Proposals) != 1 {
		t.Fatalf("official SDK editor write was skipped: %s", out.Bytes())
	}
}

func TestClineContextBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	for i := 0; i < 9; i++ {
		_, _, _, err := e.engine.Svc.Propose(core.ProposalLeftovers, e.root, fmt.Sprint(i), []core.Candidate{{Path: e.write(fmt.Sprintf("leftover-%d.txt", i)), Lifespan: "3d"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	in := pluginBridgeFixture(t, "cline", "post")
	in.Cwd, in.Event, in.HostEvent = e.root, "SessionStart", "beforeRun"
	var out bytes.Buffer
	Run("cline", e.engine, bytes.NewReader(pluginBridgeJSON(t, in)), &out, &Runner{})
	var got BridgeResponse
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || got.Context == "" || len(got.Context) > maxContext || !utf8.ValidString(got.Context) {
		t.Fatalf("context budget: %s %v", out.Bytes(), err)
	}
}

func TestClineDoesNotTreatToolOutputAsPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	path := e.write("debug.log")
	in := pluginBridgeFixture(t, "cline", "post")
	in.Cwd, in.Paths, in.Tool = e.root, nil, "run_commands"
	raw := pluginBridgeJSON(t, in)
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	payload["output"] = "wrote " + path
	payload["tool_response"] = map[string]any{"file_path": path, "success": true}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	Run("cline", e.engine, bytes.NewReader(raw), &out, &Runner{})
	if st := e.state(); len(st.Items) != 0 || len(st.Proposals) != 0 || out.String() != "{\"version\":1}\n" {
		t.Fatalf("shell output became path evidence: %+v %s", st, out.Bytes())
	}
}
