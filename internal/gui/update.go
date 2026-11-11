package gui

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/buildinfo"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/update"
)

type UpdateInfo struct {
	Current   string    `json:"current"`
	Latest    string    `json:"latest,omitempty"`
	Newer     bool      `json:"newer"`
	Name      string    `json:"name,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	URL       string    `json:"url,omitempty"`
	Published time.Time `json:"published,omitzero"`
	CheckedAt time.Time `json:"checkedAt,omitzero"`
	Error     string    `json:"error,omitempty"`
	Enabled   bool      `json:"enabled"`
	Channel   string    `json:"channel"`
	Command   string    `json:"command,omitempty"`
}

func (a *API) Update() (UpdateInfo, error)      { return a.updateInfo(false) }
func (a *API) CheckUpdate() (UpdateInfo, error) { return a.updateInfo(true) }

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
	channel := update.DetectChannel(a.deps.Executable)
	return UpdateInfo{Current: buildinfo.Version, Latest: state.Latest.Version,
		Newer: update.Newer(buildinfo.Version, state.Latest.Version), Name: state.Latest.Name,
		Notes: state.Latest.Notes, URL: state.Latest.URL, Published: state.Latest.PublishedAt,
		CheckedAt: state.CheckedAt, Error: state.LastError, Enabled: cfg.Update.Check,
		Channel: channel, Command: update.Command(channel, state.Latest.Version, a.deps.Executable)}, nil
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
