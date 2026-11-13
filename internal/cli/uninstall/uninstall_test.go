package uninstall

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/explorer"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

func TestUninstallPrintsScheduleOwnershipPreview(t *testing.T) {
	testenv.WindowsOnly(t, "Windows desktop shortcuts, Explorer menu, or Run registration")
	for _, preview := range []string{"remove scheduled task owned", "preserve scheduled task: task belongs to another installation (C:\\other, C:\\home)", "query failed for scheduled task: denied"} {
		t.Run(preview[:8], func(t *testing.T) {
			d, _, calls := fixture(t)
			d.previewSchedule = func(daemon, home, config string) string {
				if !strings.HasSuffix(daemon, "obitd.exe") || home != os.Getenv("OBIT_HOME") || config != os.Getenv("OBIT_CONFIG_DIR") {
					t.Fatal("preview lost installation scope")
				}
				return preview
			}
			out, err := execute(t, d, "", "--dry-run")
			if err != nil || !strings.Contains(out, preview) || len(*calls) != 0 {
				t.Fatalf("preview: %s %v calls=%v", out, err, *calls)
			}
		})
	}
}

func TestReviewUninstallPreservesForeignReference(t *testing.T) {
	d, _, _ := fixture(t)
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	write(t, filepath.Join(dir, "OBIT.md"), "USER AUTHORED INSTRUCTIONS")
	write(t, filepath.Join(dir, "CLAUDE.md"), "@OBIT.md\n")
	_, _ = execute(t, d, "", "--yes")
	if data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md")); err != nil || string(data) != "@OBIT.md\n" {
		t.Fatalf("foreign reference removed: %q %v", data, err)
	}
}

func TestReviewUninstallPreservesQuotedOwnershipMarker(t *testing.T) {
	d, _, _ := fixture(t)
	path := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "OBIT.md")
	original := "# USER NOTES\n```markdown\n" + integrate.Awareness + "```\n"
	write(t, path, original)
	_, _ = execute(t, d, "", "--yes")
	if data, err := os.ReadFile(path); err != nil || string(data) != original {
		t.Fatalf("quoted marker claimed user file: %q %v", data, err)
	}
}

func TestReviewUninstallReportsMalformedHookConfig(t *testing.T) {
	d, _, _ := fixture(t)
	planner := initcmd.Planner{Executable: d.executable}
	changes, err := planner.IntegrationPlans([]string{"claude"}, integrate.Scope{Global: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := integrate.Apply(changes); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, string(data)+"INVALID EDIT")
	out, err := execute(t, d, "", "--yes")
	if err == nil || strings.Contains(out, "Uninstalled integrations") {
		t.Fatalf("malformed config reported success: %s %v", out, err)
	}
}

func TestUninstallDryRunAndDefaultPreserveData(t *testing.T) {
	testenv.WindowsOnly(t, "This fixture installs Windows Run and Explorer registrations; POSIX data preservation has separate coverage")
	d, root, calls := fixture(t)
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	planner := initcmd.Planner{Executable: d.executable}
	for _, scope := range []integrate.Scope{{Global: true, Skill: true}, {Project: root, MCP: true, Skill: true}} {
		changes, err := planner.IntegrationPlans([]string{"claude", "codex"}, scope, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := integrate.Apply(changes); err != nil {
			t.Fatal(err)
		}
	}
	exe, _ := d.executable.Path()
	home, config := os.Getenv("OBIT_HOME"), os.Getenv("OBIT_CONFIG_DIR")
	startup, err := app.PlanAutostart(d.runRegistry, filepath.Join(filepath.Dir(exe), "obitd.exe"), home, config, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := startup.Apply(); err != nil {
		t.Fatal(err)
	}
	menus, err := contextmenu.Plan(d.menuRegistry, exe, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := contextmenu.Apply(d.menuRegistry, menus); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(home, "user-note.txt"), "KEEP USER DATA")
	before := snapshot(t, root)
	out, err := execute(t, d, "", "--dry-run", "--purge-data")
	if err != nil {
		t.Fatal(out, err)
	}
	for _, want := range []string{"global", root, "claude", "codex", "HKCU Run", "context menu", "scheduled task", "resident/tray", "PURGE", "graveyard:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) || len(*calls) != 0 {
		t.Fatal("dry-run changed state")
	}
	out, err = execute(t, d, "", "--yes")
	if err != nil {
		t.Fatal(out, err)
	}
	if len(*calls) != 2 || len(d.menuRegistry.(menuRegistry)) != 0 {
		t.Fatal(*calls, d.menuRegistry)
	}
	if value, err := d.runRegistry.Get(startup.Name); err != nil || value != "" {
		t.Fatal(value, err)
	}
	if data, err := os.ReadFile(filepath.Join(home, "user-note.txt")); err != nil || string(data) != "KEEP USER DATA" {
		t.Fatal(string(data), err)
	}
	if !strings.Contains(out, "KEEP data") {
		t.Fatal(out)
	}
	for _, scope := range []integrate.Scope{{Global: true}, {Project: root}} {
		changes, err := planner.IntegrationPlans([]string{"claude", "codex"}, scope, true)
		if err != nil || integrate.Pending(changes) {
			t.Fatal("integration remains", changes, err)
		}
	}
}

func TestUninstallConfirmationAndAbsentData(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		terminal    bool
		args        []string
		wantErr     bool
	}{
		{"noninteractive", "", false, nil, true},
		{"purge requires yes", "", false, []string{"--purge-data"}, true},
		{"decline", "n\n", true, nil, true},
		{"decline purge", "yes\nno\n", true, []string{"--purge-data"}, true},
		{"confirm both", "yes\nyes\n", true, []string{"--purge-data"}, false},
		{"dry", "", false, []string{"--dry-run"}, false},
		{"default", "", false, []string{"--yes"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, root, calls := fixture(t)
			d.terminal = func() bool { return tc.terminal }
			before := snapshot(t, root)
			out, err := execute(t, d, tc.input, tc.args...)
			if (err != nil) != tc.wantErr {
				t.Fatal(out, err)
			}
			if tc.wantErr && len(*calls) != 0 {
				t.Fatal("modified before confirmation", *calls)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("created data on empty uninstall", out)
			}
		})
	}
}

func TestUninstallRemovesLabelsAndPoliciesWithoutMovingFiles(t *testing.T) {
	testenv.WindowsOnly(t, "Explorer labels are Windows-only")
	d, root, _ := fixture(t)
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	svc.AfterChange = nil
	dir := filepath.Join(root, "downloads")
	path := filepath.Join(dir, "report.txt")
	write(t, path, "USER FILE")
	items, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetWatch(dir, "1d", core.WatchFirstSeen); err != nil {
		t.Fatal(err)
	}
	if err := explorer.Sync(svc, explorer.Options{Enabled: true, Everywhere: true}); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	out, err := execute(t, d, "", "--dry-run")
	if err != nil || !strings.Contains(out, "folder policy") || !strings.Contains(out, "desktop.ini") {
		t.Fatal(out, err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("label dry-run wrote files")
	}
	out, err = execute(t, d, "", "--yes")
	if err != nil {
		t.Fatal(out, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "USER FILE" {
		t.Fatal(string(data), err)
	}
	st, err := svc.Store.Inspect()
	if err != nil || len(st.Watches) != 0 || len(st.Items) != 1 || st.Items[0].ID != items[0].Item.ID {
		t.Fatal(st, err)
	}
	if targets, err := explorer.CleanupTargets(svc.Store.Home); err != nil || len(targets) != 0 {
		t.Fatal(targets, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "desktop.ini")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("label survived", err)
	}
}

func TestUninstallStopFailureKeepsData(t *testing.T) {
	d, root, _ := fixture(t)
	path := filepath.Join(root, "OBIT_HOME", "store.json")
	write(t, path, `{"version":11,"nextId":1}`)
	d.stop = func(string) error { return errors.New("fake tray would not stop") }
	out, err := execute(t, d, "", "--yes", "--purge-data")
	if err == nil || !strings.Contains(err.Error(), "would not stop") {
		t.Fatal(out, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("removed data after stop failure", err)
	}
}
