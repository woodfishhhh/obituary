package doctor

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/rivo/uniseg"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

// The compact view groups the individual checks into a few categories, one
// line each; the report and --verbose keep every check.
var categoryOrder = []string{"install", "background", "agents", "menu", "data", "hooks", "watch", "docker", "other"}

var checkCategory = map[string]string{
	"path": "install", "obitd": "install", "gui": "install", "registry-transactions": "install", "start-menu": "install",
	"desktop": "install", "tray-icon": "install", "taskbar": "install", "update": "install", "automatic-burial": "install",
	"runner": "background", "tray-startup": "background",
	"mcp": "agents", "skill": "agents", "agents": "agents", "codex-windows-shell": "agents",
	"context-menu": "menu", "explorer": "menu",
	"data": "data", "store": "data", "config": "data", "trust": "data", "project-rules": "data",
	"registration-identities": "data", "graveyard-recovery": "data",
	"hook-log": "hooks", "hook-queue": "hooks", "slow-git": "hooks",
	"watch":  "watch",
	"docker": "docker", "docker-targets": "docker", "docker-volume-recovery": "docker",
}

// fixCommands are the commands that put a failing check right. Anything else
// has no single remedy, so the compact view shows the reason only.
var fixCommands = map[string]string{
	"runner": "obit daemon enable", "tray-startup": "obit init", "start-menu": "obit init", "desktop": "obit init",
	"context-menu": "obit init --context-menu --yes", "config": "obit config", "graveyard-recovery": "obit graveyard",
	"project-rules": "obit trust", "watch": "obit watch --list", "docker-targets": "obit docker list",
	"codex-windows-shell": "obit init -g --agent codex --guard-rm",
}

// fixSentences are remedies that are not a single command.
var fixSentences = map[string]string{
	"path": "cli.doctor.fix.path", "obitd": "cli.doctor.fix.obitd", "gui": "cli.doctor.fix.gui",
}

// A problem line stays within this many terminal columns, so it does not wrap
// on a common 100-column window; the report has the whole text.
const (
	maxLineColumns = 100
	problemIndent  = 7
)

func categoryOf(name string) string {
	if category, ok := checkCategory[name]; ok {
		return category
	}
	if agent, ok := strings.CutSuffix(name, "-guard-rm"); ok {
		name = agent
	}
	if _, ok := integrate.LookupAgent(name); ok {
		return "agents"
	}
	return "other"
}

func isProblem(c DoctorCheck) bool { return c.Status == "WARN" || c.Status == "FAIL" }

// categoryStatus is the worst status in the category; INFO only when every
// check merely informs (such as Windows-only items on another system).
func categoryStatus(checks []DoctorCheck) string {
	status := "INFO"
	for _, c := range checks {
		switch {
		case c.Status == "FAIL":
			return "FAIL"
		case c.Status == "WARN":
			status = "WARN"
		case c.Status == "PASS" && status == "INFO":
			status = "PASS"
		}
	}
	return status
}

type category struct {
	Key    string
	Status string
	Checks []DoctorCheck
}

// problems lists the checks that need attention, errors first.
func (c category) problems() []DoctorCheck {
	var out []DoctorCheck
	for _, check := range c.Checks {
		if isProblem(check) {
			out = append(out, check)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Status == "FAIL" && out[j].Status != "FAIL" })
	return out
}

func groupChecks(checks []DoctorCheck) []category {
	byKey := map[string][]DoctorCheck{}
	for _, c := range checks {
		key := categoryOf(c.Name)
		byKey[key] = append(byKey[key], c)
	}
	var out []category
	for _, key := range categoryOrder {
		if members := byKey[key]; len(members) > 0 {
			out = append(out, category{Key: key, Status: categoryStatus(members), Checks: members})
		}
	}
	return out
}

func countProblems(checks []DoctorCheck) int {
	n := 0
	for _, c := range checks {
		if isProblem(c) {
			n++
		}
	}
	return n
}

func (lang diagnostics) headline(checks []DoctorCheck) string {
	data := map[string]any{"Checks": len(checks), "Problems": countProblems(checks)}
	switch countProblems(checks) {
	case 0:
		return lang.Text("cli.doctor.summary.ok", data)
	case 1:
		return lang.Text("cli.doctor.summary.problems.one", data)
	}
	return lang.Text("cli.doctor.summary.problems.other", data)
}

func (lang diagnostics) categoryName(key string) string {
	return lang.Text("cli.doctor.category."+key, nil)
}

// reason is the first clause of the detail, which states what is wrong, cut to
// the given number of terminal columns; the rest of the detail is in the report.
func reason(c DoctorCheck, columns int) string {
	text := c.Detail
	if i := strings.IndexAny(text, ";；\r\n"); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSpace(text)
	if uniseg.StringWidth(text) <= columns {
		return text
	}
	used := 0
	for i, r := range text {
		w := uniseg.StringWidth(string(r))
		if used+w > columns-1 {
			return text[:i] + "…"
		}
		used += w
	}
	return text
}

// fixFor returns the remedy shown under a problem, or "" when there is none.
func (lang diagnostics) fixFor(c DoctorCheck) string {
	if id, ok := fixSentences[c.Name]; ok {
		return lang.Text(id, nil)
	}
	if command, ok := fixCommands[c.Name]; ok {
		return command
	}
	agent, guard := strings.CutSuffix(c.Name, "-guard-rm")
	if _, ok := integrate.LookupAgent(agent); !ok {
		return ""
	}
	if guard {
		return "obit init -g --agent " + agent + " --guard-rm"
	}
	return "obit init -g --agent " + agent
}

// reportNote tells the compact view where the full report went.
type reportNote struct {
	Path   string
	Reason string // why there is no report, when Path is empty
}

// writeSummary prints the compact view: a verdict, one line per category,
// the reason and remedy under those that need attention, and where the
// details are.
func (lang diagnostics) writeSummary(w io.Writer, checks []DoctorCheck, note reportNote) {
	fmt.Fprintln(w, lang.headline(checks))
	fmt.Fprintln(w)
	for _, c := range groupChecks(checks) {
		name, problems := lang.categoryName(c.Key), c.problems()
		if len(problems) > 1 {
			name = lang.Text("cli.doctor.summary.category_count", map[string]any{"Name": name, "Count": len(problems)})
		}
		fmt.Fprintf(w, "%-4s  %s\n", cmdutil.Column(lang.Localizer, lang.Label(c.Status)), name)
		if len(problems) == 0 {
			continue
		}
		// Only the worst one is spelled out; the report lists the rest.
		fmt.Fprintf(w, "       %s: %s\n", problems[0].Name, reason(problems[0], max(maxLineColumns-problemIndent-uniseg.StringWidth(problems[0].Name)-2, 20)))
		if fix := lang.fixFor(problems[0]); fix != "" {
			fmt.Fprintf(w, "         %s\n", lang.Text("cli.doctor.summary.fix", map[string]any{"Fix": fix}))
		}
	}
	fmt.Fprintln(w)
	if note.Path != "" {
		fmt.Fprintln(w, lang.Text("cli.doctor.summary.report", map[string]any{"Path": note.Path}))
		fmt.Fprintln(w, lang.Text("cli.doctor.summary.report_commands", nil))
		return
	}
	fmt.Fprintln(w, lang.Text("cli.doctor.summary.no_report", map[string]any{"Reason": note.Reason}))
	fmt.Fprintln(w, lang.Text("cli.doctor.summary.no_report_commands", nil))
}
