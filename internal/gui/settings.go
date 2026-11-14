package gui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/shortcut"
)

type Settings struct {
	TrayIcon             bool          `json:"trayIcon"`
	Taskbar              bool          `json:"taskbar"`
	StartMenu            bool          `json:"startMenu"`
	DesktopShortcut      bool          `json:"desktopShortcut"`
	Mode                 string        `json:"mode"`
	RetentionDays        int           `json:"retentionDays"`
	ConfigPath           string        `json:"configPath"`
	ContextMenuInstalled bool          `json:"contextMenuInstalled"`
	ContextMenuCurrent   bool          `json:"contextMenuCurrent"`
	ContextMenuDetail    string        `json:"contextMenuDetail"`
	Autostart            bool          `json:"autostart"`
	Paused               bool          `json:"paused"`
	Language             string        `json:"language"`
	EffectiveLanguage    i18n.Language `json:"effectiveLanguage"`
	SessionSummary       bool          `json:"sessionSummary"`
	SessionCleanup       bool          `json:"sessionCleanup"`
	ProposalMode         string        `json:"proposalMode"` // [proposals] mode: automatic registration
	// Lifetime counts from the ledger; nil when it cannot be read.
	Lifetime *Lifetime `json:"lifetime,omitempty"`
	// Downloads is the user's Downloads folder, offered for folder policies.
	Downloads string `json:"downloads,omitempty"`
}

// Lifetime is what Obituary has cleared so far: everything it buried, and how
// much of that was purged (space freed) or resurrected.
type Lifetime struct {
	Buried      int64 `json:"buried"`
	BuriedBytes int64 `json:"buriedBytes"`
	PurgedBytes int64 `json:"purgedBytes"`
	Resurrected int64 `json:"resurrected"`
}

func (a *API) readConfig() (config.Config, error) {
	path := filepath.Join(a.deps.ConfigDir, "config.toml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return config.Default(), nil
	}
	if err != nil {
		return config.Config{}, err
	}
	return config.Parse(data, path)
}

func (a *API) Settings() (Settings, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg, err := a.readConfig()
	if err != nil {
		return Settings{}, err
	}
	language := i18n.Resolve(cfg.UI.Language, a.deps.SystemLanguage)
	menu, err := contextmenu.InspectLanguage(a.deps.ContextMenu, a.deps.Executable, language)
	if err != nil {
		return Settings{}, err
	}
	startup, err := app.PlanAutostart(a.deps.RunKey, a.deps.Daemon, a.deps.Service.Store.Home, a.deps.ConfigDir, false)
	if err != nil {
		return Settings{}, err
	}
	var lifetime *Lifetime
	if st, err := a.deps.Service.Store.Read(); err == nil {
		lifetime = &Lifetime{Buried: st.Stats.Buried, BuriedBytes: st.Stats.BuriedBytes, PurgedBytes: st.Stats.PurgedBytes, Resurrected: st.Stats.Resurrected}
	}
	start, err := shortcut.Inspect(a.deps.Shortcuts, a.deps.Executable, shortcut.StartMenu)
	if err != nil {
		return Settings{}, err
	}
	desktop, err := shortcut.Inspect(a.deps.Shortcuts, a.deps.Executable, shortcut.Desktop)
	if err != nil {
		return Settings{}, err
	}
	paused, err := app.Paused(a.deps.Service.Store.Home)
	return Settings{TrayIcon: cfg.UI.Tray, Taskbar: cfg.UI.Taskbar, StartMenu: start != "", DesktopShortcut: desktop != "", Lifetime: lifetime, Downloads: downloadsFolder(), Mode: cfg.Graveyard.Mode, RetentionDays: cfg.Graveyard.RetentionDays, Language: cfg.UI.Language, EffectiveLanguage: language,
		ConfigPath: filepath.Join(a.deps.ConfigDir, "config.toml"), ContextMenuInstalled: menu.Installed,
		ContextMenuCurrent: menu.Current, ContextMenuDetail: menu.Detail, Autostart: startup.Before != "", Paused: paused,
		SessionSummary: cfg.Hooks.SessionSummary, SessionCleanup: cfg.Hooks.SessionCleanup, ProposalMode: cfg.Proposals.Mode}, err
}

type ChangePreview struct {
	// Key names the changed resource in receipts (a registry path); Target
	// describes it for people.
	Key    string `json:"key,omitempty"`
	Target string `json:"target"`
	Before string `json:"before"`
	After  string `json:"after"`
}
type Plan struct {
	ID      string          `json:"id"`
	Action  string          `json:"action"`
	Changes []ChangePreview `json:"changes"`
}
type pendingPlan struct {
	view      Plan
	apply     func() error
	applyMenu func() (contextmenu.ApplyReceipt, error)
}

type PlanReceipt struct {
	Receipt
	Applied   []string `json:"applied"`
	Remaining []string `json:"remaining"`
	Error     string   `json:"error,omitempty"`
}

func (a *API) savePlan(action string, changes []ChangePreview, apply func() error) Plan {
	a.nextPlan++
	view := Plan{ID: strconv.FormatUint(a.nextPlan, 10), Action: action, Changes: list(changes)}
	a.plan = &pendingPlan{view: view, apply: apply}
	return view
}

func (a *API) PlanSettings(mode string, retentionDays int) (Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	path := filepath.Join(a.deps.ConfigDir, "config.toml")
	change, err := integrate.PlanGraveyardConfig(path, mode, retentionDays)
	if err != nil {
		return Plan{}, err
	}
	old, err := a.readConfig()
	if err != nil {
		return Plan{}, err
	}
	preview := []ChangePreview{{Target: path, Before: fmt.Sprintf("mode=%s retention_days=%d", old.Graveyard.Mode, old.Graveyard.RetentionDays), After: fmt.Sprintf("mode=%s retention_days=%d", mode, retentionDays)}}
	return a.savePlan("settings", preview, func() error {
		if err := integrate.Apply([]integrate.Change{change}); err != nil {
			return err
		}
		cfg, err := a.readConfig()
		if err == nil {
			a.deps.Service.Mode, a.deps.Service.Retention = cfg.Graveyard.Mode, cfg.Graveyard.Retention()
		}
		return err
	}), nil
}

// PlanSessionSummary previews [hooks] session_summary. Hooks read the config
// on every event, so the next session start follows the new value.
func (a *API) PlanSessionSummary(enabled bool) (Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	path := filepath.Join(a.deps.ConfigDir, "config.toml")
	old, err := a.readConfig()
	if err != nil {
		return Plan{}, err
	}
	if old.Hooks.SessionSummary == enabled {
		return a.savePlan("sessionSummary", []ChangePreview{}, func() error { return nil }), nil
	}
	change, err := integrate.PlanSessionSummaryConfig(path, enabled)
	if err != nil {
		return Plan{}, err
	}
	preview := []ChangePreview{{Target: path, Before: fmt.Sprintf("session_summary=%t", old.Hooks.SessionSummary), After: fmt.Sprintf("session_summary=%t", enabled)}}
	return a.savePlan("sessionSummary", preview, func() error {
		if err := integrate.Apply([]integrate.Change{change}); err != nil {
			return err
		}
		cfg, err := a.readConfig()
		if err == nil {
			a.deps.Service.SessionSummary = cfg.Hooks.SessionSummary
		}
		return err
	}), nil
}

// PlanSessionCleanup previews [hooks] session_cleanup; hooks read it per event.
func (a *API) PlanSessionCleanup(enabled bool) (Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	path := filepath.Join(a.deps.ConfigDir, "config.toml")
	old, err := a.readConfig()
	if err != nil {
		return Plan{}, err
	}
	if old.Hooks.SessionCleanup == enabled {
		return a.savePlan("sessionCleanup", []ChangePreview{}, func() error { return nil }), nil
	}
	change, err := integrate.PlanSessionCleanupConfig(path, enabled)
	if err != nil {
		return Plan{}, err
	}
	preview := []ChangePreview{{Target: path, Before: fmt.Sprintf("session_cleanup=%t", old.Hooks.SessionCleanup), After: fmt.Sprintf("session_cleanup=%t", enabled)}}
	return a.savePlan("sessionCleanup", preview, func() error {
		if err := integrate.Apply([]integrate.Change{change}); err != nil {
			return err
		}
		cfg, err := a.readConfig()
		if err == nil {
			a.deps.Service.SessionCleanup = cfg.Hooks.SessionCleanup
		}
		return err
	}), nil
}

// ApplyPlan accepts only the backend-held plan shown to the user. A second
// preview replaces the first, and the existing planners recheck external edits.
func (a *API) ApplyPlan(id string) (PlanReceipt, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.plan == nil || a.plan.view.ID != id {
		return PlanReceipt{}, fmt.Errorf("plan is no longer current; preview again")
	}
	plan := a.plan
	a.plan = nil
	if plan.applyMenu != nil {
		out, err := plan.applyMenu()
		return PlanReceipt{Receipt: Receipt{Action: plan.view.Action, Changed: len(out.Applied) > 0},
			Applied: list(out.Applied), Remaining: list(out.Remaining), Error: errorText(err)}, nil
	}
	err := plan.apply()
	return PlanReceipt{Receipt: Receipt{Action: plan.view.Action, Changed: err == nil && len(plan.view.Changes) > 0}, Applied: []string{}, Remaining: []string{}}, err
}
