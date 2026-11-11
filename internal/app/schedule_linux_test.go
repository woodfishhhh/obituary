package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPOSIXUserTimerContentAndOwnership(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	home := "/tmp/用户 $HOME %h \"quoted\""
	files, err := scheduleFiles("/tmp/bin/obitd", home, "/tmp/config")
	if err != nil {
		t.Fatal(err)
	}
	service := files[serviceName]
	for _, want := range []string{"Type=oneshot", "ExecStart=:", "--once", "$HOME", "%%h", `\"quoted\"`} {
		if !strings.Contains(service, want) {
			t.Fatalf("missing %q: %s", want, service)
		}
	}
	for _, want := range []string{"OnCalendar=*:0/5", "Persistent=true", "WantedBy=timers.target"} {
		if !strings.Contains(files[TaskName], want) {
			t.Fatal(want)
		}
	}
	unitDir := filepath.Join(dir, "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(unitDir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	args, err := readSchedule()
	if err != nil || args[3] != home {
		t.Fatal(args, err)
	}
	if err := os.WriteFile(filepath.Join(unitDir, TaskName), []byte("foreign unit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSchedule(); err == nil {
		t.Fatal("foreign timer accepted")
	}
	root, err := os.OpenRoot(unitDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := ownedUnit(root, TaskName); err == nil {
		t.Fatal("foreign timer would be overwritten")
	}
	if _, err := scheduleFiles("/tmp/obitd", home+"\nInjected=true", "/tmp/config"); err == nil {
		t.Fatal("newline accepted")
	}
}
