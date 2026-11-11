package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

func TestOnceDeduplicatesProposalErrorsAcrossProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs one-shot daemon processes")
	}
	root := t.TempDir()
	bin := filepath.Join(root, "obitd.exe")
	if out, err := exec.Command("go", "build", "-ldflags=-H=windowsgui", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	project := filepath.Join(root, "repo")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	home, config := filepath.Join(root, "home"), filepath.Join(root, "config")
	if err := os.Mkdir(config, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "config.toml"), []byte("[update]\ncheck = false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &core.Service{Store: core.Store{Home: home}, FS: winfs.FS{}}
	if _, _, _, err := svc.Propose(core.ProposalLeftovers, project, "s", []core.Candidate{{Path: filepath.Join(project, "missing.log"), Lifespan: "1d"}}); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		out, runErr := exec.Command(bin, "--once", "--home", home, "--config-dir", config).CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) || exit.ExitCode() != 1 {
			t.Fatalf("once lost failure status: %v %s", runErr, out)
		}
	}
	data, err := os.ReadFile(filepath.Join(home, "obitd.log"))
	if err != nil || strings.Count(string(data), "retry later") != 1 {
		t.Fatalf("one-shot errors repeated: %s %v", data, err)
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || len(st.Items) != 0 || len(st.Burials) != 0 {
		t.Fatalf("once changed unavailable proposal: %+v %v", st, err)
	}
}

func TestDaemonRejectsChangedHomeArgument(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	bin := filepath.Join(t.TempDir(), "obitd.exe")
	build := exec.Command("go", "build", "-ldflags=-H=windowsgui", "-o", bin, ".")
	if overlay := os.Getenv("OBIT_TEST_BUILD_OVERLAY"); overlay != "" {
		build = exec.Command("go", "build", "-overlay", overlay, "-ldflags=-H=windowsgui", "-o", bin, ".")
	}
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	for _, flag := range []string{"--home", "--config-dir"} {
		t.Run(flag, func(t *testing.T) {
			base := t.TempDir()
			home, config := filepath.Join(base, "home"), filepath.Join(base, "config")
			path := filepath.Join(base, "keep.txt")
			if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			svc := &core.Service{Store: core.Store{Home: home}, FS: winfs.FS{}, Now: func() time.Time { return time.Now().Add(-2 * time.Hour) }}
			if rows, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(-time.Hour)}); err != nil || core.Failed(rows) {
				t.Fatalf("fixture: %+v %v", rows, err)
			}
			before, err := os.ReadFile(filepath.Join(home, "store.json"))
			if err != nil {
				t.Fatal(err)
			}
			homeArg, configArg := home, config
			if flag == "--home" {
				homeArg = `\\?\` + home + "."
			} else {
				configArg = `\\?\` + config + "."
			}
			for _, mode := range []string{"--once", "--context-menu"} {
				// Missing menu operands keep a regression from sending a toast;
				// exit 2 proves directory validation happened before menu dispatch.
				cmd := exec.Command(bin, mode, "--home", homeArg, "--config-dir", configArg)
				out, runErr := cmd.CombinedOutput()
				var exit *exec.ExitError
				after, readErr := os.ReadFile(filepath.Join(home, "store.json"))
				if !errors.As(runErr, &exit) || exit.ExitCode() != 2 || readErr != nil || !bytes.Equal(before, after) {
					t.Fatalf("daemon changed neighbor ledger through unsafe argument (%s): %v %v %s", mode, runErr, readErr, out)
				}
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
				t.Fatalf("neighbor object changed: %s %v", data, err)
			}
		})
	}
}
