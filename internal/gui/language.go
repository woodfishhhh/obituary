package gui

import (
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

// PlanLanguage includes the installed Explorer labels in the same confirmation.
// The existing config and registry writers each recheck their own snapshot.
func (a *API) PlanLanguage(language string) (Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	path := filepath.Join(a.deps.ConfigDir, "config.toml")
	change, err := integrate.PlanLanguageConfig(path, language)
	if err != nil {
		return Plan{}, err
	}
	old, err := a.readConfig()
	if err != nil {
		return Plan{}, err
	}
	effective := i18n.Resolve(language, a.deps.SystemLanguage)
	menu, err := contextmenu.InspectLanguage(a.deps.ContextMenu, a.deps.Executable, effective)
	if err != nil {
		return Plan{}, err
	}
	preview := []ChangePreview{{Key: path, Target: path, Before: fmt.Sprintf("language=%s", old.UI.Language), After: fmt.Sprintf("language=%s effectiveLanguage=%s", language, effective)}}
	var changes []contextmenu.Change
	if menu.Installed {
		var labels []ChangePreview
		changes, labels, err = a.contextMenuPlan(true, effective, i18n.Resolve(old.UI.Language, a.deps.SystemLanguage))
		if err != nil {
			return Plan{}, err
		}
		preview = append(preview, labels...)
	}
	view := a.savePlan("language", preview, nil)
	a.plan.applyMenu = func() (contextmenu.ApplyReceipt, error) {
		return a.applyLanguage(change, changes)
	}
	return view, nil
}

// Config and registry are separate resources: retain the committed config path
// if a menu write fails, so Wails can show the remaining work without losing it
// to a rejected promise. Replanning also repairs a partially updated menu.
func (a *API) applyLanguage(change integrate.Change, changes []contextmenu.Change) (contextmenu.ApplyReceipt, error) {
	receipt := contextmenu.ApplyReceipt{Applied: []string{}, Remaining: []string{change.Path}}
	for _, c := range changes {
		receipt.Remaining = append(receipt.Remaining, c.Path)
	}
	if err := integrate.Apply([]integrate.Change{change}); err != nil {
		return receipt, err
	}
	receipt.Remaining = receipt.Remaining[1:]
	if change.Action != "unchanged" {
		receipt.Applied = append(receipt.Applied, change.Path)
	}
	if len(changes) == 0 {
		return receipt, nil
	}
	menu, err := contextmenu.ApplyWithReceipt(a.deps.ContextMenu, changes)
	receipt.Applied = append(receipt.Applied, menu.Applied...)
	receipt.Remaining = menu.Remaining
	return receipt, err
}
