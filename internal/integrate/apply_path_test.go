package integrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyPinsParentForWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, action := range []string{"create", "update", "remove"} {
		t.Run(action, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			rel := filepath.Join("agent", "config", "managed")
			path := filepath.Join(root, rel)
			if action != "create" {
				writeIntegrationFile(t, path, "old")
			}
			outsideFile := filepath.Join(outside, "config", "managed")
			writeIntegrationFile(t, outsideFile, "old")
			writeIntegrationFile(t, outsideFile+".bak", "earlier backup")
			c, err := readIntegrationChange(Scope{Project: root}, path)
			if err != nil {
				t.Fatal(err)
			}
			c.Action, c.data, c.delete = action, []byte("new"), action == "remove"
			attempted, swapped := false, false
			beforeIntegrationWrite = func(string) error {
				attempted = true
				dir := filepath.Join(root, "agent")
				if err := os.Rename(dir, filepath.Join(root, "original")); err != nil {
					return nil
				}
				makeIntegrationJunction(t, dir, outside)
				swapped = true
				return nil
			}
			t.Cleanup(func() { beforeIntegrationWrite = nil; _ = os.Remove(filepath.Join(root, "agent")) })
			err = Apply([]Change{c})
			beforeIntegrationWrite = nil
			if !attempted {
				t.Fatalf("directory swap was not attempted: %v", err)
			}
			if swapped {
				t.Fatalf("pinned parent allowed ancestor replacement: %v", err)
			}
			entries, readErr := os.ReadDir(filepath.Dir(outsideFile))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if got, readErr := os.ReadFile(outsideFile); readErr != nil || string(got) != "old" || len(entries) != 2 || read(t, outsideFile+".bak") != "earlier backup" {
				t.Fatalf("pinned install touched outside directory: Apply=%v read=%v contents=%q entries=%v", err, readErr, got, entries)
			}
			if err != nil {
				t.Fatalf("pinned install did not finish in authorized directory: %v", err)
			}
			original := path
			if action == "remove" {
				if _, err := os.Stat(original); !os.IsNotExist(err) {
					t.Fatalf("pinned removal did not remove authorized file: %v", err)
				}
			} else if got := read(t, original); got != "new" {
				t.Fatalf("pinned write missed authorized file: %q", got)
			}
			if action != "create" && read(t, testBackupPath(t, original)) != "old" {
				t.Fatal("pinned backup missed authorized file")
			}
		})
	}
}

func TestApplyRejectsReplacedIdentity(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"directory", "file"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, "config", "managed")
			if kind == "file" {
				writeIntegrationFile(t, path, "same bytes")
			} else if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			c, err := readIntegrationChange(Scope{Project: root}, path)
			if err != nil {
				t.Fatal(err)
			}
			c.Action, c.delete = "remove", true
			if kind == "directory" {
				c.Action, c.delete, c.data = "create", false, []byte("new")
			}
			old := path
			if kind == "directory" {
				old = filepath.Dir(path)
			}
			if err := os.Rename(old, old+"-original"); err != nil {
				t.Fatal(err)
			}
			if kind == "file" {
				writeIntegrationFile(t, path, "same bytes")
			} else if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			err = Apply([]Change{c})
			got, readErr := os.ReadFile(path)
			if err == nil || kind == "file" && (readErr != nil || string(got) != "same bytes") || kind == "directory" && !os.IsNotExist(readErr) {
				t.Fatalf("replacement identity accepted: Apply=%v read=%v contents=%q", err, readErr, got)
			}
			if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
				t.Fatalf("rejected identity created backup: %v", err)
			}
		})
	}
}

func TestApplyRejectsReplacedScopeRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "global"}[global], func(t *testing.T) {
			t.Parallel()
			parent, outside := t.TempDir(), t.TempDir()
			root := filepath.Join(parent, "scope")
			if err := os.Mkdir(root, 0o700); err != nil {
				t.Fatal(err)
			}
			c, err := readIntegrationChange(Scope{Project: root, Global: global}, filepath.Join(root, "managed"))
			if err != nil {
				t.Fatal(err)
			}
			c.Action, c.data = "create", []byte("new")
			if err := os.Rename(root, root+"-original"); err != nil {
				t.Fatal(err)
			}
			makeIntegrationJunction(t, root, outside)
			t.Cleanup(func() { _ = os.Remove(root) })
			err = Apply([]Change{c})
			entries, readErr := os.ReadDir(outside)
			if err == nil || readErr != nil || len(entries) != 0 {
				t.Fatalf("scope root junction followed: Apply=%v entries=%v read=%v", err, entries, readErr)
			}
		})
	}
}

func TestApplyRejectsLinkedDestinations(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, name := range []string{"managed", "managed.bak", "managed.bak.1", ".obituary-skill"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, "managed")
			if name == ".obituary-skill" {
				path = filepath.Join(root, name)
			}
			writeIntegrationFile(t, path, "old")
			c, err := readIntegrationChange(Scope{Global: true}, path)
			if err != nil {
				t.Fatal(err)
			}
			c.Action, c.delete = "remove", true
			outside := filepath.Join(t.TempDir(), "external")
			writeIntegrationFile(t, outside, "old")
			link := filepath.Join(root, name)
			if link == path {
				if err := os.Rename(path, path+"-original"); err != nil {
					t.Fatal(err)
				}
			}
			if name == "managed.bak.1" {
				writeIntegrationFile(t, path+".bak", "earlier backup")
			}
			makeIntegrationJunction(t, link, filepath.Dir(outside))
			err = Apply([]Change{c})
			info, statErr := os.Lstat(link)
			if err == nil || statErr != nil || info.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0 || read(t, outside) != "old" {
				t.Fatalf("linked installation entry was not preserved: Apply=%v stat=%v", err, statErr)
			}
		})
	}
}

func TestApplyChecksFinalReceiptDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	root, outside := t.TempDir(), t.TempDir()
	s := Scope{Project: root}
	plan, err := PlanSkill(s, "codex", false)
	if err != nil {
		t.Fatal(err)
	}
	file, err := skillPath(s, "codex")
	if err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(filepath.Dir(file), ".obituary-skill")
	outsideReceipt := filepath.Join(outside, "skills", "obituary", ".obituary-skill")
	var pending string
	afterIntegrationWrite := func(path string) error {
		if path != file {
			return nil
		}
		pending = read(t, receipt)
		writeIntegrationFile(t, outsideReceipt, pending)
		parent := filepath.Join(root, ".agents")
		if err := os.Rename(parent, filepath.Join(root, "original")); err != nil {
			return err
		}
		makeIntegrationJunction(t, parent, outside)
		return nil
	}
	t.Cleanup(func() { _ = os.Remove(filepath.Join(root, ".agents")) })
	err = apply(plan, afterIntegrationWrite)
	if pending == "" {
		t.Fatalf("receipt swap did not execute: %v", err)
	}
	if err == nil || read(t, outsideReceipt) != pending {
		t.Fatalf("final receipt followed replacement directory: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(outsideReceipt))
	if err != nil || len(entries) != 1 {
		t.Fatalf("final receipt left outside backup or temporary: %v %v", entries, err)
	}
}
