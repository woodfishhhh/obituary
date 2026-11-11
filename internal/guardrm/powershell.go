package guardrm

import (
	"fmt"
	"strings"
)

// ItemOptions is the filesystem-only, noninteractive Remove-Item subset.
type ItemOptions struct {
	Recursive, Force, Literal bool
	Paths                     []string
}

// ParseRemoveItem rejects unsupported parameters before anything is buried.
// -Force permits hidden/read-only items; it never disables Obituary's Guard.
func ParseRemoveItem(args []string) (ItemOptions, error) {
	var o ItemOptions
	for _, arg := range args {
		if unicodeDashPrefix(arg) {
			return o, fmt.Errorf("Unicode dash parameter prefix is unsupported; use an explicit path such as .\\%s", arg) //nolint:staticcheck // ST1005: Unicode is a proper noun; CLI tests also depend on this diagnostic.
		}
	}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		parameter := ""
		if strings.HasPrefix(arg, "-") {
			var err error
			parameter, err = itemParameter(arg)
			if err != nil {
				return o, err
			}
			if seen[parameter] {
				return o, fmt.Errorf("Parameter '%s' was specified more than once", parameter) //nolint:staticcheck // ST1005: preserve the PowerShell-compatible parameter diagnostic.
			}
			seen[parameter] = true
		}
		switch parameter {
		case "-recurse":
			o.Recursive = true
			continue
		case "-force":
			o.Force = true
			continue
		case "-path", "-literalpath":
			if len(o.Paths) != 0 || seen["-path"] && seen["-literalpath"] {
				return o, fmt.Errorf("Path and LiteralPath cannot be bound more than once") //nolint:staticcheck // ST1005: Path and LiteralPath are PowerShell parameter names.
			}
			o.Literal = parameter == "-literalpath"
			i++
			if i == len(args) || strings.HasPrefix(args[i], "-") {
				return o, fmt.Errorf("Missing an argument for parameter '%s'", arg[1:]) //nolint:staticcheck // ST1005: preserve the PowerShell-compatible parameter diagnostic.
			}
		case "":
		default:
			return o, fmt.Errorf("unsupported Remove-Item parameter '%s'", arg)
		}
		// This is a native CLI: each argv is one path, including literal commas.
		o.Paths = append(o.Paths, args[i])
	}
	return o, validateRemoveItemPaths(o)
}

func validateRemoveItemPaths(o ItemOptions) error {
	if len(o.Paths) == 0 {
		return fmt.Errorf("Missing mandatory parameter: Path") //nolint:staticcheck // ST1005: preserve the PowerShell-compatible parameter diagnostic.
	}
	if len(o.Paths) != 1 {
		return fmt.Errorf("Remove-Item accepts exactly one path")
	}
	for _, path := range o.Paths {
		if strings.ContainsRune(path, '"') {
			return fmt.Errorf("double quote is invalid in a Windows path: %s", path)
		}
		if posixAbsolutePath(path) {
			return fmt.Errorf("POSIX absolute path is ambiguous on Windows; use a drive-qualified or relative path: %s", path)
		}
		if driveRelativePath(path) {
			return fmt.Errorf("drive-relative path is unsupported; use an absolute or ordinary relative path: %s", path)
		}
		if path == "" {
			return fmt.Errorf("Cannot bind argument to parameter 'Path' because it is an empty string") //nolint:staticcheck // ST1005: preserve the PowerShell-compatible parameter diagnostic.
		}
		if ambiguousWindowsPath(path) {
			return fmt.Errorf("path has a trailing dot or space: %s", path)
		}
		if strings.HasPrefix(path, "~") {
			return fmt.Errorf("PowerShell tilde expansion is unsupported: %s", path)
		}
		if !o.Literal && strings.ContainsRune(path, '`') {
			return fmt.Errorf("PowerShell wildcard backtick escapes are unsupported: %s", path)
		}
		if !o.Literal && strings.ContainsAny(path, "*?[]") {
			return fmt.Errorf("wildcards are unsupported; use obit rm in bash for shell expansion, or -LiteralPath for literal names (including [ and ]): %s", path)
		}
		// Providers and alternate data streams do not denote filesystem items.
		for i, c := range path {
			if c == ':' && (i != 1 || (path[0] < 'A' || path[0] > 'Z') && (path[0] < 'a' || path[0] > 'z')) {
				return fmt.Errorf("Only filesystem paths are supported: %s", path) //nolint:staticcheck // ST1005: provider-rejection contract tests depend on this diagnostic.
			}
		}
	}
	return nil
}

func unicodeDashPrefix(arg string) bool {
	return strings.HasPrefix(arg, "–") || strings.HasPrefix(arg, "—") || strings.HasPrefix(arg, "―")
}

// Include unsupported and common parameters in prefix resolution: -f is
// ambiguous between Force and Filter, and -p is not shorthand for Path.
func itemParameter(arg string) (string, error) {
	name := strings.ToLower(arg)
	var matches []string
	for _, p := range []string{"-path", "-literalpath", "-recurse", "-force", "-filter", "-include", "-exclude", "-whatif", "-confirm", "-credential", "-stream",
		"-verbose", "-debug", "-erroraction", "-warningaction", "-informationaction", "-progressaction", "-errorvariable", "-warningvariable", "-informationvariable", "-outvariable", "-outbuffer", "-pipelinevariable"} {
		if p == name {
			return p, nil
		}
		if strings.HasPrefix(p, name) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return "", fmt.Errorf("unknown or ambiguous Remove-Item parameter '%s'", arg)
}
