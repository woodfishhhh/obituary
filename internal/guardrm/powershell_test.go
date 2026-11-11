package guardrm

import (
	"fmt"
	"strings"
	"testing"
)

func TestSuggestPowerShellTildeAndBacktick(t *testing.T) {
	t.Parallel()
	for _, operand := range []string{`~`, `~/keep`, `~name`, `'~/keep'`, `"~\keep"`, "'keep`name'", "'keep`?'", "keep`name"} {
		for _, parameter := range []string{"", "-Path ", "-LiteralPath "} {
			command := "Remove-Item " + parameter + operand
			if got, why := Suggest(command, Context{Shell: PowerShell}); got != command || why == "" {
				t.Errorf("provider expansion changed: %q (%s)", got, why)
			}
		}
	}
	if got, why := Suggest("rm -- ~/keep", Context{Shell: Bash}); got != "obit rm -- ~/keep" || why != "" {
		t.Fatalf("Bash tilde expansion lost: %q (%s)", got, why)
	}
}

func TestSuggestPowerShellSingleStatement(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		`Set-Alias rm Write-Output -Force; rm keep.txt`, `rg --% ; rm keep.txt`,
		`Write-Output ready; Remove-Item keep`, `Remove-Item keep; Write-Output done`,
		`Write-Output ready && Remove-Item keep`, `Write-Output ready || Remove-Item keep`,
		`Write-Output ready | Remove-Item keep`, `Remove-Item keep &`,
		"Write-Output ready\nRemove-Item keep", "Remove-Item keep\r\nWrite-Output done", `Remove-Item --% keep`,
		`Remove-Item 'a;b'`, `Remove-Item 'a|b'`,
	} {
		if got, why := Suggest(command, Context{Shell: PowerShell}); got != command || why == "" {
			t.Errorf("PowerShell statement structure changed: %q (%s)", got, why)
		}
	}
	if got, why := Suggest(`Remove-Item -Recurse -Force .\build`, Context{Shell: PowerShell}); got != `obit remove-item -Recurse -Force .\build` || why != "" {
		t.Fatalf("single PowerShell deletion lost: %q (%s)", got, why)
	}
}

func TestSuggestUnicodeDashPassesThrough(t *testing.T) {
	t.Parallel()
	for _, dash := range []string{"–", "—", "―"} {
		for _, shell := range []Shell{Bash, PowerShell} {
			command := "rm " + dash + "WhatIf"
			if shell == PowerShell {
				command = "Remove-Item " + dash + "WhatIf"
			}
			if got, why := Suggest(command, Context{Shell: shell}); got != command || why == "" {
				t.Errorf("Unicode parameter prefix suggested: %q (%s)", got, why)
			}
		}
		command := `Remove-Item .\` + dash + "WhatIf"
		if got, why := Suggest(command, Context{Shell: PowerShell}); got != `obit remove-item .\`+dash+"WhatIf" || why != "" {
			t.Errorf("explicit Unicode filename refused: %q (%s)", got, why)
		}
	}
}

func TestPowerShellDoubledQuotes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, source, decoded string }{
		{"double", `"keep"".txt"`, `keep".txt`},
		{"single", `'keep''.txt'`, `keep'.txt`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := "Remove-Item -LiteralPath " + tc.source
			words, why := lex(command, PowerShell)
			if why != "" || len(words) != 1 || len(words[0]) != 3 || words[0][2].value != tc.decoded {
				t.Errorf("doubled quote decoded incorrectly: %+v (%s), want %q", words, why, tc.decoded)
			}
			if got, why := Suggest(command, Context{Shell: PowerShell}); got != command || !strings.Contains(why, "doubled PowerShell quotes") {
				t.Errorf("doubled quotes must pass through before native argv validation: %q (%s)", got, why)
			}
		})
	}
	// Adjacent Bash quoted fragments still concatenate without a literal quote.
	words, why := lex(`rm "keep"".txt"`, Bash)
	if why != "" || len(words) != 1 || len(words[0]) != 2 || words[0][1].value != "keep.txt" {
		t.Fatalf("Bash quote concatenation changed: %+v (%s)", words, why)
	}
}

func TestPowerShellTrailingWhitespace(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{"\n", "\r\n", ";\n", ";\r\n", " \t\r\n\n", " ; \t\r\n"} {
		t.Run(fmt.Sprintf("%q", suffix), func(t *testing.T) {
			command := `Remove-Item -Recurse -Force .\build` + suffix
			want := strings.Replace(command, "Remove-Item", "obit remove-item", 1)
			if got, why := Suggest(command, Context{Shell: PowerShell}); got != want || why != "" {
				t.Fatalf("terminal whitespace must be preserved: %q (%s), want %q", got, why, want)
			}
		})
	}
	for _, command := range []string{"Remove-Item\nbuild", "Remove-Item build\nRemove-Item other", "Remove-Item build;\nWrite-Output done", "Remove-Item build;;\n", "Remove-Item 'build\n'"} {
		if got, why := Suggest(command, Context{Shell: PowerShell}); got != command || why == "" {
			t.Errorf("interior newline/extra statement changed: %q (%s)", got, why)
		}
	}
}

func TestPowerShellDriveSlashes(t *testing.T) {
	t.Parallel()
	for _, path := range []string{`D:/repo/build`, `D:\repo\build`} {
		command := "Remove-Item -Recurse -Force '" + path + "'"
		if got, why := Suggest(command, Context{Shell: PowerShell}); got != strings.Replace(command, "Remove-Item", "obit remove-item", 1) || why != "" {
			t.Errorf("drive-qualified path lost: %q (%s)", got, why)
		}
	}
	// Drive separators must not broaden provider/stream support. Check the
	// native argv boundary directly so the hook's later scope check cannot mask it.
	for _, path := range []string{`HKCU:\Software\Demo`, `Env:Demo`, `FileSystem::C:\Demo`, `D:/repo/build:stream`} {
		if _, err := ParseRemoveItem([]string{"-LiteralPath", path}); err == nil || !strings.Contains(err.Error(), "Only filesystem paths") {
			t.Errorf("provider/stream passed argv validation: %s: %v", path, err)
		}
	}
}

func TestRemoveItemRejectsWildcards(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"*.log", "prod?", "a[1].log", "keep].txt", "a[.txt", "a[😀].txt", "[S-T]?cret.txt", "ſ?cret.txt", "I*.txt", "i*.txt"} {
		for _, parameter := range []string{"", "-Path"} {
			args := []string{path}
			if parameter != "" {
				args = append([]string{parameter}, args...)
			}
			if _, err := ParseRemoveItem(args); err == nil || !strings.Contains(err.Error(), "wildcards are unsupported") {
				t.Errorf("wildcard must fail before filesystem lookup: %v: %v", args, err)
			}
		}
	}
}

func TestPowerShellStatementComments(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		"\nRemove-Item build", " \t\r\n\r\nRemove-Item build",
		"# cleanup\nRemove-Item build", "# cleanup\r\nRemove-Item build",
		"<# cleanup\n #>\nRemove-Item build", "<# comment <# does not nest #> Remove-Item build",
		"Remove-Item <# path\n #> build # done\n", "Remove-Item build;\n<# done #>\n",
		"# ` --% ; & | ‘quotes’\nRemove-Item build",
		"<# ` --% ; & | ‘quotes’ #> Remove-Item build", "Remove-Item '#literal.txt'",
		"Remove-Item 'build'# done", `Remove-Item "build"<# done #>`, "Remove-Item build#literal.txt",
	} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			want := strings.Replace(command, "Remove-Item", "obit remove-item", 1)
			if got, why := Suggest(command, Context{Shell: PowerShell}); got != want || why != "" {
				t.Fatalf("single statement/comment spelling lost: %q (%s), want %q", got, why, want)
			}
		})
	}
	for _, command := range []string{
		"# Remove-Item build", "<# Remove-Item build #>", "<# unterminated\nRemove-Item build",
		"<# outer <# inner #> #> Remove-Item build", "Remove-Item build <# unterminated",
		"Write-Output ready # comment\nRemove-Item build",
		"Remove-Item build; <# comment #> Write-Output done",
		"Remove-Item build\n# comment\nRemove-Item other",
		"Remove-Item build <# comment #> | Write-Output", "Remove-Item build && <# comment #> Remove-Item other",
		"Remove-Item\rbuild", "Remove-Item 'build\n'", "Remove-Item 'build;keep'",
	} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			if got, why := Suggest(command, Context{Shell: PowerShell}); got != command || why == "" {
				t.Fatalf("unsupported/multiple statements changed: %q (%s)", got, why)
			}
		})
	}
}
