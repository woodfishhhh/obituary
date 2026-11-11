package gui

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

func (a *API) PlanContextMenu(enabled bool) (Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg, err := a.readConfig()
	if err != nil {
		cfg = config.Default()
	}
	language := i18n.Resolve(cfg.UI.Language, a.deps.SystemLanguage)
	changes, preview, err := a.contextMenuPlan(enabled, language, language)
	if err != nil {
		return Plan{}, err
	}
	view := a.savePlan("contextMenu", preview, nil)
	a.plan.applyMenu = func() (contextmenu.ApplyReceipt, error) {
		return contextmenu.ApplyWithReceipt(a.deps.ContextMenu, changes)
	}
	return view, nil
}

// Both the context-menu switch and language changes use this same planner.
// labels is the language the menu will use; ui is the window's language, in
// which the preview describes each change.
func (a *API) contextMenuPlan(enabled bool, labels, ui i18n.Language) ([]contextmenu.Change, []ChangePreview, error) {
	if enabled {
		if _, err := os.Stat(a.deps.Executable); err != nil {
			return nil, nil, err
		}
		if _, err := os.Stat(a.deps.Daemon); err != nil {
			return nil, nil, err
		}
	}
	changes, err := contextmenu.PlanLanguage(a.deps.ContextMenu, a.deps.Executable, !enabled, labels)
	if err != nil {
		return nil, nil, err
	}
	preview := make([]ChangePreview, 0, len(changes))
	for _, change := range changes {
		preview = append(preview, menuPreview(change, ui))
	}
	return changes, preview, nil
}

// menuPreview says where a registry change shows up in Explorer and which
// entries it adds or removes; Key keeps the registry path for receipts.
func menuPreview(change contextmenu.Change, ui i18n.Language) ChangePreview {
	place := change.Path
	switch {
	case strings.HasPrefix(change.Path, `*\shell\`):
		place = ui.Text(i18n.MenuFiles)
	case strings.HasPrefix(change.Path, `Directory\Background\`):
		place = ui.Text(i18n.MenuBackground)
	case strings.HasPrefix(change.Path, `Directory\shell\`):
		place = ui.Text(i18n.MenuFolders)
	case strings.HasPrefix(change.Path, `AppUserModelId\`):
		place = ui.Text(i18n.MenuNotifyName)
	case change.Path == contextmenu.AppID:
		place = fmt.Sprintf(ui.Text(i18n.MenuSubmenu), ui.Text(i18n.Mark))
	}
	before, after := menuLabels(change.Before, ui), menuLabels(change.After, ui)
	if change.Before != nil && change.After != nil && before == after {
		after += ui.Text(i18n.MenuProgramMoved)
	}
	return ChangePreview{Key: change.Path, Target: place, Before: before, After: after}
}

// menuLabels lists the entry names a key shows, in menu order.
func menuLabels(tree contextmenu.Tree, ui i18n.Language) string {
	var names []string
	for _, key := range slices.Sorted(maps.Keys(tree)) {
		name := tree[key]["MUIVerb"]
		if name == "" {
			name = tree[key]["DisplayName"]
		}
		if name != "" {
			names = append(names, name)
		}
	}
	separator := "、"
	if ui == i18n.English {
		separator = ", "
	}
	return strings.Join(names, separator)
}

func (a *API) PlanAutostart(enabled bool) (Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if enabled {
		if _, err := os.Stat(a.deps.Daemon); err != nil {
			return Plan{}, err
		}
	}
	plan, err := app.PlanAutostart(a.deps.RunKey, a.deps.Daemon, a.deps.Service.Store.Home, a.deps.ConfigDir, enabled)
	if err != nil {
		return Plan{}, err
	}
	preview := []ChangePreview{{Target: plan.Name, Before: plan.Before, After: plan.After}}
	return a.savePlan("autostart", preview, plan.Apply), nil
}

func (a *API) PlanPause(paused bool) (Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	home := a.deps.Service.Store.Home
	before, err := app.Paused(home)
	if err != nil {
		return Plan{}, err
	}
	preview := []ChangePreview{{Target: "trayPaused", Before: fmt.Sprint(before), After: fmt.Sprint(paused)}}
	return a.savePlan("pause", preview, func() error {
		current, err := app.Paused(home)
		if err != nil {
			return err
		}
		if current != before {
			return fmt.Errorf("pause state changed since planning")
		}
		return app.SetPaused(home, paused)
	}), nil
}
