package init

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

// ApplyUninstall shares init's independent hook/MCP/Skill preflights with the
// top-level uninstaller. Manual host steps remain visible failures.
func ApplyUninstall(changes []integrate.Change) error {
	automatic := slices.DeleteFunc(slices.Clone(changes), func(c integrate.Change) bool { return c.Manual })
	err := applyIntegrationPlan(automatic, true)
	if slices.ContainsFunc(changes, func(c integrate.Change) bool { return c.Manual && !c.Implicit }) {
		err = errors.Join(err, errors.New("manual agent removal steps remain; see the uninstall plan"))
	}
	return errors.Join(err, integrate.PreservedError(changes))
}

//nolint:funlen // Existing extension planning stays unchanged; only the executable dependency is injected.
func (p Planner) planExtensions(changes []integrate.Change, scope integrate.Scope, agent string, uninstall bool) ([]integrate.Change, error) {
	installer, _ := integrate.LookupAgent(agent)
	for _, kind := range []string{"mcp", "skill"} {
		if kind == "skill" && p.SkipSkill && !uninstall {
			continue
		}
		explicit := kind == "mcp" && scope.MCP || kind == "skill" && scope.Skill
		if !explicit && !uninstall && !scope.Global {
			continue
		}
		support, available := installer.Capabilities.MCP, installer.PlanMCP != nil
		if kind == "skill" {
			support, available = installer.Capabilities.Skill, installer.PlanSkill != nil
		}
		if !available || support.Level == hook.Unsupported {
			if kind == "skill" && explicit {
				changes = append(changes, integrate.Change{Path: agent + " skill", Action: "unchanged", Manual: true, Implicit: true, Detail: "跳过：这个 agent 没有 skill 目录"})
				continue
			}
			if explicit {
				return nil, fmt.Errorf("%s --%s is unsupported: %s", agent, kind, support.Detail)
			}
			continue
		}
		executable, err := p.Executable.Path()
		if err != nil {
			return nil, err
		}
		plan := func(remove bool) ([]integrate.Change, error) {
			if kind == "mcp" {
				return installer.PlanMCP(scope, executable, remove)
			}
			return installer.PlanSkill(scope, remove)
		}
		if !explicit && !uninstall {
			remove, err := plan(true)
			var manual *integrate.ManualSteps
			if errors.As(err, &manual) {
				continue // An unrequested manual extension cannot be auto-refreshed.
			}
			if err != nil {
				changes = append(changes, integrate.Change{Path: agent + " " + kind, Extension: kind, Implicit: true, Action: "preserve", Detail: "warning: implicit refresh skipped: " + err.Error()})
				continue
			}
			if !extensionInstalled(remove, kind, agent) {
				continue
			}
		}
		extension, err := plan(uninstall)
		var manual *integrate.ManualSteps
		if errors.As(err, &manual) {
			changes = append(changes, integrate.Change{Path: agent + " " + kind, Extension: kind, Action: "preserve", Manual: true, Detail: manual.Steps})
			continue
		}
		if err != nil {
			if !uninstall && explicit {
				return nil, err
			}
			detail := err.Error()
			if !uninstall {
				detail = "warning: implicit refresh skipped: " + detail
			}
			changes = append(changes, integrate.Change{Path: agent + " " + kind, Extension: kind, Implicit: !uninstall, Action: "preserve", Detail: detail})
			continue
		}
		for _, c := range extension {
			c.Extension = kind
			c.Implicit = !explicit && !uninstall
			if c.Implicit && kind == "mcp" && agent == "claude" && scope.Global && integrate.Pending([]integrate.Change{c}) {
				c.Action = "preserve"
				c.Detail = "warning: Claude user MCP registration is outdated; implicit refresh skipped; manual replacement is not atomic"
			}
			if uninstall && strings.Contains(c.Detail, "preserved") {
				c.Action = "preserve"
			}
			changes = append(changes, c)
		}
	}
	return changes, nil
}

func extensionInstalled(remove []integrate.Change, kind, agent string) bool {
	installed := integrate.Pending(remove)
	if kind == "skill" && (agent == "claude" || agent == "codex") {
		installed = remove[0].Action == "remove"
	}
	return installed
}

func applyIntegrationPlan(changes []integrate.Change, uninstall bool) error {
	var backups []string
	defer func() { expireBackups(backups) }()
	apply := func(batch []integrate.Change) error {
		written, err := integrate.ApplyWithBackups(batch)
		backups = append(backups, written...)
		return err
	}
	if !uninstall {
		return applyInstallPlan(changes, apply)
	}
	var errs []error
	// Hook removal and each optional extension have independent preflights.
	for _, group := range []string{"", "mcp", "skill"} {
		var batch []integrate.Change
		for _, c := range changes {
			if c.Extension == group {
				batch = append(batch, c)
			}
		}
		if err := apply(batch); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func applyInstallPlan(changes []integrate.Change, apply func([]integrate.Change) error) error {
	var required []integrate.Change
	for _, c := range changes {
		if !c.Implicit {
			required = append(required, c)
		}
	}
	if err := apply(required); err != nil {
		return err
	}
	for _, kind := range []string{"mcp", "skill"} {
		var optional []integrate.Change
		for _, c := range changes {
			if c.Implicit && c.Extension == kind {
				optional = append(optional, c)
			}
		}
		if err := apply(optional); err != nil {
			fmt.Printf("warning: implicit %s refresh failed: %v\n", kind, err)
		}
	}
	return nil
}

// expireBackups gives the backups obit init wrote beside the files it changed
// the lifespan of Obituary's own backups, so they do not pile up in an agent's
// configuration folder; the graveyard still holds them for a while after.
func expireBackups(paths []string) {
	if len(paths) == 0 {
		return
	}
	svc, err := app.Open()
	if err == nil {
		err = app.ExpireBackups(svc, paths)
	}
	if err != nil {
		fmt.Printf("warning: backups kept without a lifespan: %v\n", err)
	}
}

func (p Planner) showExtensions(scope integrate.Scope, agent string) error {
	executable, err := p.Executable.Path()
	if err != nil {
		return err
	}
	for _, kind := range []string{"mcp", "skill"} {
		if kind == "skill" {
			installer, _ := integrate.LookupAgent(agent)
			if installer.PlanSkill == nil {
				continue
			}
		}
		status, err := integrate.InspectExtension(scope, agent, kind, executable)
		if err != nil {
			fmt.Printf("%s: %v\n", kind, err)
			continue
		}
		fmt.Printf("%s: %s\n", kind, status.Detail)
	}
	return nil
}
