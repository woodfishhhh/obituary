package hook

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func TestGraveNoticeToolPaths(t *testing.T) {
	for _, agent := range []string{"claude", "codex"} {
		for _, tool := range []string{"Read", "Write", "Edit", "MultiEdit", "NotebookEdit", "LS", "Glob", "Grep", "apply_patch"} {
			if agent == "claude" && tool == "apply_patch" {
				continue
			}
			t.Run(agent+"/"+tool, func(t *testing.T) {
				e, root, b := noticeFixture(t, true)
				input := map[string]any{"file_path": b.Path, "notebook_path": b.Path, "path": b.Path}
				if tool == "apply_patch" {
					input = map[string]any{"input": "*** Begin Patch\n*** Update File: " + b.Path + "\n@@\n-old\n+new\n*** End Patch"}
				}
				out := runNotice(t, e, agent, "s", root, tool, input)
				if !strings.Contains(out, "obit resurrect 42") {
					t.Fatal(out)
				}
			})
		}
	}
}

func TestGraveNoticeShellPaths(t *testing.T) {
	for _, tc := range []struct {
		agent, tool, command string
		input                map[string]any
	}{
		{"claude", "Bash", "cat buried", nil},
		{"codex", "Bash", "cat buried", nil},
		{"claude", "PowerShell", "Get-Content -LiteralPath buried", nil},
		{"codex", "exec_command", "cat buried", map[string]any{"shell": "bash"}},
		{"codex", "exec_command", "Get-Content buried", map[string]any{"shell": "pwsh"}},
	} {
		t.Run(tc.agent+"/"+tc.command, func(t *testing.T) {
			e, root, _ := noticeFixture(t, false)
			if tc.input == nil {
				tc.input = map[string]any{}
			}
			tc.input["command"] = tc.command
			if out := runNotice(t, e, tc.agent, "s", root, tc.tool, tc.input); !strings.Contains(out, "obit resurrect 42") {
				t.Fatal(out)
			}
		})
	}
}

func TestGraveNoticeAmbiguousPathsStaySilent(t *testing.T) {
	e, root, _ := noticeFixture(t, false)
	for _, path := range []string{"~/.buried", "buried:stream", `\\?\` + filepath.Join(root, "buried."), "*.txt", "/home/elsewhere"} {
		if got := absentReferences(Event{Cwd: root, References: []string{path}}); len(got) != 0 {
			t.Fatalf("ambiguous %q: %v", path, got)
		}
	}
	for _, command := range []string{"echo buried", "cd elsewhere && cat buried", "cat $file", "cat *"} {
		d := delivery{}
		prepareGraveNotices(e.Svc.Store, Event{Host: "claude", Name: "PreToolUse", Session: "s", Cwd: root, Command: command, Shell: guardrm.Bash}, &d)
		d.close()
		if d.text() != "" {
			t.Fatal(command, d.text())
		}
	}
}
