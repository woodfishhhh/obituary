//go:build windows

package init_test

import (
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/shortcut"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitDefaultDesktopPlanAndOptOut(t *testing.T) {
	clitest.CommandDir(t)
	root, exe := clitest.DoctorFixture(t)
	clitest.WriteFixture(t, filepath.Join(root, "bin", "obit-gui.exe"), "fixture")
	for _, tc := range []struct {
		args            []string
		menu, shortcuts bool
	}{
		{nil, true, true}, {[]string{"--no-context-menu"}, false, true}, {[]string{"--no-shortcuts"}, true, false},
	} {
		out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(exe), append(tc.args, "--dry-run")...)
		if err != nil {
			t.Fatal(err)
		}
		for text, want := range map[string]bool{"context-menu:": tc.menu, "start-menu:": tc.shortcuts, "desktop:": tc.shortcuts, "HKCU Run": true} {
			if strings.Contains(out, text) != want {
				t.Fatalf("%v missing/wrong %s: %s", tc.args, text, out)
			}
		}
	}
}

func TestInitShortcutDirectoriesContainOnlyLinks(t *testing.T) {
	clitest.CommandDir(t)
	root, exe := clitest.DoctorFixture(t)
	t.Setenv("OBIT_START_MENU_DIR", filepath.Join(root, "Programs"))
	clitest.WriteFixture(t, filepath.Join(root, "bin", "obit-gui.exe"), "fixture")
	out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(exe), "--dry-run")
	if err != nil {
		t.Fatal(out, err)
	}
	for _, place := range []shortcut.Location{shortcut.StartMenu, shortcut.Desktop} {
		paths, _ := shortcut.Paths(place)
		if strings.Contains(out, paths[0]+".obituary-asset") {
			t.Fatal("plan puts receipt beside shortcut", out)
		}
		if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
			t.Fatal("dry-run wrote shortcut", err)
		}
	}
	for _, args := range [][]string{{"--yes"}, {"--uninstall", "--yes"}} {
		if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(exe), args...); err != nil {
			t.Fatal(out, err)
		}
		for _, place := range []shortcut.Location{shortcut.StartMenu, shortcut.Desktop} {
			paths, _ := shortcut.Paths(place)
			entries, err := os.ReadDir(filepath.Dir(paths[0]))
			if err != nil {
				t.Fatal(err)
			}
			if len(args) == 1 && (len(entries) != 1 || entries[0].Name() != shortcut.Name) || len(args) > 1 && len(entries) != 0 {
				t.Fatal("shortcut directory has auxiliary files", entries)
			}
		}
	}
}

func TestInitSkillDefaultAndOptOut(t *testing.T) {
	clitest.CommandDir(t)
	_, exe := clitest.DoctorFixture(t)
	for _, tc := range []struct {
		args []string
		want bool
	}{{nil, true}, {[]string{"--skill"}, true}, {[]string{"--no-skill"}, false}} {
		out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(exe), append(tc.args, "--dry-run")...)
		if err != nil || strings.Contains(out, "SKILL.md") != tc.want {
			t.Fatal(tc.args, out, err)
		}
	}
	out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(exe), "--agent", "pi", "--dry-run")
	if err != nil || !strings.Contains(out, "skill") {
		t.Fatal(out, err)
	}
}

func TestInitShortcutUninstallPreservesForeignEntry(t *testing.T) {
	clitest.CommandDir(t)
	root, exe := clitest.DoctorFixture(t)
	t.Setenv("OBIT_START_MENU_DIR", filepath.Join(root, "Programs"))
	t.Setenv("OBIT_USER_HOME", filepath.Join(root, "user"))
	clitest.WriteFixture(t, filepath.Join(root, "bin", "obit-gui.exe"), "fixture")
	foreign := filepath.Join(root, "Programs", "Obituary.lnk")
	clitest.WriteFixture(t, foreign, filepath.Join(root, "bin", "obit-gui.exe"))
	if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(exe), "--yes"); err != nil {
		t.Fatal(out, err)
	}
	desktop := filepath.Join(root, "user", "Desktop", "讣告 Obituary.lnk")
	if _, err := os.Stat(desktop); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "Programs", "讣告 Obituary.lnk")); !os.IsNotExist(err) {
		t.Fatal("duplicate Start menu shortcut", err)
	}
	if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(exe), "--uninstall", "--yes"); err != nil {
		t.Fatal(out, err)
	}
	if _, err := os.Stat(desktop); !os.IsNotExist(err) {
		t.Fatal("owned desktop shortcut remains", err)
	}
	if data, err := os.ReadFile(foreign); err != nil || string(data) != filepath.Join(root, "bin", "obit-gui.exe") {
		t.Fatal("foreign shortcut changed", err)
	}
}
