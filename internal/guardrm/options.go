package guardrm

import (
	"fmt"
	"os"
	"strings"
)

// RemoveOptions is the supported rm/rmdir subset after shell expansion.
type RemoveOptions struct {
	Recursive, Force, Dir, Verbose, Help bool
	Paths                                []string
}

// ParseRemove rejects unsupported flags before any path can be moved.
func ParseRemove(args []string, directoryOnly bool) (RemoveOptions, error) {
	o := RemoveOptions{Dir: directoryOnly}
	_, posix := os.LookupEnv("POSIXLY_CORRECT")
	flags := true
	for _, arg := range args {
		if flags && arg == "--" {
			flags = false
			continue
		}
		if !flags || arg == "-" || !strings.HasPrefix(arg, "-") {
			o.Paths = append(o.Paths, arg)
			if posix {
				flags = false
			}
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if err := o.parseLongOption(arg, directoryOnly); err != nil {
				return o, err
			}
			if o.Help {
				return o, nil
			}
			continue
		}
		if err := o.parseShortOptions(arg, directoryOnly); err != nil {
			return o, err
		}
	}
	if len(o.Paths) == 0 && !o.Force {
		return o, fmt.Errorf("missing operand")
	}
	for _, path := range o.Paths {
		if posixAbsolutePath(path) {
			return o, fmt.Errorf("POSIX absolute path is ambiguous on Windows; use a drive-qualified or relative path: %s", path)
		}
		if driveRelativePath(path) {
			return o, fmt.Errorf("drive-relative path is unsupported; use an absolute or ordinary relative path: %s", path)
		}
		trimmed := strings.TrimRight(path, `/\`)
		base := trimmed[strings.LastIndexAny(trimmed, `/\`)+1:]
		if base == "." || base == ".." {
			return o, fmt.Errorf("refusing to remove '.' or '..' directory: %s", path)
		}
		if ambiguousWindowsPath(path) {
			return o, fmt.Errorf("path has a trailing dot or space: %s", path)
		}
	}
	return o, nil
}

func (o *RemoveOptions) parseLongOption(arg string, directoryOnly bool) error {
	switch arg {
	case "--help":
		o.Help = true
		return nil
	case "--verbose":
		o.Verbose = true
	case "--recursive":
		o.Recursive = true
	case "--force":
		o.Force = true
	case "--dir":
		o.Dir = true
	case "--preserve-root":
		// Guard always protects roots. Mount traversal restrictions such
		// as --one-file-system and --preserve-root=all are unsupported.
	default:
		return fmt.Errorf("unsupported option '%s'", arg)
	}
	if directoryOnly && arg != "--verbose" {
		return fmt.Errorf("unsupported rmdir option '%s'", arg)
	}
	return nil
}

func (o *RemoveOptions) parseShortOptions(arg string, directoryOnly bool) error {
	for _, flag := range arg[1:] {
		if directoryOnly && flag != 'v' {
			return fmt.Errorf("unsupported rmdir option -- '%c'", flag)
		}
		switch flag {
		case 'r', 'R':
			o.Recursive = true
		case 'f':
			o.Force = true
		case 'd':
			o.Dir = true
		case 'v':
			o.Verbose = true
		default:
			return fmt.Errorf("unsupported option -- '%c'", flag)
		}
	}
	return nil
}

func driveRelativePath(path string) bool {
	return len(path) >= 2 && path[1] == ':' &&
		(path[0] >= 'a' && path[0] <= 'z' || path[0] >= 'A' && path[0] <= 'Z') &&
		(len(path) == 2 || path[2] != '/' && path[2] != '\\')
}

// Even //server/share is rejected: native argv does not tell us whether MSYS
// conversion was enabled, or whether the caller intended a Windows UNC path.
func posixAbsolutePath(path string) bool {
	return strings.HasPrefix(path, "/")
}

func hasParentSegment(path string) bool {
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return true
		}
	}
	return false
}

// Check the original spelling before filepath.Clean or Win32 can normalize it
// to another object. Dot-navigation segments themselves are not ambiguous.
func ambiguousWindowsPath(path string) bool {
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part != "." && part != ".." && (strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ")) {
			return true
		}
	}
	return false
}

// CleanOptions retains Git's own pathspec/exclusion interpretation.
type CleanOptions struct {
	DryRun, Quiet bool
	Force         int
	Preview       []string
	Paths         []string
}

func ParseClean(args []string) (CleanOptions, error) {
	var o CleanOptions
	flags := true
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			flags = false
			o.Preview = append(o.Preview, arg)
			continue
		}
		if !flags || !strings.HasPrefix(arg, "-") || arg == "-" {
			if posixAbsolutePath(arg) {
				return o, fmt.Errorf("POSIX absolute path is ambiguous on Windows; use a drive-qualified or relative path: %s", arg)
			}
			if driveRelativePath(arg) {
				return o, fmt.Errorf("drive-relative path is unsupported; use an absolute or ordinary relative path: %s", arg)
			}
			o.Paths = append(o.Paths, arg)
			o.Preview = append(o.Preview, arg)
			continue
		}
		var err error
		if strings.HasPrefix(arg, "--") {
			i, err = o.parseLongOption(args, i)
		} else {
			i, err = o.parseShortOptions(args, i)
		}
		if err != nil {
			return o, err
		}
	}
	return o, nil
}

func (o *CleanOptions) parseLongOption(args []string, i int) (int, error) {
	arg := args[i]
	switch arg {
	case "--dry-run":
		o.DryRun = true
	case "--force":
		o.Force++
	case "--quiet":
		o.Quiet = true
		return i, nil
	case "--exclude":
		if i+1 >= len(args) {
			return i, fmt.Errorf("option '%s' requires a value", arg)
		}
		o.Preview = append(o.Preview, arg, args[i+1])
		return i + 1, nil
	default:
		if !strings.HasPrefix(arg, "--exclude=") {
			return i, fmt.Errorf("unsupported option '%s'", arg)
		}
	}
	o.Preview = append(o.Preview, arg)
	return i, nil
}

func (o *CleanOptions) parseShortOptions(args []string, i int) (int, error) {
	arg := args[i]
	for j := 1; j < len(arg); j++ {
		flag := arg[j]
		switch flag {
		case 'f':
			o.Force++
		case 'n':
			o.DryRun = true
		case 'q':
			o.Quiet = true
			continue
		case 'd', 'x', 'X':
		case 'e':
			value := arg[j+1:]
			if value == "" {
				i++
				if i == len(args) {
					return i, fmt.Errorf("option '-e' requires a value")
				}
				value = args[i]
			}
			o.Preview = append(o.Preview, "-e", value)
			j = len(arg)
			continue
		default:
			return i, fmt.Errorf("unsupported option -- '%c'", flag)
		}
		o.Preview = append(o.Preview, "-"+string(flag))
	}
	return i, nil
}
