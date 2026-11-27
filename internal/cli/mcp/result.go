package mcp

import (
	"fmt"
	"strings"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

// Every response is an object, including partial failures. Receipts survive
// audit failures; returning a Go error to AddTool would discard those receipts.
type mcpReply struct {
	Data      any    `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
	Warning   string `json:"warning,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

func mcpResult(data any, err error) mcpReply {
	r := mcpReply{Data: data}
	if err != nil {
		r.Error = err.Error()
	}
	return r
}

func mcpCommittedResult(data any, err error, committed bool) mcpReply {
	r := mcpResult(data, err)
	if committed && cmdutil.OnlyAuditErrors(err) {
		r.Error, r.Warning = "", err.Error()
	}
	return r
}

func mcpItems(results []lifecycle.Result, err error) mcpReply {
	committed := len(results) > 0
	for _, r := range results {
		committed = committed && r.Action != "" && r.Error == ""
	}
	out := mcpCommittedResult(map[string]any{"results": results}, err, committed)
	if lifecycle.Failed(results) && out.Error == "" {
		out.Error = "one or more items failed; results contain individual errors"
	}
	return out
}

func mcpGraves(results []lifecycle.GraveResult, err error) mcpReply {
	committed := len(results) > 0
	for _, r := range results {
		committed = committed && (r.Action == ledger.ActionBuried || r.Action == ledger.ActionResurrected || r.Action == ledger.ActionRecycled) && r.Error == ""
	}
	for i := range results {
		if results[i].Action == ledger.ActionRecycled && results[i].Error == "" {
			results[i].Detail = cmdutil.RecycleRecovery
		}
	}
	out := mcpCommittedResult(map[string]any{"results": results}, err, committed)
	if msg := mcpQuotaOverflowMessage(results); msg != "" {
		out.Error = msg
	} else if lifecycle.GraveFailed(results) && out.Error == "" {
		out.Error = "one or more burials failed; results contain individual errors"
	}
	if note := mcpQuotaEvictionWarning(results); note != "" {
		if out.Warning != "" {
			out.Warning += "; "
		}
		out.Warning += note
	}
	return out
}

func mcpQuotaOverflowMessage(results []lifecycle.GraveResult) string {
	var parts []string
	for _, r := range results {
		if r.QuotaOverflow == nil {
			continue
		}
		o := r.QuotaOverflow
		parts = append(parts, fmt.Sprintf("%s (%s) exceeds graveyard capacity limit %s; left safely in place and routed to pending decisions (#%d) without evicting existing graves; ask the user to decide in Obituary or raise graveyard.max_size, and do not bypass Obituary with raw rm",
			o.Path, textutil.Bytes(o.ItemBytes), textutil.Bytes(o.MaxBytes), o.ProposalID))
	}
	return strings.Join(parts, "; ")
}

func mcpQuotaEvictionWarning(results []lifecycle.GraveResult) string {
	var parts []string
	for _, r := range results {
		ev := r.QuotaEviction
		if ev == nil || (ev.PurgedCount == 0 && ev.FailedCount == 0) {
			continue
		}
		if ev.PurgedCount > 0 {
			parts = append(parts, fmt.Sprintf("Graveyard exceeded capacity limit %s: automatically purged %d oldest item(s) (freed %s), current usage %s / %s",
				textutil.Bytes(ev.MaxBytes), ev.PurgedCount, textutil.Bytes(ev.FreedBytes), textutil.Bytes(ev.AfterBytes), textutil.Bytes(ev.MaxBytes)))
		}
		if ev.FailedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d oldest graveyard item(s) could not be purged right now and will be retried later", ev.FailedCount))
		}
	}
	return strings.Join(parts, "; ")
}

func mcpDaemonWarning(reply mcpReply, home string) mcpReply {
	if err := cmdutil.EnsureDaemon(home, true); err != nil {
		warning := fmt.Sprintf("后台进程未能启动，寿命不会被自动处理；可以执行 obit daemon enable: %v", err)
		if reply.Warning != "" {
			reply.Warning += "; "
		}
		reply.Warning += warning
	}
	return reply
}
