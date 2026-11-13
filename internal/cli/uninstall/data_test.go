package uninstall

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/regtxn"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func TestUninstallRemovesOwnedUpdateMetadata(t *testing.T) {
	d, _, _ := fixture(t)
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.Update(func(*core.State) error { return nil }); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"update.json", "update.json.tmp", "update.lock", "proposal-log.json", "proposal-log.json.tmp", "proposal-log.lock", "explorer.log", "explorer.log.1"} {
		write(t, filepath.Join(svc.Store.Home, name), "{}")
	}
	out, err := execute(t, d, "", "--yes", "--purge-data")
	if err != nil {
		t.Fatal(out, err)
	}
	if _, err := os.Stat(svc.Store.Home); !os.IsNotExist(err) {
		t.Fatalf("update metadata retained: %v", err)
	}
}

// A real installation: hook logging leaves lock files beside its logs, and
// installers keep per-user locks in the default data directory.
func TestUninstallPurgeRemovesHookAndInstallerLocks(t *testing.T) {
	d, _, _ := fixture(t)
	useDefaultData(t)
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.Update(func(*core.State) error { return nil }); err != nil {
		t.Fatal(err)
	}
	hook.Logf(svc.Store.Home, "fixture")
	hook.Auditf(svc.Store.Home, "fixture")
	locks, err := userdirs.RegistryLocks()
	if err != nil || filepath.Dir(locks) != svc.Store.Home {
		t.Fatalf("registry locks %s outside data %s: %v", locks, svc.Store.Home, err)
	}
	write(t, filepath.Join(locks, regtxn.LockName("context menu")), "")
	out, err := execute(t, d, "", "--yes", "--purge-data")
	if err != nil {
		t.Fatal(out, err)
	}
	if _, err := os.Stat(svc.Store.Home); !os.IsNotExist(err) {
		t.Fatalf("owned data retained: %v\n%s", err, out)
	}
}

// The lock directory counts as owned only where installers put it, and only
// while it holds nothing but their lock files.
func TestUninstallPurgeKeepsUnrecognisedRegistryLocks(t *testing.T) {
	// Each case passes every check but the one it names.
	for name, files := range map[string]map[string]string{
		"other home":    {regtxn.LockName("context menu"): ""},
		"user file":     {"notes.txt": ""},
		"nonempty lock": {regtxn.LockName("context menu"): "USER DATA"},
	} {
		t.Run(name, func(t *testing.T) {
			d, root, _ := fixture(t)
			useDefaultData(t)
			if name == "other home" {
				t.Setenv("OBIT_HOME", filepath.Join(root, "OBIT_HOME"))
				locks, err := userdirs.RegistryLocks()
				if err != nil || os.MkdirAll(locks, 0o700) != nil {
					t.Fatal(err)
				}
			}
			svc, err := app.Open()
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.Store.Update(func(*core.State) error { return nil }); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(svc.Store.Home, "registry-locks")
			for file, data := range files {
				write(t, filepath.Join(dir, file), data)
			}
			out, err := execute(t, d, "", "--yes", "--purge-data")
			if err == nil || !strings.Contains(err.Error(), "data retained") {
				t.Fatalf("unrecognised registry-locks accepted: %v\n%s", err, out)
			}
			for file, data := range files {
				if got, err := os.ReadFile(filepath.Join(dir, file)); err != nil || string(got) != data {
					t.Fatalf("%s changed: %q %v", file, got, err)
				}
			}
		})
	}
}

// useDefaultData selects the default data directory under a fresh base, the
// directory installers also keep their registry locks in.
func useDefaultData(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	for _, key := range []string{"LOCALAPPDATA", "XDG_DATA_HOME", "HOME"} {
		t.Setenv(key, base)
	}
	t.Setenv("OBIT_HOME", "")
}

func TestReviewPurgePreservesUnownedMetadataName(t *testing.T) {
	d, _, _ := fixture(t)
	path := filepath.Join(os.Getenv("OBIT_HOME"), "audit.jsonl")
	write(t, path, "USER APPLICATION AUDIT DATA")
	out, err := execute(t, d, "", "--yes", "--purge-data")
	if data, readErr := os.ReadFile(path); err == nil || readErr != nil || string(data) != "USER APPLICATION AUDIT DATA" {
		t.Fatalf("unowned metadata was not preserved: %q %v %v %s", data, readErr, err, out)
	}
}

func TestUninstallPurgeOnlyListedGraves(t *testing.T) {
	d, root, _ := fixture(t)
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	svc.AfterChange = nil
	path := filepath.Join(root, "old.txt")
	write(t, path, "GRAVE CONTENT")
	results, err := svc.Bury([]string{path}, false, "uninstall fixture")
	if err != nil || core.GraveFailed(results) {
		t.Fatal(results, err)
	}
	grave := results[0].Burial.Grave
	write(t, path, "NEW USER CONTENT")
	before := snapshot(t, root)
	out, err := execute(t, d, "", "--yes")
	if err != nil || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("default uninstall modified data or graves", out, err)
	}
	out, err = execute(t, d, "", "--dry-run", "--purge-data")
	if err != nil || !strings.Contains(out, grave) || !strings.Contains(out, "1 recorded entry;") {
		t.Fatal(out, err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("dry-run changed graveyard")
	}
	out, err = execute(t, d, "", "--yes", "--purge-data")
	if err != nil {
		t.Fatal(out, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "NEW USER CONTENT" {
		t.Fatal(string(data), err)
	}
	if _, err := os.Stat(grave); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("grave retained", err)
	}
	if _, err := os.Stat(svc.Store.Home); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("data retained", err)
	}
}

func TestUninstallPurgeRefusesReplacedGrave(t *testing.T) {
	d, root, _ := fixture(t)
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	svc.AfterChange = nil
	path := filepath.Join(root, "payload.txt")
	write(t, path, "ORIGINAL")
	results, err := svc.Bury([]string{path}, false, "fixture")
	if err != nil || core.GraveFailed(results) {
		t.Fatal(results, err)
	}
	grave := results[0].Burial.Grave
	if err := os.Rename(grave, filepath.Join(root, "saved.txt")); err != nil {
		t.Fatal(err)
	}
	write(t, grave, "REPLACEMENT USER FILE")
	out, err := execute(t, d, "", "--yes", "--purge-data")
	if err == nil {
		t.Fatal("accepted replaced grave", out)
	}
	if data, err := os.ReadFile(grave); err != nil || string(data) != "REPLACEMENT USER FILE" {
		t.Fatal(string(data), err)
	}
	if _, err := os.Stat(filepath.Join(svc.Store.Home, "store.json")); err != nil {
		t.Fatal("lost recovery ledger", err)
	}
}

func TestUninstallPurgePreservesUnknownData(t *testing.T) {
	for _, name := range []string{"user.txt", "sessions/user.txt", "graveyard/user.txt", "store.json/user.txt"} {
		t.Run(name, func(t *testing.T) {
			d, root, _ := fixture(t)
			home := filepath.Join(root, "OBIT_HOME")
			write(t, filepath.Join(home, name), "USER CONTENT")
			before := snapshot(t, home)
			_, err := execute(t, d, "", "--yes", "--purge-data")
			if err == nil {
				t.Fatal("unknown data deletion reported success")
			}
			if !reflect.DeepEqual(before, snapshot(t, home)) {
				t.Fatal("unknown contents changed")
			}
		})
	}
}
