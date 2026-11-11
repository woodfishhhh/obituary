//go:build windows

package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
	"golang.org/x/sys/windows"
)

func TestDeleteGuardExecutablePATH(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the hook test binary under the obit executable name")
	}
	if os.Getenv("OBIT_TEST_GUARD_EXECUTABLE") != "1" {
		runGuardExecutableTest(t)
		return
	}
	t.Chdir(t.TempDir())
	t.Setenv("PATHEXT", ".EXE")
	t.Setenv("OBIT_GUARD_RM", "1")
	t.Setenv("OBIT_DISABLED", "0")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	selfDir := filepath.Dir(self)
	otherDir, emptyDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(otherDir, "obit.exe"), []byte("another executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path string
		bare       bool
	}{
		{"same", selfDir + string(os.PathListSeparator) + otherDir, true},
		{"case", strings.ToUpper(selfDir), true},
		{"different", otherDir + string(os.PathListSeparator) + selfDir, false},
		{"missing", emptyDir, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PATH", tc.path)
			assertGuardPATHSuggestion(t, tc.bare)
		})
	}
	t.Run("short-name", func(t *testing.T) {
		t.Setenv("PATH", guardShortPath(t, selfDir))
		assertGuardPATHSuggestion(t, true)
	})
	t.Run("wsl", func(t *testing.T) {
		t.Setenv("PATH", selfDir)
		real, err := filepath.EvalSymlinks(self)
		if err != nil {
			t.Fatal(err)
		}
		want := deleteReason(WSLExecutable(real)+" rm -rf build", "")
		got, err := guardDelete(t.TempDir(), Event{Shell: guardrm.Bash, Cwd: "/mnt/c/repo", Command: "rm -rf build"})
		if err != nil || got != want {
			t.Fatalf("WSL executable changed: got %q, %v; want %q", got, err, want)
		}
	})
}

func TestDeleteGuardPATHIdentity(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PATHEXT", ".EXE")
	selfDir, otherDir := t.TempDir(), t.TempDir()
	self := filepath.Join(selfDir, "obit.exe")
	for _, path := range []string{self, filepath.Join(otherDir, "obit.exe")} {
		if err := os.WriteFile(path, []byte("same contents, distinct files"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, path, self string
		want             bool
	}{
		{"same", selfDir, self, true},
		{"case", strings.ToUpper(selfDir), strings.ToLower(self), true},
		{"different", otherDir + string(os.PathListSeparator) + selfDir, self, false},
		{"missing", t.TempDir(), self, false},
		{"self-missing", selfDir, filepath.Join(selfDir, "missing.exe"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PATH", tc.path)
			if got := obitOnPATHIsSelf(tc.self); got != tc.want {
				t.Fatalf("PATH identity: got %v, want %v", got, tc.want)
			}
		})
	}
	t.Run("short-name", func(t *testing.T) {
		t.Setenv("PATH", guardShortPath(t, selfDir))
		if !obitOnPATHIsSelf(self) {
			t.Fatal("short PATH did not resolve to the same executable")
		}
	})
	t.Run("relative", func(t *testing.T) {
		t.Chdir(selfDir)
		t.Setenv("PATH", ".")
		if obitOnPATHIsSelf(self) {
			t.Fatal("relative executable lookup accepted")
		}
	})
}

func guardShortPath(t *testing.T, path string) string {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 32768)
	n, err := windows.GetShortPathName(name, &buf[0], uint32(len(buf)))
	if err != nil || n >= uint32(len(buf)) {
		t.Fatalf("short path: size=%d, error=%v", n, err)
	}
	short := windows.UTF16ToString(buf[:n])
	if strings.EqualFold(short, path) {
		t.Skip("volume does not provide an 8.3 alias")
	}
	return short
}

func runGuardExecutableTest(t *testing.T) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "obit.exe")
	if err := os.WriteFile(binary, data, 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-test.run=^TestDeleteGuardExecutablePATH$", "-test.v")
	cmd.Env = append(os.Environ(), "OBIT_TEST_GUARD_EXECUTABLE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("executable PATH subprocess: %s %v", out, err)
	}
}

func assertGuardPATHSuggestion(t *testing.T, bare bool) {
	t.Helper()
	for _, tc := range []struct {
		shell         guardrm.Shell
		command, want string
	}{
		{guardrm.Bash, "rm -rf build", "rm -rf build"},
		{guardrm.PowerShell, "rm -Recurse -Force build", "remove-item -Recurse -Force build"},
	} {
		t.Run(string(tc.shell), func(t *testing.T) {
			executable := expectedGuardExecutable(t, tc.shell)
			if bare {
				executable = "obit"
			}
			want := "Obituary's delete guard is on: `" + executable + " " + tc.want + "` provides restorable deletion; `obit resurrect` restores graveyard entries."
			got, err := guardDelete(t.TempDir(), Event{Shell: tc.shell, Cwd: t.TempDir(), Command: tc.command})
			if err != nil || got != want {
				t.Fatalf("PATH executable denial: got %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestPreToolUseDeny(t *testing.T) {
	for _, agent := range []string{"claude", "codex"} {
		for _, source := range []string{"rm -rf build", "rm -f *.log"} {
			t.Run(agent+"/"+source, func(t *testing.T) {
				home := t.TempDir()
				// A broken ledger and no Git on PATH must not affect PreToolUse.
				os.WriteFile(filepath.Join(home, "store.json"), []byte("broken"), 0o600)
				t.Setenv("PATH", "")
				t.Setenv("OBIT_GUARD_RM", "1")
				e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}}}
				tool, key, run := "Bash", "command", Claude
				if agent == "codex" {
					tool, key, run = "local_shell", "command", Codex
				}
				var command any = source
				if agent == "codex" {
					command = []string{"bash", "-c", source}
				}
				data, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": t.TempDir(), "tool_name": tool, "tool_input": map[string]any{key: command}})
				var out bytes.Buffer
				run(e, bytes.NewReader(data), &out, nil)
				var got map[string]map[string]string
				if err := json.Unmarshal(out.Bytes(), &got); err != nil {
					t.Fatalf("deny JSON: %q %v", out.String(), err)
				}
				h := got["hookSpecificOutput"]
				want := "Obituary's delete guard is on: `" + expectedGuardExecutable(t, guardrm.Bash) + " " + source + "` provides restorable deletion; `obit resurrect` restores graveyard entries."
				if len(got) != 1 || len(h) != 3 || h["hookEventName"] != "PreToolUse" || h["permissionDecision"] != "deny" || h["permissionDecisionReason"] != want {
					t.Fatalf("deny contract: %s", out.String())
				}
			})
		}
	}
}

func TestDeletePowerShellStaticCommands(t *testing.T) {
	for _, agent := range []string{"claude", "codex"} {
		for _, tc := range []struct{ name, command, want string }{
			{"prefixes", `Remove-Item build -r -fo`, `remove-item build -r -fo`},
			{"alias-prefixes", `rm -li 'build dir' -rec -for`, `remove-item -li 'build dir' -rec -for`},
			{"ambiguous-prefix", `Remove-Item build -f`, ""},
			{"preview-prefix", `Remove-Item build -what`, ""},
			{"git-clean", `git clean -fdx`, `git-clean -fdx`},
			{"git-exclude", `git clean -fd -e '*.log' -- 'build dir'`, `git-clean -fd -e '*.log' -- 'build dir'`},
			{"git-preview", `git clean -ndx`, ""},
			{"git-dynamic", `git clean -fd -- "$target"`, ""},
			{"git-quotes", `git clean -fd -- 'a''b'`, ""},
		} {
			t.Run(agent+"/"+tc.name, func(t *testing.T) {
				// PreToolUse must still work without Git on PATH.
				t.Setenv("PATH", "")
				tool := "PowerShell"
				input := map[string]any{"command": tc.command}
				if agent == "codex" {
					tool = "exec_command"
					input = map[string]any{"cmd": tc.command, "shell": "pwsh"}
				}
				want := tc.want
				if want != "" {
					want = expectedGuardExecutable(t, guardrm.PowerShell) + " " + want
				}
				if got := guardSuggestion(t, agent, `C:\repo`, tool, input); got != want {
					t.Fatalf("PowerShell deny contract: got %q, want %q", got, want)
				}
			})
		}
	}
}

func TestDeleteDeclaredShellCommands(t *testing.T) {
	t.Setenv("PATH", "")
	for _, tc := range []struct {
		shell         guardrm.Shell
		command, want string
	}{
		{guardrm.Bash, `rm -- “a b”`, ""},
		{guardrm.Bash, `rm -- ‘a b’`, ""},
		{guardrm.Bash, `git clean -fd -- “a b”`, ""},
		{guardrm.PowerShell, `git clean -fdx`, `git-clean -fdx`},
		{guardrm.PowerShell, `git clean -ndx`, ""},
		{guardrm.PowerShell, `git clean -fd -- .\build`, ""},
		{guardrm.PowerShell, `rm build -r -fo`, `remove-item build -r -fo`},
	} {
		t.Run(string(tc.shell)+"/"+tc.command, func(t *testing.T) {
			got, err := guardDelete(t.TempDir(), Event{Shell: tc.shell, ShellDeclared: true, Cwd: `C:\repo`, Command: tc.command})
			want := tc.want
			if want != "" {
				want = deleteReason(expectedGuardExecutable(t, tc.shell)+" "+want, "")
			}
			if err != nil || got != want {
				t.Fatalf("declared shell denial: %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestCodexShellInputs(t *testing.T) {
	t.Parallel()
	// Native tool shapes from upstream ExecCommandArgs / LocalShellExecAction;
	// these are protocol fixtures, not an authenticated-agent capture.
	for _, name := range []string{"codex-exec-command.json", "codex-local-shell.json"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
		var out bytes.Buffer
		Codex(e, bytes.NewReader(data), &out, nil)
		want := "`" + expectedGuardExecutable(t, guardrm.Bash) + " rm -rf build`"
		if name == "codex-exec-command.json" {
			want = "`" + expectedGuardExecutable(t, guardrm.PowerShell) + " remove-item -Recurse -Force build`"
		}
		var response claudeOutput
		if json.Unmarshal(out.Bytes(), &response) != nil || response.HookSpecificOutput.PermissionDecision != "deny" || !strings.Contains(response.HookSpecificOutput.PermissionDecisionReason, want) {
			t.Fatalf("%s: %s", name, out.String())
		}
	}
	for _, tc := range []struct {
		tool  string
		input map[string]any
		want  string
	}{
		{"Bash", map[string]any{"command": "Remove-Item -Recurse -Force build", "shell": "bash"}, ""},
		{"shell_command", map[string]any{"command": "rm -rf build"}, ""},
		{"shell_command", map[string]any{"command": "rm -rf build", "shell": "bash"}, expectedGuardExecutable(t, guardrm.Bash) + " rm -rf build"},
		{"local_shell", map[string]any{"command": []string{"pwsh.exe", "-NoProfile", "-Command", "ri -LiteralPath 'a[1]'"}}, ""},
		{"local_shell", map[string]any{"command": []string{"rm", "-rf", "build"}}, ""},
		{"local_shell", map[string]any{"command": []string{"bash", "-c", "rm x", "extra"}}, ""},
		{"local_shell", map[string]any{"command": []string{"cmd.exe", "/c", "del /s /q build"}}, ""},
		{"exec_command", map[string]any{"cmd": "echo 'rm -rf build'", "shell": "bash"}, ""},
		{"exec_command", map[string]any{"cmd": "rm -rf build", "tty": true, "shell": "bash"}, ""},
		{"exec_command", map[string]any{"cmd": "rm x; git -C somewhere clean -fd", "shell": "bash"}, ""},
		{"exec_command", map[string]any{"cmd": "rm x; echo $(rm y)", "shell": "bash"}, ""},
		{"apply_patch", map[string]any{"command": "rm -rf build", "shell": "bash"}, ""},
	} {
		data, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": `C:\repo`, "tool_name": tc.tool, "tool_input": tc.input})
		e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
		var out bytes.Buffer
		Codex(e, bytes.NewReader(data), &out, nil)
		var response claudeOutput
		if tc.want == "" {
			if out.Len() != 0 {
				t.Errorf("must pass: %s -> %s", data, out.String())
			}
			continue
		}
		if json.Unmarshal(out.Bytes(), &response) != nil || response.HookSpecificOutput.PermissionDecision != "deny" || !strings.Contains(response.HookSpecificOutput.PermissionDecisionReason, "`"+tc.want+"`") {
			t.Errorf("%s -> %s", data, out.String())
		}
	}
}

func BenchmarkPreToolUse(b *testing.B) {
	b.Setenv("OBIT_HOOK_AUDIT", "0")
	b.Setenv("OBIT_GUARD_RM", "1")
	b.Setenv("OBIT_DISABLED", "0")
	for _, agent := range []string{"claude", "codex"} {
		for _, command := range []string{"git status --short", "rm -rf build", "rm -f *.log"} {
			b.Run(agent+"/"+command, func(b *testing.B) {
				e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: b.TempDir()}}}
				tool, key, run := "Bash", "command", Claude
				if agent == "codex" {
					tool, key, run = "exec_command", "cmd", Codex
				}
				data, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": b.TempDir(), "tool_name": tool, "tool_input": map[string]string{key: command, "shell": "bash"}})
				b.ReportAllocs()
				for b.Loop() {
					run(e, bytes.NewReader(data), io.Discard, nil)
				}
			})
		}
	}
}

func TestDeleteReview4PassThrough(t *testing.T) {
	t.Parallel()
	for _, agent := range []string{"claude", "codex"} {
		tool, key := "Bash", "command"
		if agent == "codex" {
			tool, key = "exec_command", "cmd"
		}
		for _, command := range []string{
			`hash -p /usr/bin/printf rm; rm keep.txt`,
			`trap '/usr/bin/rm -rf backup' ERR; rm -r repo`,
			`npm run build && rm -rf dist`, `cd "$DIR" && rm keep`,
			`PREVIEW=-ndx; git clean $PREVIEW`, `git clean $PREVIEW`, `rm "$HELP"`,
			"git\rclean -fdx", `rmdir empty/.`, `rmdir empty/..`,
		} {
			t.Run(agent+"/"+command, func(t *testing.T) {
				if got := guardSuggestion(t, agent, `C:\work`, tool, map[string]any{key: command, "shell": "bash"}); got != "" {
					t.Fatalf("unsupported Bash command denied as %q", got)
				}
			})
		}
	}
}

func TestDeleteReview5MainScenarios(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, command string }{
		{"claude-rm", "rm -rf build"},
		{"claude-rm-semicolon", "rm -rf build;"},
		{"claude-cd-rm", "cd x && rm -rf dist"},
		{"claude-cd-rm-semicolon", "cd x && rm -rf dist;"},
		{"claude-multiple", "rm -rf node_modules dist"},
		{"claude-git-clean", "git clean -fdx"},
		{"claude-rm-glob", "rm -f *.log"},
		{"claude-rm-glob-5001", "rm -f *.log"},
		{"claude-rm-quoted-glob", `rm -f '*.log'`},
		{"claude-rm-double-quoted-glob", `rm -f "*.log"`},
		{"claude-git-clean-exclude", `git clean -fdx -e '*.log'`},
		{"claude-rm-newline", "rm -rf build\n"},
		{"claude-cd-newline-rm", "cd x &&\nrm -rf dist"},
		{"claude-rm-parent", "rm -rf ../dist"},
		{"claude-rm-inner-parent", "rm -rf ./build/../dist"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if testing.Short() && tc.name == "claude-rm-glob-5001" {
				t.Skip("creates thousands of real glob candidates")
			}
			t.Parallel()
			cwd := t.TempDir()
			if tc.name == "claude-rm-glob-5001" {
				for i := range 5001 {
					if err := os.WriteFile(filepath.Join(cwd, fmt.Sprintf("file-%05d.log", i)), nil, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			want := strings.Replace(tc.command, "rm ", expectedGuardExecutable(t, guardrm.Bash)+" rm ", 1)
			if strings.HasPrefix(tc.command, "git clean") {
				want = strings.Replace(tc.command, "git clean", expectedGuardExecutable(t, guardrm.Bash)+" git-clean", 1)
			}
			if got := guardSuggestion(t, "claude", cwd, "Bash", map[string]any{"command": tc.command}); got != want {
				t.Fatalf("main scenario: got %q, want %q", got, want)
			}
		})
	}
	for _, tc := range []struct{ name, cwd, workdir, command, shell string }{
		{"codex-wsl", "/mnt/c/repo", "", "rm -rf build", ""},
		{"codex-wsl-relative", "/mnt/c/repo", ".", "rm -rf build", ""},
		{"codex-powershell", `C:\repo`, "", `Remove-Item -Recurse -Force .\build`, "powershell.exe"},
		{"codex-powershell-relative", `C:\repo`, ".", `Remove-Item -Recurse -Force .\build`, "powershell.exe"},
		{"codex-powershell-semicolon", `C:\repo`, "", `Remove-Item -Recurse -Force .\build;`, "powershell.exe"},
		{"codex-powershell-lf", `C:\repo`, "", "Remove-Item -Recurse -Force .\\build\n", "powershell.exe"},
		{"codex-powershell-leading-lf", `C:\repo`, "", "\nRemove-Item -Recurse -Force .\\build", "powershell.exe"},
		{"codex-powershell-leading-comment", `C:\repo`, "", "# cleanup\nRemove-Item -Recurse -Force .\\build", "powershell.exe"},
		{"codex-powershell-block-comment", `C:\repo`, "", "<# cleanup\n #>\nRemove-Item -Recurse -Force .\\build", "powershell.exe"},
		{"codex-powershell-crlf", `C:\repo`, "", "Remove-Item -Recurse -Force .\\build\r\n", "powershell.exe"},
		{"codex-powershell-semicolon-newline", `C:\repo`, "", "Remove-Item -Recurse -Force .\\build;\n", "powershell.exe"},
		{"codex-powershell-drive-slash", `C:\repo`, "", `Remove-Item -Recurse -Force 'D:/repo/build'`, "powershell.exe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := strings.Replace(tc.command, "Remove-Item", expectedGuardExecutable(t, guardrm.PowerShell)+" remove-item", 1)
			if strings.HasPrefix(tc.cwd, "/") {
				// The executable must use WSL interop spelling, even for workdir=".".
				self, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				self, err = filepath.EvalSymlinks(self)
				if err != nil {
					t.Fatal(err)
				}
				want = WSLExecutable(self) + " rm -rf build"
			}
			got := guardSuggestion(t, "codex", tc.cwd, "exec_command", map[string]any{"cmd": tc.command, "workdir": tc.workdir, "shell": tc.shell})
			if got != want {
				t.Fatalf("main scenario: got %q, want %q", got, want)
			}
		})
	}
	for _, tc := range []struct{ name, command string }{
		{"codex-powershell-double-quote", `Remove-Item -LiteralPath "keep"".txt"`},
		{"codex-powershell-single-quote", `Remove-Item -LiteralPath 'keep''.txt'`},
		{"codex-powershell-middle-newline", "Remove-Item build\nWrite-Output done"},
		{"codex-powershell-wildcard", `Remove-Item -Path '*.log'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := guardSuggestion(t, "codex", `C:\repo`, "exec_command", map[string]any{"cmd": tc.command, "shell": "powershell.exe"}); got != "" {
				t.Fatalf("unsupported main scenario must pass through: %s", got)
			}
		})
	}
}

func TestDeleteUncertainSyntax(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		`rm a,b`, `rm 'a'`, `rm "a b"`, `rm a\b`, `rm a?`, `rm a*`, `rm a[1]`,
		`rm $a`, `rm ../keep`, `rm a/../keep`, `rm /tmp/keep`, `rm -Path a`,
		`rm -Recurse a`, `rm -d a`, `rm -v a`, `rm -- a`, `rmdir -v a`,
		`del a`, `ri a`, `rd a`, `erase a`, `git clean -fd`, `FOO=1 rm a`,
		`Write-Output “说明; rm x;结束”`,
	} {
		t.Run(command, func(t *testing.T) {
			if got := guardSuggestion(t, "codex", `C:\work`, "exec_command", map[string]any{"cmd": command}); got != "" {
				t.Fatalf("ambiguous command denied as %q", got)
			}
		})
	}
	for _, command := range []string{`rm build`, `rm -r build`, `rm -f build`, `rm -rf build`, `rm -fr build`, `rm -R build`, `rm --recursive --force ./build`, `rmdir empty`, `Remove-Item build`} {
		if got := guardSuggestion(t, "codex", `C:\work`, "exec_command", map[string]any{"cmd": command}); got != "" {
			t.Errorf("unknown shell must pass through: %q -> %q", command, got)
		}
	}
}

func TestGitRmPassesThrough(t *testing.T) {
	t.Parallel()
	for _, agent := range []string{"claude", "codex"} {
		for _, command := range []string{"git rm x", "git rm --cached x", "git rm -rf x"} {
			if got := guardSuggestion(t, agent, `C:\repo`, "Bash", map[string]any{"command": command, "shell": "bash"}); got != "" {
				t.Errorf("git index operation denied: %s", got)
			}
		}
	}
}

func TestPreToolUseAuditAndFailures(t *testing.T) {
	home := t.TempDir()
	e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}}}
	run := func(command string) string {
		data, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]string{"command": command}})
		var out bytes.Buffer
		Claude(e, bytes.NewReader(data), &out, nil)
		return out.String()
	}
	if run("rm -- '中文 空格'") == "" {
		t.Fatal("expected denial")
	}
	counts, err := guardrm.ReadCounts(home)
	if err != nil || counts.Denied != 1 {
		t.Fatalf("denial not counted: %+v %v", counts, err)
	}
	t.Setenv("OBIT_HOOK_AUDIT", "1")
	if run("sudo rm x") != "" {
		t.Fatal("complex command denied")
	}
	data, err := os.ReadFile(filepath.Join(home, "delete-guard.jsonl"))
	if err != nil || !bytes.Contains(data, []byte(`"kind":"skipped"`)) || !bytes.Contains(data, []byte(expectedGuardExecutable(t, guardrm.Bash)+" rm -- '中文 空格'")) {
		t.Fatalf("audit: %s %v", data, err)
	}
	if got := run("rm " + strings.Repeat("x", 2100)); !strings.Contains(got, `"permissionDecision":"deny"`) {
		t.Fatal("long explanation revoked denial")
	}
	// Neither the audit nor hook.log can be written here; denial must survive.
	bad := filepath.Join(home, "not-a-directory")
	if err := os.WriteFile(bad, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	e.Svc.Store.Home = bad
	if got := run("rm x"); !strings.Contains(got, `"permissionDecision":"deny"`) {
		t.Fatalf("audit/log failure revoked denial: %s", got)
	}
}

func TestPreToolUseAuditFailureStillDenies(t *testing.T) {
	t.Parallel()
	for _, agent := range []string{"claude", "codex"} {
		for _, failure := range []string{"read-only", "exclusive-open"} {
			t.Run(agent+"/"+failure, func(t *testing.T) {
				t.Parallel()
				home := t.TempDir()
				audit := filepath.Join(home, "delete-guard.jsonl")
				if err := os.WriteFile(audit, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				if failure == "read-only" {
					if err := os.Chmod(audit, 0o400); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := os.Chmod(audit, 0o600); err != nil {
							t.Error(err)
						}
					})
				} else {
					path, err := windows.UTF16PtrFromString(audit)
					if err != nil {
						t.Fatal(err)
					}
					handle, err := windows.CreateFile(path, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, 0, 0)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { windows.CloseHandle(handle) })
				}
				// Prove the real append fails, rather than relying on permissions
				// that the current Windows account might be able to bypass.
				if err := guardrm.Record(home, guardrm.AuditEvent{Kind: "denied"}); err == nil {
					t.Fatal("fixture did not prevent audit append")
				}
				e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}}}
				tool, key, run := "Bash", "command", Claude
				if agent == "codex" {
					tool, key, run = "exec_command", "cmd", Codex
				}
				data, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": t.TempDir(), "tool_name": tool, "tool_input": map[string]string{key: "rm -f keep.log", "shell": "bash"}})
				var out bytes.Buffer
				run(e, bytes.NewReader(data), &out, nil)
				var response claudeOutput
				want := "`" + expectedGuardExecutable(t, guardrm.Bash) + " rm -f keep.log`"
				if json.Unmarshal(out.Bytes(), &response) != nil || response.HookSpecificOutput.PermissionDecision != "deny" || !strings.Contains(response.HookSpecificOutput.PermissionDecisionReason, want) {
					t.Errorf("audit failure revoked complete deny: %s", out.String())
				}
				if log, err := os.ReadFile(filepath.Join(home, "hook.log")); err != nil || !bytes.Contains(log, []byte("delete-guard.jsonl")) {
					t.Errorf("audit failure was not logged: %s %v", log, err)
				}
			})
		}
	}
}

func TestPreToolUseDisabled(t *testing.T) {
	for _, setting := range []struct{ key, value string }{{"OBIT_GUARD_RM", "0"}, {"OBIT_DISABLED", "1"}} {
		t.Run(setting.key, func(t *testing.T) {
			for _, agent := range []string{"claude", "codex"} {
				t.Run(agent, func(t *testing.T) {
					t.Setenv("OBIT_GUARD_RM", "1")
					t.Setenv("OBIT_DISABLED", "0")
					e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
					tool, key, run := "Bash", "command", Claude
					if agent == "codex" {
						tool, key, run = "exec_command", "cmd", Codex
					}
					data, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": `C:\repo`, "tool_name": tool, "tool_input": map[string]string{key: "rm -rf build", "shell": "bash"}})
					var out bytes.Buffer
					run(e, bytes.NewReader(data), &out, nil)
					var response claudeOutput
					if json.Unmarshal(out.Bytes(), &response) != nil || response.HookSpecificOutput.PermissionDecision != "deny" {
						t.Fatalf("enabled control must deny the same payload: %s", out.String())
					}
					t.Setenv(setting.key, setting.value)
					out.Reset()
					run(e, bytes.NewReader(data), &out, nil)
					if out.Len() != 0 {
						t.Fatalf("disabled guard: %s", out.String())
					}
				})
			}
		})
	}
}

func TestDeleteSkippedScopeAudited(t *testing.T) {
	t.Setenv("OBIT_HOOK_AUDIT", "1")
	for _, tc := range []struct{ cwd, command, shell string }{
		{"/home/me/work", "rm build", "bash"},
		{"/mnt/c/work", "rm a,b", "pwsh"},
		{"/mnt/c/work", "rm ../keep", "bash"},
		{"/mnt/c/work", "git clean -fd", "bash"},
		{`C:\work`, "rm a,b", ""},
		{`C:\work`, `Set-Location HKCU:\Software; Remove-Item Demo`, "pwsh"},
		{`C:\work`, "rm build", "cmd.exe"},
	} {
		t.Run(tc.cwd+tc.command, func(t *testing.T) {
			home := t.TempDir()
			e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}}}
			data, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": tc.cwd, "tool_name": "exec_command", "tool_input": map[string]string{"cmd": tc.command, "shell": tc.shell}})
			var out bytes.Buffer
			Codex(e, bytes.NewReader(data), &out, nil)
			if out.Len() != 0 {
				t.Fatalf("skipped command denied: %s", out.String())
			}
			data, err := os.ReadFile(filepath.Join(home, "delete-guard.jsonl"))
			var event guardrm.AuditEvent
			if err != nil || json.Unmarshal(data, &event) != nil || event.Kind != "skipped" || event.Command != tc.command || event.Reason == "" {
				t.Fatalf("missing skip receipt: %s %v", data, err)
			}
		})
	}
}

func TestDeleteUnknownShellAudit(t *testing.T) {
	t.Setenv("OBIT_HOOK_AUDIT", "1")
	for _, command := range []string{"rm -rf build", "rmdir empty", `Remove-Item -Recurse -Force .\build`} {
		home := t.TempDir()
		e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}}}
		payload, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": `C:\work`, "tool_name": "exec_command", "tool_input": map[string]string{"cmd": command}})
		var out bytes.Buffer
		Codex(e, bytes.NewReader(payload), &out, nil)
		data, err := os.ReadFile(filepath.Join(home, "delete-guard.jsonl"))
		var event guardrm.AuditEvent
		if err != nil || json.Unmarshal(data, &event) != nil || out.Len() != 0 || event.Kind != "skipped" || event.Reason != "unknown shell" {
			t.Errorf("unknown shell not passed through with an audit reason: %s %s %v", out.String(), data, err)
		}
	}
}

func TestPreToolUseGlobScanDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	if os.Getenv("OBIT_TEST_GLOB_DEADLINE") != "1" {
		// Compile a slow ReadDir boundary, leaving the actual scan and hook
		// decisions intact. No wall-clock race or production test switch.
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		original := filepath.Join(root, "internal/guardrm/glob.go")
		source, err := os.ReadFile(original)
		from := "scanLeadingGlob(f.ReadDir, func() time.Duration { return time.Since(started) })"
		to := `scanLeadingGlob(func(n int) ([]os.DirEntry, error) {
			entries, err := f.ReadDir(n)
			started = 50 * time.Millisecond
			return entries, err
		}, func() time.Duration { return started })`
		if err != nil || strings.Count(string(source), from) != 1 {
			t.Fatalf("scan clock fixture target: %v", err)
		}
		scratch := t.TempDir()
		altered := filepath.Join(scratch, "slow_scan.go")
		slow := strings.Replace(string(source), "started := time.Now()", "started := time.Duration(0)", 1)
		if err := os.WriteFile(altered, []byte(strings.Replace(slow, from, to, 1)), 0o600); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(map[string]any{"Replace": map[string]string{original: altered}})
		overlay := filepath.Join(scratch, "overlay.json")
		if err := os.WriteFile(overlay, data, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "test", "-overlay", overlay, "./internal/hook", "-run", "^TestPreToolUseGlobScanDeadline$", "-count=1")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "OBIT_TEST_GLOB_DEADLINE=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("hook with expired scan deadline: %s %v", out, err)
		}
		return
	}
	for _, dash := range []bool{false, true} {
		t.Run(fmt.Sprint(dash), func(t *testing.T) {
			cwd := t.TempDir()
			name := "keep.log"
			if dash {
				name = "-i"
			}
			if err := os.WriteFile(filepath.Join(cwd, name), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			for _, agent := range []string{"claude", "codex"} {
				tool, key := "Bash", "command"
				if agent == "codex" {
					tool, key = "exec_command", "cmd"
				}
				for _, command := range []string{"rm -rf *", "rmdir *", "git clean -fdx *"} {
					got := guardSuggestion(t, agent, cwd, tool, map[string]any{key: command, "shell": "bash"})
					if (got == "") != dash {
						t.Errorf("%s %s: dash=%v, suggestion=%q", agent, command, dash, got)
					}
				}
			}
		})
	}
}

func TestDeleteReview6GlobScope(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"clean", "dash-file", "dash-directory", "missing", "not-directory"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			cwd := t.TempDir()
			switch state {
			case "dash-file":
				if err := os.WriteFile(filepath.Join(cwd, "-n"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			case "dash-directory":
				if err := os.Mkdir(filepath.Join(cwd, "-i"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "missing", "not-directory":
				cwd = filepath.Join(cwd, "absent")
				if state == "not-directory" {
					if err := os.WriteFile(cwd, nil, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			for _, command := range []string{"rm -f *.log", "rm * ?n [-a]*", "rmdir *", "git clean -fdx *"} {
				want := ""
				if state != "dash-file" && state != "dash-directory" {
					want = expectedGuardExecutable(t, guardrm.Bash) + " " + strings.Replace(command, "git clean", "git-clean", 1)
				}
				if got := guardSuggestion(t, "claude", cwd, "Bash", map[string]any{"command": command}); got != want {
					t.Errorf("%s: got %q, want %q", command, got, want)
				}
			}
			for _, command := range []string{`rm -f '*.log'`, `rm -f "*.log"`, `rm -f \*.log`, `git clean -fdx -e '*.log'`, `rm -- *.log`} {
				want := expectedGuardExecutable(t, guardrm.Bash) + " " + strings.Replace(command, "git clean", "git-clean", 1)
				if got := guardSuggestion(t, "claude", cwd, "Bash", map[string]any{"command": command}); got != want {
					t.Errorf("literal/option-terminated glob %s: got %q, want %q", command, got, want)
				}
			}
			for _, command := range []string{`rm -rf */build`, `rm -rf ?/build`, `rm -rf [ab]/build`, `cd x && rm *`} {
				if got := guardSuggestion(t, "claude", cwd, "Bash", map[string]any{"command": command}); got != "" {
					t.Errorf("uninspected glob location: %s -> %s", command, got)
				}
			}
		})
	}
}

func TestDeleteReview6UnsafePaths(t *testing.T) {
	t.Parallel()
	for _, command := range []string{`rm -rf /tmp/build`, `rmdir /c/empty`, `git clean -fdx /c/build`, `rm -rf -- '/c/build'`, `rm -rf //server/share/build`} {
		if got := guardSuggestion(t, "claude", t.TempDir(), "Bash", map[string]any{"command": command}); got != "" {
			t.Errorf("ambiguous POSIX path: %s -> %s", command, got)
		}
	}
	for _, command := range []string{`Remove-Item -LiteralPath 'keep"".txt'`, `Remove-Item 'keep".txt'`, `Remove-Item -Path 'keep"".txt'`} {
		if got := guardSuggestion(t, "codex", t.TempDir(), "exec_command", map[string]any{"cmd": command, "shell": "powershell.exe"}); got != "" {
			t.Errorf("PowerShell native quoting changes path: %s -> %s", command, got)
		}
	}
}

func TestPreToolUseLongCommandStillDenies(t *testing.T) {
	if testing.Short() {
		t.Skip("exercises shell parsing and audit output with a very large command")
	}
	t.Parallel()
	paths := make([]string, 400)
	for i := range paths {
		paths[i] = fmt.Sprintf("out%04d", i)
	}
	for _, agent := range []string{"claude", "codex"} {
		for _, name := range []string{"rm", "rmdir", "git clean"} {
			t.Run(agent+"/"+name, func(t *testing.T) {
				cwd, home := t.TempDir(), t.TempDir()
				for _, path := range paths {
					if err := os.Mkdir(filepath.Join(cwd, path), 0o700); err != nil {
						t.Fatal(err)
					}
				}
				command := name + " " + strings.Join(paths, " ")
				if name == "rm" {
					command = "rm -rf " + strings.Join(paths, " ")
				}
				tool, key, run := "Bash", "command", Claude
				if agent == "codex" {
					tool, key, run = "exec_command", "cmd", Codex
				}
				data, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": cwd, "tool_name": tool, "tool_input": map[string]string{key: command, "shell": "bash"}})
				var out bytes.Buffer
				run(&auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}}}, bytes.NewReader(data), &out, nil)
				var response claudeOutput
				if err := json.Unmarshal(out.Bytes(), &response); err != nil {
					t.Fatalf("long command lost deny: %q %v", out.String(), err)
				}
				h := response.HookSpecificOutput
				replacement := "obit " + strings.ReplaceAll(name, " ", "-")
				want := "Obituary's delete guard is on: the same command with `" + name + "` replaced by `" + replacement + "` via the installed executable (all other arguments unchanged) provides restorable deletion."
				if h.PermissionDecision != "deny" || h.PermissionDecisionReason != want || len(h.PermissionDecisionReason) > maxContext {
					t.Fatalf("long command deny contract: %s", out.String())
				}
				data, err := os.ReadFile(filepath.Join(home, "delete-guard.jsonl"))
				var receipt guardrm.AuditEvent
				if err != nil || json.Unmarshal(data, &receipt) != nil || receipt.Kind != "denied" || receipt.Command != command || !strings.HasSuffix(receipt.Equivalent, strings.Join(paths, " ")) {
					t.Fatalf("long denial receipt lost: %s %v", data, err)
				}
			})
		}
	}
	command := "# " + strings.Repeat("说明", 400) + "\nri -Recurse build"
	home := t.TempDir()
	reason, err := guardDelete(home, Event{Command: command, Cwd: t.TempDir(), Shell: guardrm.PowerShell})
	want := "Obituary's delete guard is on: the same command with `Remove-Item` replaced by `obit remove-item` via the installed executable (all other arguments unchanged) provides restorable deletion."
	if err != nil || reason != want || len(reason) > maxContext {
		t.Fatalf("long PowerShell denial: %q %v", reason, err)
	}
}

func TestPreToolUseSummaryNormalizesCommandNames(t *testing.T) {
	t.Parallel()
	padding := strings.Repeat(" ", 2000)
	for _, tc := range []struct {
		command, original, replacement, shell string
	}{
		{"rm" + padding + "-rf build", "rm", "rm", "bash"},
		{"rmdir" + padding + "build", "rmdir", "rmdir", "bash"},
		{"git" + padding + "clean -fdx " + strings.Repeat("x", 2000), "git clean", "git-clean", "bash"},
		{"ri" + padding + "-Recurse build", "Remove-Item", "remove-item", "pwsh"},
	} {
		for _, agent := range []string{"claude", "codex"} {
			if tc.shell == "pwsh" && agent == "claude" {
				continue // Claude's Bash tool does not run PowerShell source.
			}
			t.Run(agent+"/"+tc.original, func(t *testing.T) {
				home := t.TempDir()
				tool, key, run := "Bash", "command", Claude
				if agent == "codex" {
					tool, key, run = "exec_command", "cmd", Codex
				}
				data, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": t.TempDir(), "tool_name": tool, "tool_input": map[string]string{key: tc.command, "shell": tc.shell}})
				var out bytes.Buffer
				run(&auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}}}, bytes.NewReader(data), &out, nil)
				var reply claudeOutput
				if err := json.Unmarshal(out.Bytes(), &reply); err != nil {
					t.Fatalf("missing denial: %q %v", out.String(), err)
				}
				h := reply.HookSpecificOutput
				if h.PermissionDecision != "deny" || len(h.PermissionDecisionReason) > maxContext || !strings.Contains(h.PermissionDecisionReason, "`"+tc.original+"` replaced by `obit "+tc.replacement+"`") || !strings.Contains(h.PermissionDecisionReason, "all other arguments unchanged") {
					t.Errorf("normalized command mapping lost: %s", out.String())
				}
			})
		}
	}
}

func TestDeleteReasonStaysWithinByteBudget(t *testing.T) {
	t.Parallel()
	for _, size := range []int{maxContext - 1, maxContext, maxContext + 1} {
		reason := deleteReason(strings.Repeat("x", size), "the same command with `rm` replaced by `obit rm`")
		if len(reason) > maxContext || !strings.Contains(reason, "`obit rm`") {
			t.Errorf("oversized equivalent: %q", reason)
		}
	}
	reason := deleteReason(strings.Repeat("字", maxContext), strings.Repeat("字", maxContext))
	if len(reason) > maxContext || !strings.Contains(reason, "delete guard is on") {
		t.Fatalf("oversized executable revoked bounded reason: %q", reason)
	}
}

func TestDeleteWSLSafeRelativeOnly(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		`rm -f /mnt/c/work/missing/../keep.txt`, `rm -rf ../../d/data`,
		`rm build /mnt/c/keep.txt`, `rm ../keep`, `rm x/../keep`, `rm ~/keep`,
		`rm $file`, `rm build/*`, `rm prod?`, `rm a[1]`, `rm a\b`, `rm a,b`,
		`rm 'build'`, `rm "build"`, `rm C:/keep`, `rm ~name`,
		`FOO=1 rm build`, `GIT_DIR=/mnt/d/repo/.git git clean -fd`, `git clean -fd`,
		`cd /home/me; rm build`, `pushd /tmp; rm build`,
		`cd /mnt/d/work; rm build`, `cd x && rm build`, `cd; rmdir empty`,
	} {
		t.Run(command, func(t *testing.T) {
			if got := guardSuggestion(t, "codex", "/mnt/c/work", "exec_command", map[string]any{"cmd": command}); got != "" {
				t.Fatalf("WSL unsafe operand denied as %q", got)
			}
		})
	}
	for _, command := range []string{`rm -rf build node_modules`, `rm ./build`, `rmdir empty`} {
		got := guardSuggestion(t, "codex", "/mnt/c/work", "exec_command", map[string]any{"cmd": command})
		if !strings.HasPrefix(got, "'/mnt/") || !strings.HasSuffix(got, " "+command) {
			t.Errorf("safe WSL arguments changed: %q -> %q", command, got)
		}
	}
	for _, cwd := range []string{"/mnt/c/work", "/home/me/work", "", "relative"} {
		for _, input := range []map[string]any{
			{"cmd": "rm a,b", "shell": "pwsh"},
			{"command": []string{"pwsh", "-Command", "rm a,b"}},
			{"cmd": "Remove-Item build", "shell": "pwsh"},
		} {
			if got := guardSuggestion(t, "codex", cwd, "local_shell", input); got != "" {
				t.Errorf("Linux PowerShell denied as %q", got)
			}
		}
	}
}

func TestDeletePowerShellLiteralScope(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	commands := []string{
		`Set-Location HKCU:\Software; Remove-Item -Recurse Demo`,
		`Remove-Item -Recurse -Path 'prod?'`, `Remove-Item -LiteralPath 'a,b'`,
		`Remove-Item a,b`, `Remove-Item a b`, `Remove-Item -Path a b`,
		`Remove-Item -Path 'build/*'`, `Remove-Item -LiteralPath 'a[1]'`,
		`Remove-Item -Path $path`, "Remove-Item -Path a`b", `Remove-Item -p a`,
		`Remove-Item -Force:$true a`, `Remove-Item -WhatIf a`,
		`Remove-Item -LiteralPath /mnt/c/keep.txt`,
		`Write-Output “说明; rm x;结束”`, `Write-Output 'x' & Remove-Item a`,
		`. script.ps1; Remove-Item a`, `& script.ps1; Remove-Item a`,
	}
	for _, provider := range []string{`HKCU:\Software\Demo`, `Env:Demo`, `function:Demo`, `Variable:Demo`, `Cert:Demo`, `X:Demo`, `FileSystem::C:\Demo`} {
		commands = append(commands, "Remove-Item -Recurse "+provider)
	}
	for _, location := range []string{"Set-Location", "sl", "cd", "chdir", "Push-Location", "pushd", "Pop-Location", "popd", "New-PSDrive", "SET-LOCATION"} {
		commands = append(commands, location+` HKCU:\Software; Remove-Item Demo`, `Remove-Item Demo; `+location+` HKCU:\Software`)
	}
	for _, quote := range []string{"“", "”", "‘", "’", "„", "‚", "«", "»", "＂"} {
		commands = append(commands, "Write-Output "+quote+"说明; rm x;结束"+quote)
	}
	for _, shell := range []string{"", "pwsh"} {
		for _, command := range commands {
			t.Run(shell+"/"+command, func(t *testing.T) {
				if got := guardSuggestion(t, "codex", `C:\work`, "exec_command", map[string]any{"cmd": command, "shell": shell}); got != "" {
					t.Fatalf("unsupported PowerShell command denied as %q", got)
				}
			})
		}
	}
	for _, alias := range []string{"Remove-Item", "ri", "del", "erase", "rd", "rmdir", "rm"} {
		command := alias + ` -Recurse -Force .\build`
		if got := guardSuggestion(t, "codex", `C:\work`, "exec_command", map[string]any{"cmd": command, "shell": "pwsh"}); got != expectedGuardExecutable(t, guardrm.PowerShell)+` remove-item -Recurse -Force .\build` {
			t.Errorf("explicit PowerShell: %q -> %q", command, got)
		}
	}
	for _, path := range []string{`'build dir'`, `C:\work\build`, `X:/Demo`} {
		command := "Remove-Item -LiteralPath " + path
		if got := guardSuggestion(t, "codex", `C:\work`, "exec_command", map[string]any{"cmd": command, "shell": "pwsh"}); got != expectedGuardExecutable(t, guardrm.PowerShell)+" remove-item -LiteralPath "+path {
			t.Errorf("literal path changed: %q -> %q", command, got)
		}
	}
	for _, command := range []string{`rm -rf build`, `cd x && rm -rf dist`} {
		want := strings.Replace(command, "rm ", expectedGuardExecutable(t, guardrm.Bash)+" rm ", 1)
		if got := guardSuggestion(t, "claude", `C:\work`, "Bash", map[string]any{"command": command}); got != want {
			t.Errorf("Claude Bash: %q -> %q", command, got)
		}
	}
}

func TestDeleteShellContext(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"ri x", "Remove-Item x", "del x", "rd x", "erase x", "rm -Path x"} {
		if got := guardSuggestion(t, "claude", `C:\repo`, "Bash", map[string]any{"command": command}); got != "" {
			t.Errorf("Bash non-deletion %q denied as %q", command, got)
		}
	}
	for _, alias := range []string{"rm", "del", "ri", "rd", "rmdir", "erase", "Remove-Item", "RM"} {
		if got := guardSuggestion(t, "codex", `C:\repo`, "exec_command", map[string]any{"cmd": alias + " x", "shell": "pwsh"}); got != expectedGuardExecutable(t, guardrm.PowerShell)+" remove-item x" {
			t.Errorf("PowerShell %s: %q", alias, got)
		}
	}
	for _, cwd := range []string{"", "relative"} {
		if got := guardSuggestion(t, "codex", cwd, "exec_command", map[string]any{"cmd": "rm -rf x"}); got != "" {
			t.Errorf("invalid or unknown shell (%s) denied: %s", cwd, got)
		}
	}
	for _, tc := range []struct {
		argv []string
		want string
	}{
		{[]string{"bash", "-lc", "rm -rf x"}, expectedGuardExecutable(t, guardrm.Bash) + " rm -rf x"},
		{[]string{"pwsh.exe", "-NoProfile", "-Command", "rm x"}, expectedGuardExecutable(t, guardrm.PowerShell) + " remove-item x"},
		{[]string{"powershell", "-Command", "rm -rf x"}, ""},
	} {
		if got := guardSuggestion(t, "codex", `C:\repo`, "local_shell", map[string]any{"command": tc.argv}); got != tc.want {
			t.Errorf("explicit shell %v: %q, want %q", tc.argv, got, tc.want)
		}
	}
}

func TestDeleteWSLContext(t *testing.T) {
	t.Parallel()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if real, err := filepath.EvalSymlinks(self); err == nil {
		self = real
	}
	wsl := "/mnt/" + strings.ToLower(self[:1]) + filepath.ToSlash(self[2:])
	for _, command := range []string{"rm x", "rmdir empty", "rm -rf build node_modules"} {
		got := guardSuggestion(t, "codex", "/mnt/c/repo", "exec_command", map[string]any{"cmd": command})
		if !strings.Contains(got, wsl) || !strings.Contains(got, strings.Replace(command, "git clean", "git-clean", 1)) {
			t.Errorf("WSL suggestion lacks interop executable: %q", got)
		}
	}
	for _, cwd := range []string{"/home/me/repo", "/tmp/repo", "~/repo"} {
		if got := guardSuggestion(t, "codex", cwd, "local_shell", map[string]any{"command": []string{"bash", "-c", "rm x"}}); got != "" {
			t.Errorf("WSL filesystem cwd denied: %s -> %s", cwd, got)
		}
	}
	for _, command := range []string{"rm /home/me/x", "rm /tmp/x", "rm ~/x", "rm '~name'", "rm /mnt/c/x /home/me/y", "git clean -f -- /tmp/x", "rmdir /var/tmp/x", "rm /mnt/c/../../home/x"} {
		if got := guardSuggestion(t, "codex", "/mnt/c/repo", "exec_command", map[string]any{"cmd": command}); got != "" {
			t.Errorf("WSL filesystem operand denied: %s -> %s", command, got)
		}
	}
	got := guardSuggestion(t, "codex", "/mnt/c/repo", "local_shell", map[string]any{"command": []string{"pwsh", "-Command", "rm x"}})
	if got != "" {
		t.Errorf("Linux PowerShell must pass through: %s", got)
	}
	got = guardSuggestion(t, "codex", "/mnt/c/repo", "local_shell", map[string]any{"command": []string{"pwsh", "-Command", "rm /mnt/c/x,/tmp/y"}})
	if got != "" {
		t.Errorf("WSL filesystem array member denied: %s", got)
	}
}

func TestRemoveItemBinding(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"Remove-Item a b", "rm -Path a b", "ri -LiteralPath a b", "rm -f x", "rm -Unknown x", "rm -Filter '*.log' x", "rm -Include x y", "rm -Exclude x y", "rm -WhatIf x", "rm -Confirm x", "rm 'a,b'", "rm a,,b", "rm a,", "rm '-Recurse' x", "rm '-Force' x", "rm '-Path' x", "Remove-Item a,b", "Remove-Item a, b", "Remove-Item -Path a,b", "Remove-Item -LiteralPath 'a b', 'c' -Recurse", "Remove-Item -Rec -f x"} {
		if got := guardSuggestion(t, "codex", `C:\repo`, "exec_command", map[string]any{"cmd": command, "shell": "pwsh"}); got != "" {
			t.Errorf("invalid/unsupported binding denied: %s -> %s", command, got)
		}
	}
	for _, command := range []string{"Remove-Item -Path build", "Remove-Item -LiteralPath 'a b' -Recurse", "Remove-Item -Recurse -Force x", "Remove-Item -Rec a", "Remove-Item -Rec -Force x"} {
		want := strings.Replace(command, "Remove-Item", expectedGuardExecutable(t, guardrm.PowerShell)+" remove-item", 1)
		if got := guardSuggestion(t, "codex", `C:\repo`, "exec_command", map[string]any{"cmd": command, "shell": "pwsh"}); got != want {
			t.Errorf("valid literal path: %s -> %s", command, got)
		}
	}
}
