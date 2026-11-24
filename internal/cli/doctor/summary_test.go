package doctor

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rivo/uniseg"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

var chinese = diagnostics{i18n.NewLocalizer(i18n.Chinese), false}

func summaryOf(lang diagnostics, checks []DoctorCheck, note reportNote) string {
	var out bytes.Buffer
	lang.writeSummary(&out, checks, note)
	return out.String()
}

func TestSummaryOfAHealthyMachineIsOneLinePerCategory(t *testing.T) {
	checks := []DoctorCheck{
		{Name: "path", Status: "PASS", Detail: "PATH resolves to the current executable: /x"},
		{Name: "update", Status: "INFO", Detail: "check=true"},
		{Name: "runner", Status: "PASS", Detail: "scheduled task serves this data directory"},
		{Name: "claude", Status: "PASS", Detail: "integration is current"},
		{Name: "claude-guard-rm", Status: "PASS", Detail: "registered: no"},
		{Name: "store", Status: "PASS", Detail: "readable"},
	}
	out := summaryOf(english, checks, reportNote{Path: "/data/reports/doctor.html"})
	want := []string{
		"Everything looks fine (6 checks).", "",
		"PASS  Installation", "PASS  Background runner", "PASS  Agents", "PASS  Data and settings", "",
		"Full report (removed after 1 day): /data/reports/doctor.html",
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != len(want)+1 {
		t.Fatalf("%d lines: %s", len(lines), out)
	}
	for i, line := range want {
		if lines[i] != line {
			t.Errorf("line %d = %q, want %q", i, lines[i], line)
		}
	}
	if !strings.Contains(lines[len(lines)-1], "obit doctor --open") || !strings.Contains(lines[len(lines)-1], "obit doctor --verbose") {
		t.Errorf("the last line does not say how to read more: %q", lines[len(lines)-1])
	}
}

func TestSummaryExpandsOnlyTheWorstProblemOfEachCategory(t *testing.T) {
	checks := []DoctorCheck{
		{Name: "path", Status: "WARN", Detail: "PATH resolves to /other/obit; hooks use an absolute path"},
		{Name: "obitd", Status: "FAIL", Detail: "companion is missing; reinstall"},
		{Name: "gui", Status: "WARN", Detail: "cannot verify GUI companion"},
		{Name: "runner", Status: "WARN", Detail: "no runner serves this data directory; obit daemon enable installs one"},
		{Name: "claude", Status: "WARN", Detail: "integration is missing"},
		{Name: "store", Status: "PASS", Detail: "readable"},
	}
	out := summaryOf(english, checks, reportNote{Path: "/r.html"})
	for _, want := range []string{
		"Found 5 problems (6 checks).",
		"FAIL  Installation · 3 problems\n       obitd: companion is missing\n         fix: keep obit and obitd in one folder, or run the installer again\n",
		"WARN  Background runner\n       runner: no runner serves this data directory\n         fix: obit daemon enable\n",
		"WARN  Agents\n       claude: integration is missing\n         fix: obit init -g --agent claude\n",
		"PASS  Data and settings\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "PATH resolves") || strings.Contains(out, "reinstall") {
		t.Errorf("only the first clause of the worst problem belongs in the summary:\n%s", out)
	}
}

func TestSummaryCountsASingleProblemInTheSingular(t *testing.T) {
	out := summaryOf(english, []DoctorCheck{{Name: "config", Status: "FAIL", Detail: "config.toml:2: unknown key"}}, reportNote{Path: "/r.html"})
	if !strings.HasPrefix(out, "Found 1 problem (1 checks).") || !strings.Contains(out, "fix: obit config") {
		t.Fatal(out)
	}
}

func TestSummaryAnInformationOnlyCategoryIsNotReportedAsPassing(t *testing.T) {
	out := summaryOf(english, []DoctorCheck{{Name: "context-menu", Status: "INFO", Detail: "Windows only"}}, reportNote{Path: "/r.html"})
	if !strings.Contains(out, "INFO  Explorer and right-click menu") || !strings.HasPrefix(out, "Everything looks fine") {
		t.Fatal(out)
	}
}

func TestSummaryWithoutAReportSaysWhyAndHowToGetTheText(t *testing.T) {
	out := summaryOf(english, []DoctorCheck{{Name: "path", Status: "PASS"}}, reportNote{Reason: "the data directory does not exist yet"})
	if !strings.Contains(out, "Full report not saved: the data directory does not exist yet\nFull text here: obit doctor --verbose") {
		t.Fatal(out)
	}
}

func TestSummaryFollowsTheInterfaceLanguage(t *testing.T) {
	out := summaryOf(chinese, []DoctorCheck{
		{Name: "runner", Status: "WARN", Detail: "计划任务服务的不是这个数据目录；其余"},
		{Name: "store", Status: "PASS"},
	}, reportNote{Path: "/r.html"})
	for _, want := range []string{"发现 1 个问题（共检查 2 项）。", "警告  后台运行", "       runner: 计划任务服务的不是这个数据目录\n", "修复：obit daemon enable", "通过  数据与设置", "完整报告（1 天后自动清理）：/r.html"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestReasonKeepsTheFirstClauseWithinTheLimit(t *testing.T) {
	for _, tc := range []struct {
		detail  string
		columns int
		want    string
	}{
		{"first; second", 80, "first"},
		{"第一；第二", 80, "第一"},
		{"line one\nline two", 80, "line one"},
		{"  padded  ", 80, "padded"},
		{"abcdefghij", 10, "abcdefghij"},
		{"abcdefghijk", 10, "abcdefghi…"},
		{strings.Repeat("长", 20), 10, "长长长长…"}, // each is two columns wide; the ellipsis takes one
	} {
		if got := reason(DoctorCheck{Detail: tc.detail}, tc.columns); got != tc.want {
			t.Errorf("reason(%q, %d) = %q, want %q", tc.detail, tc.columns, got, tc.want)
		}
	}
}

// Every check the diagnosis can produce belongs to a named category, so none
// ends up under "Other" and nothing is left out of the summary.
func TestEveryCheckHasACategory(t *testing.T) {
	_, executable := doctorAgentFixture(t)
	checks := english.diagnose(time.Now(), executable)
	grouped := 0
	for _, c := range groupChecks(checks) {
		if c.Key == "other" {
			t.Errorf("uncategorized checks: %+v", c.Checks)
		}
		grouped += len(c.Checks)
	}
	if grouped != len(checks) {
		t.Fatalf("%d of %d checks are in a category", grouped, len(checks))
	}
	for _, name := range []string{"cursor", "cursor-guard-rm", "claude-guard-rm"} {
		if categoryOf(name) != "agents" {
			t.Errorf("%s is not an agent check", name)
		}
	}
	if categoryOf("made-up") != "other" {
		t.Error("unknown names should not be hidden in a real category")
	}
}

func TestCompactOutputOfAFreshMachineFitsOnOneScreen(t *testing.T) {
	_, executable := clitest.DoctorFixture(t)
	out, _, err := clitest.Execute(t, NewCmdWithExecutable(executable, "claude", "codex"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(out, "\n"); lines > 20 {
		t.Fatalf("%d lines:\n%s", lines, out)
	}
}

func TestSummaryLinesDoNotOutgrowACommonWindow(t *testing.T) {
	long := `GetFileAttributesEx C:\Users\someone\orca\workspaces\obituary\hawksbill\bin\obitd.exe: ` + strings.Repeat("找不到文件 ", 20)
	for _, lang := range []diagnostics{english, chinese} {
		out := summaryOf(lang, []DoctorCheck{{Name: "obitd", Status: "FAIL", Detail: long}, {Name: "tray-startup", Status: "WARN", Detail: long}}, reportNote{Path: "/r.html"})
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "       ") && uniseg.StringWidth(line) > maxLineColumns {
				t.Errorf("%d columns: %s", uniseg.StringWidth(line), line)
			}
		}
	}
}
