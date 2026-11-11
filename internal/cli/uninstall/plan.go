package uninstall

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/app"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/explorer"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/project"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

type integration struct {
	scope   string
	agents  []string
	changes []integrate.Change
}

type plan struct {
	svc          *core.Service
	state        core.State
	pending      []core.PendingGrave
	config       string
	schedule     string
	integrations []integration
	startup      *app.AutostartPlan
	menus        []contextmenu.Change
	labels       []string
}

func prepare(d dependencies) (*plan, error) {
	svc, err := app.Open()
	if err != nil {
		return nil, err
	}
	// Uninstall must never create new Explorer labels as a side effect of purge.
	svc.AfterChange = nil
	p := &plan{svc: svc}
	if p.state, err = svc.Store.Inspect(); err != nil {
		return nil, err
	}
	if p.pending, err = svc.PendingGraves(); err != nil {
		return nil, err
	}
	if p.config, err = userdirs.Config(); err != nil {
		return nil, err
	}
	exe, err := d.executable.Path()
	if err != nil {
		return nil, err
	}
	p.schedule = d.previewSchedule(filepath.Join(filepath.Dir(exe), "obitd.exe"), svc.Store.Home, p.config)
	p.startup, err = app.PlanAutostart(d.runRegistry, filepath.Join(filepath.Dir(exe), "obitd.exe"), svc.Store.Home, p.config, false)
	if err != nil {
		return nil, err
	}
	if p.menus, err = contextmenu.Plan(d.menuRegistry, exe, true); err != nil {
		return nil, err
	}
	if p.labels, err = explorer.CleanupTargets(svc.Store.Home); err != nil {
		return nil, err
	}
	p.integrations, err = integrationPlans(initcmd.Planner{Executable: d.executable})
	return p, err
}

// Route uninstall through each agent's existing planner so shared ownership
// checks and the official Claude MCP CLI remain part of the same transaction.
func integrationPlans(planner initcmd.Planner) ([]integration, error) {
	scopes := []integrate.Scope{{Global: true}}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	proj, found, err := project.Find(cwd)
	if err != nil {
		return nil, err
	}
	if found {
		scopes = append(scopes, integrate.Scope{Project: proj.Root})
	}
	var result []integration
	for _, scope := range scopes {
		entry, err := scopePlan(planner, scope)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, nil
}

func scopePlan(planner initcmd.Planner, scope integrate.Scope) (integration, error) {
	entry := integration{scope: scope.Project}
	if scope.Global {
		entry.scope = "global"
	}
	inspector := initcmd.NewAgentInspector(scope, planner.Executable)
	for _, agent := range initcmd.AgentNames() {
		present, err := inspector.Present(agent)
		if err != nil {
			return entry, err
		}
		if present {
			entry.agents = append(entry.agents, agent)
		}
	}
	var err error
	entry.changes, err = planner.IntegrationPlans(entry.agents, scope, true)
	return entry, err
}

func (p *plan) print(out io.Writer, purge bool) {
	fmt.Fprintln(out, "Uninstall plan (current data directory; global agents and current Git project):")
	for _, entry := range p.integrations {
		fmt.Fprintf(out, "  agents %s: %v (hooks / MCP / Skill)\n", entry.scope, entry.agents)
		for _, c := range entry.changes {
			if c.Action != "unchanged" {
				fmt.Fprintf(out, "    %s %s (%s)\n", c.Action, c.Path, c.Detail)
			}
		}
	}
	fmt.Fprintf(out, "  %s\n  stop resident/tray for %s\n", p.schedule, p.svc.Store.Home)
	if p.startup.Pending() {
		fmt.Fprintf(out, "  remove HKCU Run %s\n", p.startup.Name)
	}
	for _, menu := range p.menus {
		fmt.Fprintln(out, "  remove context menu", menu.Path)
	}
	for _, dir := range p.labels {
		fmt.Fprintln(out, "  remove Explorer desktop.ini labels", dir)
	}
	for _, watch := range p.state.Watches {
		fmt.Fprintf(out, "  remove folder policy #%d %s (files stay in place)\n", watch.ID, watch.Path)
	}
	p.printData(out, purge)
}

func (p *plan) apply(d dependencies, out io.Writer, purge bool) error {
	// Disable wakeups before touching policies or their labels. An unsuccessful
	// stop must block data cleanup, since a resident could recreate metadata.
	serviceErr := errors.Join(d.removeSchedule(), p.startup.Apply(), d.stop(p.svc.Store.Home))
	failures := []error{serviceErr}
	for _, entry := range p.integrations {
		failures = append(failures, initcmd.ApplyUninstall(entry.changes))
	}
	failures = append(failures, contextmenu.Apply(d.menuRegistry, p.menus))
	if serviceErr != nil {
		return errors.Join(failures...)
	}
	for _, watch := range p.state.Watches {
		failures = append(failures, p.svc.RemoveWatchID(watch.ID))
	}
	if len(p.labels) > 0 {
		failures = append(failures, explorer.Sync(p.svc, explorer.Options{}))
	}
	if err := errors.Join(failures...); err != nil {
		return err
	}
	if purge {
		return p.purgeData(out)
	}
	p.printData(out, false)
	return nil
}
