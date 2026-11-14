package app

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

// RecentBurials is a read-only menu snapshot. Clicking a row sends its ID,
// never its current menu position, through the ordinary resurrection checks.
func (r *Resident) RecentBurials() ([]core.Burial, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, g, err := r.view()
	if err != nil {
		return nil, err
	}
	return slices.Clone(g.recent), nil
}

// graves is what the resident and tray need from a graveyard snapshot. It
// stays valid while the hot ledger names the same snapshot.
type graves struct {
	ref    core.GraveyardReference
	next   time.Time // the next purge, when has
	has    bool
	count  int
	recent []core.Burial // newest first, at most 10
}

func summarize(st core.State) *graves {
	g := &graves{count: len(st.Burials)}
	if st.Graveyard != nil {
		g.ref = *st.Graveyard
	}
	only := core.State{Burials: st.Burials}
	g.next, g.has = only.NextDue()
	burials := slices.Clone(st.Burials)
	slices.SortFunc(burials, func(a, b core.Burial) int {
		if c := b.BuriedAt.Compare(a.BuriedAt); c != 0 {
			return c
		}
		return cmp.Compare(b.ID, a.ID)
	})
	g.recent = slices.Clone(burials[:min(10, len(burials))])
	return g
}

// view is what the last step read, or the whole ledger without one; the
// caller holds r.mu.
func (r *Resident) view() (core.State, *graves, error) {
	if r.hot != nil && r.graves != nil {
		return *r.hot, r.graves, nil
	}
	st, err := r.Service.Store.Inspect()
	if err != nil {
		return st, nil, err
	}
	return st, summarize(st), nil
}

func (r *Resident) Restore(id int64) (core.GraveResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hot, r.graves = nil, nil // the menu refreshed after this must not show the grave
	results, err := r.Service.Resurrect([]string{strconv.FormatInt(id, 10)})
	result := results[0]
	if err != nil {
		return result, err
	}
	if result.Error != "" {
		return result, fmt.Errorf("#%d: %s", id, result.Error)
	}
	return result, nil
}

// Counts feeds the tray's summary row: items due within a day, and graves.
func (r *Resident) Counts(now time.Time) (due, graves int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, g, err := r.view()
	if err != nil {
		return 0, 0, err
	}
	for _, item := range state.Items {
		if !item.ExpiresAt.After(now.Add(24 * time.Hour)) {
			due++
		}
	}
	return due, g.count, nil
}

func (r *Resident) Upcoming(now time.Time) ([]core.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, _, err := r.view()
	if err != nil {
		return nil, err
	}
	items := slices.DeleteFunc(slices.Clone(state.Items), func(item core.Item) bool { return item.ExpiresAt.After(now.Add(24 * time.Hour)) })
	slices.SortFunc(items, func(a, b core.Item) int { return a.ExpiresAt.Compare(b.ExpiresAt) })
	return items[:min(10, len(items))], nil
}
