package init

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

// AgentNames includes protocols without installers and installation-only targets.
func AgentNames() []string {
	var names []string
	for _, agent := range hook.Agents() {
		names = append(names, agent.Name)
	}
	for _, agent := range integrate.Agents() {
		if !slices.Contains(names, agent.Name) {
			names = append(names, agent.Name)
		}
	}
	slices.Sort(names)
	return names
}

type AgentStatus struct {
	Capabilities hook.Capabilities
	Installation string
	Detail       string
}

// InspectAgent uses the same read-only plans as installation. Protocol coverage
// is independent of whether an installer has been implemented or applied.
func InspectAgent(name string, scope integrate.Scope) (AgentStatus, error) {
	return (Planner{}).InspectAgent(name, scope)
}

func (p Planner) InspectAgent(name string, scope integrate.Scope) (AgentStatus, error) {
	return NewAgentInspector(scope, p.Executable, cmdutil.Localizer()).Inspect(name)
}

// Inspect reports one agent using this invocation's snapshot.
//
//nolint:funlen // Existing status decision tree; only its input snapshot is shared.
func (in *AgentInspector) Inspect(name string) (AgentStatus, error) {
	scope := in.scope
	var status AgentStatus
	installer, installed := integrate.LookupAgent(name)
	if installed {
		status.Capabilities = installer.Capabilities
	}
	if host, ok := hook.Lookup(name); ok && !installed {
		status.Capabilities = host.Capabilities
	} else if !installed {
		return status, fmt.Errorf(cmdutil.Text("cli.init.unknown_agent"), name)
	}
	if !installed {
		status.Installation, status.Detail = "unavailable", in.language.Text("cli.init.installer_not_available", nil)
		return status, nil
	}
	commands, err := in.commands(name)
	if err != nil {
		return status, err
	}
	changes, err := installer.Plan(scope, commands, false)
	var manual *integrate.ManualSteps
	if errors.As(err, &manual) {
		return in.inspectManualAgent(status, name, manual.DisplaySteps(in.language))
	}
	if err != nil {
		return status, err
	}
	if len(changes) == 0 {
		status.Installation, status.Detail = "unavailable", in.language.Text("cli.init.scope_unavailable", nil)
		return status, nil
	}
	if scope.MCP || scope.Skill {
		changes, err = (Planner{Executable: in.executable}).planExtensions(changes, scope, name, false)
		if err != nil {
			return status, err
		}
	}
	var steps []string
	automatic := false
	for _, change := range changes {
		if change.Manual && !change.Implicit {
			steps = append(steps, change.Path+": "+change.DisplayDetail(in.language))
		}
		if !change.Manual && !change.Implicit && change.Action != "preserve" {
			automatic = true
		}
	}
	if !automatic && len(steps) == 0 {
		for _, change := range changes {
			if !change.Implicit && change.Detail != "" {
				steps = append(steps, change.Path+": "+change.DisplayDetail(in.language))
			}
		}
	}
	if len(steps) > 0 {
		return in.inspectManualAgent(status, name, strings.Join(steps, "\n"))
	}
	if !automatic {
		status.Installation, status.Detail = "unavailable", in.language.Text("cli.init.scope_unavailable", nil)
		return status, nil
	}
	status.Installation, status.Detail = "installed", in.language.Text("cli.init.integration_current", nil)
	if integrate.Pending(changes) {
		remove, err := installer.Plan(scope, commands, true)
		if err != nil {
			return status, err
		}
		status.Installation, status.Detail = "missing", in.language.Text("cli.init.integration_missing", nil)
		if integrate.Pending(remove) {
			status.Installation, status.Detail = "outdated", in.language.Text("cli.init.integration_outdated", nil)
		}
	}
	return status, nil
}

func (in *AgentInspector) inspectManualAgent(status AgentStatus, name, steps string) (AgentStatus, error) {
	present, err := in.Present(name)
	if err != nil {
		return status, err
	}
	status.Installation, status.Detail = "missing", in.language.Text("cli.init.manual_missing", nil)+steps
	if present {
		status.Installation, status.Detail = "manual", manualStatusDetail(in.language, steps)
	}
	return status, nil
}

// AgentInstallationPresent reuses removal plans to recognize owned entries and
// markers. Manual loaders retain files on uninstall, so their install plans also
// supply receipt paths. Only those exact paths are read; no directories are scanned.
func AgentInstallationPresent(name string, scope integrate.Scope) (bool, error) {
	return (Planner{}).AgentInstallationPresent(name, scope)
}

func (p Planner) AgentInstallationPresent(name string, scope integrate.Scope) (bool, error) {
	return NewAgentInspector(scope, p.Executable, cmdutil.Localizer()).Present(name)
}

// Present recognizes owned entries using this invocation's snapshot.
func (in *AgentInspector) Present(name string) (bool, error) {
	scope := in.scope
	installer, ok := integrate.LookupAgent(name)
	if !ok {
		return false, nil
	}
	commands, err := in.commands(name)
	if err != nil {
		return false, err
	}
	plans := []func(bool) ([]integrate.Change, error){
		func(remove bool) ([]integrate.Change, error) { return installer.Plan(scope, commands, remove) },
	}
	if installer.PlanMCP != nil && installer.Capabilities.MCP.Level != hook.Unsupported {
		plans = append(plans, func(remove bool) ([]integrate.Change, error) {
			return installer.PlanMCP(scope, commands.Executable, remove)
		})
	}
	if installer.PlanSkill != nil && installer.Capabilities.Skill.Level != hook.Unsupported {
		plans = append(plans, func(remove bool) ([]integrate.Change, error) { return installer.PlanSkill(scope, remove) })
	}
	var errs []error
	present := false
	for _, plan := range plans {
		remove, err := plan(true)
		var manual *integrate.ManualSteps
		if err != nil && !errors.As(err, &manual) {
			errs = append(errs, fmt.Errorf(cmdutil.Text("cli.init.inspect_removal"), name, err))
		}
		for _, change := range remove {
			if !change.Manual && integrate.Pending([]integrate.Change{change}) {
				present = true
			}
		}
		if found, err := agentReceiptsPresent(name, remove); found {
			present = true
		} else if err != nil {
			errs = append(errs, err)
		}
		install, err := plan(false)
		// Install can reject a foreign file even though removal safely leaves
		// it untouched. Only removal/inspection errors prevent uninstall.
		if err != nil {
			continue
		}
		if found, err := agentReceiptsPresent(name, install); found {
			present = true
		} else if err != nil {
			errs = append(errs, err)
		}
	}
	return present, errors.Join(errs...)
}

func agentReceiptsPresent(name string, changes []integrate.Change) (bool, error) {
	for _, change := range changes {
		if change.Manual || !filepath.IsAbs(change.Path) || !strings.HasSuffix(change.Path, ".obituary-asset") {
			continue
		}
		data, err := os.ReadFile(change.Path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		// Discovery only: the installer still validates schema, identity and hashes.
		var receipt struct {
			Owners []string `json:"owners"`
		}
		if json.Unmarshal(data, &receipt) == nil && slices.Contains(receipt.Owners, name) {
			return true, nil
		}
	}
	return false, nil
}

func manualStatusDetail(language *i18n.Localizer, steps string) string {
	return language.Text("cli.init.manual_steps", nil) + steps + language.Text("cli.init.manual_activate", nil)
}

func CapabilityText(c hook.Capabilities) string {
	language := cmdutil.Localizer()
	var parts []string
	for _, field := range []struct {
		name string
		hook.Support
	}{
		{"delete guard", c.DeleteGuard}, {"grave notice", c.GraveNotice}, {"write observation", c.WriteObservation},
		{"session start", c.SessionStart}, {"stop", c.Stop}, {"session end", c.SessionEnd},
		{"context", c.Context}, {"MCP", c.MCP}, {"skill", c.Skill},
	} {
		level := language.Text("cli.init.unsupported", nil)
		switch field.Level {
		case hook.Supported:
			level = language.Text("cli.init.supported", nil)
		case hook.Limited:
			level = language.Text("cli.init.limited", nil)
		}
		line := language.HostText(field.name) + ": " + level
		if field.Detail != "" {
			line += "; " + language.HostText(field.Detail)
		}
		target := field.Target
		if target == (hook.Target{}) {
			target = c.Target
		}
		if text := targetTextFor(language, target); text != "" {
			line += "; " + text
		}
		if field.ActuallyVerified() {
			v := field.Verification
			line += language.Text("cli.init.verified_target", nil) + targetTextFor(language, v.Target) + "; " + language.HostText(v.Evidence)
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, "\n  ")
}

var targetKeys = map[string]string{"宿主": "cli.target.host", "发行版": "cli.target.distribution", "版本": "cli.target.version", "作用域": "cli.target.scope"}

func targetText(t hook.Target) string { return targetTextFor(nil, t) }
func targetTextFor(l *i18n.Localizer, t hook.Target) string {
	var parts []string
	for _, field := range []struct{ name, value string }{
		{"宿主", t.Host}, {"发行版", t.Distribution}, {"版本", t.Version}, {"作用域", t.Scope},
	} {
		if field.value != "" {
			name := field.name
			if l != nil {
				name = l.Text(targetKeys[name], nil)
			}
			parts = append(parts, name+"="+field.value)
		}
	}
	return strings.Join(parts, ", ")
}

func (p Planner) showAgents(scope integrate.Scope, names []string, summary bool) error {
	inspector := NewAgentInspector(scope, p.Executable, cmdutil.Localizer())
	var firstErr error
	statuses := make([]AgentStatus, 0, len(names))
	for _, name := range names {
		status, err := inspector.Inspect(name)
		if err == nil {
			var detail string
			detail, _, err = integrate.AgentWindowsShellStatus(inspector.language, name)
			if detail != "" {
				status.Detail += "; " + detail
			}
		}
		if err != nil {
			status.Installation = "unknown"
			status.Detail = err.Error()
			if firstErr == nil {
				firstErr = err
			}
		}
		statuses = append(statuses, status)
	}
	if summary {
		fmt.Println(cmdutil.Text("cli.init.installation_summary"))
		for i, name := range names {
			fmt.Printf("  %s: %s\n", name, installationSummary(statuses[i].Installation))
		}
		fmt.Println(cmdutil.Text("cli.init.capability_details"))
	}
	for i, name := range names {
		fmt.Printf("%s: %s\n  %s\n", name, statuses[i].Detail, CapabilityText(statuses[i].Capabilities))
		p.printSkillTarget(scope, name)
	}
	return firstErr
}

func installationSummary(state string) string {
	switch state {
	case "installed":
		return cmdutil.Text("cli.init.installed_current")
	case "outdated":
		return cmdutil.Text("cli.init.installed_outdated_or_incomplete")
	case "missing":
		return cmdutil.Text("cli.init.not_installed")
	case "manual":
		return cmdutil.Text("cli.init.manual_setup_currency_unverified")
	case "unavailable":
		return cmdutil.Text("cli.init.installer_unavailable")
	default:
		return cmdutil.Text("cli.init.unknown")
	}
}
