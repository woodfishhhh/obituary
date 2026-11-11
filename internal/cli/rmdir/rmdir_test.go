package rmdir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	resurrectcmd "github.com/woodfishhhh/obituary/internal/cli/resurrect"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func TestEmptyDirectoryRaceStaysRestorable(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	path := filepath.Join(repo, "empty")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	svc.FS = concurrentBurialFS{FS: svc.FS, before: func(path string) { clitest.WriteFixture(t, filepath.Join(path, "arrived"), "concurrent write") }}
	old := cmdutil.Open
	cmdutil.Open = func() (*core.Service, error) { return svc, nil }
	t.Cleanup(func() { cmdutil.Open = old })
	if _, _, err := clitest.Execute(t, NewCmd(), "empty"); err != nil {
		t.Fatal(err)
	}
	cmdutil.Open = old
	st, err := store.Read()
	if err != nil || len(st.Burials) != 1 {
		t.Fatalf("burials: %+v %v", st.Burials, err)
	}
	if _, _, err := clitest.Execute(t, resurrectcmd.NewCmd(), fmt.Sprint(st.Burials[0].ID)); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(path, "arrived")); err != nil || string(got) != "concurrent write" {
		t.Fatalf("race lost data: %q %v", got, err)
	}
}

func TestRmdirEmptyOnly(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	if err := os.Mkdir(filepath.Join(repo, "empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, filepath.Join(repo, "full", "keep"), "keep")
	clitest.WriteFixture(t, filepath.Join(repo, "file"), "keep")
	_, stderr, err := clitest.Execute(t, NewCmd(), "full", "file", "empty")
	if err == nil || !strings.Contains(stderr, "Directory not empty") || !strings.Contains(stderr, "Not a directory") {
		t.Fatalf("rmdir: %s %v", stderr, err)
	}
	st, err := store.Read()
	if err != nil || len(st.Burials) != 1 || filepath.Base(st.Burials[0].Path) != "empty" {
		t.Fatalf("rmdir burials: %+v %v", st.Burials, err)
	}
}

func TestRmdirRejectsUnsupportedOptionsBeforeAnyBurial(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, flag := range []string{"-p", "--parents", "--ignore-fail-on-non-empty", "--version", "-r", "-R", "--recursive", "-f", "--force", "-d", "--dir", "--preserve-root", "--preserve-root=all", "--one-file-system", "--no-preserve-root", "--unknown"} {
		t.Run(flag, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			path := filepath.Join(repo, "empty")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			if _, stderr, err := clitest.Execute(t, NewCmd(), flag, "empty"); err == nil || !strings.Contains(stderr, "unsupported") {
				t.Errorf("unsupported rmdir option accepted: %q %v", stderr, err)
			}
			if info, err := os.Stat(path); err != nil || !info.IsDir() {
				t.Errorf("unsupported option moved empty directory: %v", err)
			}
			if st, err := store.Read(); err != nil || len(st.Burials) != 0 {
				t.Errorf("unsupported option recorded burials: %+v %v", st.Burials, err)
			}
			command := "rmdir " + flag + " empty"
			if got, why := guardrm.Suggest(command, guardrm.Context{Shell: guardrm.Bash}); got != command || !strings.Contains(why, "unsupported") {
				t.Errorf("unsupported option suggested: %q (%s)", got, why)
			}
		})
	}
}
