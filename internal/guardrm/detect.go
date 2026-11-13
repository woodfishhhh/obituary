package guardrm

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"mvdan.cc/sh/v3/syntax"
)

// Deletion names the rm, rmdir, git clean or Remove-Item (or alias) that the
// shell would run at a command position, or returns "". Unlike Suggest it
// accepts any syntax: denying runs nothing, so it can never select more than
// the command. Strings, comments, arguments and dynamic command words do not
// count, and neither do previews (rm --help, git clean -n).
func Deletion(command string, shell Shell) string {
	switch shell {
	case Bash:
		return bashDeletion(command)
	case PowerShell:
		return powerShellDeletion(command)
	}
	return ""
}

var bashContinuations = strings.NewReplacer("\\\r\n", " \r\n", "\\\n", " \n")

func bashDeletion(s string) string {
	found := bashDeletionIn(s)
	// v3.14.1 continues a comment that ends in an escaped newline onto the
	// next line, where Bash starts a new command: read the lines unjoined too.
	if split := bashContinuations.Replace(s); found == "" && split != s && strings.Contains(s, "#") {
		found = bashDeletionIn(split)
	}
	return found
}

// bashDeletionIn walks every simple command, including those in lists,
// pipelines, subshells, substitutions, conditions, loops and functions.
func bashDeletionIn(s string) string {
	if bracketDepth(s) > maxNesting {
		return wordDeletion(s, false)
	}
	// Parse, not the streaming Stmts: v3.14.1's iterator panics when its
	// caller stops at an error.
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(s), "")
	if err != nil {
		// Bash runs the lines before a syntax error, and the parser may reject
		// syntax Bash accepts: check every word instead.
		return wordDeletion(s, false)
	}
	found := ""
	syntax.Walk(f, func(node syntax.Node) bool {
		if call, ok := node.(*syntax.CallExpr); ok && found == "" {
			found = commandDeletion(bashWords(call.Args), false)
		}
		return found == ""
	})
	return found
}

// bashWords removes quoting statically. Expansions become NUL, which no
// command name or option contains, so dynamic words never match.
func bashWords(args []*syntax.Word) []string {
	words := make([]string, len(args))
	for i, arg := range args {
		var b strings.Builder
		bashDecode(arg.Parts, false, &b)
		words[i] = b.String()
	}
	return words
}

func bashDecode(parts []syntax.WordPart, quoted bool, b *strings.Builder) {
	for _, part := range parts {
		switch p := part.(type) {
		case *syntax.Lit:
			bashDecodeLiteral(p.Value, quoted, b)
		case *syntax.SglQuoted:
			if p.Dollar && strings.ContainsRune(p.Value, '\\') {
				b.WriteByte(0) // ANSI-C escapes stay undecoded
			} else {
				b.WriteString(p.Value)
			}
		case *syntax.DblQuoted:
			bashDecode(p.Parts, true, b)
		default:
			b.WriteByte(0)
		}
	}
}

func bashDecodeLiteral(v string, quoted bool, b *strings.Builder) {
	for i := 0; i < len(v); i++ {
		switch c := v[i]; {
		case c == '\\' && i+1 < len(v):
			i++
			if quoted && !strings.ContainsRune("$`\"\\\n", rune(v[i])) {
				b.WriteByte('\\')
			}
			if v[i] != '\n' {
				b.WriteByte(v[i])
			}
		case !quoted && strings.ContainsRune("*?[{", rune(c)):
			b.WriteByte(0) // pathname or brace expansion
		default:
			b.WriteByte(c)
		}
	}
}

// wordDeletion over-approximates for source that cannot be read reliably.
func wordDeletion(s string, powerShell bool) string {
	words := strings.FieldsFunc(s, func(r rune) bool { return strings.ContainsRune(" \t\r\n;&|(){}<>`'\"$@,", r) })
	for i := range words {
		if found := commandDeletion(words[i:], powerShell); found != "" {
			return found
		}
	}
	return ""
}

// commandDeletion checks one simple command's decoded words. Path-qualified
// names count (/usr/bin/rm, rm.exe): only the final name selects the program.
func commandDeletion(words []string, powerShell bool) string {
	if len(words) == 0 {
		return ""
	}
	base := strings.ToLower(words[0][strings.LastIndexAny(words[0], `/\`)+1:])
	name := strings.TrimSuffix(base, ".exe")
	switch {
	case strings.ContainsRune(base, 0):
		return ""
	case name == "git":
		return gitDeletion(words[1:])
	case powerShell && removeItemAlias(name):
		if name != base {
			return name // a native rm.exe or rmdir.exe, not the alias
		}
		for _, arg := range words[1:] {
			// -WhatIf (alias -wi; -wh is its shortest unique prefix) only previews.
			if arg = strings.ToLower(arg); arg == "-wi" || len(arg) >= 3 && strings.HasPrefix("-whatif", arg) {
				return ""
			}
		}
		return "Remove-Item"
	case !powerShell && (name == "rm" || name == "rmdir"):
		if len(words) > 1 && (words[1] == "--help" || words[1] == "--version") {
			return ""
		}
		return name
	}
	return ""
}

// gitDeletion finds git's subcommand after the global options (git.c
// handle_options); a dynamic word may expand to an option, so it is skipped.
func gitDeletion(args []string) string {
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; arg {
		case "-C", "-c", "--git-dir", "--work-tree", "--namespace", "--config-env", "--super-prefix", "--attr-source", "--shallow-file":
			i++
		case "-h", "--help", "-v", "--version":
			return ""
		case "clean":
			if cleanPreview(args[i+1:]) {
				return ""
			}
			return "git clean"
		default:
			if !strings.HasPrefix(arg, "-") && !strings.ContainsRune(arg, 0) {
				return ""
			}
		}
	}
	return ""
}

func cleanPreview(args []string) bool {
	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return false
		}
	}
	o, err := ParseClean(args)
	return err == nil && o.DryRun
}

// psScanner reads PowerShell without a parser (none exists for Go). Comments
// and string contents are skipped; script blocks, (…), $(…), @(…), @{…} and
// the $(…) inside expandable strings are read as code. Each statement keeps
// its decoded words, with NUL for dynamic parts.
type psScanner struct {
	s     string
	i     int
	depth int
	found string
}

// maxNesting is far beyond real commands and keeps the stack small.
const maxNesting = 256

func powerShellDeletion(s string) string {
	p := psScanner{s: s}
	p.code(0, false)
	return p.found
}

// code reads statements until the unmatched close byte (0: the end). In a
// hashtable the words before = are a key, not a command.
func (p *psScanner) code(close byte, table bool) {
	if p.depth++; p.depth > maxNesting {
		// A hook's input can be megabytes: check every remaining word instead.
		p.found, p.i = wordDeletion(p.s[p.i:], true), len(p.s)
		return
	}
	defer func() { p.depth-- }()
	var words []string
	flush := func(key bool) {
		if len(words) > 1 && words[0] == "." {
			words = words[1:] // dot-sourcing runs the command too
		}
		if p.found == "" && !key {
			p.found = commandDeletion(words, true)
		}
		words = nil
	}
	defer flush(false)
	for p.i < len(p.s) && p.found == "" {
		c := p.s[p.i]
		switch {
		case close != 0 && c == close:
			p.i++
			return
		case strings.HasPrefix(p.s[p.i:], "`\r\n"):
			p.i += 3 // line continuation
		case strings.HasPrefix(p.s[p.i:], "`\n") || strings.HasPrefix(p.s[p.i:], "`\r"):
			p.i += 2
		case c == ';' || c == '\r' || c == '\n' || c == '|' || c == '&' || c == '=':
			p.i++
			flush(table && c == '=')
		case c == '#':
			p.i += strings.IndexAny(p.s[p.i:]+"\n", "\r\n")
		case strings.HasPrefix(p.s[p.i:], "<#"):
			// Delimited comments end at the first #>; they do not nest.
			if end := strings.Index(p.s[p.i+2:], "#>"); end >= 0 {
				p.i += end + 4
			} else {
				p.i = len(p.s)
			}
		case c == '(' || c == '{':
			p.i++
			p.code(closer(c), false)
			words = append(words, "\x00")
		case strings.ContainsRune(" \t\f\v,<>)}", rune(c)):
			p.i++ // separators, redirections and unmatched closers
		case psSpace(p.s[p.i:]) > 0:
			p.i += psSpace(p.s[p.i:])
		default:
			words = append(words, p.word())
		}
	}
}

// word reads one token and decodes its quotes and escapes.
func (p *psScanner) word() string {
	if p.hereString() {
		return "\x00"
	}
	var b strings.Builder
	for p.i < len(p.s) && p.found == "" {
		if q, n := psQuote(p.s[p.i:]); n > 0 {
			p.i += n
			p.quoted(q, &b)
			continue
		}
		c := p.s[p.i]
		switch {
		case c == '`' && p.i+1 < len(p.s):
			p.i++
			b.WriteByte(p.s[p.i])
			p.i++
		case (c == '$' || c == '@') && p.i+1 < len(p.s) && (p.s[p.i+1] == '(' || c == '@' && p.s[p.i+1] == '{'):
			p.i += 2
			p.code(closer(p.s[p.i-1]), p.s[p.i-1] == '{')
			b.WriteByte(0)
		case c == '$':
			p.i += 1 + variableLength(p.s[p.i+1:])
			b.WriteByte(0)
			if b.Len() == 1 {
				return b.String() // a leading variable is a token of its own: $x=…
			}
		case strings.ContainsRune(" \t\f\v\r\n;|&(){}<>,", rune(c)) || psSpace(p.s[p.i:]) > 0:
			return b.String()
		default:
			b.WriteByte(c)
			p.i++
		}
	}
	return b.String()
}

// quoted reads a string after its opening quote. Any quote of the same class
// closes it and a doubled one is literal; only "…" expands $ and runs $(…).
func (p *psScanner) quoted(q byte, b *strings.Builder) {
	for p.i < len(p.s) && p.found == "" {
		if k, n := psQuote(p.s[p.i:]); n > 0 && k == q {
			p.i += n
			if k, n := psQuote(p.s[p.i:]); n == 0 || k != q {
				return
			}
			p.i += n
			b.WriteByte(q)
			continue
		}
		switch {
		case q == '"' && strings.HasPrefix(p.s[p.i:], "$("):
			p.i += 2
			p.code(')', false)
			b.WriteByte(0)
			continue
		case q == '"' && p.s[p.i] == '$':
			b.WriteByte(0)
		case q == '"' && p.s[p.i] == '`' && p.i+1 < len(p.s):
			p.i++ // keep the escaped byte below
		}
		b.WriteByte(p.s[p.i])
		p.i++
	}
}

// hereString skips @'…'@ and @"…"@ (the terminator starts a line); only the
// expandable form runs its $(…).
func (p *psScanner) hereString() bool {
	q, n := psQuote(p.s[min(p.i+1, len(p.s)):])
	rest := strings.TrimLeft(p.s[min(p.i+1+n, len(p.s)):], " \t")
	if p.s[p.i] != '@' || n == 0 || !strings.HasPrefix(rest, "\n") && !strings.HasPrefix(rest, "\r\n") {
		return false
	}
	for p.i = len(p.s) - len(rest); p.i < len(p.s) && p.found == ""; p.i++ {
		if k, m := psQuote(p.s[p.i+1:]); p.s[p.i] == '\n' && k == q && strings.HasPrefix(p.s[p.i+1+m:], "@") {
			p.i += 2 + m
			return true
		}
		if q == '"' && strings.HasPrefix(p.s[p.i:], "$(") {
			p.i += 2
			p.code(')', false)
			p.i--
		} else if q == '"' && p.s[p.i] == '`' {
			p.i++
		}
	}
	return true
}

// psQuote recognizes the typographic quotes PowerShell treats as ' and ".
func psQuote(s string) (byte, int) {
	if s != "" && (s[0] == '\'' || s[0] == '"') {
		return s[0], 1
	}
	switch r, n := utf8.DecodeRuneInString(s); r {
	case '‘', '’', '‚', '‛':
		return '\'', n
	case '“', '”', '„':
		return '"', n
	}
	return 0, 0
}

// psSpace returns the length of a leading non-ASCII space such as U+00A0.
func psSpace(s string) int {
	if r, n := utf8.DecodeRuneInString(s); r > unicode.MaxASCII && unicode.IsSpace(r) {
		return n
	}
	return 0
}

func variableLength(s string) int {
	if strings.HasPrefix(s, "{") {
		if end := strings.IndexByte(s, '}'); end >= 0 {
			return end + 1
		}
		return len(s)
	}
	n := 0
	for n < len(s) && (s[n] == '_' || s[n] == ':' || s[n] >= '0' && s[n] <= '9' || s[n]|0x20 >= 'a' && s[n]|0x20 <= 'z') {
		n++
	}
	if n == 0 && s != "" && strings.ContainsRune("$?^_", rune(s[0])) {
		n = 1
	}
	return n
}

func closer(open byte) byte {
	if open == '(' {
		return ')'
	}
	return '}'
}
