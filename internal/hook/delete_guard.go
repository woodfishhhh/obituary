package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func guardDelete(home string, ev Event) (string, error) {
	equivalent, why := ev.Command, ev.SkipReason
	var summary string
	if Disabled() || os.Getenv("OBIT_GUARD_RM") == "0" {
		why = "delete guard is disabled"
	}
	if why == "" {
		var context guardrm.Context
		context, why = deleteGuardContext(ev)
		if why == "" {
			equivalent, summary, why = guardrm.SuggestWithSummary(ev.Command, context)
		}
	}
	if why == "" {
		if err := guardrm.Record(home, guardrm.AuditEvent{Kind: "denied", Command: ev.Command, Equivalent: equivalent}); err != nil {
			// A failed receipt must never revoke an already decided denial.
			Logf(home, "delete guard audit: %v", err)
		}
		return deleteReason(equivalent, summary), nil
	}
	if Auditing() {
		return "", guardrm.Record(home, guardrm.AuditEvent{Kind: "skipped", Command: ev.Command, Reason: why})
	}
	return "", nil
}

func deleteGuardContext(ev Event) (guardrm.Context, string) {
	context := guardrm.Context{Shell: ev.Shell, DeclaredShell: ev.ShellDeclared, WSL: strings.HasPrefix(ev.Cwd, "/") || strings.HasPrefix(ev.Cwd, "~")}
	context.Cwd, _ = NativePath(ev.Cwd) // read-only glob check; preserve ev.Cwd's shell namespace
	var why string
	if context.WSL && (!guardrm.WSLMountedPath(ev.Cwd) || context.Shell != guardrm.Bash) {
		why = "WSL requires bash in a mounted Windows directory"
	} else if context.Shell == guardrm.PowerShell && !windowsCwd(ev.Cwd) {
		why = "PowerShell deletion requires a Windows working directory"
	} else {
		context.Executable, why = deleteGuardExecutable(context)
	}
	return context, why
}

func deleteGuardExecutable(context guardrm.Context) (string, string) {
	self, err := os.Executable()
	if err != nil {
		return "", "hook executable unavailable: " + err.Error()
	}
	if real, err := filepath.EvalSymlinks(self); err == nil {
		self = real
	}
	if context.WSL {
		executable := WSLExecutable(self)
		if executable == "" {
			return "", "executable is not accessible through WSL interop"
		}
		return executable, ""
	}
	// Recheck PATH on every denial; an unavailable or different binary keeps
	// the absolute-path hook's executable. Windows PATH cannot prove WSL lookup.
	if obitOnPATHIsSelf(self) {
		return "obit", ""
	}
	if context.Shell == guardrm.PowerShell {
		return "& '" + strings.ReplaceAll(self, "'", "''") + "'", ""
	}
	return "'" + strings.ReplaceAll(filepath.ToSlash(self), "'", `'\''`) + "'", ""
}

func obitOnPATHIsSelf(self string) bool {
	candidate, err := exec.LookPath("obit")
	if err != nil || !filepath.IsAbs(candidate) {
		return false
	}
	candidate, err = filepath.EvalSymlinks(candidate)
	if err != nil {
		return false
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return false
	}
	// EvalSymlinks restores Windows long names and their stored case;
	// SameFile also verifies identity in case-sensitive Windows directories.
	candidateInfo, err := os.Stat(candidate)
	if err != nil {
		return false
	}
	selfInfo, err := os.Stat(self)
	return err == nil && os.SameFile(candidateInfo, selfInfo)
}

func deleteReason(equivalent, summary string) string {
	// This message is independent of config: recycle mode uses the system Recycle
	// Bin, while resurrect applies specifically to graveyard entries.
	reason := fmt.Sprintf("Obituary's delete guard is on: `%s` provides restorable deletion; `obit resurrect` restores graveyard entries.", equivalent)
	if len(reason) > maxContext {
		reason = "Obituary's delete guard is on: " + summary + " provides restorable deletion."
	}
	if len(reason) > maxContext {
		// Even an exceptionally long executable path must not revoke deny or
		// produce a truncated command that could select different operands.
		reason = "Obituary's delete guard is on: a restorable deletion command is available through the installed obit executable."
	}
	return reason
}

// Codex 0.167 emits Bash + command without the per-call shell. Native function
// tool shapes remain supported as compatibility inputs. Only known shell
// -c/-Command argv envelopes are unwrapped; arbitrary
// argv arrays are not shell source and must never be joined and reinterpreted.
func codexCommand(tool string, input map[string]any, cwd string) (string, guardrm.Shell, string) {
	shell := guardrm.Shell("")
	if strings.HasPrefix(cwd, "/") {
		shell = guardrm.Bash
	} else if windowsCwd(cwd) {
		shell = guardrm.Uncertain
	}
	if tty, _ := input["tty"].(bool); tty {
		return firstString(input, "cmd", "command"), shell, "interactive terminal"
	}
	switch tool {
	case "Bash", "exec_command", "shell_command", "shell", "local_shell":
	default:
		return "", shell, "unsupported tool: " + tool
	}
	if command := firstString(input, "cmd", "command"); command != "" {
		if explicit := firstString(input, "shell"); explicit != "" {
			shell = shellName(explicit)
		}
		return command, shell, ""
	}
	raw, ok := input["command"].([]any)
	if !ok {
		return "", shell, "missing shell command"
	}
	return codexArgvCommand(raw, shell)
}

func codexArgvCommand(raw []any, shell guardrm.Shell) (string, guardrm.Shell, string) {
	encoded, _ := json.Marshal(raw)
	args := make([]string, len(raw))
	for i, v := range raw {
		var ok bool
		args[i], ok = v.(string)
		if !ok {
			return string(encoded), shell, "non-string argv"
		}
	}
	if len(args) == 3 {
		base := strings.ToLower(args[0][strings.LastIndexAny(args[0], `/\`)+1:])
		if (base == "bash" || base == "bash.exe" || base == "sh" || base == "sh.exe") && (args[1] == "-c" || args[1] == "-lc") {
			return args[2], guardrm.Bash, ""
		}
	}
	if len(args) >= 3 {
		base := strings.ToLower(args[0][strings.LastIndexAny(args[0], `/\`)+1:])
		if base == "pwsh" || base == "pwsh.exe" || base == "powershell" || base == "powershell.exe" {
			for i := 1; i < len(args)-1; i++ {
				switch strings.ToLower(args[i]) {
				case "-noprofile", "-noninteractive", "-nologo":
				case "-command", "-c":
					if i == len(args)-2 {
						return args[i+1], guardrm.PowerShell, ""
					}
				default:
					return string(encoded), shell, "unsupported PowerShell argv"
				}
			}
		}
	}
	return string(encoded), shell, "unsupported shell argv"
}

func windowsCwd(cwd string) bool {
	return len(cwd) >= 3 && isLetter(cwd[0]) && cwd[1] == ':' && (cwd[2] == '\\' || cwd[2] == '/')
}

func shellName(path string) guardrm.Shell {
	switch strings.ToLower(path[strings.LastIndexAny(path, `/\`)+1:]) {
	case "bash", "bash.exe", "sh", "sh.exe":
		return guardrm.Bash
	case "pwsh", "pwsh.exe", "powershell", "powershell.exe":
		return guardrm.PowerShell
	}
	return ""
}
