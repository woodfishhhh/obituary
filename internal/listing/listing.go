// Package listing renders the active entries for `obit list`. The command and the
// resident both call it, so the bytes are the same whichever of them reads the
// ledger.
package listing

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/core/lifespan"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/texttable"
)

// TimeLayout is how expiry times are shown.
const TimeLayout = "2006-01-02 15:04"

// Options are what the caller's own settings decide about the output.
type Options struct {
	Language     i18n.Language
	JSON         bool
	StableWindow time.Duration // the caller's configured watch quiet interval; zero means the default
}

// Write prints items as of now, in the caller's language.
func Write(w io.Writer, items []ledger.Item, now time.Time, opt Options) error {
	svc := &lifecycle.Service{WatchStableWindow: opt.StableWindow}
	if opt.JSON {
		return writeJSON(w, svc, items, now)
	}
	return writeTable(w, svc, items, now, i18n.NewLocalizer(opt.Language))
}

func writeJSON(w io.Writer, svc *lifecycle.Service, items []ledger.Item, now time.Time) error {
	type row struct {
		ledger.Item
		Stability       *lifecycle.WatchStatus `json:"stability,omitempty"`
		IdentityWarning string                 `json:"identityWarning,omitempty"`
	}
	rows := make([]row, 0, len(items))
	for _, it := range items {
		rows = append(rows, row{it, svc.WatchStatus(it, now), identityWarning(it)})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func writeTable(w io.Writer, svc *lifecycle.Service, items []ledger.Item, now time.Time, language *i18n.Localizer) error {
	if len(items) == 0 {
		_, err := fmt.Fprintln(w, language.Text("cli.list.empty", nil))
		return err
	}
	out := bufio.NewWriter(w)
	table := texttable.New(out, language.Language())
	fmt.Fprintln(table, language.Text("cli.list.header", nil))
	for _, item := range items {
		fmt.Fprintf(table, "#%d\t%s\t%s\t%s\t%s\n", item.ID, item.ExpiresAt.Local().Format(TimeLayout),
			lifespan.FormatRemaining(time.Until(item.ExpiresAt)), item.Path, note(svc, item, now, language))
	}
	if err := table.Flush(); err != nil {
		return err
	}
	return out.Flush()
}

func note(svc *lifecycle.Service, item ledger.Item, now time.Time, language *i18n.Localizer) string {
	note := item.Note
	if item.Stuck() {
		// Extend clears the failure count, so it is the command-line Retry.
		note = language.Text("cli.list.stopped", map[string]any{"Attempts": item.Attempts, "Error": item.LastError, "ID": item.ID})
	} else if item.LastError != "" {
		note = language.Text("cli.list.retry", map[string]any{"Attempts": item.Attempts, "Error": item.LastError})
	}
	if warning := identityWarning(item); warning != "" {
		note += "; " + warning
	}
	if status := svc.WatchStatus(item, now); status != nil {
		if strings.HasPrefix(item.LastError, "folder child is still active") {
			note = fmt.Sprintf(language.Text("cli.list.retry_count", nil), item.Attempts)
		}
		note += "; " + language.Text("cli.list.quiet", map[string]any{"QuietFor": status.QuietFor, "StableWindow": status.StableWindow, "NextCheck": status.NextCheck.Format(time.RFC3339)})
	}
	return note
}

func identityWarning(item ledger.Item) string {
	if item.Identity != nil {
		if err := item.Identity.RequirePersistent(); err != nil {
			return err.Error()
		}
	}
	return ""
}
