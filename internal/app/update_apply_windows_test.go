package app

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/update"
)

func TestUpdaterRejectsInvalidReadiness(t *testing.T) {
	for _, tc := range []struct{ message, want string }{
		{`{"Error":"an update is already in progress"}`, "already in progress"},
		{`{}`, "missing update handoff receipt"},
		{"", "EOF"},
	} {
		if _, err := readUpdaterReady(t.Context(), strings.NewReader(tc.message)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("message=%q err=%v", tc.message, err)
		}
	}
	spawnFailure := errors.New("spawn failed")
	if _, err := launchUpdater(t.Context(), &exec.Cmd{}, func(*exec.Cmd) error { return spawnFailure }); !errors.Is(err, spawnFailure) {
		t.Fatal(err)
	}
}

func TestUpdaterDetachedReadinessAndProcessWait(t *testing.T) {
	if os.Getenv("OBIT_TEST_UPDATE_CHILD") == "1" {
		if err := json.NewEncoder(os.Stdout).Encode(update.ApplyResult{TargetVersion: "9.0.0", LogPath: "test.log"}); err != nil {
			os.Exit(2)
		}
		_, err := io.Copy(io.Discard, os.Stdin)
		if err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
	if testing.Short() {
		t.Skip("starts a detached process")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	in, release, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	defer release.Close()
	cmd := exec.Command(exe, "-test.run=^TestUpdaterDetachedReadinessAndProcessWait$")
	cmd.Stdin = in
	cmd.Env = append(os.Environ(), "OBIT_TEST_UPDATE_CHILD=1")
	detach(cmd)
	var wait func() error
	result, err := launchUpdater(t.Context(), cmd, func(cmd *exec.Cmd) error {
		if err := cmd.Start(); err != nil {
			return err
		}
		var closeHandle func()
		var err error
		wait, closeHandle, err = updaterWait(cmd.Process.Pid)
		if closeHandle != nil {
			t.Cleanup(closeHandle)
		}
		return err
	})
	if err != nil || result != (update.ApplyResult{TargetVersion: "9.0.0", LogPath: "test.log"}) {
		t.Fatalf("receipt=%+v, err=%v", result, err)
	}
	if err := release.Close(); err != nil {
		t.Fatal(err)
	}
	if err := wait(); err != nil {
		t.Fatal(err)
	}
}

func TestCopyUpdaterRegistersScratch(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "data")
	t.Setenv("OBIT_HOME", home)
	exe := filepath.Join(root, "obit.exe")
	if err := os.WriteFile(exe, []byte("original obit"), 0o700); err != nil {
		t.Fatal(err)
	}
	worker, err := copyUpdater(home, exe)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(worker)
	if err != nil || string(data) != "original obit" || worker == exe {
		t.Fatalf("worker=%s data=%q err=%v", worker, data, err)
	}
	svc, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	state, err := svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range state.Items {
		if filepath.Clean(item.Path) == filepath.Dir(worker) && !item.ExpiresAt.IsZero() {
			return
		}
	}
	t.Fatalf("worker directory %s has no registered lifespan", filepath.Dir(worker))
}
