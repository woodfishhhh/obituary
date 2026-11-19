package guardrm

import "strings"

func bashCommandDeletion(words []string) string {
	if len(words) == 0 {
		return ""
	}
	switch strings.TrimSuffix(commandBase(words[0]), ".exe") {
	case "find":
		return findDeletion(words[1:])
	case "xargs":
		if deletion := commandDeletion(xargsCommand(words[1:]), false); deletion != "" {
			return "xargs " + deletion
		}
		return ""
	default:
		return commandDeletion(words, false)
	}
}

func findDeletion(args []string) string {
	for len(args) > 0 {
		option := args[0]
		args = args[1:]
		switch option {
		case "--help", "--version":
			return ""
		case "-delete":
			return "find -delete"
		case "-exec", "-execdir", "-ok", "-okdir":
			end := findExecEnd(args)
			if end == len(args) {
				return "" // unterminated action
			}
			if deletion := commandDeletion(args[:end], false); deletion != "" {
				return "find " + option + " " + deletion
			}
			args = args[end+1:]
		case "-fprintf":
			args = args[min(2, len(args)):]
		// These primaries consume a value even if it looks like an action.
		case "-D", "-f", "-files0-from", "-regextype", "-maxdepth", "-mindepth",
			"-amin", "-anewer", "-atime", "-Bmin", "-Bnewer", "-Btime", "-cmin", "-cnewer", "-context", "-ctime",
			"-flags", "-fstype", "-gid", "-group", "-ilname", "-iname", "-inum", "-ipath", "-iregex", "-iwholename",
			"-links", "-lname", "-mmin", "-mtime", "-name", "-newer", "-path", "-perm", "-regex", "-samefile",
			"-size", "-type", "-uid", "-used", "-user", "-wholename", "-xattrname", "-xtype",
			"-fls", "-fprint", "-fprint0", "-printf":
			args = args[min(1, len(args)):]
		default:
			if len(option) == len("-newerXY") && strings.HasPrefix(option, "-newer") {
				args = args[min(1, len(args)):]
			}
		}
	}
	return ""
}

func findExecEnd(args []string) int {
	for i, arg := range args {
		if arg == ";" || arg == "+" && i > 0 && args[i-1] == "{}" {
			return i
		}
	}
	return len(args)
}

func xargsCommand(args []string) []string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return args[i+1:]
		case arg == "-" || !strings.HasPrefix(arg, "-"):
			return args[i:]
		case arg == "--help" || arg == "--version":
			return nil
		case strings.HasPrefix(arg, "--"):
			switch arg {
			case "--arg-file", "--delimiter", "--max-lines", "--max-args", "--max-procs", "--max-chars", "--process-slot-var":
				i++
			}
		default:
			// A required short-option argument ends a cluster; GNU's
			// optional -e/-i/-l arguments can only be attached.
			j := strings.IndexAny(arg[1:], "adEILnPsJRSeil")
			if j >= 0 && j+2 == len(arg) && !strings.ContainsRune("eil", rune(arg[j+1])) {
				i++
			}
		}
	}
	return nil
}
