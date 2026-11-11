//go:build linux || darwin

package app

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/runkey"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

const daemonName = "obitd"

func Running(string) bool { return false }
func Start(string) error {
	return fmt.Errorf("resident mode is unavailable; use obit daemon enable for a user timer: %w", core.ErrUnsupported)
}
func Stop(string) error { return nil } // no resident process can be started here

func DaemonPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Dir(exe), daemonName)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("%s is not executable", path)
	}
	return path, nil
}

func daemonArgs(home string) []string {
	args := []string{"--home", home}
	if config, err := userdirs.Config(); err == nil {
		args = append(args, "--config-dir", config)
	}
	return args
}

func RunOnce(home string) error {
	path, err := DaemonPath()
	if err != nil {
		return err
	}
	return spawn(path, append([]string{"--once"}, daemonArgs(home)...)...)
}

func SpawnSelf(args ...string) error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	return spawn(path, args...)
}

func spawn(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// GUI actions keep compiling; the POSIX runner uses one-shot timer invocations.
type Resident struct {
	Service *core.Service
	mu      sync.Mutex
}

func Serve(*core.Service, *log.Logger) error { return Start("") }

type RunRegistry = runkey.Registry
type AutostartPlan struct{ Name, Before, After string }

func (*AutostartPlan) Pending() bool { return false }
func (*AutostartPlan) Apply() error  { return core.ErrUnsupported }
func PlanAutostart(RunRegistry, string, string, string, bool) (*AutostartPlan, error) {
	return nil, core.ErrUnsupported
}
func SetAutostart(_ string, enabled bool) error {
	if !enabled {
		return nil
	}
	return fmt.Errorf("tray login startup is Windows-only: %w", core.ErrUnsupported)
}
func Autostart() string { return "" }
