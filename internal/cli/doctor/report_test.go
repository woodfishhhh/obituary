package doctor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/hook"
)

var reportLine = regexp.MustCompile(`(?m)^[^\n]*: (\S[^\n]*\.html)$`)

func sampleChecks() []DoctorCheck {
	caps := hook.Capabilities{DeleteGuard: hook.Support{Level: hook.Supported, Detail: "Bash tool"}}
	return []DoctorCheck{
		{Name: "path", Status: "WARN", Detail: `PATH resolves to C:\Users\me\bin\obit.exe; <b>typing obit</b> runs another executable`},
		{Name: "claude", Status: "PASS", Detail: "integration is current", Capabilities: &caps},
		{Name: "context-menu", Status: "WARN", Detail: "incomplete\nWARN *\\shell\\Obituary.Bury [] Bury now"},
		{Name: "store", Status: "PASS", Detail: "store.json is readable"},
	}
}

func TestReportIsSelfContainedAndKeepsEveryDetail(t *testing.T) {
	page, err := english.renderReport(sampleChecks(), time.Date(2026, 11, 24, 15, 4, 5, 0, time.UTC), `C:\data`, `C:\bin\obit.exe`)
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, banned := range []string{"http://", "https://", "<script", "<link", "<img", "@import", "url(", "src=", "href="} {
		if strings.Contains(html, banned) {
			t.Errorf("the report refers to something outside itself: %s", banned)
		}
	}
	for _, want := range []string{
		`<html lang="en">`, "Obituary doctor report", "Found 2 problems (4 checks).", `C:\data`, `C:\bin\obit.exe`, "2026-11-24 15:04:05",
		"PATH resolves to C:\\Users\\me\\bin\\obit.exe; &lt;b&gt;typing obit&lt;/b&gt; runs another executable", // escaped, and not cut at the first clause
		"WARN *\\shell\\Obituary.Bury [] Bury now",
		"delete guard: supported; Bash tool", "fix: obit init --context-menu --yes",
		"Installation", "Agents", "Explorer and right-click menu", "Data and settings", "uploaded anywhere",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	if strings.Contains(html, "<b>typing") {
		t.Error("check details are not escaped")
	}
	// Categories with a problem are open; the healthy one is collapsed.
	if strings.Count(html, "<details class=\"cat\" open>") != 2 || strings.Count(html, "<details class=\"cat\">") != 2 || !strings.Contains(html, "1 check<") {
		t.Errorf("open categories: %s", html)
	}
}

func TestReportFollowsTheInterfaceLanguage(t *testing.T) {
	page, err := chinese.renderReport(sampleChecks(), time.Now(), "d", "e")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<html lang="zh-CN">`, "讣告体检报告", "发现 2 个问题（共检查 4 项）。", "资源管理器与右键菜单", "没有上传到任何地方", "修复：obit init --context-menu --yes"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("report lacks %q", want)
		}
	}
}

func runDoctor(t *testing.T, executable cmdutil.Executable, args ...string) (string, string, error) {
	t.Helper()
	return clitest.Execute(t, NewCmdWithExecutable(executable, "claude", "codex"), args...)
}

func trackedReport(t *testing.T, out string) (string, ledger.Item) {
	t.Helper()
	match := reportLine.FindStringSubmatch(out)
	if match == nil {
		t.Fatalf("no report path in:\n%s", out)
	}
	state, err := (ledger.Store{Home: os.Getenv("OBIT_HOME")}).Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range state.Items {
		if item.Path == match[1] {
			return match[1], item
		}
	}
	t.Fatalf("%s is not tracked: %+v", match[1], state.Items)
	return "", ledger.Item{}
}

func TestDoctorSavesTheReportWithASevenDayLifespan(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	before := time.Now()
	out, stderr, err := runDoctor(t, executable)
	if err != nil || stderr != "" {
		t.Fatalf("%v: %s %s", err, out, stderr)
	}
	path, item := trackedReport(t, out)
	if filepath.Dir(path) != app.ReportsDir(filepath.Join(root, "data")) || !strings.HasPrefix(filepath.Base(path), "doctor-") {
		t.Errorf("report location: %s", path)
	}
	if got := item.ExpiresAt.Sub(before); got < app.ReportLifespan || got > app.ReportLifespan+time.Minute || app.ReportLifespan != 7*24*time.Hour {
		t.Errorf("lifespan %v", got)
	}
	page, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(page), "<!DOCTYPE html>") || !strings.Contains(string(page), "obitd") {
		t.Fatalf("report content: %v", err)
	}
	if !strings.Contains(out, "obit doctor --open") || !strings.Contains(out, "obit doctor --verbose") {
		t.Errorf("the summary does not say how to see more:\n%s", out)
	}
}

// An expired report is buried like any tracked file, even though it lives in
// the otherwise protected data directory.
func TestExpiredReportIsBuried(t *testing.T) {
	if testing.Short() {
		t.Skip("buries a file")
	}
	_, executable := clitest.DoctorFixture(t)
	out, _, err := runDoctor(t, executable)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := trackedReport(t, out)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	svc.Now = func() time.Time { return time.Now().Add(app.ReportLifespan + time.Minute) }
	outcomes, err := svc.Sweep(false)
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != core.ActionBuried {
		t.Fatalf("sweep: %+v %v", outcomes, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the expired report is still there: %v", err)
	}
}

func TestEachRunWritesAnotherReport(t *testing.T) {
	_, executable := clitest.DoctorFixture(t)
	seen := map[string]bool{}
	for range 3 {
		out, _, err := runDoctor(t, executable)
		if err != nil {
			t.Fatal(err)
		}
		path, _ := trackedReport(t, out)
		if seen[path] {
			t.Fatalf("%s was written twice", path)
		}
		seen[path] = true
	}
}

func TestDoctorDoesNotCreateTheDataDirectoryForAReport(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	t.Setenv("OBIT_HOME", filepath.Join(root, "missing", "data"))
	before := clitest.TreeSnapshot(t, root)
	out, _, err := runDoctor(t, executable)
	if err != nil || !strings.Contains(out, "Full report not saved: the data directory does not exist yet\nFull text here: obit doctor --verbose") {
		t.Fatalf("%v:\n%s", err, out)
	}
	if after := clitest.TreeSnapshot(t, root); len(after) != len(before) {
		t.Fatal("doctor created files for its report")
	}
}

// A report the ledger cannot take would never expire, so none is left behind.
func TestDoctorLeavesNoReportItCannotTrack(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	clitest.WriteFixture(t, filepath.Join(root, "data", "store.json"), "{")
	out, _, err := runDoctor(t, executable)
	if !errors.Is(err, cmdutil.ErrReported) || !strings.Contains(out, "Full report not saved") || !strings.Contains(out, "FAIL  Data and settings") {
		t.Fatalf("%v:\n%s", err, out)
	}
	if _, err := os.Stat(app.ReportsDir(filepath.Join(root, "data"))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a report or its folder was left behind: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "data", "store.json")); string(data) != "{" {
		t.Fatalf("the damaged ledger was changed: %q", data)
	}
}

func TestDoctorOpenStartsTheBrowserOnTheReport(t *testing.T) {
	_, executable := clitest.DoctorFixture(t)
	var opened []string
	old := openReport
	t.Cleanup(func() { openReport = old })
	openReport = func(path string) error { opened = append(opened, path); return nil }

	out, _, err := runDoctor(t, executable, "--open")
	if err != nil {
		t.Fatal(err)
	}
	path, _ := trackedReport(t, out)
	if len(opened) != 1 || opened[0] != path {
		t.Fatalf("opened %v, report %s", opened, path)
	}

	opened = nil
	out, _, err = runDoctor(t, executable, "--open", "--verbose")
	if err != nil || len(opened) != 1 || !strings.Contains(out, "claude") || strings.Contains(out, "Full report") {
		t.Fatalf("--open --verbose: %v %v\n%s", err, opened, out)
	}
	if _, _, err = runDoctor(t, executable, "--open", "--json"); err == nil || len(opened) != 1 {
		t.Fatalf("--open with --json: %v %v", err, opened)
	}
}

func TestDoctorOpenFailureIsAWarningNotAFailedCheck(t *testing.T) {
	_, executable := clitest.DoctorFixture(t)
	old := openReport
	t.Cleanup(func() { openReport = old })
	openReport = func(string) error { return errors.New("no browser") }
	out, stderr, err := runDoctor(t, executable, "--open")
	if err != nil || !strings.Contains(stderr, "no browser") || !strings.Contains(out, "Full report") {
		t.Fatalf("%v: %s | %s", err, out, stderr)
	}
}

func TestDoctorVerbosePrintsTheFullTextAndWritesNothing(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	before := clitest.TreeSnapshot(t, root)
	out, _, err := runDoctor(t, executable, "--verbose")
	if err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	checks := english.diagnose(time.Now(), executable, "claude", "codex")
	attachCapabilities(checks)
	english.writeVerbose(&want, checks)
	if out != want.String() {
		t.Fatalf("verbose output differs from the full text:\n%s\n---\n%s", out, want.String())
	}
	for _, row := range []string{"PASS path         ", "PASS obitd        ", "WARN claude       integration is missing"} {
		if !strings.Contains(out, row) {
			t.Errorf("missing the row format of earlier versions %q:\n%s", row, out)
		}
	}
	if after := clitest.TreeSnapshot(t, root); len(after) != len(before) {
		t.Fatal("--verbose wrote a report")
	}
}

func TestWriteNewFileNeverReplacesAnEarlierReport(t *testing.T) {
	dir := t.TempDir()
	first, err := writeNewFile(dir, "doctor-x", []byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := writeNewFile(dir, "doctor-x", []byte("two"))
	if err != nil || first == second {
		t.Fatalf("%s %s %v", first, second, err)
	}
	if data, _ := os.ReadFile(first); string(data) != "one" {
		t.Fatalf("the first report was replaced: %q", data)
	}
}
