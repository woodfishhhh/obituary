//go:build windows

package init_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
)

func TestContextMenuPreviewDescribesChanges(t *testing.T) {
	clitest.CommandDir(t)
	// Pin the label language: CI runners use an English Windows UI.
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("[ui]\nlanguage = \"zh-CN\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(cfgPath) })
	reg := clitest.NewMenuRegistry()
	old := initcmd.ContextMenuRegistry
	initcmd.ContextMenuRegistry = func() contextmenu.Registry { return reg }
	t.Cleanup(func() { initcmd.ContextMenuRegistry = old })
	root := t.TempDir()
	oldExe, exe := filepath.Join(root, "old", "obit.exe"), filepath.Join(root, "new", "obit.exe")
	clitest.WriteFixture(t, filepath.Join(root, "new", "obitd.exe"), "companion")
	self := cmdutil.Executable(func() (string, error) { return exe, nil })
	assertPreview := func(action string, args ...string) {
		t.Helper()
		before, err := contextmenu.Plan(reg, exe, false)
		if err != nil {
			t.Fatal(err)
		}
		out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(self), append([]string{"--context-menu", "--dry-run"}, args...)...)
		if err != nil {
			t.Fatal(out, err)
		}
		for _, want := range []string{action, "1 小时", "1 天", "7 天", "30 天", "--context-menu", "obitd.exe", "\"%1\""} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q: %s", want, out)
			}
		}
		if action != "update" {
			for _, want := range []string{"写讣告", "复活最近下葬的", "设置文件夹策略", "讣告 Obituary", `"%V\."`} {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q: %s", want, out)
				}
			}
		}
		after, err := contextmenu.Plan(reg, exe, false)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("preview changed registry", err)
		}
	}
	assertPreview("create")
	changes, err := contextmenu.Plan(reg, oldExe, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := contextmenu.Apply(reg, changes); err != nil {
		t.Fatal(err)
	}
	assertPreview("update")
	assertPreview("remove", "--uninstall")
}

func TestInitContextMenuCompatibilityFlag(t *testing.T) {
	clitest.CommandDir(t)
	_, executable := clitest.DoctorFixture(t)
	for _, args := range [][]string{{"--dry-run"}, {"--context-menu", "--dry-run"}, {"--context-menu", "--agent=codex", "--dry-run"}} {
		out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), args...)
		if err != nil || !strings.Contains(out, "context-menu:") {
			t.Fatal(out, err)
		}
	}
	if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--context-menu", "--no-context-menu", "--dry-run"); err == nil {
		t.Fatal("accepted conflicting flags")
	}
}
