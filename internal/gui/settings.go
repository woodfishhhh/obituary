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
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/shortcut"
)

type Settings struct {
	Capabilities         Capabilities  `json:"capabilities"`
	TrayIcon             bool          `json:"trayIcon"`
	Taskbar              bool          `json:"taskbar"`
	StartMenu            bool          `json:"startMenu"`
	DesktopShortcut      bool          `json:"desktopShortcut"`
	Mode                 string        `json:"mode"`
	RetentionDays        int           `json:"retentionDays"`
	GraveyardMaxSize     string        `json:"graveyardMaxSize"`
	GraveyardMaxBytes    int64         `json:"graveyardMaxBytes"`
	GraveCount           int           `json:"graveCount"`
	GraveBytes           int64         `json:"graveBytes"`
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

// service returns a request-local shallow copy of the service refreshed from
// config.toml so external CLI/editor changes take effect without restarting
// the GUI. Tests that configure a.deps.Service in memory without writing
// config.toml keep their in-memory settings until config.toml is loaded.
func (a *API) service() (*core.Service, error) {
	path := filepath.Join(a.deps.ConfigDir, "config.toml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if !a.configLoaded {
			cp := *a.deps.Service
			return &cp, nil
		}
		return applyServiceConfig(a.deps.Service, config.Default()), nil
	}
	if err != nil {
		return nil, err
	}
	cfg, err := config.Parse(data, path)
	if err != nil {
		return nil, err
	}
	a.configLoaded = true
	return applyServiceConfig(a.deps.Service, cfg), nil
}

func applyServiceConfig(base *core.Service, cfg config.Config) *core.Service {
	cp := *base
	// Preserve live Mode so Summary, Upcoming, and Bury stay aligned with the
	// GUI session's delete mode until changed via PlanSettings.
	cp.Retention = cfg.Graveyard.Retention()
	cp.GraveyardMaxBytes = cfg.Graveyard.MaxBytes()
	cp.WatchStableWindow = cfg.Watch.Stability()
	cp.WatchFileProposals = cfg.Watch.FileExpiry == "propose"
	cp.SessionSummary = cfg.Hooks.SessionSummary
	cp.SessionCleanup = cfg.Hooks.SessionCleanup
	cp.ProposalMode = cfg.Proposals.Mode
	return &cp
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
	var start, desktop string
	if capabilities := platformCapabilities(); capabilities.StartMenu || capabilities.DesktopShortcut {
		start, err = shortcut.Inspect(a.deps.Shortcuts, a.deps.Executable, shortcut.StartMenu)
		if err != nil {
			return Settings{}, err
		}
		desktop, err = shortcut.Inspect(a.deps.Shortcuts, a.deps.Executable, shortcut.Desktop)
		if err != nil {
			return Settings{}, err
		}
	}
	paused, err := app.Paused(a.deps.Service.Store.Home)
	graveCount, graveBytes, _ := a.deps.Service.GraveyardDiskUsage()
	return Settings{Capabilities: platformCapabilities(), TrayIcon: cfg.UI.Tray, Taskbar: cfg.UI.Taskbar, StartMenu: start != "", DesktopShortcut: desktop != "", Lifetime: lifetime, Downloads: downloadsFolder(), Mode: cfg.Graveyard.Mode, RetentionDays: cfg.Graveyard.RetentionDays,
		GraveyardMaxSize: cfg.Graveyard.MaxSize, GraveyardMaxBytes: cfg.Graveyard.MaxBytes(), GraveCount: graveCount, GraveBytes: graveBytes,
		Language: cfg.UI.Language, EffectiveLanguage: language,
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
	ID           string              `json:"id"`
	Action       string              `json:"action"`
	Changes      []ChangePreview     `json:"changes"`
	QuotaPreview *core.QuotaEviction `json:"quotaPreview,omitempty"`
}
type pendingPlan struct {
	view       Plan
	apply      func() error
	applyMenu  func() (contextmenu.ApplyReceipt, error)
	applyQuota func() (bool, core.QuotaEviction, error)
}

type PlanReceipt struct {
	Receipt
	Applied       []string            `json:"applied"`
	Remaining     []string            `json:"remaining"`
	QuotaEviction *core.QuotaEviction `json:"quotaEviction,omitempty"`
	Error         string              `json:"error,omitempty"`
}

func (a *API) savePlan(action string, changes []ChangePreview, apply func() error) Plan {
	a.nextPlan++
	view := Plan{ID: strconv.FormatUint(a.nextPlan, 10), Action: action, Changes: list(changes)}
	a.plan = &pendingPlan{view: view, apply: apply}
	return view
}

func (a *API) PlanSettings(mode string, retentionDays int) (Plan, error) {
	if mode == config.ModeRecycle && !platformCapabilities().Recycle {
		return Plan{}, core.ErrUnsupported
	}
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
	if plan.applyQuota != nil {
		configSaved, ev, err := plan.applyQuota()
		var summary *core.QuotaEviction
		if ev.PurgedCount > 0 || ev.FailedCount > 0 {
			ev.Results = nil
			summary = &ev
		}
		if configSaved {
			return PlanReceipt{Receipt: Receipt{Action: plan.view.Action, Changed: true},
				Applied: []string{}, Remaining: []string{}, QuotaEviction: summary, Error: errorText(err)}, nil
		}
		return PlanReceipt{Receipt: Receipt{Action: plan.view.Action, Changed: false},
			Applied: []string{}, Remaining: []string{}, QuotaEviction: summary}, err
	}
	err := plan.apply()
	return PlanReceipt{Receipt: Receipt{Action: plan.view.Action, Changed: err == nil && len(plan.view.Changes) > 0}, Applied: []string{}, Remaining: []string{}}, err
}

// PreviewGraveyardQuotaEviction inspects what lowering [graveyard] max_size
// would evict without modifying config.toml, the ledger, or any file on disk.
func (a *API) PreviewGraveyardQuotaEviction(maxSize string) (core.QuotaEviction, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, maxBytes, err := config.NormalizeByteSize(maxSize)
	if err != nil {
		return core.QuotaEviction{}, err
	}
	if maxBytes == 0 {
		_, used, err := a.deps.Service.GraveyardDiskUsage()
		return core.QuotaEviction{MaxBytes: 0, BeforeBytes: used, AfterBytes: used}, err
	}
	ev, err := a.deps.Service.PreviewGraveyardQuotaEviction(maxBytes)
	ev.Results = nil
	return ev, err
}

// PlanGraveyardMaxSize previews saving [graveyard] max_size; applying it
// updates config.toml, updates the live service limit, and runs FIFO eviction.
func (a *API) PlanGraveyardMaxSize(maxSize string) (Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	norm, maxBytes, err := config.NormalizeByteSize(maxSize)
	if err != nil {
		return Plan{}, err
	}
	path := filepath.Join(a.deps.ConfigDir, "config.toml")
	old, err := a.readConfig()
	if err != nil {
		return Plan{}, err
	}
	if old.Graveyard.MaxSize == norm {
		return a.savePlan("graveyardMaxSize", []ChangePreview{}, func() error { return nil }), nil
	}
	change, err := integrate.PlanGraveyardMaxSizeConfig(path, norm)
	if err != nil {
		return Plan{}, err
	}
	var quotaPreview *core.QuotaEviction
	var previewHash string
	var needDriftCheck bool
	if maxBytes > 0 {
		ev, err := a.deps.Service.PreviewGraveyardQuotaEviction(maxBytes)
		if err != nil {
			return Plan{}, err
		}
		previewHash = ev.GraveyardHash
		needDriftCheck = ev.BeforeBytes > maxBytes
		ev.Results = nil
		quotaPreview = &ev
	}
	preview := []ChangePreview{{Target: path, Before: "max_size=" + old.Graveyard.MaxSize, After: "max_size=" + norm}}
	a.nextPlan++
	view := Plan{ID: strconv.FormatUint(a.nextPlan, 10), Action: "graveyardMaxSize", Changes: preview, QuotaPreview: quotaPreview}
	a.plan = &pendingPlan{view: view, applyQuota: func() (bool, core.QuotaEviction, error) {
		return a.applyGraveyardMaxSizePlan(change, maxBytes, needDriftCheck, previewHash)
	}}
	return view, nil
}

func (a *API) applyGraveyardMaxSizePlan(change integrate.Change, maxBytes int64, needDriftCheck bool, previewHash string) (bool, core.QuotaEviction, error) {
	if needDriftCheck {
		currentHash, err := a.deps.Service.GraveyardSnapshotHash()
		if err != nil {
			return false, core.QuotaEviction{}, err
		}
		if currentHash != previewHash {
			return false, core.QuotaEviction{}, errors.New("graveyard changed since preview; preview again")
		}
	}
	if err := integrate.Apply([]integrate.Change{change}); err != nil {
		return false, core.QuotaEviction{}, err
	}
	cfg, err := a.readConfig()
	if err != nil {
		return true, core.QuotaEviction{}, err
	}
	a.configLoaded = true
	a.deps.Service.GraveyardMaxBytes = cfg.Graveyard.MaxBytes()
	if maxBytes <= 0 {
		return true, core.QuotaEviction{}, nil
	}
	svc := applyServiceConfig(a.deps.Service, cfg)
	ev, evErr := svc.EnforceGraveyardQuota(false)
	if core.OnlyAuditErrors(evErr) {
		evErr = nil
	}
	return true, ev, evErr
}
