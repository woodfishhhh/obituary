//go:build linux || darwin

package winfs_test

import (
	"encoding/json"
	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/doctor"
	listcmd "github.com/woodfishhhh/obituary/internal/cli/list"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/winfs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type unavailableBirthFS struct{ winfs.FS }

func (fs unavailableBirthFS) Identify(path string) (core.Identity, error) {
	id, err := fs.FS.Identify(path)
	if err == nil {
		id.FileID = "0000000000000000" + id.FileID[16:]
	}
	return id, err
}

func TestPOSIXLegacyIdentityDiagnostics(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "data")
	t.Setenv("OBIT_HOME", home)
	path := filepath.Join(dir, "payload")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &core.Service{Store: core.Store{Home: home}, FS: unavailableBirthFS{}}
	id, err := svc.FS.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.UpdateHot(func(st *core.State) error {
		st.Items = append(st.Items, core.Item{ID: st.NextID, Rev: 1, Path: path, Identity: &id, ExpiresAt: time.Now().Add(time.Hour)})
		st.NextID++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{}, {"--json"}} {
		out, _, err := clitest.Execute(t, listcmd.NewCmd(), args...)
		if err != nil || !strings.Contains(out, "obit mark") || !strings.Contains(out, "creation time") {
			t.Fatalf("list hid legacy identity: %s %v", out, err)
		}
	}
	out, _, _ := clitest.Execute(t, doctor.NewCmd(), "--json")
	var checks []doctor.DoctorCheck
	if err := json.Unmarshal([]byte(out), &checks); err != nil {
		t.Fatalf("doctor output: %s %v", out, err)
	}
	for _, check := range checks {
		if check.Name == "registration-identities" && check.Status == "WARN" && strings.Contains(check.Detail, "obit mark") {
			return
		}
	}
	t.Fatalf("doctor hid legacy identity: %s", out)
}

func TestPOSIXMissingBirthTimeRefusesAutomaticRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("creates a real Git repository; manual registration covers the same gate in short mode")
	}
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %s %v", out, err)
	}
	path := filepath.Join(repo, "artifact.log")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".obitrules"), []byte("*.log 1d\n"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: unavailableBirthFS{}}
	engine := &auto.Engine{Svc: svc, Trust: rules.TrustStore{Path: filepath.Join(dir, "trust.json")}}
	if _, err := engine.Trust.Trust(filepath.Join(repo, ".obitrules"), ""); err != nil {
		t.Fatal(err)
	}
	rows, err := engine.Apply([]string{path}, auto.Options{})
	if err != nil || len(rows) != 1 || rows[0].Action == auto.Registered || !strings.Contains(rows[0].Reason, "creation time") {
		t.Fatalf("missing birth time automatic registration accepted: %+v %v", rows, err)
	}
	if _, _, err := engine.MkTemp(time.Hour, "scratch", repo, ""); err == nil || !strings.Contains(err.Error(), "creation time") {
		t.Fatalf("missing birth time mktemp accepted: %v", err)
	}
	st, err := svc.Store.ReadSnapshot()
	if err != nil || len(st.Items) != 0 || len(st.Burials) != 0 {
		t.Fatalf("automatic registration changed ledger: %+v %v", st, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Fatalf("automatic registration changed file: %q %v", data, err)
	}
}
