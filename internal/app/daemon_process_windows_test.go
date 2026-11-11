package app

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func init() {
	beforeTests = func() {
		if os.Getenv("OBIT_TEST_STOP_HOME") != "" {
			daemonStopProcessHelper()
		}
	}
}

func daemonStopProcessHelper() {
	home := os.Getenv("OBIT_TEST_STOP_HOME")
	r, err := NewResident(&core.Service{Store: core.Store{Home: home}, FS: winfs.FS{}}, log.New(io.Discard, "", 0))
	if err != nil || r == nil {
		os.Exit(3)
	}
	for {
		if _, err := os.Stat(stopPath(home)); err == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	code, _ := strconv.Atoi(os.Getenv("OBIT_TEST_STOP_CODE"))
	if code == 0 || code == -2 {
		r.Close()
	}
	if code == -2 {
		for {
			if _, err := os.Stat(filepath.Join(home, "release")); err == nil {
				break
			}
			time.Sleep(time.Millisecond)
		}
		code = 0
	}
	os.Exit(code)
}

func TestStopChecksActualProcessExit(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a native child process")
	}
	for _, code := range []int{0, 2, -1} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			home := t.TempDir()
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(exe)
			cmd.Env = append(os.Environ(), "OBIT_TEST_STOP_HOME="+home, "OBIT_TEST_STOP_CODE="+strconv.Itoa(code))
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
			if err := waitFor(func() bool { p, err := readDaemonProcess(home); return err == nil && p.PID == uint32(cmd.Process.Pid) }, 5*time.Second, "helper not ready"); err != nil {
				t.Fatal(err)
			}
			if code == -1 {
				if err := os.Remove(filepath.Join(home, "obitd.process.json")); err != nil {
					t.Fatal(err)
				}
			}
			err = Stop(home)
			if code == 0 && err != nil || code == 2 && (err == nil || !strings.Contains(err.Error(), "code 2")) {
				t.Fatalf("exit %d: Stop returned %v", code, err)
			}
			if code == -1 && (err == nil || !strings.Contains(err.Error(), "legacy resident")) {
				t.Errorf("missing compatibility diagnostic: %v", err)
			}
			if _, err := os.Stat(stopPath(home)); !os.IsNotExist(err) {
				t.Fatalf("stopped process left stop request: %v", err)
			}
			r, err := NewResident(&core.Service{Store: core.Store{Home: home}, FS: winfs.FS{}}, log.New(io.Discard, "", 0))
			if err != nil || r == nil {
				t.Fatalf("restart: %v", err)
			}
			defer r.Close()
			if _, stop, err := r.step(time.Hour); err != nil || stop {
				t.Fatalf("restarted resident stopped: stop=%t err=%v", stop, err)
			}
		})
	}
}

func TestStopRetainsRequestUntilProcessExit(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a native child process and exercises the stop timeout")
	}
	home := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), "OBIT_TEST_STOP_HOME="+home, "OBIT_TEST_STOP_CODE=-2")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	if err := waitFor(func() bool { p, err := readDaemonProcess(home); return err == nil && p.PID == uint32(cmd.Process.Pid) }, 5*time.Second, "helper not ready"); err != nil {
		t.Fatal(err)
	}
	if err := Stop(home); err == nil || !strings.Contains(err.Error(), "process did not exit") {
		t.Fatalf("live process reported stopped: %v", err)
	}
	if _, err := os.Stat(stopPath(home)); err != nil {
		t.Fatalf("unconfirmed process lost stop request: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := Stop(home); err != nil {
		t.Fatal(err)
	}
}

func TestStopClearsRequestAfterShutdownError(t *testing.T) {
	home := t.TempDir()
	r, err := NewResident(&core.Service{Store: core.Store{Home: home}, FS: winfs.FS{}}, log.New(io.Discard, "", 0))
	if err != nil || r == nil {
		t.Fatalf("resident: %v", err)
	}
	r.runErr = errors.New("shutdown failed")
	done, stopWaiting := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		defer r.Close()
		for {
			if _, err := os.Stat(stopPath(home)); err == nil {
				return
			}
			select {
			case <-stopWaiting:
				return
			case <-time.After(time.Millisecond):
			}
		}
	}()
	err = Stop(home)
	close(stopWaiting)
	<-done
	if err == nil || !strings.Contains(err.Error(), "shutdown failed") {
		t.Fatalf("shutdown error lost: %v", err)
	}
	if _, err := os.Stat(stopPath(home)); !os.IsNotExist(err) {
		t.Fatalf("stopped process left stop request: %v", err)
	}
}

func TestDaemonExitRequiresShutdownReceipt(t *testing.T) {
	home := t.TempDir()
	if err := writeDaemonProcess(home, false, nil); err != nil {
		t.Fatal(err)
	}
	check, closeHandle, err := daemonExitCheck(home)
	if err != nil {
		t.Fatal(err)
	}
	defer closeHandle()
	if err := check(); err == nil {
		t.Fatal("unfinished shutdown reported success")
	}
	if err := writeDaemonProcess(home, true, nil); err != nil {
		t.Fatal(err)
	}
	if err := check(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "obitd.process.json")); err != nil {
		t.Fatal(err)
	}
}
