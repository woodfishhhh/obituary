package guardrm

import (
	"regexp"
	"strconv"
	"strings"
)

var dockerVolumeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)

func dockerReplacement(words []word, context Context, segments [][]word, segment int) (replacement, string) {
	if len(words) < 3 || words[1].value != "volume" || words[2].value != "rm" {
		return replacement{}, ""
	}
	if context.WSL || context.DeclaredShell || !words[1].literal || !words[2].literal {
		return replacement{}, "Docker client or shell cannot be verified"
	}
	if why := deletionPosition(segments, segment); why != "" {
		return replacement{}, why
	}
	names := words[3:]
	if len(names) > 0 && names[0].value == "--" {
		names = names[1:]
	}
	if len(names) == 0 {
		return replacement{}, "no explicit volume names"
	}
	for _, w := range names {
		if w.expands || w.leadingGlob || !dockerVolumeName.MatchString(w.value) {
			return replacement{}, "Docker volume names must be explicit"
		}
	}
	return replacement{start: words[0].start, end: words[2].end, name: "bury --docker-volume"}, ""
}

// Parse Docker command positions separately from values such as compose -f down.
//
//nolint:funlen,nestif // Docker global and subcommand flags must be parsed in one command-position pass.
func dockerDeletion(args []string) string {
	var group, verb string
	var help, preview, volumes bool
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		key, value, assigned := strings.Cut(arg, "=")
		if strings.HasPrefix(key, "-") {
			enabled := !assigned
			if assigned {
				enabled, _ = strconv.ParseBool(value)
			}
			switch key {
			case "--help", "-h":
				help = enabled
			case "--dry-run":
				preview = enabled
			case "--volumes":
				volumes = !assigned || !dockerFalse(value)
			case "-v":
				if group == "compose" {
					volumes = !assigned || !dockerFalse(value)
				}
			case "--context", "-c", "--host", "-H", "--config", "--log-level", "-l", "--tlscacert", "--tlscert", "--tlskey", "--file", "--project-name", "-p", "--project-directory", "--env-file", "--profile", "--parallel", "--ansi", "--progress", "--filter", "--timeout", "-t", "--rmi":
				if !assigned {
					i++
				}
			case "-f":
				if group == "compose" && !assigned {
					i++
				}
			}
			continue
		}
		if group == "" {
			group = arg
		} else if verb == "" {
			verb = arg
		}
	}
	if help || preview {
		return ""
	}
	switch group + " " + verb {
	case "volume rm", "volume remove":
		return "docker volume rm"
	case "volume prune":
		return "docker volume prune"
	case "compose down", "system prune":
		if volumes {
			return "docker " + group + " " + verb + " --volumes"
		}
	}
	return ""
}

func dockerFalse(s string) bool {
	b, err := strconv.ParseBool(s)
	return err == nil && !b
}
