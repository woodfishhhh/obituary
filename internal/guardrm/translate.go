package guardrm

import "strings"

type deletionCommand struct {
	name           string
	end, startArgs int
	ps             bool
}

type globCheck struct {
	needed, ambiguous bool
}

func parseSuggestion(command string, context Context) ([][]word, string) {
	if context.Shell != Bash && context.Shell != PowerShell {
		return nil, "unknown shell"
	}
	if context.WSL && context.Shell != Bash {
		return nil, "only bash deletion is supported through WSL interop"
	}
	if context.Shell == Bash && strings.ContainsRune(command, '\r') {
		return nil, "carriage returns are unsupported in bash"
	}
	segments, why := lex(command, context.Shell)
	if why != "" {
		return nil, why
	}
	if context.Shell == PowerShell {
		if why := powerShellSyntax(command, segments); why != "" {
			return nil, why
		}
	}
	return segments, ""
}

func deletionSyntax(command string, context Context, segments [][]word, first word) string {
	if !first.literal {
		return "dynamic or quoted command word"
	}
	base := first.value[strings.LastIndexAny(first.value, `/\`)+1:]
	// Shared rm/rmdir/git spellings can select different operands in the two
	// shells. Reuse both lexers when only an owner's declaration is known.
	// A full Remove-Item command still uses the normal PowerShell rules.
	if context.DeclaredShell && (strings.EqualFold(base, "rm") || strings.EqualFold(base, "rmdir") || base == "git") && !sameDeclaredShellWords(command, context.Shell, segments) {
		return "declared shell command contains ambiguous arguments"
	}
	if strings.EqualFold(base, "rm") && base != first.value {
		return "path-qualified rm is unsupported"
	}
	if (context.Shell != Bash || context.WSL) && changesLocation(base) {
		return "PowerShell location or drive changes are unsupported"
	}
	switch base {
	case "eval", "exec", "sudo", "xargs", "find", "alias", "unalias", "source", ".",
		"command", "builtin", "env", "time", "timeout", "sh", "bash", "zsh", "fish",
		"if", "then", "else", "elif", "fi", "for", "while", "until", "do", "done",
		"case", "esac", "select", "function", "!", "[[", "coproc", "set", "shopt", "enable",
		"pwsh", "pwsh.exe", "powershell", "powershell.exe", "cmd", "cmd.exe", "wsl", "wsl.exe":
		return "unsupported command structure: " + first.value
	}
	return ""
}

func parseDeletion(words []word, context Context) (deletionCommand, string) {
	first := words[0]
	ps := context.Shell == PowerShell && removeItemAlias(first.value)
	if context.Shell == PowerShell && !ps && first.value != "git" {
		return deletionCommand{}, ""
	}
	if !ps && first.value != "rm" && first.value != "rmdir" && first.value != "git" {
		return deletionCommand{}, ""
	}
	d := deletionCommand{name: first.value, end: first.end, startArgs: 1, ps: ps}
	if d.name != "git" {
		return d, ""
	}
	if context.WSL {
		return deletionCommand{}, "git clean is unsupported through WSL interop"
	}
	if len(words) < 2 {
		return deletionCommand{}, ""
	}
	if !words[1].literal || strings.HasPrefix(words[1].value, "-") {
		return deletionCommand{}, "git prefix options or dynamic subcommand"
	}
	if words[1].value != "clean" {
		return deletionCommand{}, ""
	}
	d.name, d.end, d.startArgs = "git-clean", words[1].end, 2
	return d, ""
}

func parseDeletionArgs(command string, context Context, words []word, d deletionCommand, segment int) ([]string, globCheck, string) {
	var args []string
	var glob globCheck
	beforeDoubleDash := true
	for _, w := range words[d.startArgs:] {
		if why := deletionArgumentSyntax(context, w, d); why != "" {
			return nil, glob, why
		}
		if context.Shell == Bash && beforeDoubleDash && w.expands {
			return nil, glob, "dynamic argument before -- may expand into options"
		}
		if context.Shell == Bash && beforeDoubleDash && w.leadingGlob {
			// A preceding cd can depend on CDPATH or the shell's logical cwd.
			// Only inspect the envelope cwd, never guess that changed location.
			glob.needed = true
			if segment != 0 || strings.ContainsRune(w.value, '/') || context.Cwd == "" {
				glob.ambiguous = true
			}
		}
		if w.value == "--" {
			beforeDoubleDash = false
		}
		if d.ps && !w.literal && strings.HasPrefix(w.value, "-") {
			return nil, glob, "quoted PowerShell parameter-like path"
		}
		if context.WSL && (!w.literal || strings.ContainsAny(command[w.start:w.end], "\\,'\"")) {
			return nil, glob, "WSL requires unquoted literal arguments"
		}
		args = append(args, w.value)
	}
	return args, glob, ""
}

func deletionArgumentSyntax(context Context, w word, d deletionCommand) string {
	if d.ps && w.doubledQuote {
		return "doubled PowerShell quotes are unsupported"
	}
	// Reuse git-clean's native argv/preview path. Stay outside PowerShell's
	// version-dependent native quoting and tilde expansion (MicrosoftDocs/
	// PowerShell-Docs, Microsoft.PowerShell.Core/About/about_Parsing.md).
	if context.Shell == PowerShell && d.name == "git-clean" && (w.doubledQuote || w.value == "" || strings.ContainsAny(w.value, ",\"") || strings.HasPrefix(w.value, "~")) {
		return "unsupported PowerShell native argument"
	}
	if unicodeDashPrefix(w.value) {
		return "Unicode dash parameter prefixes are unsupported"
	}
	return ""
}

func deletionPaths(args []string, d deletionCommand) ([]string, bool, string) {
	if d.ps {
		// Resolve full names and unambiguous prefixes once, using the same
		// parameter set as the native command, including unsupported names.
		opts, err := ParseRemoveItem(args)
		if err != nil {
			return nil, false, err.Error()
		}
		paths := opts.Paths
		if len(paths) != 1 || !literalItemPath(paths[0]) {
			return nil, false, "PowerShell requires one literal filesystem path"
		}
		return paths, false, ""
	}
	if d.name == "git-clean" {
		o, err := ParseClean(args)
		if err != nil {
			return nil, false, err.Error()
		}
		if o.DryRun {
			return nil, true, ""
		}
		return o.Paths, false, ""
	}
	if optionsAfterOperand(args) {
		return nil, false, "options after operands depend on POSIXLY_CORRECT"
	}
	opts, err := ParseRemove(args, d.name == "rmdir")
	if err != nil {
		return nil, false, err.Error()
	}
	if opts.Help {
		return nil, true, ""
	}
	return opts.Paths, false, ""
}

func deletionPathSyntax(paths []string, context Context) string {
	for _, arg := range paths {
		if ambiguousWindowsPath(arg) {
			return "path has a trailing dot or space"
		}
		if context.WSL && !safeWSLRelative(arg) {
			return "WSL requires safe relative paths"
		}
		if strings.EqualFold(arg, "/s") || strings.EqualFold(arg, "/q") {
			return "cmd deletion switches"
		}
	}
	return ""
}

func deletionPosition(segments [][]word, segment int) string {
	if segment != len(segments)-1 {
		return "deletion must be the final command segment"
	}
	for _, prefix := range segments[:segment] {
		if !literalCD(prefix) {
			return "only literal cd statements may precede deletion"
		}
	}
	return ""
}

func translateDeletion(command string, context Context, segments [][]word, segment int) (replacement, globCheck, string) {
	words := segments[segment]
	if why := deletionSyntax(command, context, segments, words[0]); why != "" {
		return replacement{}, globCheck{}, why
	}
	d, why := parseDeletion(words, context)
	if why != "" || d.name == "" {
		return replacement{}, globCheck{}, why
	}
	args, glob, why := parseDeletionArgs(command, context, words, d, segment)
	if why != "" {
		return replacement{}, glob, why
	}
	paths, skip, why := deletionPaths(args, d)
	if why != "" || skip {
		return replacement{}, glob, why
	}
	if why := deletionPathSyntax(paths, context); why != "" {
		return replacement{}, glob, why
	}
	if why := deletionPosition(segments, segment); why != "" {
		return replacement{}, glob, why
	}
	if d.ps {
		d.name = "remove-item"
	}
	return deletionReplacement(context, words[0].start, d.end, d.name), glob, ""
}
