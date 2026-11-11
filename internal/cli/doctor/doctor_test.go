//go:build windows

package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	daemoncmd "github.com/woodfishhhh/obituary/internal/cli/daemon"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/windows"
)

func TestDoctorReportsPausedProjectRules(t *testing.T) {
	if testing.Short() {
		t.Skip("project rule diagnosis uses a real Git repository")
	}
	root := t.TempDir()
	testenv.InitRepo(t, root)
	t.Chdir(root)
	clitest.WriteFixture(t, filepath.Join(root, ".obitrules"), "[broken 2h\n")
	checks := diagnoseProjectRules(t.TempDir())
	if len(checks) != 1 || checks[0].Status != "WARN" || !strings.Contains(checks[0].Detail, ".obitrules:1:") || !strings.Contains(checks[0].Detail, "automatic registration paused") {
		t.Fatalf("doctor hid paused rules: %+v", checks)
	}
}

func BenchmarkDoctor(b *testing.B) {
	root, executable := clitest.DoctorFixture(b)
	b.Chdir(root)
	b.ResetTimer()
	for b.Loop() {
		if _, _, err := clitest.Execute(b, NewCmdWithExecutable(executable), "--json"); err != nil {
			b.Fatal(err)
		}
	}
}

func TestDoctorNewChecksJSONAndTextReadOnly(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	home, path := filepath.Join(root, "data"), filepath.Join(root, "watched")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	id, err := (winfs.FS{}).Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := (core.Store{Home: home}).Update(func(st *core.State) error {
		st.Watches = []core.WatchPolicy{{ID: 1, Path: path, Identity: id, Consent: "pending"}}
		st.Items = []core.Item{{ID: 2, Path: filepath.Join(path, "download"), Source: "watch", CreatedAt: now.Add(-25 * time.Hour), Watch: &core.WatchTiming{StableSince: now}}}
		st.NextID = 3
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	index, err := json.Marshal(map[string]map[string]string{"directory": {"path": path}})
	if err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, filepath.Join(home, "explorer.json"), string(index))
	clitest.WriteFixture(t, filepath.Join(home, "explorer.log"), strings.Repeat("2026/11/10 12:00:00 Explorer labels: "+path+": locked\n", 2))
	clitest.WriteFixture(t, filepath.Join(root, "bin", "obit-gui.exe"), "never execute")
	cmdutil.RunnerStart = func(string) error { t.Fatal("doctor started a daemon"); return nil }
	oldProbe, calls := regtxnProbe, 0
	t.Cleanup(func() { regtxnProbe = oldProbe })
	regtxnProbe = func() error { calls++; return errors.New("KTM disabled") }
	before := clitest.TreeSnapshot(t, root)
	checks, err := doctorChecks(t, executable)
	if err != nil || len(checks) != 32 {
		t.Fatalf("doctor inventory: %d %v", len(checks), err)
	}
	if w := checks["watch"].Watch; w == nil || w.Policies != 1 || w.Pending != 1 || w.Unavailable != 0 || w.Unstable != 1 {
		t.Fatalf("watch JSON: %+v", w)
	}
	if e := checks["explorer"].Explorer; e == nil || !e.Enabled || e.CleanupTargets != 1 || e.RepeatedFailures != 1 || e.FirstFailure != path {
		t.Fatalf("explorer JSON: %+v", e)
	}
	if checks["gui"].GUI == nil || checks["gui"].Status != "PASS" || checks["registry-transactions"].KTM == nil || checks["registry-transactions"].KTM.Available {
		t.Fatalf("companion/KTM JSON: %+v", checks)
	}
	out, _, err := clitest.Execute(t, NewCmdWithExecutable(executable, "claude", "codex"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if err != nil || len(lines) != 32 || calls != 2 {
		t.Fatalf("doctor text/probe count: %d lines, %d probes, %v: %s", len(lines), calls, err, out)
	}
	for _, name := range []string{"watch", "explorer", "gui", "registry-transactions", "tray-startup", "context-menu", "start-menu", "desktop", "tray-icon", "taskbar", "registration-identities"} {
		if !strings.Contains(out, name+" ") {
			t.Fatalf("missing text row %s: %s", name, out)
		}
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
		t.Fatal("doctor changed data, labels, configuration or binaries")
	}
}

func TestDoctorInvalidHomeKeepsCheckInventory(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	t.Setenv("OBIT_HOME", filepath.Join(root, "data:stream"))
	before := clitest.TreeSnapshot(t, root)
	checks, err := doctorChecks(t, executable)
	if !errors.Is(err, cmdutil.ErrReported) || len(checks) != 32 {
		t.Fatalf("invalid home lost check rows: %d %v", len(checks), err)
	}
	for _, name := range []string{"runner", "tray-startup", "watch", "explorer", "registration-identities"} {
		if checks[name].Status != "FAIL" {
			t.Fatalf("invalid home check: %+v", checks[name])
		}
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
		t.Fatal("invalid home diagnosis wrote files")
	}
}

func TestDoctorCodexWindowsShell(t *testing.T) {
	for _, tc := range []struct{ value, status, detail string }{
		{"", "WARN", "unset"}, {"bash", "PASS", "bash"}, {"powershell", "PASS", "powershell"}, {"cmd", "FAIL", "codex_windows_shell"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("OBIT_CONFIG_DIR", t.TempDir())
			path, err := config.Path()
			if err != nil {
				t.Fatal(err)
			}
			clitest.WriteFixture(t, path, "codex_windows_shell = '"+tc.value+"'\n")
			got := diagnoseCodexWindowsShell()
			if got.Status != tc.status || !strings.Contains(got.Detail, tc.detail) {
				t.Fatalf("shell diagnosis: %+v", got)
			}
		})
	}
	t.Setenv("OBIT_CONFIG_DIR", t.TempDir())
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, path, "[agent_shells]\ncodex = 'bash'\n")
	if got := diagnoseCodexWindowsShell(); got.Status != "PASS" || !strings.Contains(got.Detail, "bash") {
		t.Fatalf("table declaration diagnosis: %+v", got)
	}
}

func TestDoctorWarnsForPurgeInProgress(t *testing.T) {
	svc, b := clitest.PurgingCommandFixture(t)
	c := diagnoseRecovery(svc.Store.Home)
	if c.Status != "WARN" || !strings.Contains(c.Detail, "1 graveyard records need attention") || !strings.Contains(c.Detail, "purging") || !strings.Contains(c.Detail, b.LastPurgeError) {
		t.Fatalf("doctor hid purge in progress: %+v", c)
	}
}

func TestDoctorWarnsForInvalidCleanupReceipt(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	root, executable := clitest.DoctorFixture(t)
	clitest.WriteFixture(t, filepath.Join(root, "data", "store.json"), `{"version":7,"nextId":8,"restoreCleanups":[{"kind":"resurrected","burial":{"id":7}}]}`)
	checks, err := doctorChecks(t, executable)
	if err != nil || checks["graveyard-recovery"].Status != "WARN" || !strings.Contains(checks["graveyard-recovery"].Detail, "cleanup paths do not name this ledger's grave") {
		t.Fatalf("doctor hid invalid cleanup receipt: %+v %v", checks["graveyard-recovery"], err)
	}
}

func TestDoctorWarnsForUnverifiedWatchDirectory(t *testing.T) {
	clitest.DoctorFixture(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "tree", "file.zip")
	clitest.WriteFixture(t, path, "fresh")
	rows, err := svc.Bury([]string{filepath.Dir(path)}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury: %+v %v", rows, err)
	}
	b := *rows[0].Burial
	b.Size, b.Source, b.Reason = 0, "watch", core.ReasonExpired
	data, err := json.Marshal(struct {
		Version int `json:"version"`
		core.Burial
		WatchCheck core.WatchSnapshot `json:"watchCheck"`
		WatchError string             `json:"watchError"`
	}{3, b, core.WatchSnapshot{Identity: *b.Identity, Directory: true}, "rollback destination is occupied"})
	if err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, filepath.Dir(b.Grave)+".json", string(data))
	if err := svc.Store.Update(func(st *core.State) error { st.Burials = nil; return nil }); err != nil {
		t.Fatal(err)
	}
	before := clitest.TreeSnapshot(t, svc.Store.Home)
	c := diagnoseRecovery(svc.Store.Home)
	if c.Status != "WARN" || !strings.Contains(c.Detail, "manual attention") || !strings.Contains(c.Detail, "occupied") {
		t.Fatalf("doctor hid watch rollback failure: %+v", c)
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, svc.Store.Home)) {
		t.Fatal("watch rollback diagnosis changed evidence")
	}
}

func TestDoctorWarnsForEmptyGrave(t *testing.T) {
	clitest.DoctorFixture(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "payload.txt")
	clitest.WriteFixture(t, path, "keep")
	rows, err := svc.Bury([]string{path}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury: %+v %v", rows, err)
	}
	if err := os.Remove(rows[0].Burial.Grave); err != nil {
		t.Fatal(err)
	}
	c := diagnoseRecovery(svc.Store.Home)
	if c.Status != "WARN" || !strings.Contains(c.Detail, "grave is empty") || !strings.Contains(c.Detail, "obit purge") {
		t.Fatalf("doctor hid empty grave: %+v", c)
	}
}

func TestDoctorSeesEmptyGraveWithCommittedManifest(t *testing.T) {
	clitest.DoctorFixture(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "payload.txt")
	clitest.WriteFixture(t, path, "keep")
	rows, err := svc.Bury([]string{path}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatal(rows, err)
	}
	b := *rows[0].Burial
	b.Size = 0
	data, err := json.Marshal(struct {
		Version int `json:"version"`
		core.Burial
	}{1, b})
	if err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, filepath.Dir(b.Grave)+".json", string(data))
	if err := os.Remove(b.Grave); err != nil {
		t.Fatal(err)
	}
	before := clitest.TreeSnapshot(t, svc.Store.Home)
	c := diagnoseRecovery(svc.Store.Home)
	if c.Status != "WARN" || !strings.Contains(c.Detail, "grave is empty") {
		t.Fatalf("committed manifest hid empty grave from doctor: %+v", c)
	}
	pending, err := svc.PendingGraves()
	if err != nil || len(pending) != 1 || pending[0].Status != "empty" {
		t.Fatalf("empty grave missing from listing: %+v %v", pending, err)
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, svc.Store.Home)) {
		t.Fatal("empty grave diagnosis changed evidence")
	}
}

func TestDisabledScheduleFailsDoctorAndIsNotScheduled(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	home := filepath.Join(root, "data")
	cmdutil.RunnerInstalled = func() bool { return true }
	cmdutil.RunnerTarget = func() (string, error) { return `D:\obitd.exe --home "` + home + `"`, app.ErrScheduleDisabled }
	checks, err := doctorChecks(t, executable)
	if !errors.Is(err, cmdutil.ErrReported) || checks["runner"].Status != "FAIL" || !strings.Contains(checks["runner"].Detail, "scheduled task is disabled: ") {
		t.Errorf("disabled task diagnosis: %+v, %v", checks["runner"], err)
	}
	cmdutil.RunnerRunning = func(string) bool { return true }
	if check := diagnoseRunner(home); check.Status != "FAIL" || !strings.Contains(check.Detail, "scheduled task is disabled: ") {
		t.Errorf("resident runner must not hide disabled task: %+v", check)
	}
	out, _, err := clitest.Execute(t, daemoncmd.StatusCmd(userdirs.Data), "--json")
	var status daemoncmd.DaemonStatus
	if err != nil || json.Unmarshal([]byte(out), &status) != nil || status.Mode == "scheduled" || status.ScheduleServesHome {
		t.Errorf("disabled task status: %s, %v", out, err)
	}
}

func TestDoctorReadOnlyAndIntegrationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	root, executable := clitest.DoctorFixture(t)
	before := clitest.TreeSnapshot(t, root)
	checks, err := doctorChecks(t, executable)
	if err != nil || checks["runner"].Status != "WARN" || checks["claude"].Status != "WARN" || !strings.Contains(checks["claude"].Detail, "missing") || checks["codex"].Status != "WARN" {
		t.Fatalf("fresh installation: %+v: %v", checks, err)
	}
	if after := clitest.TreeSnapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("doctor modified a fresh installation")
	}
	for _, agent := range []string{"claude", "codex"} {
		out, stderr, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--yes")
		if err != nil {
			t.Fatalf("init: %v: %s %s", err, out, stderr)
		}
	}
	before = clitest.TreeSnapshot(t, root)
	checks, err = doctorChecks(t, executable)
	if err != nil || checks["claude"].Status != "PASS" || checks["codex"].Status != "PASS" {
		t.Fatalf("installed: %+v: %v", checks, err)
	}
	out, _, err := clitest.Execute(t, NewCmdWithExecutable(executable, "claude", "codex"))
	if err != nil || !strings.Contains(out, "PASS") || !strings.Contains(out, "WARN") {
		t.Fatalf("text: %s: %v", out, err)
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
		t.Fatal("doctor modified the installed integration or store")
	}
	for _, path := range []string{filepath.Join(root, "claude", "settings.json"), filepath.Join(root, "codex", "hooks.json")} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		updated := strings.ReplaceAll(string(data), "obit hook", "C:/old/obit.exe hook")
		updated = strings.ReplaceAll(updated, filepath.ToSlash(filepath.Join(root, "bin", "obit.exe")), "C:/old/obit.exe")
		clitest.WriteFixture(t, path, updated)
	}
	before = clitest.TreeSnapshot(t, root)
	checks, err = doctorChecks(t, executable)
	for _, agent := range []string{"claude", "codex"} {
		if err != nil || checks[agent].Status != "WARN" || !strings.Contains(checks[agent].Detail, "outdated") {
			t.Fatalf("stale: %+v: %v", checks, err)
		}
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
		t.Fatal("doctor repaired an outdated integration")
	}
}

func TestDoctorFailuresAreReportedWithoutWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, tc := range []struct{ name, path, content, check string }{
		{"invalid store", "data/store.json", "{", "store"},
		{"newer store", "data/store.json", `{"version":999}`, "store"},
		{"duplicate ids", "data/store.json", `{"version":2,"items":[{"id":1},{"id":1}]}`, "store"},
		{"invalid config", "config/config.toml", "[graveyard", "config"},
		{"invalid config value", "config/config.toml", "[graveyard]\nretention_days = 0\n", "config"},
		{"invalid trust", "data/trusted_rules.json", "{", "trust"},
		{"invalid claude", "claude/settings.json", "{", "claude"},
		{"invalid codex", "codex/hooks.json", "{", "codex"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, executable := clitest.DoctorFixture(t)
			clitest.WriteFixture(t, filepath.Join(root, tc.path), tc.content)
			before := clitest.TreeSnapshot(t, root)
			checks, err := doctorChecks(t, executable)
			if !errors.Is(err, cmdutil.ErrReported) || checks[tc.check].Status != "FAIL" {
				t.Fatalf("%+v: %v", checks, err)
			}
			out, _, err := clitest.Execute(t, NewCmdWithExecutable(executable, "claude", "codex"))
			if !errors.Is(err, cmdutil.ErrReported) || !strings.Contains(out, "FAIL") {
				t.Fatalf("%s: %v", out, err)
			}
			if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
				t.Fatal("doctor changed damaged input")
			}
		})
	}
}

func TestDoctorPATHAndCompanion(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, tc := range []struct{ scenario, check, status, detail string }{
		{"missing PATH", "path", "WARN", "not found"},
		{"other executable", "path", "WARN", "another executable"},
		{"current executable unavailable", "path", "FAIL", "cannot locate executable"},
		{"missing companion", "obitd", "FAIL", ""},
		{"directory companion", "obitd", "FAIL", ""},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			root, executable := clitest.DoctorFixture(t)
			switch tc.scenario {
			case "missing PATH":
				t.Setenv("PATH", filepath.Join(root, "empty"))
			case "other executable":
				clitest.WriteFixture(t, filepath.Join(root, "other", "obit.exe"), "other")
				t.Setenv("PATH", filepath.Join(root, "other"))
			case "current executable unavailable":
				executable = func() (string, error) { return "", errors.New("cannot locate executable") }
			default:
				path := filepath.Join(root, "bin", "obitd.exe")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if tc.scenario == "directory companion" {
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				}
			}
			checks, err := doctorChecks(t, executable)
			if checks[tc.check].Status != tc.status || tc.status == "FAIL" && !errors.Is(err, cmdutil.ErrReported) || tc.status == "WARN" && err != nil {
				t.Fatalf("%+v: %v", checks, err)
			}
			if !strings.Contains(checks[tc.check].Detail, tc.detail) {
				t.Fatalf("missing impact detail: %+v", checks[tc.check])
			}
			if tc.status == "WARN" && (!strings.Contains(checks[tc.check].Detail, "hooks use an absolute path") || !strings.Contains(checks[tc.check].Detail, "typing obit")) {
				t.Fatalf("PATH warning must explain hook and manual invocation: %+v", checks[tc.check])
			}
		})
	}
}

func TestDoctorRunnerStates(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, tc := range []struct {
		name                       string
		installed, serves, running bool
		target, want               string
	}{
		{"none", false, false, false, "", "WARN"},
		{"resident", false, false, true, "", "PASS"},
		{"scheduled", true, true, false, "bin/obitd.exe", "PASS"},
		{"other home", true, false, false, "bin/obitd.exe", "FAIL"},
		{"gone executable", true, true, false, "gone/obitd.exe", "FAIL"},
		{"wrong executable", true, true, false, "bin/obit.exe", "FAIL"},
		{"resident covers broken task", true, false, true, "bin/obitd.exe", "WARN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, executable := clitest.DoctorFixture(t)
			cmdutil.RunnerInstalled = func() bool { return tc.installed }
			cmdutil.RunnerServes = func(string) bool { return tc.serves }
			cmdutil.RunnerRunning = func(string) bool { return tc.running }
			cmdutil.RunnerTarget = func() (string, error) {
				return windows.ComposeCommandLine([]string{filepath.Join(root, tc.target), "--home", filepath.Join(root, "data")}), nil
			}
			checks, err := doctorChecks(t, executable)
			if checks["runner"].Status != tc.want || (tc.want == "FAIL") != errors.Is(err, cmdutil.ErrReported) {
				t.Fatalf("%+v: %v", checks, err)
			}
		})
	}
}

func TestDoctorDoesNotCreateMissingDataOrLocks(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	home := filepath.Join(root, "missing", "data")
	t.Setenv("OBIT_HOME", home)
	before := clitest.TreeSnapshot(t, root)
	checks, err := doctorChecks(t, executable)
	if err != nil || checks["data"].Status != "WARN" || checks["store"].Status != "PASS" {
		t.Fatalf("%+v: %v", checks, err)
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
		t.Fatal("doctor created data or lock files")
	}
	clitest.WriteFixture(t, filepath.Join(root, "not-a-directory"), "file")
	t.Setenv("OBIT_HOME", filepath.Join(root, "not-a-directory"))
	checks, err = doctorChecks(t, executable)
	if !errors.Is(err, cmdutil.ErrReported) || checks["data"].Status != "FAIL" {
		t.Fatalf("%+v: %v", checks, err)
	}
}

func TestDoctorCountsOnlyRecentHookErrorsIncludingRotation(t *testing.T) {
	root, _ := clitest.DoctorFixture(t)
	now := time.Now().Truncate(time.Second)
	line := func(at time.Time) string { return at.Format(time.RFC3339) + " hook error\r\n" }
	clitest.WriteFixture(t, filepath.Join(root, "data", "hook.log"), line(now)+line(now.Add(-24*time.Hour))+line(now.Add(-25*time.Hour))+line(now.Add(time.Hour)))
	clitest.WriteFixture(t, filepath.Join(root, "data", "hook.log.1"), line(now.Add(-time.Hour)))
	clitest.WriteFixture(t, filepath.Join(root, "data", "hook-audit.log"), line(now))
	home, _ := userdirs.Data()
	got := diagnoseHookLog(home, now)
	if got.Status != "WARN" || got.Errors != 3 {
		t.Fatalf("recent errors: %+v", got)
	}
	if got := diagnoseHookLog(home, now.Add(49*time.Hour)); got.Status != "PASS" || got.Errors != 0 {
		t.Fatalf("expired errors: %+v", got)
	}
	if err := os.Remove(filepath.Join(home, "hook.log")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(home, "hook.log"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := diagnoseHookLog(home, now); got.Status != "FAIL" {
		t.Fatalf("unreadable log: %+v", got)
	}
}

func TestDoctorDetectsDeniedDirectoryWriteAccessWithoutAProbeFile(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	home := filepath.Join(root, "data")
	original, err := windows.GetNamedSecurityInfo(home, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	originalACL, _, err := original.DACL()
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := user.User.Sid.String()
	denied, err := windows.SecurityDescriptorFromString("D:(D;;0x6;;;" + sid + ")(A;;FA;;;" + sid + ")")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := denied.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(home, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := windows.SetNamedSecurityInfo(home, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, originalACL, nil); err != nil {
			t.Error(err)
		}
	})
	before := clitest.TreeSnapshot(t, root)
	checks, err := doctorChecks(t, executable)
	if !errors.Is(err, cmdutil.ErrReported) || checks["data"].Status != "FAIL" || checks["store"].Status != "PASS" {
		t.Fatalf("%+v: %v", checks, err)
	}
	for _, want := range []string{home, "check data access", "hook continues", "same account", "OBIT_HOME"} {
		if !strings.Contains(checks["data"].Detail, want) {
			t.Errorf("permission diagnosis missing %q: %+v", want, checks["data"])
		}
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
		t.Fatal("doctor modified the unwritable data directory")
	}
}

func TestDoctorDetectsDeniedLockAndSessionsWithoutWrites(t *testing.T) {
	for _, name := range []string{"store.lock", "sessions"} {
		t.Run(name, func(t *testing.T) {
			root, executable := clitest.DoctorFixture(t)
			home := filepath.Join(root, "data")
			path := filepath.Join(home, name)
			if name == "sessions" {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			} else {
				clitest.WriteFixture(t, path, "lock remains unchanged")
			}
			clitest.DenyAccess(t, path, windows.FILE_WRITE_DATA)
			before := clitest.TreeSnapshot(t, root)
			checks, err := doctorChecks(t, executable)
			if !errors.Is(err, cmdutil.ErrReported) || checks["data"].Status != "FAIL" || !strings.Contains(checks["data"].Detail, path) || !strings.Contains(checks["data"].Detail, "hook continues") {
				t.Fatalf("denied data path not diagnosed: %+v %v", checks["data"], err)
			}
			if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
				t.Fatal("doctor wrote to data or created a probe")
			}
		})
	}
}

func TestDoctorIgnoresSkippedAndLegacyDiscoveryDenials(t *testing.T) {
	for _, reason := range []string{"Access is denied.", "access denied", "permission denied", "拒绝访问。"} {
		t.Run(reason, func(t *testing.T) {
			home := t.TempDir()
			now := time.Now().Truncate(time.Second)
			stamp := now.Format(time.RFC3339) + " "
			message := fmt.Sprintf("discover project from %q: resolve project directory %s: %s", `C:\fixture\app\resources`, `C:\fixture`, reason)
			legacy := fmt.Sprintf(`codex SessionEnd: %q; session="test" cwd="C:\\fixture\\app\\resources" home="C:\\data" user="fixture" sid="fixture-sid" pid=1; `, message) +
				hook.PermissionHint + `; permission_diagnostic={"user":"fixture","sid":"fixture-sid","parent_image":"wsl.exe","operation":"resolve project directory","path":"C:\\fixture"}`
			clitest.WriteFixture(t, filepath.Join(home, "hook.log.1"), stamp+legacy+"\n")
			clitest.WriteFixture(t, filepath.Join(home, "hook.log"), stamp+`skip: project discovery unavailable; cwd="C:\\fixture" reason="permission denied"`+"\n")
			before := clitest.TreeSnapshot(t, home)
			got := diagnoseHookLog(home, now)
			if got.Status != "PASS" || got.Errors != 0 || strings.Contains(got.Detail, "denied access") {
				t.Fatalf("unavailable cwd warned about hook account access: %+v", got)
			}
			if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, home)) {
				t.Fatal("hook-log diagnostic changed legacy logs")
			}
			clitest.WriteFixture(t, filepath.Join(home, "hook.log"), stamp+`codex Stop: "open C:\\data\\store.lock: Access is denied."; permission_diagnostic={"operation":"open","path":"C:\\data\\store.lock"}`+"\n")
			got = diagnoseHookLog(home, now)
			if got.Status != "WARN" || got.Errors != 1 || !strings.Contains(got.Detail, "store.lock") {
				t.Fatalf("cwd skip hid a genuine ledger denial: %+v", got)
			}
		})
	}
}

func TestDoctorKeepsOtherDiscoveryAndPermissionErrors(t *testing.T) {
	for _, tc := range []struct{ message, diagnostic string }{
		{`discover project from "C:\fixture": resolve project directory C:\fixture: path changed`, `{"operation":"resolve project directory","path":"C:\\fixture"}`},
		{`discover project from "C:\fixture": resolve project directory C:\fixture: context deadline exceeded`, `{"operation":"resolve project directory","path":"C:\\fixture"}`},
		{`discover project from "C:\fixture": resolve project directory C:\fixture: context canceled`, `{"operation":"resolve project directory","path":"C:\\fixture"}`},
		{`discover project from "C:\fixture": open C:\data\store.lock: Access is denied.`, `{"operation":"open","path":"C:\\data\\store.lock"}`},
		{`snapshot project "C:\fixture": resolve project directory C:\fixture: Access is denied.`, `{"operation":"resolve project directory","path":"C:\\fixture"}`},
		{`discover project from "C:\fixture": resolve project directory C:\fixture: Access is denied.`, `{broken`},
	} {
		t.Run(tc.message+tc.diagnostic, func(t *testing.T) {
			home := t.TempDir()
			now := time.Now().Truncate(time.Second)
			line := fmt.Sprintf("%s codex SessionEnd: %q; permission_diagnostic=%s\n", now.Format(time.RFC3339), tc.message, tc.diagnostic)
			clitest.WriteFixture(t, filepath.Join(home, "hook.log"), line)
			got := diagnoseHookLog(home, now)
			if got.Status != "WARN" || got.Errors != 1 {
				t.Fatalf("unrelated error disappeared: %+v", got)
			}
		})
	}
}

func TestDoctorSurfacesRecentHookAccountDenial(t *testing.T) {
	root, _ := clitest.DoctorFixture(t)
	home := filepath.Join(root, "data")
	now := time.Now().Truncate(time.Second)
	hook.LogEventError(home, "codex", hook.Event{Name: "SessionStart", Session: "denied", Cwd: root},
		&os.PathError{Op: "open", Path: filepath.Join(home, "store.lock"), Err: os.ErrPermission})
	// The current doctor's access can succeed after a different account failed.
	if got := diagnoseData(home); got.Status != "PASS" {
		t.Fatal(got)
	}
	got := diagnoseHookLog(home, now.Add(time.Second))
	if got.Status != "WARN" || got.Errors != 1 || !strings.Contains(got.Detail, "same account") || !strings.Contains(got.Detail, "OBIT_HOME") {
		t.Fatalf("recent denied hook account was hidden: %+v", got)
	}
	for _, want := range []string{`"user":`, `"sid":`, `"integrity_sid":`, `"parent_image":`, "store.lock", hook.PermissionReportHint} {
		if !strings.Contains(got.Detail, want) {
			t.Errorf("latest denial summary missing %q: %+v", want, got)
		}
	}
	if got := diagnoseHookLog(home, now.Add(25*time.Hour)); got.Status != "PASS" || strings.Contains(got.Detail, "same account") {
		t.Fatalf("expired permission failure still displayed: %+v", got)
	}
}

func TestDoctorSelectsLatestPermissionFailureAcrossRotatedLogs(t *testing.T) {
	home := t.TempDir()
	now := time.Now().Truncate(time.Second)
	line := func(at time.Time, message string) string { return at.Format(time.RFC3339) + " " + message + "\n" }
	current := line(now.Add(-2*time.Hour), "codex Stop: Access is denied.") +
		line(now.Add(-time.Minute), "claude: input: invalid JSON") +
		line(now.Add(time.Hour), "codex Stop: permission denied")
	rotated := line(now.Add(-time.Hour), "codex SessionStart: permission denied") +
		line(now.Add(-25*time.Hour), "codex Stop: Access is denied.")
	clitest.WriteFixture(t, filepath.Join(home, "hook.log"), current)
	clitest.WriteFixture(t, filepath.Join(home, "hook.log.1"), rotated)
	before := clitest.TreeSnapshot(t, home)
	got := diagnoseHookLog(home, now)
	if got.Status != "WARN" || got.Errors != 3 || !strings.Contains(got.Detail, "latest denied access at "+now.Add(-time.Hour).Format(time.RFC3339)) {
		t.Fatalf("latest permission error was not selected by timestamp: %+v", got)
	}
	if !strings.Contains(got.Detail, "identity and failed path unavailable") || !strings.Contains(got.Detail, hook.PermissionReportHint) {
		t.Fatalf("legacy diagnosis invented identity or omitted next step: %+v", got)
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, home)) {
		t.Fatal("hook-log diagnostic wrote files")
	}
}

func TestDoctorSelectsLaterPermissionRecordWithinSameSecond(t *testing.T) {
	home := t.TempDir()
	now := time.Now().Truncate(time.Second)
	stamp := now.Format(time.RFC3339) + " "
	// Background tick and the event itself can fail in the same second. The
	// current file is newer than the rotated file even when their timestamps tie.
	current := stamp + "background tick: Access is denied.\n" + stamp +
		`codex SessionStart: denied; permission_diagnostic={"user":"current-user","sid":"current-sid","operation":"open","path":"C:\\fixture\\store.lock"}` + "\n"
	clitest.WriteFixture(t, filepath.Join(home, "hook.log"), current)
	clitest.WriteFixture(t, filepath.Join(home, "hook.log.1"), stamp+"codex Stop: Access is denied.\n")
	got := diagnoseHookLog(home, now)
	if got.Status != "WARN" || got.Errors != 3 || !strings.Contains(got.Detail, "current-user") || !strings.Contains(got.Detail, "store.lock") {
		t.Fatalf("same-second diagnostic was hidden by an earlier record: %+v", got)
	}
}

func TestDoctorDetectsUnreadableStoreAndTrustFiles(t *testing.T) {
	for _, tc := range []struct{ name, file string }{{"store", "store.json"}, {"trust", "trusted_rules.json"}} {
		t.Run(tc.name, func(t *testing.T) {
			root, executable := clitest.DoctorFixture(t)
			path := filepath.Join(root, "data", tc.file)
			clitest.WriteFixture(t, path, `{}`)
			p, err := windows.UTF16PtrFromString(path)
			if err != nil {
				t.Fatal(err)
			}
			handle, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer windows.CloseHandle(handle)
			checks, err := doctorChecks(t, executable)
			if !errors.Is(err, cmdutil.ErrReported) || checks[tc.name].Status != "FAIL" {
				t.Fatalf("%+v: %v", checks, err)
			}
		})
	}
}

func TestDoctorWarnsForGraveWithoutManifest(t *testing.T) {
	svc, _, p := clitest.PendingCommandBurial(t, true)
	if err := os.Remove(p.Manifest); err != nil {
		t.Fatal(err)
	}
	check := diagnoseRecovery(svc.Store.Home)
	if check.Status != "WARN" || !strings.Contains(check.Detail, "without a matching manifest") {
		t.Fatalf("unindexed grave not diagnosed: %+v", check)
	}
}

func TestDoctorWarnsForConflictingManifestRetention(t *testing.T) {
	svc, _, p := clitest.PendingCommandBurial(t, false)
	if err := svc.Store.Update(func(st *core.State) error {
		b := *p.Burial
		b.PurgeAt = b.BuriedAt.Add(time.Second)
		st.Burials = append(st.Burials, b)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	check := diagnoseRecovery(svc.Store.Home)
	if check.Status != "WARN" || !strings.Contains(check.Detail, "conflicts") {
		t.Fatalf("conflict not diagnosed: %+v", check)
	}
	if _, err := os.Stat(p.Manifest); err != nil {
		t.Fatal("doctor modified manifest", err)
	}
}
