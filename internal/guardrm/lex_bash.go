package guardrm

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// lexBash uses shfmt's parser (mvdan/sh syntax/example_test.go) for shell
// structure and byte positions. Only our existing subset is eligible; parsing
// more Bash syntax must not imply that it has a safe obit equivalent.
func lexBash(s string) ([][]word, string) {
	if strings.ContainsRune(s, '`') {
		return nil, "backticks are unsupported"
	}
	// The parser ignores NUL bytes, which must not create a command word or
	// option that differs from the bytes forwarded to the user's shell.
	if strings.ContainsRune(s, 0) {
		return nil, "NUL bytes are unsupported in bash"
	}
	// v3.14.1 can join statements across a comment ending in an escaped
	// newline. Multiple continuations also erase empty legacy segments.
	// Keep these combinations outside our subset rather than reparse them.
	if continuations := strings.Count(s, "\\\n"); continuations > 1 || continuations > 0 && strings.ContainsRune(s, '#') {
		return nil, "complex bash line continuations are unsupported"
	}
	// Parsers are reusable but not concurrent; each call owns its parser.
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(s), "")
	if err != nil {
		return nil, "invalid bash syntax"
	}
	var segments [][]word
	for _, stmt := range f.Stmts {
		if why := bashStatement(s, stmt, &segments); why != "" {
			return nil, why
		}
	}
	return segments, ""
}

func bashStatement(s string, stmt *syntax.Stmt, segments *[][]word) string {
	if stmt.Negated || stmt.Background || stmt.Coprocess || len(stmt.Redirs) != 0 {
		return "redirection, background or compound command"
	}
	switch cmd := stmt.Cmd.(type) {
	case *syntax.BinaryCmd:
		if cmd.Op != syntax.AndStmt {
			return "only &&, ; and newline may precede deletion"
		}
		if why := bashStatement(s, cmd.X, segments); why != "" {
			return why
		}
		return bashStatement(s, cmd.Y, segments)
	case *syntax.CallExpr:
		if len(cmd.Assigns) != 0 {
			return "environment assignment prefix is unsupported"
		}
		words := make([]word, 0, len(cmd.Args))
		for _, arg := range cmd.Args {
			w := word{start: int(arg.Pos().Offset()), end: int(arg.End().Offset()), literal: true}
			var value strings.Builder
			if why := bashWordParts(s, arg.Parts, false, &w, &value); why != "" {
				return why
			}
			w.value = value.String()
			// Simple escaped newlines disappear from Lit.Value, keeping their
			// source span. A quoted/escaped command word is still unsupported.
			w.literal = w.literal && s[w.start:w.end] == w.value
			if w.start >= 2 && s[w.start-2:w.start] == "\\\n" {
				w.literal = false
			}
			words = append(words, w)
		}
		*segments = append(*segments, words)
		return ""
	default:
		return "redirection, expansion or compound command"
	}
}

// syntax exposes quoting and expansion nodes, but intentionally does not
// unquote their values. expand.Literal is not a substitute here: it evaluates
// parameters/tilde and can look up users. Keep only static decoding, preserving
// short parameter spellings and never evaluating the user's shell environment.
func bashWordParts(s string, parts []syntax.WordPart, quoted bool, w *word, value *strings.Builder) string {
	for _, part := range parts {
		switch p := part.(type) {
		case *syntax.Lit:
			for i := 0; i < len(p.Value); i++ {
				c := p.Value[i]
				if c == '\\' {
					w.literal = false
					i++
					if i == len(p.Value) {
						return "unfinished escape"
					}
					// v3.14.1 retains escaped LF after an odd run of backslashes.
					// Keep residual continuations outside our static subset.
					if p.Value[i] == '\n' {
						return "residual bash line continuations are unsupported"
					}
					if quoted && !strings.ContainsRune("$`\"\\", rune(p.Value[i])) {
						value.WriteByte('\\')
					}
					value.WriteByte(p.Value[i])
					continue
				}
				if !quoted && strings.ContainsRune("<>(){}@", rune(c)) {
					return "redirection, expansion or compound command"
				}
				if c == '$' || !quoted && c == '~' && int(p.Pos().Offset())+i == w.start {
					w.expands = true
				}
				if c == '$' && i+1 == len(p.Value) && bashDollarBoundary(s, p.End()) {
					return "shell expansion or command substitution"
				}
				if !quoted && strings.ContainsRune("$*?[", rune(c)) {
					w.literal = false
					if value.Len() == 0 && c != '$' {
						w.leadingGlob = true
					}
				}
				value.WriteByte(c)
			}
		case *syntax.SglQuoted:
			if p.Dollar {
				return "shell expansion or command substitution"
			}
			w.literal = false
			value.WriteString(p.Value)
		case *syntax.DblQuoted:
			if p.Dollar {
				return "shell expansion or command substitution"
			}
			w.literal = false
			if why := bashWordParts(s, p.Parts, true, w, value); why != "" {
				return why
			}
		case *syntax.ParamExp:
			if !p.Short {
				return "shell expansion or command substitution"
			}
			raw := s[p.Pos().Offset():p.End().Offset()]
			if strings.HasSuffix(raw, "$") && bashDollarBoundary(s, p.End()) {
				return "shell expansion or command substitution"
			}
			if !quoted && strings.ContainsRune(raw, '@') {
				return "redirection, expansion or compound command"
			}
			w.literal, w.expands = false, true
			value.WriteString(raw)
		default:
			return "shell expansion or command substitution"
		}
	}
	return ""
}

// Keep the previous refusal of dollar/quote and dollar/bracket boundaries,
// even when Bash can parse them as adjacent, individually simple parts.
func bashDollarBoundary(s string, end syntax.Pos) bool {
	return int(end.Offset()) < len(s) && strings.ContainsRune("({['\"", rune(s[end.Offset()]))
}
