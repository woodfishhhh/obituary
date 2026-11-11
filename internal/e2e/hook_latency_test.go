//go:build windows

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core"
)

// Measure the production executable, including runtime/init, Cobra dispatch,
// real Git, durable writes and the resident-runner handoff. Setup and verification
// stay outside the clock so measurements isolate hook execution.
func TestHookProcessLatencyReport(t *testing.T) {
	if testing.Short() {
		t.Skip("measures production hook processes with real Git and fsync")
	}
	root, store := commandRepo(t)
	seedHookLatencyLedger(t, root, store)
	baseline, err := store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"PreToolUse", "PostToolUse"} {
		t.Run(event, func(t *testing.T) {
			input, name := processLatencyPayload(t, root, event)
			var samples []time.Duration
			for range 30 {
				// Reset only this fixture's ledger, before starting the clock.
				if err := store.Update(func(st *core.State) error { *st = baseline; return nil }); err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command(binaryPath(t, "obit"), "hook", "claude")
				cmd.Stdin = bytes.NewReader(input)
				start := time.Now()
				out, err := cmd.CombinedOutput()
				samples = append(samples, time.Since(start))
				if err != nil || len(out) != 0 {
					t.Fatalf("hook protocol: %s %v", out, err)
				}
				checkProcessLatencyState(t, store, event)
			}
			clitest.ReportHookLatency(t, name, "process start to exit; 100 items; fsync enabled; resident runner", samples)
		})
	}
}

func seedHookLatencyLedger(t *testing.T, root string, store core.Store) {
	t.Helper()
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	svc.AfterChange = nil // fixture setup does not need an Explorer refresh
	paths := make([]string, 100)
	for i := range paths {
		paths[i] = filepath.Join(root, fmt.Sprintf("existing-%03d.log", i))
		writeFixture(t, paths[i], "existing fixture")
	}
	results, err := svc.Add(paths, core.AddOptions{ExpiresAt: time.Now().Add(48 * time.Hour), Project: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Error != "" {
			t.Fatal(r.Error)
		}
	}
	if err := store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(root): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func processLatencyPayload(t *testing.T, root, event string) ([]byte, string) {
	t.Helper()
	payload := map[string]any{"hook_event_name": event, "session_id": "latency", "cwd": root}
	name := event + "/ordinary"
	if event == "PreToolUse" {
		payload["tool_name"], payload["tool_input"] = "Bash", map[string]string{"command": "git status --short"}
	} else {
		path := filepath.Join(root, "debug.log")
		writeFixture(t, path, "fresh fixture")
		payload["tool_name"], payload["tool_input"] = "Write", map[string]string{"file_path": path}
		name = event + "/approved"
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return data, name
}

func checkProcessLatencyState(t *testing.T, store core.Store, event string) {
	t.Helper()
	state, err := store.ReadSnapshot()
	want := 100
	if event == "PostToolUse" {
		want++
	}
	if err != nil || len(state.Items) != want || len(state.Proposals) != 0 {
		t.Fatalf("hook registration: items=%d want=%d proposals=%d err=%v", len(state.Items), want, len(state.Proposals), err)
	}
	if data, err := os.ReadFile(filepath.Join(store.Home, "hook.log")); !os.IsNotExist(err) {
		t.Fatalf("hook failed open: %s %v", data, err)
	}
}
