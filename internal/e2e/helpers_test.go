//go:build windows

package e2e_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func binaryPath(t *testing.T, name string) string {
	t.Helper()
	dir, err := binaries()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, name+".exe")
}

// autoOn turns automatic registration on in OBIT_CONFIG_DIR (it is off by
// default) for tests of what it registers, followed by extra settings. The
// marker in OBIT_HOME stands for an install already past the one-time switch
// to off, so a resident's tick leaves the setting alone.
func autoOn(t *testing.T, extra string) {
	t.Helper()
	writeFixture(t, filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml"), "[proposals]\nmode = \"tiered\"\n"+extra)
	writeFixture(t, filepath.Join(os.Getenv("OBIT_HOME"), "migrated-1.3-auto-off"), "")
}

func writeFixture(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func commandRepo(t *testing.T) (string, core.Store) {
	t.Helper()
	obit, obitd := binaryPath(t, "obit"), binaryPath(t, "obitd")
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	testenv.InitRepo(t, repo)
	t.Chdir(repo)
	home := filepath.Join(root, "data")
	t.Setenv("OBIT_HOME", home)
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	autoOn(t, "")
	// A resident for this disposable home makes the CLI take its normal
	// Running branch before any scheduler lookup. No OS runner is installed.
	ctx, cancel := context.WithCancel(context.Background())
	daemon := exec.CommandContext(ctx, obitd, "--home", home, "--config-dir", os.Getenv("OBIT_CONFIG_DIR"))
	daemon.Env = os.Environ()
	var output bytes.Buffer
	daemon.Stdout, daemon.Stderr = &output, &output
	if err := daemon.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stop := exec.Command(obit, "daemon", "stop")
		stop.Env = daemon.Env
		out, err := stop.CombinedOutput()
		if err != nil {
			cancel()
			t.Errorf("stop isolated daemon: %s %v", out, err)
		}
		if err := daemon.Wait(); err != nil {
			log, _ := os.ReadFile(filepath.Join(home, "obitd.log"))
			t.Errorf("isolated daemon: %s %v\n%s", output.String(), err, log)
		}
		cancel()
	})
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if app.Running(home) {
			waitRunnerIdle(home)
			return repo, core.Store{Home: home}
		}
	}
	cancel()
	t.Fatal("isolated daemon did not start")
	return "", core.Store{}
}

// waitRunnerIdle lets the isolated resident finish its first tick. A hook
// that finds store.lock busy queues its write by design and returns no
// context, which would make protocol assertions race the resident.
func waitRunnerIdle(home string) {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(home, "sweep.lock")); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	lockable := func(name string) bool {
		release, ok, err := core.TryLockExclusive(filepath.Join(home, name))
		if ok {
			release()
		}
		return ok && err == nil
	}
	var idleSince time.Time
	for time.Now().Before(deadline) {
		if lockable("sweep.lock") && lockable("store.lock") {
			if idleSince.IsZero() {
				idleSince = time.Now()
			} else if time.Since(idleSince) >= 200*time.Millisecond {
				return
			}
		} else {
			idleSince = time.Time{}
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func execute(t *testing.T, command string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(binaryPath(t, "obit"), append([]string{command}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

type commitFailureFS struct {
	core.FS
	blocked string
}

func (f commitFailureFS) MovePinned(path string, want core.Identity, to string) error {
	if err := f.FS.MovePinned(path, want, to); err != nil {
		return err
	}
	return os.Mkdir(f.blocked, 0o700)
}
