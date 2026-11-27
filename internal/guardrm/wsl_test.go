package guardrm

import (
	"strings"
	"testing"
)

func TestWSLUnregisterDeletion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		shell   Shell
		command string
		want    string
	}{
		// Basic forms across Bash and PowerShell
		{"bash-basic", Bash, "wsl --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"ps-basic", PowerShell, "wsl --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"bash-exe", Bash, "wsl.exe --unregister Ubuntu-24.04", "wsl --unregister Ubuntu-24.04"},
		{"ps-exe", PowerShell, "wsl.exe --unregister Ubuntu-24.04", "wsl --unregister Ubuntu-24.04"},
		{"bash-case", Bash, "WSL.EXE --Unregister Debian", "wsl --unregister Debian"},
		{"ps-case", PowerShell, "WSL.EXE --UNREGISTER Ubuntu", "wsl --unregister Ubuntu"},
		// Full paths (Windows and WSL /mnt/c)
		{"ps-win-path", PowerShell, `C:\Windows\System32\wsl.exe --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"ps-call-win-path", PowerShell, `& 'C:\Windows\System32\WSL.EXE' --Unregister Ubuntu-24.04`, "wsl --unregister Ubuntu-24.04"},
		{"bash-win-quoted", Bash, `'C:\Windows\System32\wsl.exe' --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"bash-mnt-path", Bash, "/mnt/c/Windows/System32/wsl.exe --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"bash-mnt-upper", Bash, "/mnt/c/Windows/System32/WSL.EXE --Unregister Ubuntu", "wsl --unregister Ubuntu"},
		// Options before/after and = form
		{"bash-equals", Bash, "wsl --unregister=Ubuntu-24.04", "wsl --unregister Ubuntu-24.04"},
		{"ps-equals", PowerShell, "wsl --unregister=Ubuntu-24.04", "wsl --unregister Ubuntu-24.04"},
		{"ps-equals-quoted", PowerShell, `wsl --unregister="Ubuntu-24.04"`, "wsl --unregister Ubuntu-24.04"},
		{"bash-user-before", Bash, "wsl -u root --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"ps-user-before", PowerShell, "wsl -u root --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"bash-long-user-before", Bash, "wsl --user root --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"bash-distro-opt-before", Bash, "wsl -d Debian --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"bash-opt-after", Bash, "wsl --unregister Ubuntu -u root", "wsl --unregister Ubuntu"},
		{"bash-opt-between", Bash, "wsl --unregister -u root Ubuntu", "wsl --unregister Ubuntu"},
		{"bash-help-after-unregister", Bash, "wsl --unregister Ubuntu --help", "wsl --unregister Ubuntu"},
		// Bare / dynamic / unsafe distro names are still conservatively blocked
		{"bash-bare", Bash, "wsl --unregister", "wsl --unregister"},
		{"ps-bare", PowerShell, "wsl --unregister", "wsl --unregister"},
		{"bash-equals-empty", Bash, "wsl --unregister=", "wsl --unregister"},
		{"bash-var-distro", Bash, "wsl --unregister $DISTRO", "wsl --unregister"},
		{"ps-var-distro", PowerShell, "wsl --unregister $distro", "wsl --unregister"},
		{"bash-space-distro", Bash, `wsl --unregister "Ubuntu 24.04"`, "wsl --unregister"},
		// Compound commands, pipelines, subshells, substitutions
		{"bash-and", Bash, "echo start && wsl --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"bash-or", Bash, "false || wsl.exe --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"bash-semi", Bash, "wsl --list; wsl --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"bash-pipe", Bash, "wsl --unregister Ubuntu | cat", "wsl --unregister Ubuntu"},
		{"bash-subshell", Bash, "(wsl --unregister Ubuntu)", "wsl --unregister Ubuntu"},
		{"bash-subst", Bash, "echo $(wsl --unregister Ubuntu)", "wsl --unregister Ubuntu"},
		{"bash-backtick", Bash, "echo `wsl --unregister Ubuntu`", "wsl --unregister Ubuntu"},
		{"bash-quoted-cmd", Bash, `'wsl' --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"bash-parse-error", Bash, "wsl --unregister Ubuntu\necho 'unterminated", "wsl --unregister Ubuntu"},
		{"bash-xargs", Bash, "printf 'Ubuntu\n' | xargs wsl --unregister", "xargs wsl --unregister"},
		{"bash-find-exec", Bash, `find . -name x -exec wsl --unregister Ubuntu \;`, "find -exec wsl --unregister Ubuntu"},
		{"ps-semi", PowerShell, "wsl --list; wsl --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"ps-and", PowerShell, "Write-Output go && wsl.exe --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"ps-pipe", PowerShell, "wsl --unregister Ubuntu | Out-Null", "wsl --unregister Ubuntu"},
		{"ps-subexpr", PowerShell, `Write-Output "$(wsl --unregister Ubuntu)"`, "wsl --unregister Ubuntu"},
		{"ps-scriptblock", PowerShell, "if ($true) { wsl --unregister Ubuntu }", "wsl --unregister Ubuntu"},
		{"ps-dot-source", PowerShell, ". wsl.exe --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"ps-backtick-escape", PowerShell, "w`sl --unregister Ubuntu", "wsl --unregister Ubuntu"},
		// cmd /c and powershell -Command wrappers
		{"bash-cmd-c-quoted", Bash, `cmd /c "wsl --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"bash-cmd-exe-c-chain", Bash, `cmd.exe /C "echo hi && C:\Windows\System32\wsl.exe --unregister Ubuntu-24.04"`, "wsl --unregister Ubuntu-24.04"},
		{"bash-cmd-double-slash", Bash, `cmd //c "wsl --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"bash-cmd-c-unquoted", Bash, `cmd /c wsl --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"ps-cmd-c-quoted", PowerShell, `cmd /c "wsl --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"ps-cmd-c-caret", PowerShell, `cmd.exe /c "w^sl --un^register Ubuntu"`, "wsl --unregister Ubuntu"},
		// BLOCKER-1: cmd.exe /c outer double-quote stripping (including spaced WSL2 paths)
		{"cmd-outer-quotes-sys32", CMD, `""C:\Windows\System32\wsl.exe" --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"bash-cmd-c-outer-quotes-sys32", Bash, `cmd /c '""C:\Windows\System32\wsl.exe" --unregister Ubuntu"'`, "wsl --unregister Ubuntu"},
		{"ps-cmd-c-outer-quotes-sys32", PowerShell, `cmd /c '""C:\Windows\System32\wsl.exe" --unregister Ubuntu"'`, "wsl --unregister Ubuntu"},
		{"cmd-nested-outer-quotes-progfiles", CMD, `cmd /c ""C:\Program Files\WSL\wsl.exe" --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"cmd-outer-quotes-progfiles", CMD, `""C:\Program Files\WSL\wsl.exe" --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"cmd-quoted-path-quoted-distro", CMD, `"C:\Program Files\WSL\wsl.exe" --unregister "Ubuntu"`, "wsl --unregister Ubuntu"},
		// BLOCKER-2: cmd /c multi-arg forms, attached /c, and CMD @ prefix
		{"bash-cmd-c-unquoted-caret", Bash, `cmd /c w^sl --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"ps-cmd-c-unquoted-caret", PowerShell, `cmd /c w^sl --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"bash-cmd-exe-c-unquoted-caret", Bash, `cmd.exe /c w^sl --un^register Ubuntu`, "wsl --unregister Ubuntu"},
		{"ps-cmd-exe-c-unquoted-caret", PowerShell, `cmd.exe /c w^sl --un^register Ubuntu`, "wsl --unregister Ubuntu"},
		{"bash-cmd-c-inline-amp-escape", Bash, `cmd /c echo ok\&wsl --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"bash-cmd-c-inline-amp-quoted", Bash, `cmd /c echo 'ok&wsl' --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"ps-cmd-c-inline-amp-quoted", PowerShell, `cmd /c echo 'ok&wsl' --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"bash-cmd-c-paren-escape", Bash, `cmd /c \(wsl --unregister Ubuntu\)`, "wsl --unregister Ubuntu"},
		{"cmd-at-prefix", CMD, `@wsl --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"bash-cmd-c-at-prefix", Bash, `cmd /c "@wsl --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"ps-cmd-c-at-prefix", PowerShell, `cmd /c "@wsl --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"cmd-nested-at-prefix", CMD, `cmd /c "@wsl --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"bash-cmd-c-attached-quote", Bash, `cmd /c"wsl --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"ps-cmd-c-attached-quote", PowerShell, `cmd /c"wsl --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"cmd-nested-c-attached-quote", CMD, `cmd /c"wsl --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"bash-cmd-c-attached-word", Bash, `cmd /cwsl --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"ps-cmd-c-attached-word", PowerShell, `cmd /cwsl --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"cmd-nested-c-attached-word", CMD, `cmd /cwsl --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"bash-ps-command", Bash, `powershell -Command "wsl --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"bash-pwsh-c", Bash, `pwsh.exe -NoProfile -c "wsl.exe --Unregister=Ubuntu-24.04"`, "wsl --unregister Ubuntu-24.04"},
		{"ps-ps-command", PowerShell, `powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "wsl --unregister Ubuntu"`, "wsl --unregister Ubuntu"},
		{"bash-ps-command-unquoted", Bash, `powershell -Command wsl --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"bash-ps-command-spaced-path", Bash, `powershell -Command "C:\Program Files\WSL\wsl.exe" --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"bash-ps-nested-command", Bash, `powershell -c powershell -c wsl --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		// Direct CMD shell context
		{"cmd-basic", CMD, "wsl --unregister Ubuntu", "wsl --unregister Ubuntu"},
		{"cmd-win-path", CMD, `"C:\Windows\System32\wsl.exe" --Unregister Ubuntu-24.04`, "wsl --unregister Ubuntu-24.04"},
		{"cmd-chain", CMD, `echo ok && wsl.exe --unregister=Ubuntu`, "wsl --unregister Ubuntu"},
		{"cmd-pipe", CMD, `echo y | wsl --unregister Ubuntu`, "wsl --unregister Ubuntu"},
		{"cmd-caret", CMD, `w^sl --un^register Ubuntu`, "wsl --unregister Ubuntu"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Deletion(tc.command, tc.shell); got != tc.want {
				t.Fatalf("Deletion(%q, %q) = %q, want %q", tc.command, tc.shell, got, tc.want)
			}
		})
	}
}

func TestWSLNonUnregisterCommandsPass(t *testing.T) {
	t.Parallel()
	commands := []string{
		"wsl --list",
		"wsl -l -v",
		"wsl --list --verbose",
		"wsl --status",
		"wsl --version",
		"wsl --help",
		"wsl -h",
		"wsl --help --unregister Ubuntu",
		"wsl --export Ubuntu backup.tar",
		"wsl --export Ubuntu --unregister",
		`wsl --import Ubuntu C:\wsl\ubuntu backup.tar`,
		`wsl --import Ubuntu C:\wsl --unregister`,
		"wsl --terminate Ubuntu",
		"wsl -t Ubuntu",
		"wsl --shutdown",
		"wsl -d Ubuntu -- echo hello",
		"wsl -d Ubuntu -- echo --unregister",
		"wsl -d Ubuntu -- echo wsl --unregister Ubuntu",
		"wsl -d Ubuntu echo --unregister",
		"wsl echo --unregister Ubuntu",
		"wsl --exec echo --unregister Ubuntu",
		"wsl -e echo --unregister Ubuntu",
		// Inner rm -rf must keep existing behavior, never be misclassified as --unregister.
		"wsl -d Ubuntu -- rm -rf /tmp/foo",
		// Plain text, comments, arguments, commit messages
		`echo "wsl --unregister Ubuntu"`,
		"echo wsl --unregister Ubuntu",
		`grep -rn -- "--unregister" .`,
		`git commit -m "guard wsl --unregister"`,
		"# wsl --unregister Ubuntu",
		"wsl --manage Ubuntu --move D:\\wsl",
		"wsl --manage Ubuntu --move --unregister",
		// Safe cmd /c and powershell -Command invocations
		`cmd /c "echo wsl --unregister Ubuntu"`,
		`cmd /c echo "a && wsl --unregister Ubuntu"`,
		`cmd /c "@echo wsl --unregister Ubuntu"`,
		`cmd /c "wsl --list -v"`,
		`cmd /c "del /s /q build"`,
		`powershell -Command "Write-Output 'wsl --unregister Ubuntu'"`,
		`powershell -Command "wsl --status"`,
		`powershell -File ./cleanup.ps1`,
		`pwsh -fi ./script.ps1 "a; wsl --unregister Ubuntu"`,
		`pwsh -fil ./script.ps1 "a; wsl --unregister Ubuntu"`,
	}
	for _, command := range commands {
		for _, shell := range []Shell{Bash, PowerShell} {
			t.Run(string(shell)+"/"+command, func(t *testing.T) {
				t.Parallel()
				if got := Deletion(command, shell); got != "" {
					t.Fatalf("Deletion(%q, %q) = %q, want none", command, shell, got)
				}
			})
		}
	}
	for _, command := range []string{
		"wsl --list -v",
		"wsl --export Ubuntu backup.tar",
		"wsl -d Ubuntu -- echo --unregister",
		"echo wsl --unregister Ubuntu",
		"@echo wsl --unregister Ubuntu",
		`echo "a && wsl --unregister Ubuntu"`,
		`cmd /c echo "a && wsl --unregister Ubuntu"`,
		`cmd /c "@echo wsl --unregister Ubuntu"`,
		"echo a ^& wsl --unregister Ubuntu",
		"rem wsl --unregister Ubuntu",
		"@rem wsl --unregister Ubuntu",
		"del /s /q build",
		"rm -rf build",
	} {
		t.Run("cmd/"+command, func(t *testing.T) {
			t.Parallel()
			if got := Deletion(command, CMD); got != "" {
				t.Fatalf("Deletion(%q, CMD) = %q, want none", command, got)
			}
		})
	}
}

func TestWSLEmbeddedDeletion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, command, want string
	}{
		{"backtick", "echo `wsl --unregister Ubuntu`", "wsl --unregister Ubuntu"},
		{"dollar", "echo '$(wsl.exe --unregister=Ubuntu-24.04)'", "wsl --unregister Ubuntu-24.04"},
		{"heredoc", "cat <<'EOF'\n`wsl --unregister Debian`\nEOF", "wsl --unregister Debian"},
		{"safe-backtick", "echo `wsl --list -v`", ""},
		{"safe-export", "echo '$(wsl --export Ubuntu backup.tar)'", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := EmbeddedDeletion(tc.command); got != tc.want {
				t.Fatalf("EmbeddedDeletion(%q) = %q, want %q", tc.command, got, tc.want)
			}
		})
	}
}

func TestWSLUnregisterNeverRewrites(t *testing.T) {
	t.Parallel()
	for _, shell := range []Shell{Bash, PowerShell, CMD} {
		command := "wsl --unregister Ubuntu"
		got, why := Suggest(command, Context{Shell: shell})
		if got != command || why == "" {
			t.Errorf("%s: Suggest(%q) = (%q, %q), want unchanged with reason", shell, command, got, why)
		}
	}
}

// Nested powershell -c wrappers must scale linearly with depth, never
// branching exponentially across large hook inputs (up to 4 MiB).
func TestWSLWrapperDeepNestingLinear(t *testing.T) {
	payload := strings.Repeat("a ", 4096)
	nested := strings.Repeat("powershell -c ", maxWrapperDepth+2) + "Write-Output " + payload
	for _, shell := range []Shell{Bash, PowerShell} {
		if got := Deletion(nested, shell); got != "" {
			t.Fatalf("%s: Deletion(nested safe) = %q, want none", shell, got)
		}
	}
	// Compare allocations between depth 2 and full maxWrapperDepth+2 nesting:
	// linear scaling stays well under 8x, whereas 2^(depth-1) branching is > 30x.
	shallow := strings.Repeat("powershell -c ", 2) + "Write-Output " + payload
	shallowAllocs := testing.AllocsPerRun(2, func() {
		_ = Deletion(shallow, Bash)
	})
	deepAllocs := testing.AllocsPerRun(2, func() {
		_ = Deletion(nested, Bash)
	})
	if deepAllocs > shallowAllocs*8 {
		t.Fatalf("nested powershell -c scaled super-linearly: shallow=%.0f allocs, deep=%.0f allocs (ratio %.1fx)", shallowAllocs, deepAllocs, deepAllocs/shallowAllocs)
	}
}
