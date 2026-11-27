package gui

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

type ItemList struct {
	Items []core.Item `json:"items"`
	// Effective [graveyard] mode executed by Bury: graveyard or recycle.
	Mode string `json:"mode,omitempty"`
}

func (a *API) effectiveMode() string {
	if a.deps.Service.Mode == core.ModeRecycle {
		return core.ModeRecycle
	}
	return core.ModeGraveyard
}

// Within is empty, "24h" or "7d". Already overdue entries remain visible.
func (a *API) Upcoming(within string) (ItemList, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	durations := map[string]time.Duration{"": 0, "24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour}
	duration, ok := durations[within]
	if !ok {
		return ItemList{}, fmt.Errorf("within must be empty, 24h or 7d")
	}
	state, err := a.deps.Service.Store.Inspect()
	if err != nil {
		return ItemList{}, err
	}
	items := make([]core.Item, 0, len(state.Items))
	for _, item := range state.Items {
		if duration == 0 || !item.ExpiresAt.After(a.deps.Now().Add(duration)) {
			items = append(items, item)
		}
	}
	slices.SortFunc(items, func(a, b core.Item) int {
		if n := a.ExpiresAt.Compare(b.ExpiresAt); n != 0 {
			return n
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return ItemList{Items: items, Mode: a.effectiveMode()}, nil
}

func (a *API) Extend(refs []string, lifespan string) (ItemReply, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := requireRefs(refs); err != nil {
		return ItemReply{Results: []core.Result{}}, err
	}
	duration, err := core.ParseDuration(lifespan)
	if err != nil {
		return ItemReply{Results: []core.Result{}}, err
	}
	results, err := a.deps.Service.Extend(refs, duration)
	return ItemReply{Results: list(results), Error: errorText(err)}, nil
}

// Retry makes items whose burial failed due again now.
func (a *API) Retry(refs []string) (ItemReply, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := requireRefs(refs); err != nil {
		return ItemReply{Results: []core.Result{}}, err
	}
	results, err := a.deps.Service.Retry(refs)
	return ItemReply{Results: list(results), Error: errorText(err)}, nil
}

func (a *API) Cancel(refs []string) (ItemReply, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := requireRefs(refs); err != nil {
		return ItemReply{Results: []core.Result{}}, err
	}
	results, err := a.deps.Service.Cancel(refs)
	return ItemReply{Results: list(results), Error: errorText(err)}, nil
}

// Bury moves tracked items into the graveyard right away (or Recycle Bin in recycle mode).
func (a *API) Bury(refs []string) (BuryReply, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := requireRefs(refs); err != nil {
		return BuryReply{Results: []core.GraveResult{}, BuriedIDs: []int64{}}, err
	}
	svc, err := a.service()
	if err != nil {
		return BuryReply{Results: []core.GraveResult{}, BuriedIDs: []int64{}, Error: errorText(err)}, nil
	}
	results, err := svc.BuryTracked(refs)
	return BuryReply{Results: list(results), BuriedIDs: buriedIDs(results), Error: errorText(err)}, nil
}

func buriedIDs(results []core.GraveResult) []int64 {
	ids := make([]int64, 0, len(results))
	for _, r := range results {
		if r.Action == core.ActionBuried && r.Burial != nil {
			ids = append(ids, r.Burial.ID)
		}
	}
	return ids
}
