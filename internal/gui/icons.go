package gui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/shortcut"
)

// SetTaskbarSetter connects the native shell without exporting a Wails method.
func SetTaskbarSetter(a *API, set func(bool) error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.setTaskbar = set
}

func (a *API) PlanTrayIcon(enabled bool) (Plan, error) { return a.planIcon("trayIcon", enabled) }
func (a *API) PlanTaskbar(enabled bool) (Plan, error)  { return a.planIcon("taskbar", enabled) }

func (a *API) planIcon(action string, enabled bool) (Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg, err := a.readConfig()
	if err != nil {
		return Plan{}, err
	}
	ui := i18n.Resolve(cfg.UI.Language, a.deps.SystemLanguage)
	path := filepath.Join(a.deps.ConfigDir, "config.toml")
	old, key, planner := cfg.UI.Tray, i18n.TrayIcon, integrate.PlanTrayConfig
	detail := ui.Text(i18n.IconLive)
	if action == "taskbar" {
		old, key, planner = cfg.UI.Taskbar, i18n.TaskbarIcon, integrate.PlanTaskbarConfig
		detail = ""
		if a.setTaskbar == nil {
			detail = ui.Text(i18n.IconNextWindow)
		}
	}
	if old == enabled {
		return a.savePlan(action, []ChangePreview{}, func() error { return nil }), nil
	}
	change, err := planner(path, enabled)
	if err != nil {
		return Plan{}, err
	}
	preview := []ChangePreview{{Key: path, Target: ui.Text(key), Before: iconValue(ui, old), After: iconValue(ui, enabled) + detail}}
	view := a.savePlan(action, preview, nil)
	a.plan.applyMenu = func() (contextmenu.ApplyReceipt, error) {
		receipt := contextmenu.ApplyReceipt{Applied: []string{}, Remaining: []string{path}}
		if err := integrate.Apply([]integrate.Change{change}); err != nil {
			return receipt, err
		}
		receipt.Applied, receipt.Remaining = []string{path}, []string{}
		if action == "taskbar" && a.setTaskbar != nil {
			return receipt, a.setTaskbar(enabled)
		}
		return receipt, nil
	}
	return view, nil
}

func iconValue(ui i18n.Language, enabled bool) string {
	if enabled {
		return ui.Text(i18n.Enabled)
	}
	return ui.Text(i18n.Disabled)
}

func (a *API) PlanStartMenu(enabled bool) (Plan, error) {
	return a.planShortcut(shortcut.StartMenu, enabled)
}
func (a *API) PlanDesktopShortcut(enabled bool) (Plan, error) {
	return a.planShortcut(shortcut.Desktop, enabled)
}

func (a *API) planShortcut(place shortcut.Location, enabled bool) (Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg, err := a.readConfig()
	if err != nil {
		return Plan{}, err
	}
	ui := i18n.Resolve(cfg.UI.Language, a.deps.SystemLanguage)
	changes, err := shortcut.Plan(a.deps.Shortcuts, a.deps.Executable, place, enabled)
	if err != nil {
		return Plan{}, err
	}
	action, key := "startMenu", i18n.StartMenuShortcut
	if place == shortcut.Desktop {
		action, key = "desktopShortcut", i18n.DesktopShortcut
	}
	preview := []ChangePreview{}
	for _, change := range changes {
		if change.Action == "preserve" {
			return Plan{}, fmt.Errorf("%s: %s", ui.Text(i18n.ShortcutPreserved), change.Path)
		}
		if change.Action == "unchanged" || !strings.HasSuffix(change.Path, ".lnk") {
			continue
		}
		preview = append(preview, ChangePreview{Key: change.Path, Target: ui.Text(key), Before: iconValue(ui, !enabled), After: iconValue(ui, enabled)})
	}
	if !enabled && len(preview) == 0 {
		present, err := shortcut.Inspect(a.deps.Shortcuts, a.deps.Executable, place)
		if err != nil {
			return Plan{}, err
		}
		if present != "" {
			return Plan{}, fmt.Errorf("%s: %s", ui.Text(i18n.ShortcutPreserved), present)
		}
	}
	view := a.savePlan(action, preview, nil)
	a.plan.applyMenu = func() (contextmenu.ApplyReceipt, error) { return applyShortcutPlan(changes) }
	return view, nil
}

func applyShortcutPlan(changes []integrate.Change) (contextmenu.ApplyReceipt, error) {
	receipt := contextmenu.ApplyReceipt{Applied: []string{}, Remaining: []string{}}
	for _, change := range changes {
		if change.Action != "unchanged" {
			receipt.Remaining = append(receipt.Remaining, change.Path)
		}
	}
	if err := integrate.Apply(changes); err != nil {
		return receipt, err
	}
	receipt.Applied, receipt.Remaining = receipt.Remaining, []string{}
	return receipt, nil
}
