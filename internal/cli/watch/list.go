package watch

import (
	"fmt"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

type watchChild struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
	*core.WatchStatus
}

type listing struct {
	core.WatchPolicy
	Exemptions []core.WatchExemption `json:"exemptions"`
	Stability  []watchChild          `json:"stability,omitempty"`
	Skipped    []core.WatchSkip      `json:"skipped,omitempty"`
	ScanError  string                `json:"scanError,omitempty"`
}

func (o *options) printPolicies(svc *core.Service) error {
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

func policyListing(svc *core.Service, st core.State, p core.WatchPolicy, now time.Time) listing {
	row := listing{WatchPolicy: p, Exemptions: svc.WatchExemptionDetails(st, p.ID)}
	path, err := svc.WatchPath(p)
	if err == nil {
		row.Skipped, err = core.WatchDownloadSkips(path)
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
	fmt.Printf("#%d %s  %s  age=%s  %s\n", p.ID, p.Path, p.Lifespan, p.Age, p.Consent)
	for _, e := range p.Exclusions {
		fmt.Printf("  excluded #%d %s; clear: obit watch --policy %d --clear-exclusion %d\n", e.ID, e.Path, p.ID, e.ID)
	}
	for _, e := range row.Exemptions {
		at := "unknown (legacy exemption)"
		if !e.KeptAt.IsZero() {
			at = e.KeptAt.Format(time.RFC3339)
		}
		path := e.Path
		if path == "" {
			path = "path unavailable"
		}
		fmt.Printf("  exempt %s; reason=%s; time=%s\n", path, e.Reason, at)
	}
	for _, child := range row.Stability {
		fmt.Printf("  #%d %s; quiet for %s more (stable_window=%s); next check %s\n", child.ID, child.Path, child.QuietFor, child.StableWindow, child.NextCheck.Format(time.RFC3339))
	}
	for _, skip := range row.Skipped {
		fmt.Printf("  %s: %s; %s\n", skip.Path, skip.Reason, skip.Manage)
	}
	if row.ScanError != "" {
		fmt.Println("  scan:", row.ScanError)
	}
}
