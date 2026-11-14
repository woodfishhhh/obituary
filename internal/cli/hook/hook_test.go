package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

type neutralTestAdapter struct{ hook.BridgeAdapter }

func (neutralTestAdapter) Decode(raw []byte, invocation hook.Invocation) (hook.Event, error) {
	var input struct{ Kind, Cwd, Command string }
	if err := json.Unmarshal(raw, &input); err != nil {
		return hook.Event{}, err
	}
	if input.Kind == "panic" {
		panic("bad host decoder")
	}
	if input.Kind == "" {
		input.Kind = invocation.Event
	}
	if input.Kind == "before" {
		return hook.Event{Name: "PreToolUse", Cwd: input.Cwd, Command: input.Command, Shell: "bash"}, nil
	}
	return hook.Event{Name: "SessionStart", Cwd: input.Cwd, Session: "test"}, nil
}

func init() {
	hook.Register(hook.Agent{Name: "test-native", Adapter: neutralTestAdapter{}})
}

func executeHookPayload(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdin.json")
	clitest.WriteFixture(t, path, input)
	in, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	old := os.Stdin
	os.Stdin = in
	defer func() { os.Stdin = old }()
	out, stderr, err := clitest.Execute(t, NewCmd(), args...)
	if stderr != "" {
		t.Errorf("hook stderr: %s", stderr)
	}
	return out, err
}

func TestMalformedHookInputLogsHostEventAndRemainsSilent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_HOME", home)
	for _, agent := range []string{"claude", "codex"} {
		out, err := executeHookPayload(t, `{"hook_event_name":"Stop","cwd":"C:\Users\private-user\Coding\repo"}`, agent)
		if err != nil || out != "" {
			t.Fatalf("invalid input changed fail-open protocol: %q %v", out, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(home, "hook.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"claude: input:", "codex: input:", `event_hint="Stop"`, "input_prefix="} {
		if !strings.Contains(string(data), want) {
			t.Errorf("CLI diagnostic missing %q: %s", want, data)
		}
	}
	if strings.Contains(string(data), "private-user") || strings.Count(string(data), "\n") != 2 {
		t.Fatalf("invalid input metadata leaked or split log records: %s", data)
	}
}

func TestAgentDecodePrecedesServiceOpen(t *testing.T) {
	root := t.TempDir()
	home, config := filepath.Join(root, "data"), filepath.Join(root, "config")
	t.Setenv("OBIT_HOME", home)
	t.Setenv("OBIT_CONFIG_DIR", config)
	t.Setenv("PATH", t.TempDir())
	clitest.WriteFixture(t, filepath.Join(config, "config.toml"), "[broken")
	clitest.WriteFixture(t, filepath.Join(home, "store.json"), "broken ledger")
	// A file at .git proves PreToolUse never tries project discovery.
	clitest.WriteFixture(t, filepath.Join(root, ".git"), "invalid git pointer")
	for _, args := range [][]string{{"test-native"}, {"test-native", "--event", "before"}} {
		kind := "before"
		if len(args) > 1 {
			kind = ""
		}
		input, _ := json.Marshal(map[string]string{"Kind": kind, "Cwd": root, "Command": "rm scratch.log"})
		out, err := executeHookPayload(t, "\xef\xbb\xbf"+string(input), args...)
		if err != nil || !strings.Contains(out, `"deny":true`) || !strings.Contains(out, "restorable deletion") {
			t.Fatalf("decoded pre event opened service: %q %v", out, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(home, "store.json")); err != nil || string(data) != "broken ledger" {
		t.Fatalf("ledger touched: %s %v", data, err)
	}
	for _, path := range []string{"store.lock", "hook.log", "sessions"} {
		if _, err := os.Stat(filepath.Join(home, path)); !os.IsNotExist(err) {
			t.Fatalf("pre event initialized %s: %v", path, err)
		}
	}
}

func TestHookDefersExplorerRefresh(t *testing.T) {
	testenv.WindowsOnly(t, "Explorer label projection is Windows-only")
	if testing.Short() {
		t.Skip("uses a real Git repository and Explorer projection")
	}
	root, store := clitest.CommandRepo(t)
	autoOn(t, "[explorer]\neverywhere=true\nin_git=true\n")
	if err := store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(root): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first.log", "second.log"} {
		clitest.WriteFixture(t, filepath.Join(root, name), "fixture")
	}
	oldOnce := cmdutil.RunOnce
	t.Cleanup(func() { cmdutil.RunOnce = oldOnce })
	calls := 0
	cmdutil.RunOnce = func(home string) error {
		if home != store.Home {
			t.Fatalf("background home = %q, want %q", home, store.Home)
		}
		calls++
		return nil
	}
	input, _ := json.Marshal(map[string]any{"hook_event_name": "PostToolUse", "session_id": "labels", "cwd": root, "tool_name": "apply_patch",
		"tool_input": map[string]string{"command": "*** Begin Patch\n*** Add File: first.log\n+x\n*** Add File: second.log\n+x\n*** End Patch"}})
	out, err := executeHookPayloadInBubble(t, string(input), "codex")
	if err != nil || out != "" {
		t.Fatalf("hook response: %q %v", out, err)
	}
	st, err := store.Read()
	if err != nil || len(st.Items) != 2 || calls != 1 {
		t.Fatalf("registration and one handoff: items=%d calls=%d err=%v", len(st.Items), calls, err)
	}
	if _, err := os.Stat(filepath.Join(root, "desktop.ini")); !os.IsNotExist(err) {
		t.Fatalf("hook refreshed Explorer synchronously: %v", err)
	}
	assertBackgroundExplorerRefresh(t, root)
}

func assertBackgroundExplorerRefresh(t *testing.T, root string) {
	t.Helper()
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.Tick(svc); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "desktop.ini")); err != nil {
		t.Fatalf("background tick did not refresh Explorer: %v", err)
	}
}

func TestPostToolUseDeliversQueuedProposal(t *testing.T) {
	if testing.Short() {
		t.Skip("queued proposal replay uses a real Git repository")
	}
	clitest.IsolateRunner(t)
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			root, store := clitest.CommandRepo(t)
			autoOn(t, "")
			path := filepath.Join(root, "debug.log")
			clitest.WriteFixture(t, path, "fixture")
			svc, err := app.Open()
			if err != nil {
				t.Fatal(err)
			}
			rows, err := svc.CaptureObservations([]string{path}, "pending")
			if err != nil || len(rows) != 1 {
				t.Fatalf("capture: %+v %v", rows, err)
			}
			if err := store.DeferObservations(rows); err != nil {
				t.Fatal(err)
			}
			input, _ := json.Marshal(map[string]any{"hook_event_name": "PostToolUse", "session_id": "pending", "cwd": root})
			out, err := executeHookPayloadInBubble(t, string(input), agent)
			if err != nil {
				t.Fatal(err)
			}
			checkLatencyResult(t, store, "PostToolUse/pending", out)
			checkDeliveredProposal(t, store, root, "pending")
			if log := readHookLog(t, store); log != "" {
				t.Fatalf("queued proposal failed open: %s", log)
			}
			if store.HasDeferredObservations() {
				t.Fatal("successful replay retained the queued write")
			}
			if out, err := executeHookPayloadInBubble(t, string(input), agent); err != nil || out != "" {
				t.Fatalf("proposal repeated: %q %v", out, err)
			}
			checkDeliveredProposal(t, store, root, "pending")
		})
	}
}

func TestPostToolUseDeliveryDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("delivery deadlines use a real Git repository and ledger")
	}
	isolateLatencyRunner(t)
	for _, agent := range []string{"claude", "codex"} {
		for _, stage := range []string{"immediate", "locked", "committed"} {
			t.Run(agent+"/"+stage, func(t *testing.T) {
				root, store := clitest.CommandRepo(t)
				autoOn(t, "")
				input := latencyPayload(t, root, "PostToolUse/pending")
				resetLatencyFixture(t, root, store, "PostToolUse/pending")
				release := injectDeliveryDeadline(t, store, stage)
				out, err := executeHookPayloadInBubble(t, input, agent)
				release()
				if err != nil {
					t.Fatal(err)
				}
				got := checkPendingLatency(t, store, root, agent, out, "")
				want := map[string]string{"immediate": "immediate", "locked": "observation", "committed": "notice"}[stage]
				if got != want {
					t.Fatalf("delivery stage: got %s, want %s", got, want)
				}
			})
		}
	}
}

func TestHookLatencyReport(t *testing.T) {
	if testing.Short() {
		t.Skip("measures hook latency with real Git and filesystem operations")
	}
	isolateLatencyRunner(t)
	for _, name := range []string{"PreToolUse/delete", "PreToolUse/ordinary", "PostToolUse/approved", "PostToolUse/pending", "SessionStart/startup", "SessionStart/resume", "Stop/dispatch"} {
		t.Run(name, func(t *testing.T) {
			root, store := clitest.CommandRepo(t)
			autoOn(t, "")
			input := latencyPayload(t, root, name)
			var samples []time.Duration
			deferred := map[string]int{}
			for range 10 {
				resetLatencyFixture(t, root, store, name)
				previousLog := readHookLog(t, store)
				// Measure real elapsed time outside the bubble so stalls remain in
				// the report without changing the functional result being checked.
				start := time.Now()
				out, err := executeHookPayloadInBubble(t, input, "claude")
				elapsed := time.Since(start)
				if err != nil {
					t.Fatal(err)
				}
				samples = append(samples, elapsed)
				if name == "PostToolUse/pending" {
					deferred[checkPendingLatency(t, store, root, "claude", out, previousLog)]++
				} else {
					checkLatencyResult(t, store, name, out)
					if log := readHookLog(t, store); log != previousLog {
						t.Fatalf("hook failed open: %s", log)
					}
				}
			}
			scope := fmt.Sprintf("service open + handler wall time, virtual lock clock; process launch measured separately; deferred observations=%d notices=%d included; replay checks outside samples", deferred["observation"], deferred["notice"])
			clitest.ReportHookLatency(t, name, scope, samples)
		})
	}
}

func latencyPayload(t *testing.T, root, name string) string {
	t.Helper()
	event, variant, _ := strings.Cut(name, "/")
	payload := map[string]any{"hook_event_name": event, "session_id": "latency", "cwd": root, "source": variant}
	if event == "PreToolUse" {
		command := "git status --short"
		if variant == "delete" {
			command = "rm -rf build"
		}
		payload["tool_name"], payload["tool_input"] = "Bash", map[string]string{"command": command}
	}
	if event == "PostToolUse" {
		path := filepath.Join(root, "debug.log")
		clitest.WriteFixture(t, path, "fixture")
		payload["tool_name"], payload["tool_input"] = "Write", map[string]string{"file_path": path}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func resetLatencyFixture(t *testing.T, root string, store core.Store, name string) {
	t.Helper()
	if err := store.Update(func(st *core.State) error {
		*st = core.State{NextID: 1}
		if name == "PostToolUse/approved" {
			st.Projects = map[string]core.ProjectState{core.ProjectKey(root): {Consent: core.ConsentApproved}}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if name == "SessionStart/startup" {
		if err := os.RemoveAll(filepath.Join(store.Home, "sessions")); err != nil {
			t.Fatal(err)
		}
	}
	if name == "SessionStart/resume" {
		svc, err := app.Open()
		if err != nil {
			t.Fatal(err)
		}
		if err := app.Engine(svc).Snapshot(root, "latency"); err != nil {
			t.Fatal(err)
		}
	}
}

func checkLatencyResult(t *testing.T, store core.Store, name, out string) {
	t.Helper()
	st, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	switch name {
	case "PreToolUse/delete":
		if !strings.Contains(out, `"permissionDecision":"deny"`) {
			t.Fatalf("deletion was not denied: %s", out)
		}
	case "PostToolUse/approved":
		if len(st.Items) != 1 || len(st.Proposals) != 0 || out != "" {
			t.Fatalf("expected silent registration: %+v %s", st, out)
		}
	case "PostToolUse/pending":
		if len(st.Items) != 0 || len(st.Proposals) != 1 || !strings.Contains(out, "obit approve") {
			t.Fatalf("expected proposal: %+v %s", st, out)
		}
	default:
		if out != "" {
			t.Fatalf("unexpected response: %s", out)
		}
	}
}

func TestNativeHookFallbackOnFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_HOME", filepath.Join(root, "data"))
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	clitest.WriteFixture(t, filepath.Join(root, "config", "config.toml"), "[broken")
	for _, input := range []string{"bad json", `{"Kind":"panic"}`, `{"Kind":"start"}`, strings.Repeat(" ", hook.MaxInput+1)} {
		out, err := executeHookPayload(t, input, "test-native")
		if err != nil || out != "{\"version\":1}\n" {
			t.Fatalf("failure lost neutral response: %q %v", out, err)
		}
	}
	t.Setenv("OBIT_DISABLED", "1")
	out, err := executeHookPayload(t, "", "test-native")
	if err != nil || out != "{\"version\":1}\n" {
		t.Fatalf("disabled hook lost neutral response: %q %v", out, err)
	}
}

func TestFinalHookBypassesFinishDebounce(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			root, store := clitest.CommandRepo(t)
			autoOn(t, "")
			svc, err := app.Open()
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			oldSpawn := cmdutil.SpawnSelf
			t.Cleanup(func() { cmdutil.SpawnSelf = oldSpawn })
			cmdutil.SpawnSelf = func(args ...string) error {
				calls++
				if len(args) != 6 || args[0] != "hook" || args[1] != "finish" || args[3] != root || args[5] != "session" {
					t.Fatalf("unexpected spawn: %v", args)
				}
				_, _, err := clitest.Execute(t, NewCmd(), args[1:]...)
				return err
			}
			run := func(event string) {
				data, err := json.Marshal(map[string]string{"hook_event_name": event, "session_id": "session", "cwd": root})
				if err != nil {
					t.Fatal(err)
				}
				var out bytes.Buffer
				if agent == "claude" {
					hook.Claude(app.Engine(svc), bytes.NewReader(data), &out, cmdutil.Runner(svc))
				} else {
					hook.Codex(app.Engine(svc), bytes.NewReader(data), &out, cmdutil.Runner(svc))
				}
				if out.Len() != 0 {
					t.Errorf("%s must be silent: %s", event, out.String())
				}
			}
			run("SessionStart")
			run("Stop")
			run("Stop")
			if calls != 1 {
				t.Fatalf("Stop must debounce: %d spawns", calls)
			}
			clitest.WriteFixture(t, filepath.Join(root, "test-results", "report.html"), "x") // after the last Stop
			path := filepath.Join(root, "test-results")
			run("SessionEnd")
			st, err := store.Read()
			// A rule covers it but the hook never saw it written: a proposal.
			if calls != 2 || err != nil || len(st.Items) != 0 || len(st.Proposals) != 1 || st.Proposals[0].Candidates[0].Path != path {
				t.Fatalf("final event must finish and handle last-minute leftovers: spawns=%d items=%+v proposals=%+v err=%v", calls, st.Items, st.Proposals, err)
			}
		})
	}
}

func TestHookAuditBothSources(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, sources := range [][]string{nil, {"hook-audit.log"}, {"delete-guard.jsonl"}, {"hook-audit.log", "delete-guard.jsonl"}} {
		t.Run(strings.Join(sources, "+"), func(t *testing.T) {
			_, store := clitest.CommandRepo(t)
			for _, source := range sources {
				clitest.WriteFixture(t, filepath.Join(store.Home, source), "old-"+source+"\nnew-"+source+"\n")
			}
			out, stderr, err := clitest.Execute(t, NewCmd(), "audit", "-n", "1")
			if err != nil || stderr != "" {
				t.Fatalf("hook audit: %s %s %v", out, stderr, err)
			}
			for _, source := range sources {
				if !strings.Contains(out, "== "+source+" ==") || !strings.Contains(out, "new-"+source) || strings.Contains(out, "old-"+source) {
					t.Errorf("source/line limit missing: %s", out)
				}
			}
			if strings.Contains(out, "no audit trail") != (len(sources) == 0) {
				t.Errorf("inaccurate empty audit message: %s", out)
			}
		})
	}
	t.Run("empty-files", func(t *testing.T) {
		_, store := clitest.CommandRepo(t)
		for _, source := range []string{"hook-audit.log", "delete-guard.jsonl"} {
			clitest.WriteFixture(t, filepath.Join(store.Home, source), "\n")
		}
		out, _, err := clitest.Execute(t, NewCmd(), "audit")
		if err != nil || !strings.Contains(out, "no audit trail") {
			t.Fatalf("empty audit files: %q %v", out, err)
		}
	})
}

func TestFinishReplaysTimedOutObservationOfPreexistingFile(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	clitest.IsolateRunner(t)
	root, store := clitest.CommandRepo(t)
	autoOn(t, "")
	path := filepath.Join(root, "debug.log")
	clitest.WriteFixture(t, path, "before session")
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(root): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	engine := app.Engine(svc)
	if err := engine.Snapshot(root, "queued"); err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, path, "edited during session")
	release, err := core.LockFile(filepath.Join(store.Home, "store.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	text, hookErr := hook.Handle(engine, hook.Event{Name: "PostToolUse", Cwd: root, Session: "queued", Paths: []string{path}}, nil)
	release()
	if text != "" || !errors.Is(hookErr, context.DeadlineExceeded) {
		t.Fatalf("hook: %q, %v", text, hookErr)
	}
	if _, _, err := clitest.Execute(t, NewCmd(), "finish", "--project", root, "--session", "queued"); err != nil {
		t.Fatal(err)
	}
	st, err := store.Read()
	if err != nil || len(st.Items) != 1 || !strings.EqualFold(st.Items[0].Path, path) || st.Items[0].Session != "queued" || st.Items[0].Source != "rule" {
		t.Fatalf("finish lost preexisting edited file: %+v, %v", st.Items, err)
	}
}
