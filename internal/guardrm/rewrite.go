// Package guardrm recognizes the small shell subset supported by deletion guards.
package guardrm

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

type replacement struct {
	start, end int
	text, name string
}

type Shell string

const (
	Bash       Shell = "bash"
	PowerShell Shell = "powershell"
	Uncertain  Shell = "uncertain" // Windows cwd without explicit shell evidence
)

// Context comes from the tool envelope or the owner's shell declaration,
// never from guessing the shell from its arguments.
type Context struct {
	Shell         Shell
	DeclaredShell bool   // owner declaration when the host omitted per-call shell evidence
	Executable    string // already shell-quoted; empty uses obit
	WSL           bool
	Cwd           string // native path, used only for the optional glob ambiguity read
}

// Suggest returns a copyable alternative for a denied command, never tool input.
// No shell is started. A leading Bash glob may require a bounded cwd scan;
// arguments are copied byte-for-byte, and no ledger or Git state is read.
func Suggest(command string, context Context) (string, string) {
	return suggest(command, context, nil)
}

// SuggestWithSummary also describes the command-word replacement, without
// copying operands. Hooks can keep a denial within their output budget.
func SuggestWithSummary(command string, context Context) (equivalent, summary, why string) {
	equivalent, why = suggest(command, context, &summary)
	return
}

func suggest(command string, context Context, summary *string) (string, string) {
	segments, why := parseSuggestion(command, context)
	if why != "" {
		return command, why
	}
	var changes []replacement
	needsGlobCheck, ambiguousGlobLocation := false, false
	for segment, words := range segments {
		if len(words) == 0 {
			continue
		}
		change, glob, why := translateDeletion(command, context, segments, segment)
		if why != "" {
			return command, why
		}
		needsGlobCheck = needsGlobCheck || glob.needed
		ambiguousGlobLocation = ambiguousGlobLocation || glob.ambiguous
		if change.name == "" {
			continue
		}
		changes = append(changes, change)
		if summary != nil {
			*summary = replacementSummary(change.name)
		}
	}
	if len(changes) == 0 {
		return command, "no supported deletion command"
	}
	// All syntax, dry-run and prefix decisions precede filesystem work.
	if needsGlobCheck {
		if ambiguousGlobLocation {
			return command, "leading glob location is ambiguous"
		}
		if why := checkLeadingGlob(context.Cwd); why != "" {
			return command, why
		}
	}
	return applyReplacements(command, changes)
}

func deletionReplacement(context Context, start, end int, name string) replacement {
	executable := context.Executable
	if executable == "" {
		executable = "obit"
	}
	return replacement{start: start, end: end, text: executable + " " + name, name: name}
}

func replacementSummary(name string) string {
	original := name
	switch name {
	case "git-clean":
		original = "git clean"
	case "remove-item":
		original = "Remove-Item"
	}
	return fmt.Sprintf("the same command with `%s` replaced by `obit %s` via the installed executable (all other arguments unchanged)", original, name)
}

func applyReplacements(command string, changes []replacement) (string, string) {
	var out strings.Builder
	last := 0
	for _, change := range changes {
		out.WriteString(command[last:change.start])
		out.WriteString(change.text)
		last = change.end
	}
	out.WriteString(command[last:])
	return out.String(), ""
}

func sameDeclaredShellWords(command string, shell Shell, words [][]word) bool {
	// Both languages' refusal rules apply to declarations. Matching decoded
	// words alone is insufficient: our PowerShell lexer does not decode smart
	// quotes, and Bash's parser does not reject carriage returns by itself.
	if strings.ContainsRune(command, '\r') {
		return false
	}
	// PowerShell arrays, path separators and wildcards differ from Bash even
	// when the token text matches. Do not guess their expansion from config.
	if strings.ContainsAny(command, `\,*?[]`) {
		return false
	}
	other := PowerShell
	if shell == PowerShell {
		other = Bash
	}
	parsed, why := lex(command, other)
	if why != "" || len(parsed) != len(words) {
		return false
	}
	psWords := words
	if other == PowerShell {
		psWords = parsed
	}
	if why := powerShellSyntax(command, psWords); why != "" {
		return false
	}
	for i, segment := range words {
		if len(segment) != len(parsed[i]) {
			return false
		}
		for j, w := range segment {
			o := parsed[i][j]
			if w.value != o.value || w.expands || o.expands || w.doubledQuote || o.doubledQuote {
				return false
			}
		}
	}
	return true
}

func powerShellSyntax(command string, segments [][]word) string {
	// Count lexical statements, not newlines or punctuation in comments.
	if len(segments) != 1 {
		return "PowerShell requires a single statement"
	}
	for _, w := range segments[0] {
		raw := command[w.start:w.end]
		if strings.ContainsAny(raw, ";&|\r\n") || strings.Contains(raw, "--%") {
			return "PowerShell argument contains unsupported syntax"
		}
		for _, r := range raw {
			if r > unicode.MaxASCII && unicode.Is(unicode.Quotation_Mark, r) {
				return "Unicode quotes are unsupported"
			}
		}
	}
	return ""
}

func literalCD(words []word) bool {
	if len(words) < 1 || len(words) > 2 || !words[0].literal || words[0].value != "cd" {
		return false
	}
	if len(words) == 1 {
		return true
	}
	arg := words[1]
	return !arg.expands && !strings.HasPrefix(arg.value, "-") && !strings.ContainsAny(arg.value, "$*?[`")
}

func safeWSLRelative(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") || strings.ContainsAny(p, ":$*?[\\,'\"") {
		return false
	}
	return !hasParentSegment(p)
}

func optionsAfterOperand(args []string) bool {
	operand := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if operand {
				return true
			}
			if arg == "--" {
				return false
			}
		} else {
			operand = true
		}
	}
	return false
}

func literalItemPath(p string) bool {
	if strings.ContainsAny(p, ",*?[$`") {
		return false
	}
	for i, c := range p {
		if c == ':' && (i != 1 || len(p) <= 2 || p[2] != '\\' && p[2] != '/' || (p[0] < 'a' || p[0] > 'z') && (p[0] < 'A' || p[0] > 'Z')) {
			return false
		}
	}
	return true
}

func changesLocation(name string) bool {
	switch strings.ToLower(name) {
	case "set-location", "sl", "cd", "chdir", "push-location", "pushd", "pop-location", "popd", "new-psdrive":
		return true
	}
	return false
}

func removeItemAlias(name string) bool {
	switch strings.ToLower(name) {
	case "remove-item", "rm", "del", "ri", "rd", "rmdir", "erase":
		return true
	}
	return false
}

// WSLMountedPath recognizes only the Windows drive mounts supported by interop.
func WSLMountedPath(p string) bool {
	p = path.Clean(p)
	return len(p) >= 6 && strings.HasPrefix(p, "/mnt/") &&
		(p[5] >= 'a' && p[5] <= 'z' || p[5] >= 'A' && p[5] <= 'Z') && (len(p) == 6 || p[6] == '/')
}
