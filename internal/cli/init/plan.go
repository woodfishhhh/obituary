package init

import (
	"errors"
	"fmt"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

// Planner binds executable discovery to one installation or inspection.
type Planner struct {
	Executable cmdutil.Executable
	SkipSkill  bool
}

func IntegrationPlan(agent string, scope integrate.Scope, uninstall bool) ([]integrate.Change, error) {
	return (Planner{}).IntegrationPlan(agent, scope, uninstall)
}

func IntegrationPlans(agents []string, scope integrate.Scope, uninstall bool) ([]integrate.Change, error) {
	return (Planner{}).IntegrationPlans(agents, scope, uninstall)
}

func (p Planner) IntegrationPlan(agent string, scope integrate.Scope, uninstall bool) ([]integrate.Change, error) {
	return p.IntegrationPlans([]string{agent}, scope, uninstall)
}

// All plans observe the same originals. Compose before Apply so hooks and MCP
// never overwrite each other when the host stores both in one configuration.
func (p Planner) IntegrationPlans(agents []string, scope integrate.Scope, uninstall bool) ([]integrate.Change, error) {
	var changes []integrate.Change
	for _, agent := range agents {
		plan, err := p.integrationPlan(agent, scope, uninstall)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", agent, err)
		}
		changes = append(changes, plan...)
	}
	return integrate.ComposeAgentChanges(changes)
}

func (p Planner) integrationPlan(agent string, scope integrate.Scope, uninstall bool) ([]integrate.Change, error) {
	installer, ok := integrate.LookupAgent(agent)
	if !ok {
		return nil, fmt.Errorf(cmdutil.Text("cli.init.agent_has_no_installer"), agent)
	}
	if scope.GuardRM && !uninstall && installer.Capabilities.DeleteGuard.Level == hook.Unsupported {
		return nil, fmt.Errorf(cmdutil.Text("cli.init.guard_rm_is_unsupported"), cmdutil.Localizer().HostText(installer.Capabilities.DeleteGuard.Detail))
	}
	executable, err := p.Executable.Path()
	if err != nil {
		return nil, err
	}
	commands := integrate.Commands{Native: p.Executable.HookCommand(agent), POSIX: p.Executable.WSLHookCommand(agent), Executable: executable}
	changes, err := installer.Plan(scope, commands, uninstall)
	var manual *integrate.ManualSteps
	if errors.As(err, &manual) {
		changes = []integrate.Change{{Path: agent, Action: "preserve", Manual: true, Detail: manual.Steps}}
	} else if err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" && scope.GuardRM && !uninstall && slices.Contains([]string{"codex", "codebuddy", "codebuddy-ide", "minimax-code"}, agent) {
		declaration, err := integrate.PlanAgentWindowsShell(scope, agent)
		if err != nil {
			return nil, err
		}
		changes = append(changes, declaration)
	}
	return p.planExtensions(changes, scope, agent, uninstall)
}

func DeleteGuardStatus(language *i18n.Localizer, registered, covered bool) (string, bool) {
	registration, coverage := language.Text("cli.init.guard_opt_in", nil), language.Text("cli.init.guard_none", nil)
	if registered {
		registration, coverage = language.Text("cli.init.guard_registered", nil), language.Text("cli.init.guard_incomplete", nil)
	}
	if covered {
		coverage = language.Text("cli.init.guard_complete", nil)
	}
	var disabledBy []string
	if os.Getenv("OBIT_GUARD_RM") == "0" {
		disabledBy = append(disabledBy, "OBIT_GUARD_RM=0")
	}
	if hook.Disabled() {
		disabledBy = append(disabledBy, "OBIT_DISABLED")
	}
	environment := language.Text("cli.init.guard_active", nil)
	if len(disabledBy) > 0 {
		environment = language.Text("cli.init.guard_disabled_by", nil) + strings.Join(disabledBy, ", ")
	}
	return fmt.Sprintf(language.Text("cli.init.guard_status", nil), registration, coverage, environment),
		registered && (!covered || len(disabledBy) > 0)
}
