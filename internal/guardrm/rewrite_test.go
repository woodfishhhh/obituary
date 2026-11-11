package guardrm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/syntax"
)

func TestSuggestDecisionBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, command, want, why string
		context                  Context
	}{
		{"empty", "", "", "no supported deletion command", Context{Shell: Bash}},
		{"git-alone", "git", "git", "no supported deletion command", Context{Shell: Bash}},
		{"git-other", "git status", "git status", "no supported deletion command", Context{Shell: Bash}},
		{"git-quoted-subcommand", `git "clean" -fd`, `git "clean" -fd`, "git prefix options or dynamic subcommand", Context{Shell: Bash}},
		{"git-prefix", "git -C repo clean -fd", "git -C repo clean -fd", "git prefix options or dynamic subcommand", Context{Shell: Bash}},
		{"git-preview", "git clean -ndx", "git clean -ndx", "no supported deletion command", Context{Shell: Bash}},
		{"rm-help", "rm --help", "rm --help", "no supported deletion command", Context{Shell: Bash}},
		{"rmdir-help", "rmdir --help", "rmdir --help", "no supported deletion command", Context{Shell: Bash}},
		{"command-quoted", `"rm" keep`, `"rm" keep`, "dynamic or quoted command word", Context{Shell: Bash}},
		{"wrapper", "env rm keep", "env rm keep", "unsupported command structure: env", Context{Shell: Bash}},
		{"other-command", "echo keep", "echo keep", "no supported deletion command", Context{Shell: Bash}},
		{"powershell-other", "Write-Output keep", "Write-Output keep", "no supported deletion command", Context{Shell: PowerShell}},
		{"powershell-wsl", "rm keep", "rm keep", "only bash deletion is supported through WSL interop", Context{Shell: PowerShell, WSL: true}},
		{"wsl-git", "git status", "git status", "git clean is unsupported through WSL interop", Context{Shell: Bash, WSL: true}},
		{"wsl-quoted", `rm 'keep'`, `rm 'keep'`, "WSL requires unquoted literal arguments", Context{Shell: Bash, WSL: true}},
		{"wsl-comma", "rm keep,other", "rm keep,other", "WSL requires unquoted literal arguments", Context{Shell: Bash, WSL: true}},
		{"cmd-s", "rm /S keep", "rm /S keep", "POSIX absolute path is ambiguous on Windows; use a drive-qualified or relative path: /S", Context{Shell: Bash}},
		{"cmd-q", "rmdir /q keep", "rmdir /q keep", "POSIX absolute path is ambiguous on Windows; use a drive-qualified or relative path: /q", Context{Shell: Bash}},
		{"cmd-git", "git clean -fd -- /s", "git clean -fd -- /s", "POSIX absolute path is ambiguous on Windows; use a drive-qualified or relative path: /s", Context{Shell: Bash}},
		{"rm", "rm -rf keep", "obit rm -rf keep", "", Context{Shell: Bash}},
		{"rmdir", "rmdir keep", "obit rmdir keep", "", Context{Shell: Bash}},
		{"git-clean", "git clean -fd -- keep", "obit git-clean -fd -- keep", "", Context{Shell: Bash}},
		{"powershell", "rm -r keep", "obit remove-item -r keep", "", Context{Shell: PowerShell}},
		{"wsl", "rm keep", "obit rm keep", "", Context{Shell: Bash, WSL: true}},
		{"explicit-executable", "rm  -- 'keep 文字'", "'/c/Program Files/obit.exe' rm  -- 'keep 文字'", "", Context{Shell: Bash, Executable: "'/c/Program Files/obit.exe'"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, why := Suggest(tc.command, tc.context)
			if got != tc.want || why != tc.why {
				t.Fatalf("decision = (%q, %q), want (%q, %q)", got, why, tc.want, tc.why)
			}
		})
	}
}

func TestSuggestSummaryAndGlobDecisions(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	for _, tc := range []struct {
		command, original, replacement string
		shell                          Shell
	}{
		{"rm keep", "rm", "rm", Bash},
		{"rmdir keep", "rmdir", "rmdir", Bash},
		{"git clean -fd", "git clean", "git-clean", Bash},
		{"ri keep", "Remove-Item", "remove-item", PowerShell},
	} {
		got, summary, why := SuggestWithSummary(tc.command, Context{Shell: tc.shell, Executable: "custom-obit"})
		want := "the same command with `" + tc.original + "` replaced by `obit " + tc.replacement + "` via the installed executable (all other arguments unchanged)"
		if why != "" || !strings.HasPrefix(got, "custom-obit "+tc.replacement) || summary != want {
			t.Fatalf("summary decision = (%q, %q, %q), want %q", got, summary, why, want)
		}
	}
	command := "rm *.log"
	context := Context{Shell: Bash, Cwd: cwd}
	wantSummary := "the same command with `rm` replaced by `obit rm` via the installed executable (all other arguments unchanged)"
	got, summary, why := SuggestWithSummary(command, context)
	if got != "obit "+command || why != "" || summary != wantSummary {
		t.Fatalf("safe glob decision = (%q, %q, %q)", got, summary, why)
	}
	if err := os.WriteFile(filepath.Join(cwd, "-i"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got, summary, why = SuggestWithSummary(command, context)
	if got != command || why != "leading glob before -- may expand into options" || summary != wantSummary {
		t.Fatalf("option glob decision = (%q, %q, %q)", got, summary, why)
	}
	got, summary, why = SuggestWithSummary(command, Context{Shell: Bash})
	if got != command || why != "leading glob location is ambiguous" || summary != wantSummary {
		t.Fatalf("unknown glob location decision = (%q, %q, %q)", got, summary, why)
	}
	got, summary, why = SuggestWithSummary("rm --help", context)
	if got != "rm --help" || why != "no supported deletion command" || summary != "" {
		t.Fatalf("help decision = (%q, %q, %q)", got, summary, why)
	}
}

func TestSuggestExitStatusIsolation(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		`rm -r repo || /usr/bin/rm -rf backup`, `false || rm -rf backup`,
		`rm -r repo && echo done`, `rm -r repo; echo done`, `rm a; rm b`,
		`printf x | rm x`, `rm x | cat`, `rm x &`, `echo ready & rm x`,
		`cd x || rm backup`, `cd x | rm backup`,
		"echo ready\nrm x", "rm x\necho done",
		`/usr/bin/rm x`, `/bin/rm x`, `./rm x`, `tools/rm x`,
		`/usr/bin/rm x && rm y`, `echo ready; /bin/rm x`,
		`echo ready; rm x`, `echo 'a || b' && rm x`,
	} {
		t.Run(command, func(t *testing.T) {
			if got, why := Suggest(command, Context{Shell: Bash}); got != command || why == "" {
				t.Fatalf("unsafe control flow changed: %q (%s)", got, why)
			}
		})
	}
	for _, command := range []string{`rm -rf build`, `cd x && rm -rf dist`, `cd; rm x`, `cd 'a || b' && rm x`} {
		got, why := Suggest(command, Context{Shell: Bash})
		if got == command || why != "" {
			t.Errorf("terminal deletion lost: %q (%s)", got, why)
		}
	}
}

func TestSuggestUnknownShell(t *testing.T) {
	t.Parallel()
	for _, shell := range []Shell{Uncertain, ""} {
		for _, command := range []string{`rm keep`, `rm -rf build`, `rmdir empty`, `Remove-Item -Recurse -Force .\build`} {
			if got, why := Suggest(command, Context{Shell: shell}); got != command || why != "unknown shell" {
				t.Errorf("unknown shell: %q -> %q (%s)", command, got, why)
			}
		}
	}
}

func TestSuggestDeclaredShellConservativeSyntax(t *testing.T) {
	t.Parallel()
	for _, shell := range []Shell{Bash, PowerShell} {
		for _, command := range []string{
			`rm -- “a b”`, `rm ‘a b’`, `rm ”a b“`, `rm ’a b‘`,
			`rmdir “a b”`, `git clean -fd -- ‘a b’`,
			`rm 'a;b'`, `rm 'a|b'`, `rm 'a&b'`, `rm --% keep`,
			"rm 'a\nb'", "rm 'a\rb'", "rm keep\r\n", `cd build && rm keep`,
		} {
			t.Run(string(shell)+"/"+command, func(t *testing.T) {
				if got, why := Suggest(command, Context{Shell: shell, DeclaredShell: true}); got != command || why == "" {
					t.Fatalf("ambiguous declared shell operands suggested: %q (%s)", got, why)
				}
			})
		}
		command := `rm '临时 文件'`
		want := `obit rm '临时 文件'`
		if shell == PowerShell {
			want = `obit remove-item '临时 文件'`
		}
		if got, why := Suggest(command, Context{Shell: shell, DeclaredShell: true}); got != want || why != "" {
			t.Errorf("unambiguous declared shell refused: %q (%s)", got, why)
		}
	}
	// Explicit Bash evidence retains Bash's literal Unicode-quote filenames.
	if got, why := Suggest(`rm -- “a b”`, Context{Shell: Bash}); got != `obit rm -- “a b”` || why != "" {
		t.Errorf("explicit Bash behavior changed: %q (%s)", got, why)
	}
}

func TestSuggestPowerShellParameterPrefixes(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		`Remove-Item build -r -fo`, `rm -r -fo build`, `ri -li 'build dir' -rec -for`,
		`Remove-Item -pa build -R -FO`, `Remove-Item -lit build`,
	} {
		t.Run(command, func(t *testing.T) {
			want := "obit remove-item" + command[strings.IndexByte(command, ' '):]
			if got, why := Suggest(command, Context{Shell: PowerShell}); got != want || why != "" {
				t.Fatalf("unambiguous parameter prefix refused: %q (%s), want %q", got, why, want)
			}
		})
	}
	for _, command := range []string{
		`Remove-Item build -f`, `Remove-Item -p build`, `Remove-Item build -fi *.log`,
		`Remove-Item build -what`, `Remove-Item build -conf`, `Remove-Item '-r'`,
		`Remove-Item -li build other`, `Remove-Item -pa build -lit other`,
		`Remove-Item -r -recurse build`, `Remove-Item -fo:$false build`,
		`Remove-Item -li`, `Remove-Item -li $target`, `Remove-Item -pa *.log`,
	} {
		if got, why := Suggest(command, Context{Shell: PowerShell}); got != command || why == "" {
			t.Errorf("unsupported parameter prefix suggested: %q (%s)", got, why)
		}
	}
}

func TestSuggestPowerShellGitClean(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		`git clean -fdx`, `git clean -ffdx -- '临时 文件'`,
		`git clean -fd -e '*.log' -- build`, `git clean --force -d --exclude=keep -- .\build`,
		`git clean -fd -- *.log`, `git clean -fd -- 'a[1].log'`,
		"\n# preview belongs to the CLI\ngit clean -fd -- build;\n",
	} {
		t.Run(command, func(t *testing.T) {
			want := strings.Replace(command, "git clean", "obit git-clean", 1)
			if got, why := Suggest(command, Context{Shell: PowerShell}); got != want || why != "" {
				t.Fatalf("static git clean refused: %q (%s), want %q", got, why, want)
			}
		})
	}
	for _, command := range []string{
		`git clean -ndx`, `git clean --dry-run -fdx`, `git clean -ifd`, `git -C repo clean -fd`,
		`git clean $flags`, `git clean -fd -- "$target"`, `git clean -fd -- $(Get-Location)`,
		`git clean --% -fd`, `git clean -fd -- 'a''b'`, `git clean -fd -- 'a"b'`,
		`git clean -fd -- a,b`, `git clean -fd -- ~/build`, `git clean -fd -- ''`,
		`git clean -fd -- “a b”`, `git clean -fd -- 'build.'`,
		`git clean -fd | Write-Output`, `git clean -fd; Remove-Item other`,
	} {
		if got, why := Suggest(command, Context{Shell: PowerShell}); got != command || why == "" {
			t.Errorf("unsupported git clean suggested: %q (%s)", got, why)
		}
	}
}

func TestSuggestOnlyLiteralCDPrefixes(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		`hash -p /usr/bin/printf rm; rm keep.txt`,
		`trap '/usr/bin/rm -rf backup' ERR; rm -r repo`,
		`PREVIEW=-ndx; git clean $PREVIEW`, `FOO=1; rm keep`, `FOO=1 rm keep`,
		`set -e; rm keep`, `alias rm=printf; rm keep`, `shopt -s expand_aliases; rm keep`,
		`export POSIXLY_CORRECT=1; rm keep`, `source setup; rm keep`,
		`f() { printf x; }; rm keep`, `npm run build && rm -rf dist`,
		`echo ready; rm keep`, `git status && rm keep`, `true && cd x; rm keep`,
		`cd x y; rm keep`, `cd -P x; rm keep`, `cd -- x; rm keep`, `cd -; rm keep`,
		`cd $DIR; rm keep`, `cd "$DIR"; rm keep`, `cd '$DIR'; rm keep`,
		`cd x*; rm keep`, `cd 'x*'; rm keep`, `cd x?; rm keep`, `cd x[1]; rm keep`,
		`cd ~; rm keep`, `cd $(pwd); rm keep`, "cd `pwd`; rm keep",
		`cd x > out; rm keep`, `cd x | rm keep`, `cd x || rm keep`, `cd x & rm keep`,
	} {
		t.Run(command, func(t *testing.T) {
			if got, why := Suggest(command, Context{Shell: Bash}); got != command || why == "" {
				t.Fatalf("unsafe prefix suggested: %q (%s)", got, why)
			}
		})
	}
	for _, prefix := range []string{"", "cd && ", "cd; ", "cd x && ", "cd x; ", `cd 'a b' && `, `cd "a;b"; `, "cd x; cd ../y && "} {
		command := prefix + "rm -rf dist"
		if got, why := Suggest(command, Context{Shell: Bash}); got != prefix+"obit rm -rf dist" || why != "" {
			t.Errorf("literal cd lost: %q (%s)", got, why)
		}
		if prefix != "" {
			if got, why := Suggest(command, Context{Shell: Bash, WSL: true}); got != command || why == "" {
				t.Errorf("WSL cwd change suggested: %q (%s)", got, why)
			}
		}
	}
}

func TestSuggestDynamicOptionsBeforeDoubleDash(t *testing.T) {
	t.Parallel()
	// Single commands isolate option expansion from the prefix-statement gate.
	for _, command := range []string{
		`git clean $PREVIEW`, `git clean "$PREVIEW"`, `rm "$HELP"`, `rm $HELP`,
		`rmdir $HELP`, `rm -f "$HELP" keep`, `rm -$FLAGS keep`, `rm $FLAGS -- keep`,
		`rm ${HELP}`, `rm $(printf -- --help)`, "rm `printf -- --help`", `rm ~/keep`,
	} {
		t.Run(command, func(t *testing.T) {
			if got, why := Suggest(command, Context{Shell: Bash}); got != command || why == "" {
				t.Fatalf("dynamic option suggested: %q (%s)", got, why)
			}
		})
	}
	for _, command := range []string{
		`rm -- $HELP`, `rm -rf -- "$DIR" build/*`, `rmdir -- $DIR`,
		`rm 'a b'`, `rm "a b"`, `rm a\ b`, `rm '$HELP'`, `rm \$HELP`,
		`rm -rf build/*.log`, `rm -- ~/keep`,
	} {
		if got, why := Suggest(command, Context{Shell: Bash}); got != "obit "+command || why != "" {
			t.Errorf("literal or operand expansion lost: %q (%s)", got, why)
		}
	}
	if got, why := Suggest(`git clean -fd -- "$PATHSPEC" *.log`, Context{Shell: Bash}); got != `obit git-clean -fd -- "$PATHSPEC" *.log` || why != "" {
		t.Errorf("git operand expansion lost: %q (%s)", got, why)
	}
}

func TestSuggestBashCRPassesThrough(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"git\rclean -fdx", "rm\rkeep", "\rrm keep", "rm keep\r", "rm -- 'keep\rname'", "rm keep # comment\r"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			if got, why := Suggest(command, Context{Shell: Bash}); got != command || why == "" {
				t.Fatalf("CR changed shell meaning: %q (%s)", got, why)
			}
		})
	}
}

func TestSuggestTrailingSemicolon(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"rm -rf build;", "rm -rf build; \t", "rm -rf build;\n\t", "cd x && rm -rf dist;", "rm -rf build; # done"} {
		t.Run(command, func(t *testing.T) {
			want := strings.Replace(command, "rm ", "obit rm ", 1)
			if got, why := Suggest(command, Context{Shell: Bash}); got != want || why != "" {
				t.Fatalf("terminal semicolon: %q (%s), want %q", got, why, want)
			}
		})
	}
	for _, command := range []string{"rm -rf build &&", "rm -rf build && \t", "rm -rf build &&\n", "cd x && rm -rf dist &&", "rm build;;", "rm build; &&", "rm build;\nrm dist"} {
		if got, why := Suggest(command, Context{Shell: Bash}); got != command || why == "" {
			t.Errorf("incomplete/invalid command suggested: %q (%s)", got, why)
		}
	}
}

func TestSuggestReview6Terminators(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"rm -rf build\n", "rm -rf build\n\n\t", "cd x &&\nrm -rf dist", "cd x &&\n\nrm -rf dist\n", "cd x\nrm -rf dist", "rm -rf build;\n# done\n"} {
		t.Run(command, func(t *testing.T) {
			want := strings.Replace(command, "rm ", "obit rm ", 1)
			if got, why := Suggest(command, Context{Shell: Bash}); got != want || why != "" {
				t.Fatalf("terminal newline/continuation: %q (%s), want %q", got, why, want)
			}
		})
	}
	for _, command := range []string{`Remove-Item -Recurse -Force .\build;`, "Remove-Item build; \t"} {
		if got, why := Suggest(command, Context{Shell: PowerShell}); got != strings.Replace(command, "Remove-Item", "obit remove-item", 1) || why != "" {
			t.Errorf("PowerShell terminal semicolon: %q (%s)", got, why)
		}
	}
	for _, command := range []string{
		"rm build\necho done", "rm build\nrm other", "rm build &&\necho done", "rm build &&\n", "rm build\n;", "cat <<'EOF'\nrm build\nEOF\n", "cd x &&\ncat <<'EOF'\nrm build\nEOF",
		"rm -- #comment\\\nkeep", "git clean -fdx\n\\\n\\\n",
	} {
		if got, why := Suggest(command, Context{Shell: Bash}); got != command || why == "" {
			t.Errorf("later statement/incomplete/heredoc: %q (%s)", got, why)
		}
	}
	for _, command := range []string{`Remove-Item build; Write-Output done`, `Remove-Item build; Remove-Item other`, `Remove-Item build;;`} {
		if got, why := Suggest(command, Context{Shell: PowerShell}); got != command || why == "" {
			t.Errorf("PowerShell later/invalid statement: %q (%s)", got, why)
		}
	}
}

func TestSuggestCommands(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"rm -rf build/*.log", "obit rm -rf build/*.log"},
		{"  rm  -- '中文 空格'", "  obit rm  -- '中文 空格'"},
		{`rm -- "a b" a\ b "$DIR"`, `obit rm -- "a b" a\ b "$DIR"`},
		{"cd x && rm -rf dist", "cd x && obit rm -rf dist"},
		{"git clean -fdx -- '临时 文件'", "obit git-clean -fdx -- '临时 文件'"},
		{"rm -- -i", "obit rm -- -i"},
		{"rm -f x # rm y", "obit rm -f x # rm y"},
		{"rm \\\n-f x", "obit rm \\\n-f x"},
		{"cd 'a && b' && rm x", "cd 'a && b' && obit rm x"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, why := Suggest(tc.in, Context{Shell: Bash})
			if got != tc.want || why != "" {
				t.Fatalf("rewrite = %q (%s), want %q", got, why, tc.want)
			}
		})
	}
}

func TestSuggestLeavesStringsAlone(t *testing.T) {
	t.Parallel()
	for _, in := range []string{
		`echo "rm -rf x"`, `printf '%s' 'rm x'`, `echo rm -rf x`,
		`echo "&& rm x"`, `echo \; rm x`, `echo \| rm x`,
		`echo "a\"; rm x"`, `# rm -rf x`, `printf '%s' "rmdir x"`,
		`git-clean -fd`, `myrm -f x`, `obit rm -f x`,
		"rm -- a\x00b",
	} {
		if got, _ := Suggest(in, Context{Shell: Bash}); got != in {
			t.Errorf("changed argument/comment: %q -> %q", in, got)
		}
	}
}

func TestSuggestComplexCommandsPassThrough(t *testing.T) {
	t.Parallel()
	for _, in := range []string{
		"rm a; cat <<EOF\nrm x\nEOF", `rm a; echo $(rm b)`, "rm a; echo `rm b`",
		`rm a; cat <(rm b)`, `rm a; echo >(rm b)`, `rm a; (rm b)`,
		`rm a; eval 'rm b'`, `rm a; exec rm b`, `rm a; sudo rm b`,
		`rm a; xargs rm`, `rm a; find . -delete`, `rm a; find . -exec rm {} \;`,
		`rm a; f() { rm b; }`, `rm a; alias rm='echo'`, `rm a; source setup`,
		`rm a; . setup`, `rm a; if true; then rm b; fi`, `rm a & rm b`,
		`rm a > out`, `rm a 2>/dev/null`, `rm a; $COMMAND b`,
		`rm a; command rm b`, `rm a; env rm b`, `rm a; bash -c 'rm b'`,
		`rm a; git -C dir clean -fd`, `rm a; rm -ri b`, `rm a; rm -I b`,
		`rm a; rm --interactive=always b`, `rm a; git clean -ifd`,
		`rm a; rmdir -p b`, `rm "unfinished`, `rm a; echo \`,
		`rm a; "rm" b`, `rm a; r\m b`, `rm a; echo ${x:-$(rm b)}`,
		`"" rm x`, `"FOO=1" rm x`, `F\OO=1 rm x`,
		`rm a; /usr/bin/sudo rm b`, `rm a; /bin/bash -c 'rm b'`,
		`printf '%s' "${var:-"a; rm x"}"`,
		`rm a; printf '%s' $'quoted\' text'`,
		"rm -- 'literal`name'",
	} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			got, why := Suggest(in, Context{Shell: Bash})
			if got != in || why == "" {
				t.Fatalf("complex command must pass through with reason: %q -> %q (%s)", in, got, why)
			}
		})
	}
}

func TestSuggestArgumentsStayByteForByte(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{`'a;b'`, `"a|b"`, `'rm -rf x'`, `"a && rm x"`, `a\ b`, `目录/*.log`, `"$DIR"`, `''`, `'#literal'`} {
		in := "rm -- " + arg
		if got, why := Suggest(in, Context{Shell: Bash}); got != "obit "+in || why != "" {
			t.Errorf("argument altered: %q (%s)", got, why)
		}
	}
}

// Exercise every corpus entry under each configuration to cover shell-specific
// parsing. An empty Cwd keeps this independent of filesystem state.
func FuzzSuggest(f *testing.F) {
	f.Add(`git <# cleanup #> clean -fd -- 'build dir'`)
	f.Add(`git clean -fd -- 'a''b'`)
	f.Add(`Remove-Item build -r -fo`)
	f.Fuzz(func(t *testing.T, command string) {
		for _, context := range []Context{
			{Shell: Bash},
			{Shell: Bash, Executable: `'/c/Program Files/obit.exe'`},
			{Shell: Bash, WSL: true},
			{Shell: Bash, WSL: true, Executable: `'/mnt/c/Program Files/obit.exe'`},
			{Shell: PowerShell},
			{Shell: PowerShell, Executable: `& 'C:\Program Files\obit.exe'`},
			{Shell: PowerShell, WSL: true},
		} {
			got, why := Suggest(command, context)
			again, whyAgain := Suggest(command, context)
			if got != again || why != whyAgain {
				t.Fatalf("Suggest is nondeterministic (%+v): %q -> (%q, %q), then (%q, %q)", context, command, got, why, again, whyAgain)
			}
			equivalent, summary, summaryWhy := SuggestWithSummary(command, context)
			equivalentAgain, summaryAgain, summaryWhyAgain := SuggestWithSummary(command, context)
			if equivalent != equivalentAgain || summary != summaryAgain || summaryWhy != summaryWhyAgain {
				t.Fatalf("SuggestWithSummary is nondeterministic (%+v): %q -> (%q, %q, %q), then (%q, %q, %q)", context, command, equivalent, summary, summaryWhy, equivalentAgain, summaryAgain, summaryWhyAgain)
			}
			if equivalent != got || summaryWhy != why {
				t.Fatalf("suggestion APIs disagree (%+v): %q -> (%q, %q), summary API (%q, %q)", context, command, got, why, equivalent, summaryWhy)
			}
			if (got == command) != (why != "") {
				t.Fatalf("suggestion/reason mismatch (%+v): %q -> %q (%q)", context, command, got, why)
			}
			if got == command {
				continue
			}
			if summary == "" {
				t.Fatalf("suggestion lacks summary (%+v): %q -> %q", context, command, got)
			}
			assertSuggestCommandWords(t, command, got, context)
		}
	})
}

func assertSuggestCommandWords(t *testing.T, command, got string, context Context) {
	t.Helper()
	executable := context.Executable
	if executable == "" {
		executable = "obit"
	}
	// Compare complete round trips at source spans, so obit/rm in operands,
	// comments or cd paths cannot be mistaken for a command-word replacement.
	restored := func(start, end int, name string) bool {
		inserted := executable + " " + name
		if !strings.HasPrefix(got, command[:start]+inserted) {
			return false
		}
		return got[:start]+command[start:end]+got[start+len(inserted):] == command
	}
	matched := false
	if context.Shell == Bash {
		if strings.ContainsAny(command, "\x00\r`") {
			t.Fatalf("Bash suggestion contains forbidden bytes: %q -> %q", command, got)
		}
		// Validate syntax and inspect the AST directly to check suggestions
		// independently of guardrm's lexer and replacement plan.
		file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
		if err != nil {
			t.Fatalf("invalid Bash received a suggestion: %q -> %q: %v", command, got, err)
		}
		syntax.Walk(file, func(node syntax.Node) bool {
			if literal, ok := node.(*syntax.Lit); ok {
				// A backslash run with odd length before LF still needs shell
				// decoding. These residual continuations are outside our subset.
				slashes := 0
				for _, c := range literal.Value {
					if c == '\n' && slashes%2 == 1 {
						t.Fatalf("residual Bash continuation received a suggestion: %q -> %q", command, got)
					}
					if c == '\\' {
						slashes++
					} else {
						slashes = 0
					}
				}
			}
			call, ok := node.(*syntax.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			start, end := int(call.Args[0].Pos().Offset()), int(call.Args[0].End().Offset())
			name := command[start:end]
			switch name {
			case "rm", "rmdir":
			case "git":
				if len(call.Args) < 2 || command[call.Args[1].Pos().Offset():call.Args[1].End().Offset()] != "clean" {
					return true
				}
				end, name = int(call.Args[1].End().Offset()), "git-clean"
			default:
				return true
			}
			matched = matched || restored(start, end, name)
			return true
		})
	} else {
		// PowerShell supports one statement. Skip only leading whitespace and
		// comments, then read its literal command word without using lex.
		start := skipPowerShellTrivia(command, 0)
		end := start
		for end < len(command) && !strings.ContainsRune(" \t\r\n;&|", rune(command[end])) {
			end++
		}
		switch strings.ToLower(command[start:end]) {
		case "remove-item", "rm", "del", "ri", "rd", "rmdir", "erase":
			matched = restored(start, end, "remove-item")
		case "git":
			sub := skipPowerShellTrivia(command, end)
			end = sub
			for end < len(command) && !strings.ContainsRune(" \t\r\n;&|", rune(command[end])) {
				end++
			}
			matched = command[sub:end] == "clean" && restored(start, end, "git-clean")
		}
	}
	if !matched {
		t.Fatalf("suggestion changed bytes outside command words (%+v): %q -> %q", context, command, got)
	}
}

func skipPowerShellTrivia(command string, start int) int {
	for start < len(command) {
		if strings.ContainsRune(" \t\r\n", rune(command[start])) {
			start++
		} else if command[start] == '#' {
			end := strings.IndexAny(command[start:], "\r\n")
			if end < 0 {
				break
			}
			start += end
		} else if strings.HasPrefix(command[start:], "<#") {
			end := strings.Index(command[start+2:], "#>")
			if end < 0 {
				break
			}
			start += end + 4
		} else {
			break
		}
	}
	return start
}

func BenchmarkSuggest(b *testing.B) {
	command := strings.Repeat(`cd "rm -rf 字符串" && `, 20) + "rm -rf build/*.log"
	for b.Loop() {
		Suggest(command, Context{Shell: Bash})
	}
}

func TestShellForms(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{`Remove-Item -Recurse -Force -Path 'build/*'`, ""},
		{`ri -LiteralPath 'a[1].log'`, ""},
		{`erase -Force C:\tmp\x`, `obit remove-item -Force C:\tmp\x`},
		{`rm -Recurse -Force build`, `obit remove-item -Recurse -Force build`},
		{`del -Path *.log`, ""},
		{`rd -Recurse build`, `obit remove-item -Recurse build`},
		{`rmdir -Recurse build`, `obit remove-item -Recurse build`},
		{`del /s /q build`, ""}, {`rd /s /q build`, ""}, {`rmdir /s /q build`, ""},
		{`Remove-Item -WhatIf x`, ""}, {`Remove-Item -Confirm x`, ""},
		{`Remove-Item a,b`, ""}, {`Remove-Item $paths`, ""},
		{`Remove-Item HKCU:\Software\x`, ""}, {`rm a; Write-Output @'\nrm x\n'@`, ""},
	} {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, why := Suggest(tc.in, Context{Shell: PowerShell})
			if tc.want == "" {
				if got != tc.in || why == "" {
					t.Fatalf("must pass through: %q (%s)", got, why)
				}
			} else if got != tc.want || why != "" {
				t.Fatalf("suggestion = %q (%s), want %q", got, why, tc.want)
			}
		})
	}
}
