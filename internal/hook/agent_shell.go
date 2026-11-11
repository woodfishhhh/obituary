package hook

import (
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

// Like Codex's original declaration path, keep Decode free of I/O. Only an
// adapter's ambiguous local command opts in; explicit shells and skips survive.
func configureAgentGuard(agent string, ev *Event) error {
	if ev.NoticeOnly || ev.Name != "PreToolUse" || ev.Shell != guardrm.Uncertain || ev.SkipReason != "" {
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// CLI/IDE share hook codebuddy; execute_command is the native IDE tool.
	if agent == "codebuddy" && ev.Tool == "execute_command" {
		agent = "codebuddy-ide"
	}
	if shell := cfg.WindowsShell(agent); shell != "" {
		ev.Shell = guardrm.Shell(shell)
		ev.ShellDeclared = true
	}
	return nil
}

// CodeBuddy and MiniMax's documented envelopes omit the actual execution
// backend. These optional hints only veto a declaration; they never prove local
// execution. Unknown explicit backend/environment metadata is left untouched.
type declaredShellInput struct {
	claudeInput
	Local            *bool  `json:"local"`
	Remote           bool   `json:"remote"`
	Backend          string `json:"backend"`
	ExecutionBackend string `json:"execution_backend"`
	EnvironmentID    string `json:"environment_id"`
}

func (in declaredShellInput) declareShell(ev *Event) {
	ev.Command = firstString(in.ToolInput, "command")
	// Never run NativePath here: /mnt/c on an SSH/cloud host is not this C:.
	if !windowsCwd(ev.Cwd) {
		ev.SkipReason = "shell declaration requires a native Windows working directory"
		return
	}
	if in.Remote || in.Local != nil && !*in.Local || in.Backend != "" || in.ExecutionBackend != "" || in.EnvironmentID != "" {
		ev.SkipReason = "execution backend cannot be verified as the declared local runtime"
		return
	}
	for _, key := range []string{"backend", "execution_backend", "environment_id", "cwd", "workdir"} {
		if value := in.ToolInput[key]; value != nil && value != "" {
			ev.SkipReason = "tool execution override is outside the declared local runtime"
			return
		}
	}
	if local, ok := in.ToolInput["local"]; ok && local != true {
		ev.SkipReason = "tool execution is not local"
		return
	}
	if remote, ok := in.ToolInput["remote"]; ok && remote != false {
		ev.SkipReason = "tool execution may be remote"
		return
	}
	ev.Shell = guardrm.Uncertain
}
