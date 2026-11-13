package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPOSIXLaunchdContentAndOwnership(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_USER_HOME", root)
	data, err := launchdContent("/tmp/bin/obitd", "/tmp/用户 <data> & \"quoted\"", "/tmp/config")
	if err != nil {
		t.Fatal(err)
	}
	args, err := launchdArguments(data)
	if err != nil || args[3] != "/tmp/用户 <data> & \"quoted\"" {
		t.Fatalf("plist argv: %v %v", args, err)
	}
	for _, want := range []string{"<integer>300</integer>", "<key>RunAtLoad</key>", "&lt;data&gt;"} {
		if !strings.Contains(string(data), want) {
			t.Fatal(want)
		}
	}
	units, err := PreviewSchedule(args[0], args[3], args[5], true)
	if err != nil || units[0].Action != "create" {
		t.Fatal(units, err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("preview wrote user files: %v %v", entries, err)
	}
	if _, err := launchdArguments(append(data, '\n')); err == nil {
		t.Fatal("edited plist accepted")
	}
}

func fakeLaunchctl(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "launchctl")
	log := filepath.Join(dir, "calls")
	t.Setenv("LAUNCHCTL_TEST_LOG", log)
	t.Setenv("LAUNCHCTL_TEST_STATE", filepath.Join(dir, "state"))
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$LAUNCHCTL_TEST_LOG\"\n" +
		"case \"$1\" in\nprint) test \"$(cat \"$LAUNCHCTL_TEST_STATE\" 2>/dev/null)\" = on || exit 113;;\n" +
		"bootstrap) printf on > \"$LAUNCHCTL_TEST_STATE\";;\nbootout) printf off > \"$LAUNCHCTL_TEST_STATE\";\n" +
		"if test -n \"$LAUNCHCTL_TEST_EDIT\"; then printf 'user edit' > \"$LAUNCHCTL_TEST_EDIT\"; fi;;\nenable) :;;\n*) exit 2;;\nesac\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	before := launchctlExecutable
	launchctlExecutable = path
	t.Cleanup(func() { launchctlExecutable = before })
	return log
}

func TestPOSIXLaunchdInstallStatusAndRemoval(t *testing.T) {
	if testing.Short() {
		t.Skip("executes a fake launchctl")
	}
	root := t.TempDir()
	t.Setenv("OBIT_USER_HOME", root)
	log := fakeLaunchctl(t)
	home := filepath.Join(root, "data")
	if err := installLaunchd("/tmp/bin/obitd", home, filepath.Join(root, "config")); err != nil {
		t.Fatal(err)
	}
	if !ScheduleInstalled() || !ScheduleServes(home) || ScheduleServes(home+"-other") {
		t.Fatal("launchd status does not match installed arguments")
	}
	if err := RemoveSchedule(); err != nil {
		t.Fatal(err)
	}
	if ScheduleInstalled() {
		t.Fatal("launchd plist remains after removal")
	}
	if err := RemoveSchedule(); err != nil {
		t.Fatal(err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"bootstrap " + launchdDomain(), "bootout " + launchdService(), "enable " + launchdService()} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("missing launchctl invocation %q: %s", want, calls)
		}
	}
}

func TestPOSIXLaunchdPreservesForeignFiles(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_USER_HOME", root)
	path := filepath.Join(root, "Library", "LaunchAgents", TaskName+".plist")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("user settings"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := installLaunchd("/tmp/obitd", root, root); err == nil {
		t.Fatal("install overwrote unowned plist")
	}
	if err := RemoveSchedule(); err == nil {
		t.Fatal("removal accepted unowned plist")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "user settings" {
		t.Fatal("unowned plist changed", err)
	}
}

func TestPOSIXLaunchdPreservesEditDuringUnload(t *testing.T) {
	if testing.Short() {
		t.Skip("executes a fake launchctl")
	}
	root := t.TempDir()
	t.Setenv("OBIT_USER_HOME", root)
	fakeLaunchctl(t)
	if err := installLaunchd("/tmp/obitd", root, root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "Library", "LaunchAgents", TaskName+".plist")
	t.Setenv("LAUNCHCTL_TEST_EDIT", path)
	if err := RemoveSchedule(); err == nil {
		t.Fatal("removed an agent edited during bootout")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "user edit" {
		t.Fatalf("lost edited launchd plist: %s %v", data, err)
	}
}
