//go:build windows

package app

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func explorerService(t *testing.T) (*core.Service, string) {
	t.Helper()
	t.Setenv("OBIT_HOME", t.TempDir())
	t.Setenv("OBIT_CONFIG_DIR", t.TempDir())
	if err := os.WriteFile(filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml"), []byte("[explorer]\neverywhere=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "report.log"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc.Now = func() time.Time { return time.Date(2026, 11, 10, 0, 0, 0, 0, time.Local) }
	return svc, dir
}

func labelText(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "desktop.ini"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 2 || data[0] != 255 || data[1] != 254 {
		t.Fatalf("bad encoding: %x", data)
	}
	units := make([]uint16, (len(data)-2)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[2+i*2:])
	}
	return string(utf16.Decode(units))
}

func addLabel(t *testing.T, svc *core.Service, path string) string {
	t.Helper()
	r, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: svc.Now().Add(3 * 24 * time.Hour)})
	if err != nil || len(r) != 1 || r[0].Error != "" || r[0].Item == nil {
		t.Fatalf("add: %+v %v", r, err)
	}
	return strconv.FormatInt(r[0].Item.ID, 10)
}

func assertNoINI(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, "desktop.ini")); !os.IsNotExist(err) {
		t.Fatalf("INI remains: %v", err)
	}
}

func TestExplorerDefaultScopeLeavesScratchAlone(t *testing.T) {
	fixture, dir := explorerService(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	svc.Now = fixture.Now
	addLabel(t, svc, dir)
	assertNoINI(t, filepath.Dir(dir))
	assertNoINI(t, dir)
	if data, err := os.ReadFile(filepath.Join(svc.Store.Home, "explorer.log")); !os.IsNotExist(err) {
		t.Fatalf("unexpected sync failure: %s %v", data, err)
	}
}

func labelTextIfPresent(t *testing.T, dir string) string {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, "desktop.ini")); os.IsNotExist(err) {
		return ""
	}
	return labelText(t, dir)
}

func TestExplorerOldServiceUsesCurrentConfig(t *testing.T) {
	for _, trigger := range []string{"tick", "change"} {
		t.Run(trigger, func(t *testing.T) {
			svc, dir := explorerService(t)
			addLabel(t, svc, filepath.Join(dir, "report.log"))
			configPath := filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml")
			if err := os.WriteFile(configPath, []byte("[explorer]\nenabled=false\neverywhere=true\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			for round := range 2 {
				if round == 1 {
					fresh, err := Open()
					if err != nil {
						t.Fatal(err)
					}
					fresh.AfterChange(fresh)
				}
				if trigger == "tick" {
					before := labelTextIfPresent(t, dir)
					if _, _, err := Tick(svc); err != nil {
						t.Fatal(err)
					}
					if after := labelTextIfPresent(t, dir); before != after {
						t.Fatal("idle tick synchronized Explorer without a ledger change")
					}
					svc.AfterChange(svc)
				} else {
					svc.AfterChange(svc)
				}
				if _, err := os.Stat(filepath.Join(dir, "desktop.ini")); !os.IsNotExist(err) {
					t.Fatalf("old service ignored disabled Explorer config: round=%d err=%v", round, err)
				}
			}
		})
	}
}

func TestExplorerLifespanChangesAndRename(t *testing.T) {
	svc, dir := explorerService(t)
	path := filepath.Join(dir, "report.log")
	id := addLabel(t, svc, path)
	if !strings.Contains(labelText(t, dir), "report.log=report（11-13 下葬）") {
		t.Fatal("registration label missing")
	}
	if _, err := svc.Extend([]string{id}, 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(labelText(t, dir), "report.log=report（11-14 下葬）") {
		t.Fatal("extended label missing")
	}
	renamed := filepath.Join(dir, "renamed.log")
	if err := os.Rename(path, renamed); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(labelText(t, dir), "report.log=") {
		t.Fatal("idle sweep unexpectedly synchronized Explorer")
	}
	svc.AfterChange(svc)
	text := labelText(t, dir)
	if strings.Contains(text, "report.log=") || !strings.Contains(text, "renamed.log=renamed（11-14 下葬）") {
		t.Fatal(text)
	}
	if _, err := svc.Cancel([]string{id}); err != nil {
		t.Fatal(err)
	}
	assertNoINI(t, dir)
	data, err := os.ReadFile(renamed)
	if err != nil || string(data) != "payload" {
		t.Fatalf("real file changed: %q %v", data, err)
	}
}

func TestExplorerBurialAndResurrection(t *testing.T) {
	svc, dir := explorerService(t)
	path := filepath.Join(dir, "report.log")
	addLabel(t, svc, path)
	r, err := svc.Bury([]string{path}, false, "")
	if err != nil || len(r) != 1 || r[0].Action != core.ActionBuried {
		t.Fatalf("bury: %+v %v", r, err)
	}
	assertNoINI(t, dir)
	r, err = svc.Resurrect([]string{strconv.FormatInt(r[0].Burial.ID, 10)})
	if err != nil || len(r) != 1 || r[0].Action != core.ActionResurrected {
		t.Fatalf("resurrect: %+v %v", r, err)
	}
	assertNoINI(t, dir)
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "payload" {
		t.Fatalf("restore: %q %v", data, err)
	}
}

func TestExplorerFailureDoesNotFailBurialOrResurrection(t *testing.T) {
	svc, dir := explorerService(t)
	path := filepath.Join(dir, "report.log")
	addLabel(t, svc, path)
	if err := os.Rename(filepath.Join(dir, "desktop.ini"), filepath.Join(dir, "old.ini")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "desktop.ini"), 0o700); err != nil {
		t.Fatal(err)
	}
	r, err := svc.Bury([]string{path}, false, "")
	if err != nil || len(r) != 1 || r[0].Action != core.ActionBuried {
		t.Fatalf("label failure blocked bury: %+v %v", r, err)
	}
	r, err = svc.Resurrect([]string{strconv.FormatInt(r[0].Burial.ID, 10)})
	if err != nil || len(r) != 1 || r[0].Action != core.ActionResurrected {
		t.Fatalf("label failure blocked resurrection: %+v %v", r, err)
	}
	data, err := os.ReadFile(filepath.Join(svc.Store.Home, "explorer.log"))
	if err != nil || !strings.Contains(string(data), "desktop.ini") {
		t.Fatalf("missing error log: %q %v", data, err)
	}
}

func TestExplorerGitOptInAndDisableCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("checks Explorer exclusions with a real Git repository")
	}
	svc, dir := explorerService(t)
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
		return string(out)
	}
	testenv.InitRepo(t, dir)
	addLabel(t, svc, filepath.Join(dir, "report.log"))
	assertNoINI(t, dir)
	svc.AfterChange = func(current *core.Service) {
		syncExplorer(current, config.Explorer{Enabled: true, InGit: true, Everywhere: true})
	}
	svc.AfterChange(svc)
	if !strings.Contains(labelText(t, dir), "report.log=") {
		t.Fatal("opt-in missing")
	}
	if out := git("check-ignore", "desktop.ini"); strings.TrimSpace(out) != "desktop.ini" {
		t.Fatal(out)
	}
	if err := os.WriteFile(filepath.Join(dir, "another.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc.AfterChange = func(current *core.Service) { syncExplorer(current, config.Explorer{}) }
	svc.AfterChange(svc)
	assertNoINI(t, dir)
}

func TestExplorerTracksMovedParentAndBuriedChildLabels(t *testing.T) {
	if testing.Short() {
		t.Skip("resolves moved and buried directory identities across the real volume")
	}
	svc, dir := explorerService(t)
	path := filepath.Join(dir, "report.log")
	addLabel(t, svc, path)
	identity, err := (winfs.FS{}).Identify(dir)
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(labelText(t, moved), "report.log=") {
		t.Fatal("parent rename lost labels")
	}
	svc.Now = func() time.Time { return time.Date(2026, 11, 14, 0, 0, 0, 0, time.Local) }
	r, err := svc.Bury([]string{moved}, false, "")
	if err != nil || len(r) != 1 || r[0].Action != core.ActionBuried {
		t.Fatalf("bury: %+v %v", r, err)
	}
	grave, err := svc.FS.Locate(identity)
	if err != nil {
		t.Fatal(err)
	}
	assertNoINI(t, grave)
	r, err = svc.Resurrect([]string{strconv.FormatInt(r[0].Burial.ID, 10)})
	if err != nil || len(r) != 1 || r[0].Action != core.ActionResurrected {
		t.Fatalf("restore: %+v %v", r, err)
	}
	assertNoINI(t, moved)
}

func TestExplorerDoesNotLabelUncommittedBurial(t *testing.T) {
	svc, dir := explorerService(t)
	path := filepath.Join(dir, "report.log")
	addLabel(t, svc, path)
	grave := filepath.Join(svc.Store.Home, "graveyard", "pending")
	if err := os.MkdirAll(grave, 0o700); err != nil {
		t.Fatal(err)
	}
	// A move can succeed before its ledger commit. Its stale Item is still
	// live in the ledger, but adding desktop.ini would corrupt the grave.
	if err := os.Rename(path, filepath.Join(grave, "report.log")); err != nil {
		t.Fatal(err)
	}
	svc.AfterChange(svc)
	assertNoINI(t, grave)
	if data, err := os.ReadFile(filepath.Join(grave, "report.log")); err != nil || string(data) != "payload" {
		t.Fatalf("payload: %q %v", data, err)
	}
}
