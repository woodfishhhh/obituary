package guardrm

import (
	"regexp"
	"strings"
)

var wslDistroName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

const (
	maxWSLDistroLen = 128
	maxWrapperDepth = 8
)

func nativeEqualsCommand(word string) bool {
	name := strings.TrimSuffix(commandBase(word), ".exe")
	return name == "docker" || name == "wsl"
}

func safeWSLDistroName(name string) bool {
	return len(name) > 0 && len(name) <= maxWSLDistroLen && wslDistroName.MatchString(name)
}

// wslDeletion checks wsl / wsl.exe arguments in command position for the
// irreversible --unregister option. It stops before the Linux command line
// (--, -e/--exec, or the first positional command operand) so inner commands
// such as `wsl -d Ubuntu -- echo --unregister` are never misclassified.
func wslDeletion(args []string) string {
	unregister := false
	distro := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		key, value, assigned := strings.Cut(arg, "=")
		if !strings.HasPrefix(key, "-") {
			if !unregister {
				break // first non-option word starts the Linux command inside WSL
			}
			if distro == "" {
				distro = arg
			}
			continue
		}
		switch lower := strings.ToLower(key); lower {
		case "--unregister":
			unregister = true
			if assigned {
				distro = value
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				distro = args[i]
			}
		case "-e", "--exec", "-h", "--help":
			if !unregister {
				return ""
			}
		default:
			if !assigned {
				i += wslOptionValueCount(lower)
			}
		}
	}
	if !unregister {
		return ""
	}
	if safeWSLDistroName(distro) {
		return "wsl --unregister " + distro
	}
	return "wsl --unregister"
}

func wslOptionValueCount(opt string) int {
	switch opt {
	case "-d", "--distribution", "--distribution-id", "-u", "--user",
		"--cd", "--shell-type", "-t", "--terminate", "-s", "--set-default",
		"--set-default-version", "--manage", "--move", "--mount", "--name", "--location",
		"--vhd-size", "--partition", "--type", "--options", "--set-sparse",
		"--set-default-user":
		return 1
	case "--export", "--import-in-place", "--set-version":
		return 2
	case "--import":
		return 3
	}
	return 0
}

func cmdWSLDeletion(args []string, depth int) string {
	if depth >= maxWrapperDepth {
		return ""
	}
	for i, arg := range args {
		tail, ok := splitCmdExecFlag(arg)
		if !ok {
			continue
		}
		rest := args[i+1:]
		if tail != "" {
			rest = append([]string{tail}, rest...)
		}
		if len(rest) == 0 {
			return ""
		}
		if len(rest) == 1 {
			return cmdLineWSLDeletion(rest[0], depth+1)
		}
		return cmdLineWSLDeletion(joinCmdArgs(rest), depth+1)
	}
	return ""
}

func splitCmdExecFlag(arg string) (string, bool) {
	for _, prefix := range []string{"//c", "//k", "/c", "/k"} {
		if len(arg) >= len(prefix) && strings.EqualFold(arg[:len(prefix)], prefix) {
			return arg[len(prefix):], true
		}
	}
	return "", false
}

// joinCmdArgs reconstructs a Windows CreateProcess command line for multi-arg
// cmd /c invocations, quoting only arguments with spaces or tabs that are not
// already wrapped in double quotes.
func joinCmdArgs(args []string) string {
	var b strings.Builder
	for i, arg := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		quoted := len(arg) >= 2 && arg[0] == '"' && arg[len(arg)-1] == '"'
		if strings.ContainsAny(arg, " \t") && !quoted {
			b.WriteByte('"')
			b.WriteString(arg)
			b.WriteByte('"')
		} else {
			b.WriteString(arg)
		}
	}
	return b.String()
}

// cmdLineWSLDeletion scans a CMD command string statically for command-position
// wsl --unregister (or nested cmd/powershell wrappers). Other CMD commands pass.
func cmdLineWSLDeletion(s string, depth int) string {
	if depth >= maxWrapperDepth {
		return ""
	}
	if found := cmdScanLine(s, depth); found != "" {
		return found
	}
	// Windows cmd.exe /c rule 2 strips the first '"' and the last '"' when the
	// command starts with '"', preserving trailing redirections and operators.
	trimmed := strings.TrimSpace(s)
	if len(trimmed) >= 2 && trimmed[0] == '"' {
		if last := strings.LastIndexByte(trimmed, '"'); last > 0 {
			if stripped := trimmed[1:last] + trimmed[last+1:]; stripped != s {
				return cmdScanLine(stripped, depth+1)
			}
		}
	}
	return ""
}

type cmdLineScanner struct {
	words   []string
	cur     strings.Builder
	inToken bool
	inQuote bool
}

func (c *cmdLineScanner) flush() {
	if c.inToken {
		c.words = append(c.words, c.cur.String())
		c.cur.Reset()
		c.inToken = false
	}
}

func cmdScanLine(s string, depth int) string {
	if depth >= maxWrapperDepth {
		return ""
	}
	var sc cmdLineScanner
	segStart := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if sc.inQuote {
			if ch == '"' {
				sc.inQuote = false
			} else {
				sc.cur.WriteByte(ch)
			}
			continue
		}
		switch {
		case ch == '"':
			sc.inQuote, sc.inToken = true, true
		case ch == '^' && i+1 < len(s):
			i++
			sc.cur.WriteByte(s[i])
			sc.inToken = true
		case ch == ' ' || ch == '\t':
			sc.flush()
			if cmdLineComment(sc.words) {
				i = cmdSkipToNewline(s, i)
				sc.words, segStart = nil, i+1
			}
		case ch == '&' || ch == '|' || ch == '(' || ch == ')' || ch == '\r' || ch == '\n':
			sc.flush()
			if found := cmdSegmentWSLDeletion(s[segStart:i], sc.words, depth); found != "" {
				return found
			}
			sc.words, segStart = nil, i+1
		default:
			sc.cur.WriteByte(ch)
			sc.inToken = true
		}
	}
	sc.flush()
	return cmdSegmentWSLDeletion(s[segStart:], sc.words, depth)
}

func cmdStripAtWords(words []string) (string, []string, bool) {
	for i, w := range words {
		if first := strings.TrimLeft(w, "@"); first != "" {
			return first, words[i+1:], true
		}
	}
	return "", nil, false
}

func cmdSegmentWSLDeletion(raw string, words []string, depth int) string {
	first, rest, ok := cmdStripAtWords(words)
	if !ok {
		return ""
	}
	if strings.TrimSuffix(commandBase(first), ".exe") == "cmd" {
		if tail, ok := cmdRawExecTail(raw); ok {
			return cmdLineWSLDeletion(tail, depth+1)
		}
	}
	return wslWrapperWords(first, rest, depth)
}

// cmdRawExecTail returns the raw command tail following /c or /k in a CMD
// segment so nested cmd.exe /c outer double quotes remain intact.
func cmdRawExecTail(seg string) (string, bool) {
	i := 0
	for i < len(seg) && (seg[i] == '@' || seg[i] == ' ' || seg[i] == '\t') {
		i++
	}
	i = skipCmdRawToken(seg, i)
	for {
		i = skipCmdSpaces(seg, i)
		if i >= len(seg) {
			return "", false
		}
		if tail, ok := splitCmdExecFlag(seg[i:]); ok {
			return tail, true
		}
		i = skipCmdRawToken(seg, i)
	}
}

func skipCmdSpaces(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

func skipCmdRawToken(s string, i int) int {
	inQuote := false
	for i < len(s) {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '^':
			if !inQuote && i+1 < len(s) {
				i++
			}
		case ' ', '\t':
			if !inQuote {
				return i
			}
		}
		i++
	}
	return i
}

func cmdLineComment(words []string) bool {
	first, rest, ok := cmdStripAtWords(words)
	if !ok || len(rest) != 0 {
		return false
	}
	return strings.EqualFold(first, "rem") || strings.HasPrefix(first, "::")
}

func cmdSkipToNewline(s string, i int) int {
	if nl := strings.IndexByte(s[i:], '\n'); nl >= 0 {
		return i + nl
	}
	return len(s) - 1
}

func psWrapperWSLDeletion(args []string, depth int) string {
	if depth >= maxWrapperDepth {
		return ""
	}
	for i := 0; i < len(args); i++ {
		lower := strings.ToLower(args[i])
		if strings.HasPrefix(lower, "/") {
			lower = "-" + lower[1:]
		}
		if !strings.HasPrefix(lower, "-") {
			return psScriptWSLDeletion(strings.Join(args[i:], " "), depth+1)
		}
		switch lower {
		case "-encodedcommand", "-e", "-ec", "-enc", "-help", "-h", "-?":
			return ""
		case "-executionpolicy", "-ep", "-ex", "-windowstyle", "-w",
			"-outputformat", "-of", "-o", "-inputformat", "-if",
			"-workingdirectory", "-wd", "-configurationname", "-config",
			"-settingsfile", "-version", "-v":
			i++
		default:
			if len(lower) >= 2 && strings.HasPrefix("-file", lower) {
				return ""
			}
			if lower == "-c" || (len(lower) >= 3 && strings.HasPrefix("-command", lower)) {
				return psCommandTailWSLDeletion(args[i+1:], depth+1)
			}
		}
	}
	return ""
}

func psCommandTailWSLDeletion(rest []string, depth int) string {
	if len(rest) == 0 {
		return ""
	}
	if len(rest) == 1 {
		return psScriptWSLDeletion(rest[0], depth)
	}
	// Preserve a spaced wsl executable path whose outer quotes were removed by
	// the parent shell without recursing into nested wrappers twice.
	if strings.ContainsAny(rest[0], " \t") {
		base := commandBase(rest[0])
		if !strings.ContainsRune(base, 0) && strings.TrimSuffix(base, ".exe") == "wsl" {
			if found := wslDeletion(rest[1:]); found != "" {
				return found
			}
		}
	}
	return psScriptWSLDeletion(strings.Join(rest, " "), depth)
}

func psScriptWSLDeletion(script string, depth int) string {
	found := powerShellDeletionDepth(script, depth)
	if strings.Contains(found, "wsl --unregister") {
		return found
	}
	return ""
}

func wslWrapperWords(first string, args []string, depth int) string {
	base := commandBase(first)
	if strings.ContainsRune(base, 0) {
		return ""
	}
	switch strings.TrimSuffix(base, ".exe") {
	case "wsl":
		return wslDeletion(args)
	case "cmd":
		return cmdWSLDeletion(args, depth)
	case "powershell", "pwsh":
		return psWrapperWSLDeletion(args, depth)
	}
	return ""
}
