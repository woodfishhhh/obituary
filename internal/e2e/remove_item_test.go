//go:build windows

package e2e_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Check the explicit PowerShell suggestion and the direct native argv contract
// separately. This does not certify a live PowerShell host or translate its
// invocation into Bash. The child records its actual os.Args before the CLI.
func TestRemoveItemSuggestionAndNativeArgv(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	proxy, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	self, err := filepath.EvalSymlinks(binaryPath(t, "obit"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		command string
		argv    []string
		buried  []string
		suggest bool
	}{
		{"Remove-Item a,b", []string{"a,b"}, []string{"a,b"}, false},
		{"Remove-Item -Path 'a,b'", []string{"-Path", "a,b"}, []string{"a,b"}, false},
		{"Remove-Item -LiteralPath 'a,b'", []string{"-LiteralPath", "a,b"}, []string{"a,b"}, false},
		{"Remove-Item -Path 'a b'", []string{"-Path", "a b"}, []string{"a b"}, true},
		{"Remove-Item -LiteralPath 'a[1]' -Rec -Force", []string{"-LiteralPath", "a[1]", "-Rec", "-Force"}, []string{"a[1]"}, false},
		{"Remove-Item -Fo a", []string{"-Fo", "a"}, []string{"a"}, true},
		{"Remove-Item a -r -fo", []string{"a", "-r", "-fo"}, []string{"a"}, true},
		{"Remove-Item -li 'a b' -rec", []string{"-li", "a b", "-rec"}, []string{"a b"}, true},
		{"Remove-Item -Force a", []string{"-Force", "a"}, []string{"a"}, true},
		{"Remove-Item a b", []string{"a", "b"}, nil, false},
		{"Remove-Item -Path a b", []string{"-Path", "a", "b"}, nil, false},
		{"Remove-Item -LiteralPath a b", []string{"-LiteralPath", "a", "b"}, nil, false},
		{"Remove-Item -f a", []string{"-f", "a"}, nil, false},
		{"Remove-Item -p a", []string{"-p", "a"}, nil, false},
		{"Remove-Item -Force -Force a", []string{"-Force", "-Force", "a"}, nil, false},
		{"Remove-Item a -Unknown", []string{"a", "-Unknown"}, nil, false},
		{"Remove-Item a -Path b", []string{"a", "-Path", "b"}, nil, false},
	} {
		t.Run(tc.command, func(t *testing.T) {
			repo, store := commandRepo(t)
			for _, file := range []string{"a", "b", "a b", "c", "a[1]", "a,b"} {
				writeFixture(t, filepath.Join(repo, file), file)
			}
			payload, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": repo, "tool_name": "exec_command", "tool_input": map[string]string{"cmd": tc.command, "shell": "pwsh"}})
			var output bytes.Buffer
			probe := exec.Command(self, "hook", "codex")
			probe.Stdin, probe.Stdout = bytes.NewReader(payload), &output
			if err := probe.Run(); err != nil {
				t.Fatal(err)
			}
			var reply struct {
				Hook struct {
					Reason string `json:"permissionDecisionReason"`
				} `json:"hookSpecificOutput"`
			}
			if json.Unmarshal(output.Bytes(), &reply) != nil {
				t.Fatalf("literal Remove-Item was not denied: %s", output.String())
			}
			suggestion := "& '" + strings.ReplaceAll(self, "'", "''") + "' remove-item" + strings.TrimPrefix(tc.command, "Remove-Item")
			if !tc.suggest {
				// Still denied, without an equivalent; direct native
				// invocations have their own argv contract.
				if !strings.Contains(reply.Hook.Reason, "no exact obit equivalent applies") {
					t.Fatalf("unsupported form received a suggestion: %s", output.String())
				}
			} else {
				parts := strings.Split(reply.Hook.Reason, "`")
				if len(parts) < 3 || parts[1] != suggestion {
					t.Fatalf("suggestion changed arguments: %s", output.String())
				}
			}
			capture := filepath.Join(t.TempDir(), "argv.json")
			child := exec.Command(proxy, append([]string{"remove-item"}, tc.argv...)...)
			child.Env = append(os.Environ(), "OBIT_E2E_ARGV_BINARY="+self, "OBIT_DELETE_ARGV_CAPTURE="+capture)
			out, err := child.CombinedOutput()
			if (err == nil) != (len(tc.buried) > 0) {
				t.Fatalf("CLI result: %s %v", out, err)
			}
			data, err := os.ReadFile(capture)
			var got []string
			if err != nil || json.Unmarshal(data, &got) != nil || !slices.Equal(got, append([]string{"remove-item"}, tc.argv...)) {
				t.Fatalf("native argv: %s %v, want %v", data, err, tc.argv)
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != len(tc.buried) {
				t.Fatalf("burials: %+v %v", st.Burials, err)
			}
			for _, file := range []string{"a", "b", "a b", "c", "a[1]", "a,b"} {
				if slices.Contains(tc.buried, file) {
					continue
				}
				if data, err := os.ReadFile(filepath.Join(repo, file)); err != nil || string(data) != file {
					t.Errorf("unselected file lost: %s %q %v", file, data, err)
				}
			}
		})
	}
}
