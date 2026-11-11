package gui

import (
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/rules"
)

type Summary struct {
	TrackedCount      int    `json:"trackedCount"`
	GraveCount        int    `json:"graveCount"`
	GraveBytes        int64  `json:"graveBytes"`
	PendingGraveCount int    `json:"pendingGraveCount"`
	DecisionCount     int    `json:"decisionCount"`
	Paused            bool   `json:"paused"`
	Error             string `json:"error,omitempty"`
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
	out := Summary{TrackedCount: len(state.Items), DecisionCount: len(state.Proposals), Paused: paused}
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
	}
	return out, nil
}
