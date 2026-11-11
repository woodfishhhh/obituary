package userdirs

import (
	"fmt"
	"sync"
)

// AgentDirectory is resolved on every call so environment overrides keep working
// in tests and installations. Resolve belongs in this package and must validate
// all external path inputs through directoryAbs. It only resolves; it never writes.
type AgentDirectory struct {
	Name    string
	Resolve func() (string, error)
}

var agentDirectories = struct {
	sync.RWMutex
	byName map[string]AgentDirectory
}{byName: make(map[string]AgentDirectory)}

func RegisterAgentDirectory(dir AgentDirectory) {
	if dir.Name == "" || dir.Resolve == nil {
		panic("userdirs: invalid agent directory: " + dir.Name)
	}
	agentDirectories.Lock()
	defer agentDirectories.Unlock()
	if _, exists := agentDirectories.byName[dir.Name]; exists {
		panic("userdirs: duplicate agent directory: " + dir.Name)
	}
	agentDirectories.byName[dir.Name] = dir
}

// AgentDir is the common installer entry point for an agent's user configuration
// root. Additional locations such as shared skills keep their own named getters.
func AgentDir(name string) (string, error) {
	agentDirectories.RLock()
	dir, ok := agentDirectories.byName[name]
	agentDirectories.RUnlock()
	if !ok {
		return "", fmt.Errorf("unknown agent directory %q", name)
	}
	return dir.Resolve()
}

// ClaudeDir is Claude Code's configuration directory: $CLAUDE_CONFIG_DIR, or ~/.claude.
func ClaudeDir() (string, error) { return agentDir("CLAUDE_CONFIG_DIR", ".claude") }

// CodexDir is Codex's configuration directory: $CODEX_HOME, or ~/.codex.
func CodexDir() (string, error) { return agentDir("CODEX_HOME", ".codex") }

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "claude", Resolve: ClaudeDir})
	RegisterAgentDirectory(AgentDirectory{Name: "codex", Resolve: CodexDir})
}
