package watch

import (
	"fmt"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

type watchChild struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
	*lifecycle.WatchStatus
}

type listing struct {
	ledger.WatchPolicy
	Exemptions []ledger.WatchExemption `json:"exemptions"`
	Stability  []watchChild            `json:"stability,omitempty"`
	Skipped    []lifecycle.WatchSkip   `json:"skipped,omitempty"`
	ScanError  string                  `json:"scanError,omitempty"`
}

func (o *options) printPolicies(svc *lifecycle.Service) error {
	st, err := svc.Store.Read()
	if err != nil {
		return err
	}
	rows := make([]listing, 0, len(st.Watches))
	for _, p := range st.Watches {
		row := policyListing(svc, st, p, time.Now())
		rows = append(rows, row)
		if !o.asJSON {
			printPolicy(row)
		}
	}
	if o.asJSON {
		return cmdutil.PrintJSON(rows)
	}
	return nil
}

func policyListing(svc *lifecycle.Service, st ledger.State, p ledger.WatchPolicy, now time.Time) listing {
	row := listing{WatchPolicy: p, Exemptions: svc.WatchExemptionDetails(st, p.ID)}
	path, err := svc.WatchPath(p)
	if err == nil {
		row.Skipped, err = lifecycle.WatchDownloadSkips(path)
	}
	if err != nil {
		row.ScanError = err.Error()
	}
	for _, it := range st.Items {
		if it.WatchID != p.ID {
			continue
		}
		if status := svc.WatchStatus(it, now); status != nil {
			row.Stability = append(row.Stability, watchChild{it.ID, it.Path, status})
		}
	}
	return row
}

func printPolicy(row listing) {
	p := row.WatchPolicy
	fmt.Printf(cmdutil.Text("cli.watch.age"), p.ID, p.Path, p.Lifespan, cmdutil.Localizer().Label(p.Age), cmdutil.Localizer().Label(p.Consent))
	for _, e := range p.Exclusions {
		fmt.Printf(cmdutil.Text("cli.watch.excluded_clear_obit_watch_policy_clear_exclusion"), e.ID, e.Path, p.ID, e.ID)
	}
	for _, e := range row.Exemptions {
		at := cmdutil.Text("cli.watch.unknown_legacy_exemption")
		if !e.KeptAt.IsZero() {
			at = e.KeptAt.Format(time.RFC3339)
		}
		path := e.Path
		if path == "" {
			path = cmdutil.Text("cli.watch.path_unavailable")
		}
		fmt.Printf(cmdutil.Text("cli.watch.exempt_reason_time"), path, e.Reason, at)
	}
	for _, child := range row.Stability {
		fmt.Printf(cmdutil.Text("cli.watch.quiet_for_more_stable_window_next_check"), child.ID, child.Path, child.QuietFor, child.StableWindow, child.NextCheck.Format(time.RFC3339))
	}
	for _, skip := range row.Skipped {
		fmt.Printf("  %s: %s; %s\n", skip.Path, skip.Reason, skip.Manage)
	}
	if row.ScanError != "" {
		fmt.Println(cmdutil.Text("cli.watch.scan"), row.ScanError)
	}
}
