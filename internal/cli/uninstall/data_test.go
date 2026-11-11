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
	for _, name := range []string{"update.json", "update.json.tmp", "update.lock", "proposal-log.json", "proposal-log.json.tmp", "proposal-log.lock"} {
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
