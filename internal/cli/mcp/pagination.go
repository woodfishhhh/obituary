package mcp

import (
	"cmp"
	"fmt"
	"slices"
	"sort"
	"strconv"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

// Advance by the last ID so removals between pages cannot shift unread entries
// behind the cursor. String cursors preserve exact IDs through SDK input
// schemas that decode numbers as float64.
type mcpPageInput struct {
	Limit   int    `json:"limit,omitempty" jsonschema:"Results per page; 0 uses 200; maximum 200"`
	After   string `json:"after,omitempty" jsonschema:"Previous nextAfter (decimal ID string); omitted starts at the beginning"`
	Version string `json:"version,omitempty" jsonschema:"Previous page version; a different current set returns changed and a warning to restart"`
}

func mcpPageBounds(in mcpPageInput, ids []int64) (int, int, map[string]any, error) {
	var after int64
	var err error
	if in.After != "" {
		after, err = strconv.ParseInt(in.After, 10, 64)
	}
	if in.Limit < 0 || in.Limit > 200 || after < 0 || err != nil {
		return 0, 0, nil, fmt.Errorf("limit must be between 0 and 200, after must be a nonnegative decimal ID string")
	}
	if in.Limit == 0 {
		in.Limit = 200
	}
	total := len(ids)
	start := sort.Search(len(ids), func(i int) bool { return ids[i] > after })
	// Include unnumbered scan errors only on the first page.
	if in.After == "" {
		start = 0
	}
	end := start + min(in.Limit, total-start)
	// Do not split one ID across pages (a corrupt manifest may duplicate it).
	if end < total && end > start && ids[end] == ids[end-1] {
		groupStart := end - 1
		for groupStart > start && ids[groupStart-1] == ids[end] {
			groupStart--
		}
		if groupStart > start {
			end = groupStart
		} else {
			// A single group may exceed the requested size, but never the hard
			// receipt bound. Otherwise defer the whole tail group to the next page.
			for end < total && ids[end] == ids[start] {
				end++
			}
		}
	}
	if end-start > 200 {
		return 0, 0, nil, fmt.Errorf("too many records share one ID; inspect the graveyard with the CLI")
	}
	data := map[string]any{"total": total, "limit": in.Limit, "hasMore": end < total}
	if end < total {
		data["nextAfter"] = strconv.FormatInt(ids[end-1], 10)
	}
	return start, end, data, nil
}

func mcpPageResult[T any](key string, rows []T, id func(T) int64, in mcpPageInput, readErr error) mcpReply {
	if readErr != nil {
		return mcpResult(nil, readErr)
	}
	slices.SortFunc(rows, func(a, b T) int { return cmp.Compare(id(a), id(b)) })
	ids := make([]int64, len(rows))
	keys := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = id(row)
		keys[i] = fmt.Sprintf("%d:%s:%s", ids[i], key, mcpMemberKey(any(row)))
	}
	start, end, data, err := mcpPageBounds(in, ids)
	if err != nil {
		return mcpResult(nil, err)
	}
	data[key] = rows[start:end]
	return mcpVersionedResult(data, keys, in, nil)
}

func mcpGravePage(burials []ledger.Burial, pending []core.PendingGrave, in mcpPageInput, readErr error) mcpReply {
	type row struct {
		id      int64
		burial  *ledger.Burial
		pending *core.PendingGrave
	}
	rows := make([]row, 0, len(burials)+len(pending))
	for i := range burials {
		rows = append(rows, row{id: burials[i].ID, burial: &burials[i]})
	}
	for i := range pending {
		rows = append(rows, row{id: pending[i].Number(), pending: &pending[i]})
	}
	slices.SortStableFunc(rows, func(a, b row) int { return cmp.Compare(a.id, b.id) })
	ids := make([]int64, len(rows))
	keys := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.id
		status := "buried"
		if r.pending != nil {
			status = r.pending.Status + ":" + r.pending.Manifest
		}
		keys[i] = fmt.Sprintf("%d:%s", r.id, status)
	}
	start, end, data, err := mcpPageBounds(in, ids)
	if err != nil {
		return mcpResult(nil, err)
	}
	bs, ps := []ledger.Burial{}, []core.PendingGrave{}
	for _, r := range rows[start:end] {
		if r.burial != nil {
			bs = append(bs, *r.burial)
		} else {
			ps = append(ps, *r.pending)
		}
	}
	data["burials"], data["pending"], data["pendingCount"] = bs, ps, len(pending)
	return mcpVersionedResult(data, keys, in, readErr)
}
