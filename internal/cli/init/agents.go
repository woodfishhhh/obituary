package init

import (
	"encoding/json"
	"errors"
	"fmt"
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
	return NewAgentInspector(scope, p.Executable).Inspect(name)
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
		return status, fmt.Errorf("unknown agent %q", name)
	}
	if !installed {
		status.Installation, status.Detail = "unavailable", "安装尚未提供 (installer not available)"
		return status, nil
	}
	commands, err := in.commands(name)
	if err != nil {
		return status, err
	}
	changes, err := installer.Plan(scope, commands, false)
	var manual *integrate.ManualSteps
	if errors.As(err, &manual) {
		return in.inspectManualAgent(status, name, manual.Steps)
	}
	if err != nil {
		return status, err
	}
	if len(changes) == 0 {
		status.Installation, status.Detail = "unavailable", "安装尚未提供 (no automatic configuration target for this scope)"
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
			steps = append(steps, change.Path+": "+change.Detail)
		}
		if !change.Manual && !change.Implicit && change.Action != "preserve" {
			automatic = true
		}
	}
	if !automatic && len(steps) == 0 {
		for _, change := range changes {
			if !change.Implicit && change.Detail != "" {
				steps = append(steps, change.Path+": "+change.Detail)
			}
		}
	}
	if len(steps) > 0 {
		return in.inspectManualAgent(status, name, strings.Join(steps, "\n"))
	}
	if !automatic {
		status.Installation, status.Detail = "unavailable", "安装尚未提供 (no automatic configuration target for this scope)"
		return status, nil
	}
	status.Installation, status.Detail = "installed", "已安装 (integration is current)"
	if integrate.Pending(changes) {
		remove, err := installer.Plan(scope, commands, true)
		if err != nil {
			return status, err
		}
		status.Installation, status.Detail = "missing", "未安装 (integration is missing)"
		if integrate.Pending(remove) {
			status.Installation, status.Detail = "outdated", "需要刷新 (integration is outdated or incomplete)"
		}
	}
	return status, nil
}

func (in *AgentInspector) inspectManualAgent(status AgentStatus, name, steps string) (AgentStatus, error) {
	present, err := in.Present(name)
	if err != nil {
		return status, err
	}
	status.Installation, status.Detail = "missing", "未安装；接入需要手动步骤 (not installed; manual setup required): "+steps
	if present {
		status.Installation, status.Detail = "manual", manualStatusDetail(steps)
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
	return NewAgentInspector(scope, p.Executable).Present(name)
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
			errs = append(errs, fmt.Errorf("inspect %s removal: %w", name, err))
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

func manualStatusDetail(steps string) string {
	return "需要手动步骤 (manual steps required): " + steps + "\nNext: complete these steps in the host; manual activation cannot be verified automatically."
}

func CapabilityText(c hook.Capabilities) string {
	var parts []string
	for _, field := range []struct {
		name string
		hook.Support
	}{
		{"delete guard", c.DeleteGuard}, {"grave notice", c.GraveNotice}, {"write observation", c.WriteObservation},
		{"session start", c.SessionStart}, {"stop", c.Stop}, {"session end", c.SessionEnd},
		{"context", c.Context}, {"MCP", c.MCP}, {"skill", c.Skill},
	} {
		level := "不支持 (unsupported)"
		switch field.Level {
		case hook.Supported:
			level = "支持 (supported)"
		case hook.Limited:
			level = "有限 (limited)"
		}
		line := field.name + ": " + level
		if field.Detail != "" {
			line += "; " + field.Detail
		}
		target := field.Target
		if target == (hook.Target{}) {
			target = c.Target
		}
		if text := targetText(target); text != "" {
			line += "; " + text
		}
		if field.ActuallyVerified() {
			v := field.Verification
			line += "; 实际验证: " + targetText(v.Target) + "; " + v.Evidence
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, "\n  ")
}

func targetText(t hook.Target) string {
	var parts []string
	for _, field := range []struct{ name, value string }{
		{"宿主", t.Host}, {"发行版", t.Distribution}, {"版本", t.Version}, {"作用域", t.Scope},
	} {
		if field.value != "" {
			parts = append(parts, field.name+"="+field.value)
		}
	}
	return strings.Join(parts, ", ")
}

func (p Planner) showAgents(scope integrate.Scope, names []string, summary bool) error {
	inspector := NewAgentInspector(scope, p.Executable)
	var firstErr error
	statuses := make([]AgentStatus, 0, len(names))
	for _, name := range names {
		status, err := inspector.Inspect(name)
		if err == nil {
			var detail string
			detail, _, err = integrate.AgentWindowsShellStatus(name)
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
		fmt.Println("安装摘要 (installation summary):")
		for i, name := range names {
			fmt.Printf("  %s: %s\n", name, installationSummary(statuses[i].Installation))
		}
		fmt.Println("\n能力明细 (capability details):")
	}
	for i, name := range names {
		fmt.Printf("%s: %s\n  %s\n", name, statuses[i].Detail, CapabilityText(statuses[i].Capabilities))
	}
	return firstErr
}

func installationSummary(state string) string {
	switch state {
	case "installed":
		return "已安装，最新 (installed, current)"
	case "outdated":
		return "已安装，需要刷新 (installed, outdated or incomplete)"
	case "missing":
		return "未安装 (not installed)"
	case "manual":
		return "已有安装记录，手动激活及版本未核实 (manual setup, currency unverified)"
	case "unavailable":
		return "无自动安装支持 (installer unavailable)"
	default:
		return "无法核实安装及版本 (unknown)"
	}
}
