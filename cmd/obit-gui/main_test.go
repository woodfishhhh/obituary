//go:build windows

package main

import (
	"bytes"
	"debug/pe"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/gui"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

func TestSmokeAndInputErrorsWithoutWindow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_HOME", filepath.Join(root, "home"))
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	var output bytes.Buffer
	if err := run([]string{"--smoke"}, &output); err != nil {
		t.Fatal(err)
	}
	var summary gui.Summary
	if err := json.Unmarshal(output.Bytes(), &summary); err != nil || summary.TrackedCount != 0 || summary.Error != "" {
		t.Fatalf("summary=%s err=%v", output.Bytes(), err)
	}
	for _, args := range [][]string{{"--smoke", "extra"}, {"--smoke", "--width=10"}, {"--unknown"}} {
		if err := run(args, &output); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "config.toml"), []byte("bad = ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--smoke"}, &output); err == nil {
		t.Fatal("smoke hid config error")
	}
}

func TestProductionExecutableSmokeWithoutCGO(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the real Windows GUI executable")
	}
	exe := filepath.Join(t.TempDir(), "obit-gui.exe")
	cmd := exec.Command("go", "build", "-tags", "desktop,production", "-ldflags", "-H windowsgui", "-o", exe, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native build: %s %v", output, err)
	}
	output, err := exec.Command(exe, "--smoke").CombinedOutput()
	if err != nil {
		t.Fatalf("smoke: %s %v", output, err)
	}
	var summary gui.Summary
	if err := json.Unmarshal(output, &summary); err != nil || summary.Error != "" {
		t.Fatalf("smoke JSON: %s %v", output, err)
	}
	file, err := pe.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	// Plain builds work without generated resources. The packaging job verifies
	// icons, versions and manifests after the GoReleaser generation hook.
	_, err = os.Stat("resource_windows_" + runtime.GOARCH + ".syso")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err == nil && file.Section(".rsrc") == nil {
		t.Fatal("icon/manifest resource was not linked")
	}
}

func TestWindowOptionsBindAPIAndExitOnClose(t *testing.T) {
	a, err := gui.Open()
	if err != nil {
		t.Fatal(err)
	}
	options := windowOptions(a, 900, 600)
	if options.Title != config.UILanguage().Text(i18n.AppName) || options.Width != 900 || options.Height != 600 || options.HideWindowOnClose || options.SingleInstanceLock == nil || options.SingleInstanceLock.OnSecondInstanceLaunch == nil || len(options.Bind) != 1 || options.Bind[0] != a {
		t.Fatalf("window options: %+v", options)
	}
	if _, err := assets.ReadFile("frontend/dist/index.html"); err != nil {
		t.Fatal(err)
	}
}
