package guardrm

import "strings"

// ReferencedPaths reuses the guard's static lexer, based on mvdan/sh. Only
// commands whose operands are paths qualify; arbitrary strings (echo, scripts,
// search patterns, options, substitutions) must never become path evidence.
func ReferencedPaths(command string, shell Shell) []string {
	if shell != Bash && shell != PowerShell {
		return nil
	}
	segments, why := lex(command, shell)
	if why != "" || len(segments) != 1 || len(segments[0]) < 2 {
		return nil
	}
	words := segments[0]
	name := words[0].value
	if !words[0].literal {
		return nil
	}
	allowed := false
	if shell == Bash {
		allowed = name == "cat" || name == "ls" || name == "stat" || name == "head" || name == "tail"
	} else {
		switch strings.ToLower(name) {
		case "get-content", "get-childitem", "get-item", "test-path", "gc", "gci", "cat", "ls", "dir":
			allowed = true
		}
	}
	if !allowed {
		return nil
	}
	var paths []string
	for _, w := range words[1:] {
		if w.value == "--" || shell == PowerShell && (strings.EqualFold(w.value, "-LiteralPath") || strings.EqualFold(w.value, "-Path")) {
			continue
		}
		if w.expands || w.value == "" || strings.HasPrefix(w.value, "-") || strings.ContainsAny(w.value, "*?[]$~\x00\r\n") {
			return nil
		}
		paths = append(paths, w.value)
	}
	return paths
}
