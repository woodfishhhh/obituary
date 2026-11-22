//go:build darwin && desktop

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrayLaunchdContentAndOwnership(t *testing.T) {
	data, err := trayLaunchdContent("/Applications/Obituary.app/Contents/MacOS/obitd", "/tmp/data <x>", "/tmp/config")
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyTrayLaunchd(data); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<key>SuccessfulExit</key><false/>", "<key>ThrottleInterval</key><integer>60</integer>", "<string>--managed</string>"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %s in tray job", want)
		}
	}
	if !strings.Contains(string(data), "<key>RunAtLoad</key><true/>") || !strings.Contains(string(data), "&lt;x&gt;") {
		t.Fatalf("unexpected tray agent: %s", data)
	}
	if err := verifyTrayLaunchd(append(data, '\n')); err == nil {
		t.Fatal("edited tray agent accepted")
	}
}

func TestTrayAutostartPlanPreviewDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_USER_HOME", root)
	plan, err := PlanAutostart(nil, "/Applications/Obituary.app/Contents/MacOS/obitd", filepath.Join(root, "data"), filepath.Join(root, "config"), true)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Pending() || !strings.HasSuffix(plan.Name, trayTaskName+".plist") {
		t.Fatalf("plan=%+v", plan)
	}
	if _, err := os.Stat(plan.Name); !os.IsNotExist(err) {
		t.Fatalf("preview wrote %s: %v", plan.Name, err)
	}
}
