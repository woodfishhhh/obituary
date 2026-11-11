//go:build windows

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestGraveNoticeHookLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository and hook subprocesses")
	}
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			root, _ := clitest.CommandRepo(t)
			svc, err := app.Open()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "old", "payload.txt")
			writeFixture(t, path, "restore this")
			rows, err := svc.Bury([]string{filepath.Dir(path)}, false, "")
			if err != nil || core.GraveFailed(rows) {
				t.Fatal(rows, err)
			}
			id := rows[0].Burial.ID
			call := func(session string) string {
				t.Helper()
				payload, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": session, "cwd": root, "tool_name": "Read", "tool_input": map[string]string{"file_path": path}})
				cmd := exec.Command(binaryPath(t, "obit"), "hook", agent, "--notice-only")
				cmd.Stdin = bytes.NewReader(payload)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("hook exit: %s %v", out, err)
				}
				return string(out)
			}
			out := call("first")
			if !strings.Contains(out, fmt.Sprintf("entire directory #%d", id)) || !strings.Contains(out, fmt.Sprintf("obit resurrect %d", id)) || strings.Contains(out, "permissionDecision") {
				t.Fatal(out)
			}
			if out := call("first"); out != "" {
				t.Fatal("duplicate notice", out)
			}
			if out := call("second"); out == "" {
				t.Fatal("new session missed notice")
			}
			restored, err := svc.Resurrect([]string{fmt.Sprint(id)})
			if err != nil || core.GraveFailed(restored) {
				t.Fatal(restored, err)
			}
			if out := call("third"); out != "" {
				t.Fatal("restored path notice", out)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "restore this" {
				t.Fatal("content changed", err)
			}
			rows, err = svc.Bury([]string{filepath.Dir(path)}, false, "")
			if err != nil || core.GraveFailed(rows) {
				t.Fatal(rows, err)
			}
			purged, err := svc.PurgeRefs([]string{fmt.Sprint(rows[0].Burial.ID)}, false)
			if err != nil || core.GraveFailed(purged) {
				t.Fatal(purged, err)
			}
			if out := call("fourth"); out != "" {
				t.Fatal("purged path notice", out)
			}
		})
	}
}
