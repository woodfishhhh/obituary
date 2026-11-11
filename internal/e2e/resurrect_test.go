//go:build windows

package e2e_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestResurrectLastAcrossProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	repo, store := commandRepo(t)
	run := func(command string, args ...string) string {
		t.Helper()
		out, stderr, err := execute(t, command, args...)
		if err != nil {
			t.Fatalf("%s: %s %s %v", command, out, stderr, err)
		}
		return out
	}
	assertRestored := func(paths []string) {
		t.Helper()
		out := run("resurrect", "--last", "--json")
		var rows []core.GraveResult
		if json.Unmarshal([]byte(out), &rows) != nil || len(rows) != len(paths) || core.GraveFailed(rows) {
			t.Fatalf("batch receipts: %s", out)
		}
		for _, path := range paths {
			if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
				t.Fatalf("restore %s: %q %v", path, data, err)
			}
		}
	}
	mine := []string{filepath.Join(repo, "first"), filepath.Join(repo, "second")}
	for _, path := range mine {
		writeFixture(t, path, "keep")
	}
	t.Setenv("OBIT_SESSION_ID", "mine")
	run("bury", mine...)
	t.Setenv("OBIT_SESSION_ID", "other")
	foreignSession := filepath.Join(repo, "other-session")
	writeFixture(t, foreignSession, "keep")
	run("rm", foreignSession)
	t.Setenv("OBIT_SESSION_ID", "mine")
	assertRestored(mine)
	if _, err := os.Stat(foreignSession); !os.IsNotExist(err) {
		t.Fatal("another process's session restored", err)
	}

	t.Setenv("OBIT_SESSION_ID", "")
	run("rm", mine...)
	otherRepo := filepath.Join(filepath.Dir(repo), "other-repo")
	if out, err := exec.Command("git", "init", "-q", otherRepo).CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	t.Chdir(otherRepo)
	foreignProject := filepath.Join(otherRepo, "other-project")
	writeFixture(t, foreignProject, "keep")
	run("bury", foreignProject)
	t.Chdir(repo)
	assertRestored(mine)
	if _, err := os.Stat(foreignProject); !os.IsNotExist(err) {
		t.Fatal("another process's project restored", err)
	}
	st, err := store.ReadSnapshot()
	if err != nil || len(st.Burials) != 2 {
		t.Fatalf("foreign burials not preserved: %+v %v", st.Burials, err)
	}
}
