//go:build linux && desktop

package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/runkey"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

const daemonName = "obitd"

func stopPath(home string) string { return filepath.Join(home, "obitd.stop") }

func Running(home string) bool {
	release, ok, err := core.TryLockExclusive(filepath.Join(home, "obitd.lock"))
	if err != nil || !ok {
		return err == nil && !ok
	}
	release()
	return false
}

func Start(home string) error {
	if Running(home) {
		return nil
	}
	path, err := DaemonPath()
	if err != nil {
		return err
	}
	if err := startAt(home, path); err != nil {
		return err
	}
	return waitFor(func() bool { return Running(home) }, 5*time.Second, "daemon did not start")
}

func startAt(home, path string) error {
	cmd := exec.Command(path, append([]string{"--tray"}, daemonArgs(home)...)...)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", path, err)
	}
	return cmd.Process.Release()
}

func Stop(home string) error {
	if !Running(home) {
		return clearStopRequest(home)
	}
	if err := os.WriteFile(stopPath(home), nil, 0o600); err != nil {
		return err
	}
	if err := waitFor(func() bool { return !Running(home) }, 10*time.Second, "daemon did not stop"); err != nil {
		return err
	}
	return clearStopRequest(home)
}

func clearStopRequest(home string) error {
	err := os.Remove(stopPath(home))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

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

func RunOnce(home string) error { return runDaemonFlag(home, "--once") }

func runDaemonFlag(home, flag string) error {
	path, err := DaemonPath()
	if err != nil {
		return err
	}
	return spawn(path, append([]string{flag}, daemonArgs(home)...)...)
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
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

func waitFor(cond func() bool, limit time.Duration, msg string) error {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if cond() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New(msg)
}

type RunRegistry = runkey.Registry

type AutostartPlan struct {
	Name, Before, After string
	path                string
	content             string
	enabled             bool
}

func (p *AutostartPlan) Pending() bool { return p.Before != p.After }

func (p *AutostartPlan) Apply() error {
	current, err := readAutostart(p.path)
	if err != nil {
		return err
	}
	if current != p.Before {
		return fmt.Errorf("autostart entry changed since planning")
	}
	if !p.Pending() {
		return nil
	}
	if !p.enabled {
		return os.Remove(p.path)
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.path), ".obituary-autostart-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(p.content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Link(tmpName, p.path); err != nil {
		return fmt.Errorf("write autostart entry: %w", err)
	}
	return os.Remove(tmpName)
}

func PlanAutostart(_ RunRegistry, daemon, home, config string, enabled bool) (*AutostartPlan, error) {
	dir, err := userdirs.Autostart()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "obituary.desktop")
	before, err := readAutostart(path)
	if err != nil {
		return nil, err
	}
	if strings.ContainsAny(daemon+home+config, "\r\n\x00") {
		return nil, errors.New("autostart paths must fit on a single desktop entry line")
	}
	content := desktopEntry(daemon, home, config)
	after := ""
	if enabled {
		after = content
	}
	if before != "" && !managedDesktopEntry(before) {
		return nil, fmt.Errorf("autostart entry is not managed by Obituary: %s", path)
	}
	return &AutostartPlan{Name: path, Before: before, After: after, path: path, content: content, enabled: enabled}, nil
}

func readAutostart(path string) (string, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return "", fmt.Errorf("autostart entry is not a managed regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	return string(data), err
}

func managedDesktopEntry(content string) bool {
	return strings.Contains(content, "\nX-Obituary-Managed=true\n") || strings.HasPrefix(content, "X-Obituary-Managed=true\n")
}

func desktopEntry(daemon, home, config string) string {
	return "[Desktop Entry]\nType=Application\nName=Obituary\nComment=Keep expiring files safe\nExec=" + desktopArg(daemon) + " --tray --home " + desktopArg(home) + " --config-dir " + desktopArg(config) + "\nIcon=obituary\nTerminal=false\nX-GNOME-Autostart-enabled=true\nX-Obituary-Managed=true\n"
}

func desktopArg(value string) string {
	// Desktop entries unescape the string value before interpreting Exec quoting.
	value = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "%", "%%").Replace(value)
	return "\"" + value + "\""
}

func SetAutostart(home string, enabled bool) error {
	daemon, err := DaemonPath()
	if err != nil {
		return err
	}
	config, err := userdirs.Config()
	if err != nil {
		return err
	}
	plan, err := PlanAutostart(nil, daemon, home, config, enabled)
	if err != nil {
		return err
	}
	return plan.Apply()
}

func Autostart() string {
	dir, err := userdirs.Autostart()
	if err != nil {
		return ""
	}
	path := filepath.Join(dir, "obituary.desktop")
	content, err := readAutostart(path)
	if err != nil || !managedDesktopEntry(content) {
		return ""
	}
	return path
}
