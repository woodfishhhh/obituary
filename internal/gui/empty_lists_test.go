package gui

import (
	"encoding/json"
	"regexp"
	"testing"
)

// A fresh ledger has no watches, proposals or burials. Every list the window
// reads must still arrive as [], not null: one null list blanked the whole
// window when Settings read policies.length.
func TestEmptyLedgerListsAreArrays(t *testing.T) {
	a, _ := fixture(t)
	null := regexp.MustCompile(`"(policies|proposals|rules|rows|restored|items|lines|results|changes|applied|remaining)":null`)
	check := func(name string, v any, err error) {
		t.Helper()
		data, merr := json.Marshal(v)
		if merr != nil {
			t.Fatalf("%s: %v", name, merr)
		}
		if null.Match(data) {
			t.Fatalf("%s sent a null list (err %v): %s", name, err, data)
		}
	}
	watches, err := a.Watches()
	check("Watches", watches, err)
	proposals, err := a.Proposals()
	check("Proposals", proposals, err)
	rules, err := a.Rules()
	check("Rules", rules, err)
	graves, err := a.Graveyard(GraveyardQuery{Limit: 50})
	check("Graveyard", graves, err)
	upcoming, err := a.Upcoming("")
	check("Upcoming", upcoming, err)
	logs, err := a.Logs(LogQuery{Source: "audit.jsonl", Lines: 50})
	check("Logs", logs, err)
	unknown := []string{"999"}
	resurrect, err := a.Resurrect(unknown)
	check("Resurrect", resurrect, err)
	purge, err := a.Purge(unknown)
	check("Purge", purge, err)
	extend, err := a.Extend(unknown, "1d")
	check("Extend", extend, err)
	cancel, err := a.Cancel(unknown)
	check("Cancel", cancel, err)
	approve, err := a.Approve(unknown, []string{""})
	check("Approve", approve, err)
	reject, err := a.Reject(unknown, []string{""})
	check("Reject", reject, err)
	summary, err := a.PlanSessionSummary(false)
	check("PlanSessionSummary", summary, err)
	for name, plan := range map[string]func(bool) (Plan, error){"PlanTrayIcon": a.PlanTrayIcon, "PlanTaskbar": a.PlanTaskbar, "PlanStartMenu": a.PlanStartMenu, "PlanDesktopShortcut": a.PlanDesktopShortcut} {
		p, err := plan(false)
		check(name, p, err)
		if err == nil {
			r, err := a.ApplyPlan(p.ID)
			check(name+" receipt", r, err)
		}
	}
}
