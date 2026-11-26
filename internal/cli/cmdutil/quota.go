package cmdutil

import (
	"fmt"
	"io"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

// FormatQuotaOverflow renders a human- and AI-readable Plan A reminder when an
// oversized object stays in place and moves to pending decisions.
func FormatQuotaOverflow(l *i18n.Localizer, o *core.QuotaOverflow, dryRun bool) string {
	key := "cli.quota.overflow"
	if dryRun {
		key = "cli.quota.overflow_dry_run"
	}
	return l.Text(key, map[string]any{
		"Path":       o.Path,
		"Size":       textutil.Bytes(o.ItemBytes),
		"Limit":      textutil.Bytes(o.MaxBytes),
		"ProposalID": o.ProposalID,
	})
}

// PrintQuotaEviction writes a concise FIFO auto-purge summary to w when quota
// enforcement purged or retried any graveyard items.
func PrintQuotaEviction(w io.Writer, l *i18n.Localizer, ev *core.QuotaEviction, dryRun bool) {
	if ev == nil {
		return
	}
	if ev.PurgedCount > 0 {
		key := "cli.quota.evicted"
		if dryRun {
			key = "cli.quota.evicted_dry_run"
		}
		fmt.Fprintln(w, l.Text(key, map[string]any{
			"Limit": textutil.Bytes(ev.MaxBytes),
			"Count": ev.PurgedCount,
			"Freed": textutil.Bytes(ev.FreedBytes),
			"Used":  textutil.Bytes(ev.AfterBytes),
		}))
	}
	if ev.FailedCount > 0 {
		fmt.Fprintln(w, l.Text("cli.quota.eviction_partial_failed", map[string]any{
			"Failed": ev.FailedCount,
		}))
	}
}
