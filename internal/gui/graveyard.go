package gui

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

type GraveyardQuery struct {
	Filter string `json:"filter"`
	Limit  int    `json:"limit"`
	After  string `json:"after"`
}

type GraveRow struct {
	ID      int64              `json:"id"`
	Status  string             `json:"status"`
	Burial  *core.Burial       `json:"burial,omitempty"`
	Pending *core.PendingGrave `json:"pending,omitempty"`
}

type GraveyardPage struct {
	Rows         []GraveRow         `json:"rows"`
	Total        int                `json:"total"`
	PendingCount int                `json:"pendingCount"`
	HasMore      bool               `json:"hasMore"`
	NextAfter    string             `json:"nextAfter"`
	Restored     []core.GraveResult `json:"restored"`
	Error        string             `json:"error,omitempty"`
}

// Use a stable key so removals between pages cannot shift unread entries behind
// the cursor. GUI sorts by time, so its opaque cursor includes time, ID and
// manifest (even for unnumbered errors).
type graveCursor struct {
	Time     time.Time `json:"time"`
	ID       int64     `json:"id"`
	Manifest string    `json:"manifest"`
	Filter   string    `json:"filter"`
}

func rowCursor(row GraveRow, filter string) graveCursor {
	c := graveCursor{ID: row.ID, Filter: filter}
	if row.Burial != nil {
		c.Time = row.Burial.BuriedAt
	}
	if row.Pending != nil {
		c.Manifest = row.Pending.Manifest
	}
	return c
}

func compareGraves(a, b graveCursor) int {
	if n := b.Time.Compare(a.Time); n != 0 {
		return n
	}
	if n := cmp.Compare(b.ID, a.ID); n != 0 {
		return n
	}
	return cmp.Compare(a.Manifest, b.Manifest)
}

func graveBounds(query GraveyardQuery) (int, *graveCursor, error) {
	if query.Limit < 0 || query.Limit > 200 {
		return 0, nil, fmt.Errorf("limit must be between 0 and 200")
	}
	limit := query.Limit
	if limit == 0 {
		limit = 200
	}
	if query.After == "" {
		return limit, nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(query.After)
	var cursor graveCursor
	if err != nil || json.Unmarshal(data, &cursor) != nil || cursor.Filter != query.Filter {
		return 0, nil, fmt.Errorf("invalid cursor; restart the graveyard query")
	}
	return limit, &cursor, nil
}

func (a *API) Graveyard(query GraveyardQuery) (GraveyardPage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	limit, cursor, err := graveBounds(query)
	if err != nil {
		return GraveyardPage{}, err
	}
	view, readErr := a.deps.Service.ReadGraveyard()
	if view == nil {
		return GraveyardPage{}, readErr
	}
	rows := graveRows(view, query.Filter)
	page := GraveyardPage{Rows: []GraveRow{}, Total: len(rows), PendingCount: len(view.Pending), Restored: list(view.Restored), Error: errorText(readErr)}
	for _, row := range rows {
		if cursor != nil && compareGraves(rowCursor(row, query.Filter), *cursor) <= 0 {
			continue
		}
		if len(page.Rows) == limit {
			page.HasMore = true
			break
		}
		page.Rows = append(page.Rows, row)
	}
	if page.HasMore {
		data, err := json.Marshal(rowCursor(page.Rows[len(page.Rows)-1], query.Filter))
		if err != nil {
			return page, err
		}
		page.NextAfter = base64.RawURLEncoding.EncodeToString(data)
	}
	return page, nil
}

func graveRows(view *core.GraveyardView, filter string) []GraveRow {
	rows := make([]GraveRow, 0, len(view.Burials)+len(view.Pending))
	for i := range view.Burials {
		b := &view.Burials[i]
		rows = append(rows, GraveRow{ID: b.ID, Status: "buried", Burial: b})
	}
	for i := range view.Pending {
		p := &view.Pending[i]
		rows = append(rows, GraveRow{ID: p.Number(), Status: p.Status, Burial: p.Burial, Pending: p})
	}
	rows = slices.DeleteFunc(rows, func(row GraveRow) bool {
		path := ""
		if row.Burial != nil {
			path = row.Burial.Path
		}
		if row.Pending != nil {
			path += "\n" + row.Pending.Path + "\n" + row.Pending.Manifest
		}
		return !strings.Contains(strings.ToLower(path), strings.ToLower(filter))
	})
	slices.SortStableFunc(rows, func(a, b GraveRow) int { return compareGraves(rowCursor(a, filter), rowCursor(b, filter)) })
	return rows
}

func (a *API) Resurrect(refs []string) (GraveReply, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := requireRefs(refs); err != nil {
		return GraveReply{Results: []core.GraveResult{}}, err
	}
	results, err := a.deps.Service.Resurrect(refs)
	return GraveReply{Results: list(results), Error: errorText(err)}, nil
}

func (a *API) Purge(refs []string) (GraveReply, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := requireRefs(refs); err != nil {
		return GraveReply{Results: []core.GraveResult{}}, err
	}
	results, err := a.deps.Service.PurgeRefs(refs, false)
	return GraveReply{Results: list(results), Error: errorText(err)}, nil
}
