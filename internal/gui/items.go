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
	return ItemList{Items: items}, nil
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

func (a *API) Cancel(refs []string) (ItemReply, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := requireRefs(refs); err != nil {
		return ItemReply{Results: []core.Result{}}, err
	}
	results, err := a.deps.Service.Cancel(refs)
	return ItemReply{Results: list(results), Error: errorText(err)}, nil
}
