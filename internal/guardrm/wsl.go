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
		"--set-default-version", "--manage", "--mount", "--name", "--location",
		"--vhd-size", "--partition", "--type", "--options", "--set-sparse",
		"--set-default-user":
		return 1
	case "--export", "--import-in-place", "--move", "--set-version":
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
		lower := strings.ToLower(arg)
		if lower != "/c" && lower != "/k" && lower != "//c" && lower != "//k" {
			continue
		}
		rest := args[i+1:]
		if len(rest) == 0 {
			return ""
		}
		if len(rest) == 1 {
			return cmdLineWSLDeletion(rest[0], depth+1)
		}
		return cmdWordListWSLDeletion(rest, depth+1)
	}
	return ""
}

func cmdWordListWSLDeletion(words []string, depth int) string {
	start := 0
	for i, w := range words {
		if w == "&" || w == "&&" || w == "|" || w == "||" || w == "(" || w == ")" {
			if found := wslWrapperWords(words[start:i], depth); found != "" {
				return found
			}
			start = i + 1
		}
	}
	return wslWrapperWords(words[start:], depth)
}

// cmdLineWSLDeletion scans a CMD command string statically for command-position
// wsl --unregister (or nested cmd/powershell wrappers). Other CMD commands pass.
func cmdLineWSLDeletion(s string, depth int) string {
	if depth >= maxWrapperDepth {
		return ""
	}
	var words []string
	var cur strings.Builder
	inToken, inQuote := false, false
	flush := func() {
		if inToken {
			words = append(words, cur.String())
			cur.Reset()
			inToken = false
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			if c == '"' {
				inQuote = false
			} else {
				cur.WriteByte(c)
			}
			continue
		}
		switch {
		case c == '"':
			inQuote, inToken = true, true
		case c == '^' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
			inToken = true
		case c == ' ' || c == '\t':
			flush()
			if cmdLineComment(words) {
				i = cmdSkipToNewline(s, i)
				words = nil
			}
		case c == '&' || c == '|' || c == '(' || c == ')' || c == '\r' || c == '\n':
			flush()
			if found := wslWrapperWords(words, depth); found != "" {
				return found
			}
			words = nil
		default:
			cur.WriteByte(c)
			inToken = true
		}
	}
	flush()
	return wslWrapperWords(words, depth)
}

func cmdLineComment(words []string) bool {
	return len(words) == 1 && (strings.EqualFold(words[0], "rem") || strings.HasPrefix(words[0], "::"))
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
		case "-file", "-f", "-encodedcommand", "-e", "-ec", "-enc", "-help", "-h", "-?":
			return ""
		case "-executionpolicy", "-ep", "-ex", "-windowstyle", "-w",
			"-outputformat", "-of", "-o", "-inputformat", "-if",
			"-workingdirectory", "-wd", "-configurationname", "-config",
			"-settingsfile", "-version", "-v":
			i++
		default:
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
	if found := wslWrapperWords(rest, depth); found != "" {
		return found
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

func wslWrapperWords(words []string, depth int) string {
	if len(words) == 0 {
		return ""
	}
	base := commandBase(words[0])
	if strings.ContainsRune(base, 0) {
		return ""
	}
	switch strings.TrimSuffix(base, ".exe") {
	case "wsl":
		return wslDeletion(words[1:])
	case "cmd":
		return cmdWSLDeletion(words[1:], depth)
	case "powershell", "pwsh":
		return psWrapperWSLDeletion(words[1:], depth)
	}
	return ""
}
