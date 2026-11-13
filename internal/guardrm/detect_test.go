package guardrm

import (
	"strings"
	"testing"
)

func TestDeletionFindsEveryCommandPosition(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		shell         Shell
		command, want string
	}{
		// Forms that previously passed because no exact equivalent applies.
		{Bash, "rm -rf build && npm run build", "rm"},
		{Bash, "rm -rf node_modules package-lock.json && npm install", "rm"},
		{Bash, `rm -rf "$OUT"/*`, "rm"},
		{Bash, "rm -rf ~/scratch", "rm"},
		{Bash, "rm -rf /c/Users/me/proj/build", "rm"},
		{Bash, "rm -rf /tmp/x", "rm"},
		{Bash, "rm -rf build 2>/dev/null", "rm"},
		{Bash, "rm -rf build || true", "rm"},
		{Bash, "git clean -fdx && npm ci", "git clean"},
		{PowerShell, "Remove-Item -Recurse -Force build; npm run build", "Remove-Item"},
		{PowerShell, "Remove-Item -Recurse -Force build -ErrorAction SilentlyContinue", "Remove-Item"},
		// Every other command position the shells run.
		{Bash, "npm run build && rm -rf dist", "rm"},
		{Bash, "false || rmdir empty", "rmdir"},
		{Bash, "rm x | cat", "rm"},
		{Bash, "echo ready & rm x", "rm"},
		{Bash, "(rm x)", "rm"},
		{Bash, "{ rm x; }", "rm"},
		{Bash, "echo $(rm x)", "rm"},
		{Bash, "echo `rm x`", "rm"},
		{Bash, "cat <(rm x)", "rm"},
		{Bash, "cat <<EOF\n$(rm x)\nEOF", "rm"},
		{Bash, "if true; then rm x; fi", "rm"},
		{Bash, `for f in *.log; do rm "$f"; done`, "rm"},
		{Bash, "cleanup() { rm -rf build; }", "rm"},
		{Bash, "! rm x", "rm"},
		{Bash, "time rm x", "rm"},
		{Bash, "FOO=1 rm x", "rm"},
		{Bash, "export X=$(rm y)", "rm"},
		{Bash, "case x in a) rm y;; esac", "rm"},
		{Bash, `\rm x`, "rm"},
		{Bash, `'rm' x`, "rm"},
		{Bash, `"rm" x`, "rm"},
		{Bash, `r\m x`, "rm"},
		{Bash, `r''m x`, "rm"},
		{Bash, "/usr/bin/rm -rf x", "rm"},
		{Bash, `'C:\Program Files\Git\usr\bin\rm.exe' -rf x`, "rm"},
		{Bash, `"$HOME"/bin/rm x`, "rm"},
		{Bash, "git -C repo clean -fd", "git clean"},
		{Bash, "git --git-dir .git -c core.x=y clean -f", "git clean"},
		{Bash, `git "clean" -fd`, "git clean"},
		{Bash, "git clean $PREVIEW", "git clean"},
		{Bash, "git clean -ifd", "git clean"},
		{Bash, "rm x\nfi", "rm"},
		{Bash, "rm x\necho 'unterminated", "rm"},
		{Bash, "echo 'unterminated && rm x", "rm"},
		{Bash, "echo a # comment \\\nrm x", "rm"},
		{Bash, "rm keep\r", "rm"},
		{PowerShell, "npm run build; Remove-Item build", "Remove-Item"},
		{PowerShell, "Get-ChildItem *.log | Remove-Item", "Remove-Item"},
		{PowerShell, "npm ci && rm -r -fo build", "Remove-Item"},
		{PowerShell, "if (Test-Path build) { Remove-Item build -Recurse }", "Remove-Item"},
		{PowerShell, "foreach ($f in $files) { del $f }", "Remove-Item"},
		{PowerShell, "$files | ForEach-Object { ri $_ }", "Remove-Item"},
		{PowerShell, "try { rd build } catch { }", "Remove-Item"},
		{PowerShell, "$out = erase build", "Remove-Item"},
		{PowerShell, "& 'Remove-Item' build", "Remove-Item"},
		{PowerShell, ". rmdir build", "Remove-Item"},
		{PowerShell, `Write-Output "$(Remove-Item build)"`, "Remove-Item"},
		{PowerShell, "@{ a = (Remove-Item build) }", "Remove-Item"},
		{PowerShell, `Microsoft.PowerShell.Management\Remove-Item build`, "Remove-Item"},
		{PowerShell, "Re`move-Item build", "Remove-Item"},
		{PowerShell, "Remove-Item `\n  -Recurse build", "Remove-Item"},
		{PowerShell, "<# cleanup #> Remove-Item build # done", "Remove-Item"},
		{PowerShell, "Write-Output «说明; rm x;结束»", "Remove-Item"},
		{PowerShell, "Write-Output done\u00a0;\u00a0rm x", "Remove-Item"},
		{PowerShell, `& 'C:\Program Files\Git\usr\bin\rm.exe' -rf build`, "rm"},
		{PowerShell, "git clean -fdx; npm ci", "git clean"},
		{PowerShell, "git `\r\n  clean -fdx", "git clean"},
		{PowerShell, "git -C repo clean -fd", "git clean"},
		{PowerShell, "@\"\n$(git clean -fdx)\n\"@", "git clean"},
	} {
		t.Run(string(tc.shell)+"/"+tc.command, func(t *testing.T) {
			t.Parallel()
			if got := Deletion(tc.command, tc.shell); got != tc.want {
				t.Fatalf("Deletion = %q, want %q", got, tc.want)
			}
		})
	}
}

// Strings, comments, arguments, other programs and previews never count.
func TestDeletionIgnoresTextAndArguments(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		shell   Shell
		command string
	}{
		{Bash, `echo "rm -rf build"`},
		{Bash, "echo rm -rf build"},
		{Bash, `printf '%s\n' 'git clean -fdx'`},
		{Bash, "# rm -rf build"},
		{Bash, "ls # && rm -rf build"},
		{Bash, `grep -rn "rm -rf" .`},
		{Bash, "cat <<'EOF'\nrm -rf build\nEOF"},
		{Bash, "git rm --cached x"},
		{Bash, "git commit -m clean && git log --grep clean"},
		{Bash, "obit rm -rf build && npm run build"},
		{Bash, "'/c/Program Files/obit.exe' git-clean -fdx && npm ci"},
		{Bash, "npm run rm && man rm && which rmdir"},
		{Bash, "rm --help"},
		{Bash, "git clean -ndx"},
		{Bash, "git --help clean"},
		{Bash, "$RM -rf x"},
		{Bash, "r* -rf x"},
		{Bash, "myrm -rf x; git-clean -fd; rm.sh x"},
		{Bash, "Remove-Item build; del x; rd y"},
		{Bash, "rm() { echo; }"},
		{Bash, ""},
		{PowerShell, "Write-Output 'Remove-Item build; rm x'"},
		{PowerShell, `Write-Output "rm x; del y"`},
		{PowerShell, "Write-Output “说明; rm x;结束”"},
		{PowerShell, "Write-Output ‘rm x; del y’"},
		{PowerShell, "# Remove-Item build"},
		{PowerShell, "<# rm x\n del y #>"},
		{PowerShell, "Get-ChildItem rm, del | Write-Output"},
		{PowerShell, "Get-Help Remove-Item"},
		{PowerShell, "obit remove-item build; npm run build"},
		{PowerShell, "@{ rm = 1; del = 'x' }"},
		{PowerShell, "@'\nRemove-Item build\n'@"},
		{PowerShell, "git clean -n; git status"},
		{PowerShell, "function rm { }"},
		{PowerShell, "$rm = 'x'"},
	} {
		t.Run(string(tc.shell)+"/"+tc.command, func(t *testing.T) {
			t.Parallel()
			if got := Deletion(tc.command, tc.shell); got != "" {
				t.Fatalf("Deletion = %q, want none", got)
			}
		})
	}
	for _, shell := range []Shell{Uncertain, ""} {
		if got := Deletion("rm -rf build", shell); got != "" {
			t.Errorf("unknown shell %q: Deletion = %q", shell, got)
		}
	}
}

// A hook's input can be 4 MiB; deep nesting must neither overflow the stack
// nor hide the deletion.
func TestDeletionDeepNesting(t *testing.T) {
	t.Parallel()
	for _, shell := range []Shell{Bash, PowerShell} {
		for _, open := range []string{"(", "$(", "{ ", "\"$("} {
			command := strings.Repeat(open, (4<<20)/len(open)) + "rm x"
			if got := Deletion(command, shell); got == "" {
				t.Errorf("%s %q nesting hid the deletion", shell, open)
			}
		}
	}
	if got, why := Suggest(strings.Repeat("(", 4<<20)+"rm x", Context{Shell: Bash}); why != "nesting is too deep" {
		t.Errorf("deep Bash nesting: %q (%s)", got, why)
	}
}
