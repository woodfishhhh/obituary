package gui

import (
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/rules"
)

type Summary struct {
	TrackedCount      int    `json:"trackedCount"`
	GraveCount        int    `json:"graveCount"`
	GraveBytes        int64  `json:"graveBytes"`
	GraveQuotaBytes   int64  `json:"graveQuotaBytes,omitempty"`
	GraveMaxBytes     int64  `json:"graveMaxBytes,omitempty"`
	QuotaPendingCount int    `json:"quotaPendingCount,omitempty"`
	PendingGraveCount int    `json:"pendingGraveCount"`
	DecisionCount     int    `json:"decisionCount"`
	Paused            bool   `json:"paused"`
	Error             string `json:"error,omitempty"`
	AutoMode          string `json:"autoMode,omitempty"` // [proposals] mode, for the home page
	Mode              string `json:"mode,omitempty"`     // [graveyard] mode: graveyard or recycle
}

func (a *API) Summary() (Summary, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	state, err := a.deps.Service.Store.Inspect()
	if err != nil {
		return Summary{}, err
	}
	paused, err := app.Paused(a.deps.Service.Store.Home)
	if err != nil {
		return Summary{}, err
	}
	files, err := a.ruleFiles()
	if err != nil {
		return Summary{}, err
	}
	existing := lifecycle.ExistingProposals(state.Proposals)
	quotaPending := 0
	for _, p := range existing {
		if p.Kind == ledger.ProposalGraveyardQuota {
			quotaPending++
		}
	}
	out := Summary{
		TrackedCount:      len(state.Items),
		DecisionCount:     len(existing),
		QuotaPendingCount: quotaPending,
		Paused:            paused,
		Mode:              a.effectiveMode(),
	}
	if cfg, err := a.readConfig(); err == nil {
		out.AutoMode = cfg.Proposals.Mode
		out.GraveMaxBytes = cfg.Graveyard.MaxBytes()
	}
	for _, it := range state.Items {
		if it.Stuck() { // retries stopped: it waits for Retry or Stop tracking
			out.DecisionCount++
		}
	}
	for _, f := range files.Rules {
		if f.Status == rules.Untrusted || f.Status == rules.Changed {
			out.DecisionCount++
		}
	}
	view, err := a.deps.Service.ReadGraveyard()
	if view == nil {
		return out, err
	}
	out.Error = errorText(err)
	out.PendingGraveCount = len(view.Pending)
	seen := map[int64]bool{}
	for _, row := range graveRows(view, "") {
		if row.Burial == nil || seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		out.GraveCount++
		out.GraveBytes += row.Burial.Size
		if row.Burial.Kind == "" && row.Burial.Source != "quota-decision" {
			out.GraveQuotaBytes += row.Burial.Size
		}
	}
	return out, nil
}
