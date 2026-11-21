package gui

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/buildinfo"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/update"
)

// UpdateInfo is the release check. ApplyBlocked and ApplyDetail say, for a
// manual install that cannot update itself, which update.Blocked* condition
// fails and its detail, such as the missing file names.
type UpdateInfo struct {
	Current      string            `json:"current"`
	Latest       string            `json:"latest,omitempty"`
	Newer        bool              `json:"newer"`
	Name         string            `json:"name,omitempty"`
	Notes        string            `json:"notes,omitempty"`
	URL          string            `json:"url,omitempty"`
	Published    time.Time         `json:"published,omitzero"`
	CheckedAt    time.Time         `json:"checkedAt,omitzero"`
	Error        string            `json:"error,omitempty"`
	Enabled      bool              `json:"enabled"`
	Channel      string            `json:"channel"`
	Command      string            `json:"command,omitempty"`
	CanApply     bool              `json:"canApply"`
	ApplyBlocked string            `json:"applyBlocked,omitempty"`
	ApplyDetail  string            `json:"applyDetail,omitempty"`
	LastApply    *update.LastApply `json:"lastApply,omitempty"`
}

type ApplyResult = update.ApplyResult

// UpdateProgress is what the update helper reports. The window stays open
// through the download and closes itself once the phase is "ready".
func (a *API) UpdateProgress() (update.Progress, error) {
	return update.ReadProgress(a.deps.Service.Store.Home)
}

func (a *API) Update() (UpdateInfo, error)      { return a.updateInfo(false) }
func (a *API) CheckUpdate() (UpdateInfo, error) { return a.updateInfo(true) }
func (a *API) ApplyUpdate() (ApplyResult, error) {
	return app.StartUpdater(context.Background(), a.deps.Executable, buildinfo.Version, true)
}

func (a *API) updateInfo(check bool) (UpdateInfo, error) {
	a.mu.Lock()
	cfg, err := a.readConfig()
	a.mu.Unlock()
	if err != nil {
		return UpdateInfo{}, err
	}
	checker := update.Checker{Home: a.deps.Service.Store.Home, Now: a.deps.Now}
	if a.deps.UpdateChecker != nil {
		checker = *a.deps.UpdateChecker
		checker.Home = a.deps.Service.Store.Home
	}
	state, err := update.Read(checker.Home)
	if check {
		state, err = checker.Check(context.Background(), cfg.Update.Check, true)
	}
	if err != nil {
		state.LastError = err.Error()
	}
	if check {
		a.mu.Lock()
		cfg, err = a.readConfig()
		a.mu.Unlock()
		if err != nil {
			return UpdateInfo{}, err
		}
	}
	last, err := update.ReadLastApply(checker.Home)
	if err != nil {
		return UpdateInfo{}, err
	}
	ap := update.Review(a.deps.Executable, buildinfo.Version, state.Latest.Version)
	newer := update.Newer(buildinfo.Version, state.Latest.Version)
	return UpdateInfo{Current: buildinfo.Version, Latest: state.Latest.Version,
		Newer: newer, Name: state.Latest.Name,
		Notes: state.Latest.Notes, URL: state.Latest.URL, Published: state.Latest.PublishedAt,
		CheckedAt: state.CheckedAt, Error: state.LastError, Enabled: cfg.Update.Check,
		Channel: ap.Channel, Command: update.Command(ap.Channel, state.Latest.Version, a.deps.Executable),
		CanApply: ap.Can, ApplyBlocked: ap.Blocked, ApplyDetail: ap.Detail, LastApply: last}, nil
}

func (a *API) PlanUpdateCheck(enabled bool) (Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	path := filepath.Join(a.deps.ConfigDir, "config.toml")
	old, err := a.readConfig()
	if err != nil {
		return Plan{}, err
	}
	if old.Update.Check == enabled {
		return a.savePlan("updateCheck", []ChangePreview{}, func() error { return nil }), nil
	}
	change, err := integrate.PlanUpdateCheckConfig(path, enabled)
	if err != nil {
		return Plan{}, err
	}
	preview := []ChangePreview{{Target: path, Before: fmt.Sprintf("check=%t", old.Update.Check), After: fmt.Sprintf("check=%t", enabled)}}
	return a.savePlan("updateCheck", preview, func() error { return integrate.Apply([]integrate.Change{change}) }), nil
}
