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
			next, separator, why := powerShellSeparator(s, i, len(words))
			if why != "" {
				return nil, why
			}
			i = next
			if separator != 0 {
				segments, words = append(segments, words), nil
				lastSeparator = separator
			}
			continue
		}
		w, why := powerShellWord(s, i)
		if why != "" {
			return nil, why
		}
		i = w.end
		words = append(words, w)
	}
	if len(words) == 0 && lastSeparator == ';' {
		return segments, ""
	}
	return append(segments, words), ""
}

func powerShellSeparator(s string, i, words int) (int, byte, string) {
	c := s[i]
	if c == '\r' {
		c = '\n'
	}
	if c == '\n' && words == 0 {
		return i + 1, 0, "" // blank line, or continuation after && / ;
	}
	if words == 0 {
		return i, 0, "empty command segment"
	}
	if c == '|' {
		return i, 0, "only &&, ; and newline may precede deletion"
	}
	i++
	if i < len(s) && s[i] == c && (c == '&' || c == '|') {
		i++
	} else if c == '&' {
		return i, 0, "background command"
	}
	if c == '\n' {
		c = ';'
	}
	return i, c, ""
}

func powerShellWord(s string, i int) (word, string) {
	w := word{start: i, literal: true}
	var value strings.Builder
	var q byte
	for i < len(s) {
		c := s[i]
		if q == 0 && i > w.start && (s[i-1] == '\'' || s[i-1] == '"') && (c == '#' || strings.HasPrefix(s[i:], "<#")) {
			break // a comment may immediately follow a closing quote
		}
		if c == '`' {
			return w, "backticks are unsupported"
		}
		if q == '\'' {
			var why string
			i, q, why = powerShellSingleQuotedByte(s, i, &w, &value)
			if why != "" {
				return w, why
			}
			continue
		}
		if c == '$' || c == ',' && q == '"' {
			return w, "dynamic PowerShell argument or literal comma"
		}
		if q == '"' {
			i, q = powerShellDoubleQuotedByte(s, i, &w, &value)
			continue
		}
		if strings.ContainsRune(" \t\r\n;&|", rune(c)) {
			break
		}
		if strings.ContainsRune("<>(){}@", rune(c)) {
			return w, "redirection, expansion or compound command"
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
		return w, "unfinished quote"
	}
	w.end, w.value = i, value.String()
	return w, ""
}

func powerShellSingleQuotedByte(s string, i int, w *word, value *strings.Builder) (int, byte, string) {
	c := s[i]
	i++
	if c == '\'' {
		if i < len(s) && s[i] == '\'' {
			w.doubledQuote = true
			value.WriteByte('\'')
			return i + 1, '\'', ""
		}
		return i, 0, ""
	}
	if c == ',' {
		return i, '\'', "literal commas in PowerShell paths are unsupported"
	}
	value.WriteByte(c)
	return i, '\'', ""
}

func powerShellDoubleQuotedByte(s string, i int, w *word, value *strings.Builder) (int, byte) {
	c := s[i]
	i++
	if c == '"' {
		if i < len(s) && s[i] == '"' {
			w.doubledQuote = true
			value.WriteByte('"')
			return i + 1, '"'
		}
		return i, 0
	}
	value.WriteByte(c)
	return i, '"'
}
