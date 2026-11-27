package hook

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func guardDelete(home string, ev Event) (string, error) {
	equivalent, why := ev.Command, ev.SkipReason
	var summary, guidance string
	if Disabled() || os.Getenv("OBIT_GUARD_RM") == "0" {
		why = "delete guard is disabled"
	}
	if why == "" {
		why = codexWindowsEmbeddedDeletion(ev)
		guidance = why
	}
	if why == "" {
		var context guardrm.Context
		context, why = deleteGuardContext(ev)
		if why == "" {
			equivalent, summary, why, guidance = suggestDeletion(ev, context)
		}
	}
	if why == "" || guidance != "" {
		receipt := guardrm.AuditEvent{Kind: "denied", Command: ev.Command, Equivalent: equivalent}
		if why != "" {
			receipt.Equivalent, receipt.Reason = "", why
		}
		if err := guardrm.Record(home, receipt); err != nil {
			// A failed receipt must never revoke an already decided denial.
			Logf(home, "delete guard audit: %v", err)
		}
		if guidance != "" {
			return guidance, nil
		}
		return deleteReason(equivalent, summary), nil
	}
	if Auditing() {
		return "", guardrm.Record(home, guardrm.AuditEvent{Kind: "skipped", Command: ev.Command, Reason: why})
	}
	return "", nil
}

const codexWindowsEmbeddedDeletionReason = "Obituary's delete guard is on: Codex on Windows passes commands through another shell, where quoting may fail, so a deletion inside backticks or $(...) in this command may really execute, even inside quotes or a quoted heredoc. Text containing backticks stays inert when an editing tool writes it to a file and the command receives the file path."

// codexWindowsEmbeddedDeletion needs no shell, working directory or
// executable: whichever shell finally reads the text, the deletion can run.
func codexWindowsEmbeddedDeletion(ev Event) string {
	if ev.Host != "codex" || runtime.GOOS != "windows" || guardrm.EmbeddedDeletion(ev.Command) == "" {
		return ""
	}
	return codexWindowsEmbeddedDeletionReason
}

// deletionGuidance denies a literal deletion that has no exact equivalent:
// denying runs nothing, so it cannot select more than the command would.
func deletionGuidance(ev Event, why string, resolve func() string) string {
	if why == "" {
		return ""
	}
	deletion := guardrm.Deletion(ev.Command, ev.Shell)
	if ev.ShellDeclared {
		// A declaration may name the wrong shell: read the command as both.
		deletion = cmp.Or(deletion, guardrm.Deletion(ev.Command, guardrm.Bash), guardrm.Deletion(ev.Command, guardrm.PowerShell))
	}
	if deletion == "" {
		return ""
	}
	if strings.Contains(deletion, "docker ") {
		return "Obituary's delete guard is on: this Docker command can delete volume data. No exact equivalent applies; review the explicit volume names on the intended Docker daemon."
	}
	if _, after, ok := strings.Cut(deletion, "wsl --unregister"); ok {
		return wslUnregisterGuidance(strings.TrimSpace(after))
	}
	executable := resolve()
	// Statements, not instructions: the agent reads this as tool context.
	const discovered = " For targets selected at runtime, the safe alternative is to list and review the paths first, then give them as explicit operands to `%s rm -- <path>...`."
	reason := fmt.Sprintf("Obituary's delete guard is on: `%s` deletes permanently and no exact obit equivalent applies to this command (%s). `%s bury <path>...` provides restorable deletion for literal paths, `%s git-clean` for git clean; `obit resurrect` restores graveyard entries."+discovered, deletion, why, executable, executable, executable)
	if len(reason) > maxContext {
		reason = fmt.Sprintf("Obituary's delete guard is on: `%s` deletes permanently and no exact obit equivalent applies to this command. `obit bury <path>...` provides restorable deletion for literal paths, `obit git-clean` for git clean, through the installed obit executable."+discovered, deletion, "obit")
	}
	return reason
}

func wslUnregisterGuidance(distro string) string {
	target := "<distro>"
	if distro != "" {
		target = distro
	}
	return fmt.Sprintf("Obituary's delete guard is on: `wsl --unregister` permanently deletes the entire WSL distribution and its virtual disk (`ext4.vhdx`), which Obituary cannot bury or resurrect. No obit equivalent applies; the safe backup step before manual user confirmation is `wsl --export %s <backup.tar>`.", target)
}

func deleteGuardContext(ev Event) (guardrm.Context, string) {
	// Only a Windows executable needs WSL interop translation. A Linux binary
	// uses native paths even when the kernel or environment belongs to WSL.
	wsl := runtime.GOOS == "windows" && (strings.HasPrefix(ev.Cwd, "/") || strings.HasPrefix(ev.Cwd, "~"))
	context := guardrm.Context{Shell: ev.Shell, DeclaredShell: ev.ShellDeclared, WSL: wsl}
	context.Cwd, _ = NativePath(ev.Cwd) // read-only glob check; preserve ev.Cwd's shell namespace
	var why string
	if context.WSL && (!guardrm.WSLMountedPath(ev.Cwd) || context.Shell != guardrm.Bash) {
		why = "WSL requires bash in a mounted Windows directory"
	} else if context.Shell == guardrm.PowerShell && !windowsCwd(ev.Cwd) {
		why = "PowerShell deletion requires a Windows working directory"
	} else if context.Shell == guardrm.CMD && !windowsCwd(ev.Cwd) {
		why = "CMD deletion requires a Windows working directory"
	}
	return context, why
}

func suggestDeletion(ev Event, context guardrm.Context) (equivalent, summary, why, guidance string) {
	var unavailable string
	context.ResolveExecutable = func() string {
		executable, reason := resolveDeleteExecutable(context)
		unavailable = reason
		return executable
	}
	equivalent, summary, why = guardrm.SuggestWithSummary(ev.Command, context)
	guidance = deletionGuidance(ev, why, context.ResolveExecutable)
	if unavailable != "" {
		return ev.Command, "", unavailable, ""
	}
	return
}

var resolveDeleteExecutable = deleteGuardExecutable

func deleteGuardExecutable(context guardrm.Context) (string, string) {
	self, err := os.Executable()
	if err != nil {
		return "", "hook executable unavailable: " + err.Error()
	}
	if real, err := filepath.EvalSymlinks(self); err == nil {
		self = real
	}
	return deleteGuardPath(context, self)
}

func deleteGuardPath(context guardrm.Context, self string) (string, string) {
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
		if base == "cmd" || base == "cmd.exe" {
			if cmd, ok := codexCmdArgv(args[1:]); ok {
				return cmd, guardrm.CMD, ""
			}
			return string(encoded), shell, "unsupported CMD argv"
		}
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

func codexCmdArgv(args []string) (string, bool) {
	for i := 0; i < len(args)-1; i++ {
		switch strings.ToLower(args[i]) {
		case "/d", "/s", "/q", "/a", "/u":
		case "/c", "/k":
			if i == len(args)-2 {
				return args[i+1], true
			}
			return "", false
		default:
			return "", false
		}
	}
	return "", false
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
	case "cmd", "cmd.exe":
		return guardrm.CMD
	}
	return ""
}
