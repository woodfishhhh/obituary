//go:build windows

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestFolderResidentStopsNormallyWithPendingDirectories(t *testing.T) {
	if testing.Short() {
		t.Skip("native resident and CLI flow")
	}
	root := t.TempDir()
	t.Chdir(root)
	for _, key := range []string{"OBIT_HOME", "OBIT_CONFIG_DIR", "OBIT_USER_HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME"} {
		t.Setenv(key, filepath.Join(root, key))
	}
	home, config := os.Getenv("OBIT_HOME"), os.Getenv("OBIT_CONFIG_DIR")
	writeFixture(t, filepath.Join(config, "config.toml"), "[watch]\nstable_window = \"1s\"\n[explorer]\nenabled = false\n")
	dir := filepath.Join(root, "folder")
	for _, name := range []string{"file.txt", "one/keep.txt", "two/keep.txt"} {
		writeFixture(t, filepath.Join(dir, name), "keep")
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, binaryPath(t, "obitd"), "--tray=false", "--home", home, "--config-dir", config)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		out, err := exec.Command(binaryPath(t, "obit"), "daemon", "stop").CombinedOutput()
		if err != nil {
			cancel()
			t.Errorf("stop: %s %v", out, err)
		}
		if err := cmd.Wait(); err != nil {
			data, _ := os.ReadFile(filepath.Join(home, "obitd.log"))
			t.Errorf("resident exit: %v %s\n%s", err, output.String(), data)
		}
		if app.Running(home) {
			t.Error("resident remains running")
		}
	})
	waitFolderResident(t, home)
	if out, stderr, err := execute(t, "watch", dir, "2s", "--no-daemon"); err != nil {
		t.Fatal(out, stderr, err)
	}
	out, _, err := execute(t, "proposals", "--json")
	var proposals []core.Proposal
	if err != nil || json.Unmarshal([]byte(out), &proposals) != nil || len(proposals) != 1 {
		t.Fatalf("policy proposal: %s %v", out, err)
	}
	if out, stderr, err := execute(t, "approve", fmt.Sprint(proposals[0].ID)); err != nil {
		t.Fatal(out, stderr, err)
	}
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		st, err := (core.Store{Home: home}).Read()
		if err == nil && len(st.Proposals) == 2 && len(st.Burials) == 1 {
			break
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("folder flow: proposals=%d burials=%d err=%v", len(st.Proposals), len(st.Burials), err)
		}
	}
}

func TestDaemonStartAfterLegacyStop(t *testing.T) {
	if testing.Short() {
		t.Skip("native daemon restart and CLI flow")
	}
	root := t.TempDir()
	for _, key := range []string{"OBIT_HOME", "OBIT_CONFIG_DIR", "OBIT_USER_HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME"} {
		t.Setenv(key, filepath.Join(root, key))
	}
	home := os.Getenv("OBIT_HOME")
	cmd := exec.Command(binaryPath(t, "obitd"), "--tray=false", "--home", home, "--config-dir", os.Getenv("OBIT_CONFIG_DIR"))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = exec.Command(binaryPath(t, "obit"), "daemon", "stop").CombinedOutput()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if app.Running(home) {
			t.Error("test daemon remains running")
		}
	})
	waitFolderResident(t, home)
	processFile := filepath.Join(home, "obitd.process.json")
	if err := os.Remove(processFile); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(binaryPath(t, "obit"), "daemon", "stop").CombinedOutput()
	if err == nil || !bytes.Contains(out, []byte("legacy resident")) {
		t.Fatalf("legacy stop diagnostic: %s %v", out, err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "obitd.stop")); !os.IsNotExist(err) {
		t.Fatalf("legacy stop request remains: %v", err)
	}
	if out, err := exec.Command(binaryPath(t, "obit"), "daemon", "start").CombinedOutput(); err != nil {
		t.Fatalf("restart: %s %v", out, err)
	}
	waitFolderResident(t, home)
	if out, err := exec.Command(binaryPath(t, "obit"), "daemon", "stop").CombinedOutput(); err != nil {
		t.Fatalf("restarted daemon exit: %s %v", out, err)
	}
}

func waitFolderResident(t *testing.T, home string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if !app.Running(home) {
			continue
		}
		var process struct {
			PID    int  `json:"pid"`
			Closed bool `json:"closed"`
		}
		data, err := os.ReadFile(filepath.Join(home, "obitd.process.json"))
		if err != nil || json.Unmarshal(data, &process) != nil || process.PID == 0 || process.Closed {
			continue
		}
		// The startup log is emitted after the process receipt's writer closes.
		// A visible mutex or file alone can still precede that close on Windows.
		data, err = os.ReadFile(filepath.Join(home, "obitd.log"))
		if err == nil && bytes.Contains(data, fmt.Appendf(nil, "obitd started (pid %d,", process.PID)) {
			return
		}
	}
	t.Fatal("resident did not start")
}

func TestOnceHelper(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	if home := os.Getenv("OBIT_TEST_ONCE_HOME"); home != "" {
		cmd := exec.Command(binaryPath(t, "obitd"), "--once", "--home", home, "--config-dir", os.Getenv("OBIT_TEST_ONCE_CONFIG"))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("--once: %s, %v", out, err)
		}
	}
}

func TestOnceRecoversWithoutNextDue(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	blocked := filepath.Join(home, "store.json.tmp")
	svc := &core.Service{Store: core.Store{Home: home}, FS: commitFailureFS{FS: winfs.FS{}, blocked: blocked}}
	path := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Bury([]string{path}, false, ""); err == nil {
		t.Fatal("commit fault not reached")
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.Read()
	if _, due := st.NextDue(); err != nil || due {
		t.Fatalf("fixture has a due time: %+v, %v", st, err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestOnceHelper$")
	cmd.Env = append(os.Environ(), "OBIT_TEST_ONCE_HOME="+home, "OBIT_TEST_ONCE_CONFIG="+filepath.Join(dir, "config"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--once: %s, %v", out, err)
	}
	st, err = svc.Store.Read()
	if err != nil || len(st.Burials) != 1 || st.Burials[0].Path != path {
		t.Fatalf("--once skipped pending manifest: %+v, %v", st, err)
	}
	data, err := os.ReadFile(st.Burials[0].Grave)
	if err != nil || string(data) != "keep" {
		t.Fatal("--once lost buried content", err)
	}
}
