package guardrm

import (
	"strings"
)

type word struct {
	start, end   int
	value        string
	literal      bool
	expands      bool // Bash expansion other than pathname globbing
	leadingGlob  bool // first decoded character is an unquoted glob metacharacter
	doubledQuote bool // PowerShell quote escaping cannot be suggested to native argv
}

// lex keeps source spans, so suggestions never reserialize shell arguments.
// Unsupported syntax invalidates the entire command, including earlier segments.
func lex(s string, shell Shell) ([][]word, string) {
	if shell == Bash {
		return lexBash(s)
	}
	var segments [][]word
	var words []word
	var lastSeparator byte
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "<#") {
			// PowerShell delimited comments end at the first #>; they do not nest.
			end := strings.Index(s[i+2:], "#>")
			if end < 0 {
				return nil, "unfinished PowerShell block comment"
			}
			i += end + 4
			continue
		}
		switch s[i] {
		case ' ', '\t':
			i++
			continue
		case '#':
			for i < len(s) && s[i] != '\n' && s[i] != '\r' {
				i++
			}
			continue
		case ';', '\r', '\n', '&', '|':
			c := s[i]
			if c == '\r' {
				c = '\n'
			}
			if c == '\n' && len(words) == 0 {
				i++ // blank line, or continuation after && / ;
				continue
			}
			if len(words) == 0 {
				return nil, "empty command segment"
			}
			if c == '|' {
				return nil, "only &&, ; and newline may precede deletion"
			}
			i++
			if i < len(s) && s[i] == c && (c == '&' || c == '|') {
				i++
			} else if c == '&' {
				return nil, "background command"
			}
			segments, words = append(segments, words), nil
			lastSeparator = c
			if c == '\n' {
				lastSeparator = ';'
			}
			continue
		}
		w := word{start: i, literal: true}
		var value strings.Builder
		var q byte
		for i < len(s) {
			c := s[i]
			if q == 0 && i > w.start && (s[i-1] == '\'' || s[i-1] == '"') && (c == '#' || strings.HasPrefix(s[i:], "<#")) {
				break // a comment may immediately follow a closing quote
			}
			if c == '`' {
				return nil, "backticks are unsupported"
			}
			if q == '\'' {
				i++
				if c == '\'' {
					if i < len(s) && s[i] == '\'' {
						w.doubledQuote = true
						value.WriteByte('\'')
						i++
						continue
					}
					q = 0
				} else {
					if c == ',' {
						return nil, "literal commas in PowerShell paths are unsupported"
					}
					value.WriteByte(c)
				}
				continue
			}
			if c == '$' || c == ',' && q == '"' {
				return nil, "dynamic PowerShell argument or literal comma"
			}
			if q == '"' {
				i++
				if c == '"' {
					if i < len(s) && s[i] == '"' {
						w.doubledQuote = true
						value.WriteByte('"')
						i++
						continue
					}
					q = 0
				} else {
					value.WriteByte(c)
				}
				continue
			}
			if strings.ContainsRune(" \t\r\n;&|", rune(c)) {
				break
			}
			if strings.ContainsRune("<>(){}@", rune(c)) {
				return nil, "redirection, expansion or compound command"
			}
			if c == '\'' || c == '"' {
				q, w.literal = c, false
			} else {
				value.WriteByte(c)
				if c == '*' || c == '?' || c == '[' {
					w.literal = false
				}
			}
			i++
		}
		if q != 0 {
			return nil, "unfinished quote"
		}
		w.end, w.value = i, value.String()
		words = append(words, w)
	}
	if len(words) == 0 && lastSeparator == ';' {
		return segments, ""
	}
	return append(segments, words), ""
}
