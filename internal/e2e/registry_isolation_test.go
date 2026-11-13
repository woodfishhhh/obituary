//go:build windows

package e2e_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// Replace the production fallback with a disposable key even when testing a
// broken resolver, so a regression cannot overwrite the user's installation.
func registryIsolationBinary(t *testing.T, fallback string) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "internal", "contextmenu", "registry_windows.go")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	const literal = "`Software\\Classes`"
	if strings.Count(string(data), literal) != 1 {
		t.Fatal("context-menu fallback must have exactly one overlay target")
	}
	dir := t.TempDir()
	replacement := filepath.Join(dir, "registry_windows.go")
	writeFixture(t, replacement, strings.Replace(string(data), literal, "`"+fallback+"`", 1))
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{source: replacement}})
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, "overlay.json")
	writeFixture(t, manifest, string(overlay))
	binary := filepath.Join(dir, "obit.exe")
	cmd := exec.Command("go", "build", "-overlay", manifest, "-o", binary, "./cmd/obit")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), testenv.MutantGoEnv(t)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build isolated fallback: %s %v", out, err)
	}
	writeFixture(t, filepath.Join(dir, "obitd.exe"), "companion presence fixture; never executed")
	return binary
}

func TestInitSubprocessRegistryIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the real init command with a disposable fallback")
	}
	t.Setenv("OBIT_TEST_REGISTRY", "1")
	fallback, classes := testenv.ClassesKey(t), testenv.ClassesKey(t)
	t.Setenv("OBIT_TEST_REGISTRY_ROOT", strings.TrimSuffix(classes, `\Classes`))
	binary := registryIsolationBinary(t, fallback)
	repo := t.TempDir()
	testenv.InitRepo(t, repo)
	cmd := exec.Command(binary, "init", "--yes", "--no-skill", "--no-shortcuts")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init child: %s %v", out, err)
	}
	const verb = `*\shell\Obituary.Resurrect`
	leaked, err := (contextmenu.WindowsRegistry{Root: fallback}).ReadTree(verb)
	if err != nil || leaked != nil {
		t.Fatalf("init child wrote the simulated real Classes root: %v %v", leaked, err)
	}
	got, err := (contextmenu.WindowsRegistry{Root: classes}).ReadTree(verb)
	if err != nil || !strings.Contains(got[`command`][""], binary) {
		t.Fatalf("init child did not use its disposable Classes root: %v %v", got, err)
	}
	assertSubprocessRunKey(t, binary)
}

func assertSubprocessRunKey(t *testing.T, binary string) {
	t.Helper()
	key := testenv.ReadTestRegistry(t, os.Getenv("OBIT_TEST_REGISTRY_ROOT"), "Run")
	names, err := key.ReadValueNames(-1)
	if err != nil || len(names) != 1 || !strings.HasPrefix(names[0], "Obituary-") {
		t.Fatalf("isolated Run value missing: %v %v", names, err)
	}
	value, _, err := key.GetStringValue(names[0])
	if err != nil || !strings.Contains(value, filepath.Dir(binary)) {
		t.Fatalf("wrong isolated startup command: %s %v", value, err)
	}
}

func TestInitSubprocessRejectsUnsafeRegistryRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the real init command")
	}
	repo := t.TempDir()
	testenv.InitRepo(t, repo)
	for _, root := range []string{"", `Software\Classes`, `Software\obituary-test-a\..\Classes`} {
		for _, noMenu := range []bool{false, true} {
			t.Setenv("OBIT_TEST_REGISTRY_ROOT", root)
			args := []string{"init", "--yes", "--no-skill", "--no-shortcuts"}
			if noMenu {
				args = append(args, "--no-context-menu")
			}
			cmd := exec.Command(binaryPath(t, "obit"), args...)
			cmd.Dir = repo
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "OBIT_TEST_REGISTRY_ROOT") {
				t.Fatalf("unsafe root accepted: %q menu=%t: %s %v", root, !noMenu, out, err)
			}
		}
	}
}

func TestInitSubprocessShortcutsStayInTestHome(t *testing.T) {
	if testing.Short() {
		t.Skip("runs init with the native shortcut codec")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "obit.exe")
	data, err := os.ReadFile(binaryPath(t, "obit"))
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, binary, string(data))
	writeFixture(t, filepath.Join(dir, "obit-gui.exe"), "shortcut target fixture; never executed")
	cmd := exec.Command(binary, "init", "--global", "--yes", "--no-skill")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init shortcuts: %s %v", out, err)
	}
	for _, resolve := range []func() (string, error){userdirs.Programs, userdirs.Desktop} {
		folder, err := resolve()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(folder, "讣告 Obituary.lnk")); err != nil {
			t.Fatalf("isolated shortcut missing: %v", err)
		}
	}
}
