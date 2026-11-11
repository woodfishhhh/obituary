package integrate

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/woodfishhhh/obituary/internal/hook"
)

// Commands are prepared by the CLI from the installed executable. Plan functions
// select the form their host executes. POSIX can be empty when WSL is unavailable.
type Commands struct {
	Native     string
	POSIX      string
	Executable string
}

// IntegrationMethod identifies how the agent loads Obituary's integration.
type IntegrationMethod string

const (
	NativeHooks IntegrationMethod = "原生 hook"
	Plugin      IntegrationMethod = "插件"
	RulesAndMCP IntegrationMethod = "只有规则和 MCP"
)

// Agent describes one installation target. Plans only read and return Changes;
// Apply retains ownership, original-content and project-boundary checks. Optional
// extension planners are filled only for verified host formats and scopes.
type Agent struct {
	Name         string
	Method       IntegrationMethod
	Capabilities hook.Capabilities
	Plan         func(Scope, Commands, bool) ([]Change, error)
	PlanMCP      func(Scope, string, bool) ([]Change, error)
	PlanSkill    func(Scope, bool) ([]Change, error)
}

var agentRegistry = struct {
	sync.RWMutex
	byName map[string]Agent
}{byName: make(map[string]Agent)}

func RegisterAgent(agent Agent) {
	if agent.Name == "" || agent.Plan == nil {
		panic("integrate: invalid agent registration: " + agent.Name)
	}
	agentRegistry.Lock()
	defer agentRegistry.Unlock()
	if _, exists := agentRegistry.byName[agent.Name]; exists {
		panic("integrate: duplicate agent registration: " + agent.Name)
	}
	agentRegistry.byName[agent.Name] = agent
}

func LookupAgent(name string) (Agent, bool) {
	agentRegistry.RLock()
	defer agentRegistry.RUnlock()
	agent, ok := agentRegistry.byName[name]
	return agent, ok
}

func Agents() []Agent {
	agentRegistry.RLock()
	defer agentRegistry.RUnlock()
	agents := make([]Agent, 0, len(agentRegistry.byName))
	for _, agent := range agentRegistry.byName {
		agents = append(agents, agent)
	}
	slices.SortFunc(agents, func(a, b Agent) int { return strings.Compare(a.Name, b.Name) })
	return agents
}

// PlanAgent is ready for the multi-agent CLI wiring. Existing public planners
// stay available so their behavior and current callers remain unchanged.
func PlanAgent(name string, scope Scope, commands Commands, uninstall bool) ([]Change, error) {
	agent, ok := LookupAgent(name)
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", name)
	}
	return agent.Plan(scope, commands, uninstall)
}

func init() {
	for _, name := range []string{"claude", "codex"} {
		host, ok := hook.Lookup(name)
		if !ok {
			panic("integrate: missing hook adapter: " + name)
		}
		plan := func(scope Scope, commands Commands, uninstall bool) ([]Change, error) {
			if name == "claude" {
				return PlanClaudeExec(scope, commands.Executable, uninstall)
			}
			posix := commands.POSIX
			if posix == "" {
				posix = commands.Native
			}
			return PlanCodex(scope, posix, commands.Native, uninstall)
		}
		RegisterAgent(Agent{
			Name: name, Method: NativeHooks, Capabilities: host.Capabilities, Plan: plan,
			PlanMCP: func(scope Scope, executable string, uninstall bool) ([]Change, error) {
				return PlanMCP(scope, name, executable, uninstall)
			},
			PlanSkill: func(scope Scope, uninstall bool) ([]Change, error) {
				return PlanSkill(scope, name, uninstall)
			},
		})
	}
}
