package init

import (
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

// AgentInspector owns the snapshot for one diagnostic invocation. Executable
// resolution is shared across agents, just like the read-only file snapshot.
type AgentInspector struct {
	scope      integrate.Scope
	executable cmdutil.Executable
	prefixes   *integrate.Commands
	commandErr error
}

func NewAgentInspector(scope integrate.Scope, executable cmdutil.Executable) *AgentInspector {
	return &AgentInspector{scope: scope.ForInspection(), executable: executable}
}

func (in *AgentInspector) commands(agent string) (integrate.Commands, error) {
	if in.prefixes == nil && in.commandErr == nil {
		executable, err := in.executable.Path()
		in.commandErr = err
		if err == nil {
			in.prefixes = &integrate.Commands{
				Native: in.executable.HookCommand(""), POSIX: in.executable.WSLHookCommand(""), Executable: executable,
			}
		}
	}
	if in.commandErr != nil {
		return integrate.Commands{}, in.commandErr
	}
	commands := *in.prefixes
	commands.Native += agent
	if commands.POSIX != "" {
		commands.POSIX += agent
	}
	return commands, nil
}
