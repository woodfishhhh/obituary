//go:build darwin && desktop

package app

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/runkey"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

const daemonName = "obitd"

func DesktopSession() bool {
	return os.Getenv("SSH_CONNECTION") == "" && userTrayLaunchctl("print", trayLaunchdDomain()) == nil
}

const legacyTrayName = "site.woodfish.obituary.tray"
const trayTaskName = legacyTrayName + ".supervised"

const trayLaunchdOwner = "<!-- obituary tray user agent v1 -->\n"

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
	if target := Autostart(); target != "" {
		if err := userTrayLaunchctl("print", trayLaunchdService()); err != nil {
			var status *exec.ExitError
			if !errors.As(err, &status) || status.ExitCode() != 113 {
				return err
			}
			if err := userTrayLaunchctl("bootstrap", trayLaunchdDomain(), target); err != nil {
				return err
			}
		} else if err := userTrayLaunchctl("kickstart", trayLaunchdService()); err != nil {
			return err
		}
		return waitFor(func() bool { return Running(home) }, 5*time.Second, "managed tray did not start")
	}
	cmd := exec.Command(path, append([]string{"--tray"}, daemonArgs(home)...)...)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", path, err)
	}
	if err := cmd.Process.Release(); err != nil {
		return err
	}
	return waitFor(func() bool { return Running(home) }, 5*time.Second, "daemon did not start")
}

func Stop(home string) error {
	if !Running(home) {
		if err := cancelPendingRestart(home); err != nil {
			return err
		}
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

func cancelPendingRestart(home string) error {
	if Running(home) {
		return nil
	}
	dir, err := userdirs.LaunchAgents()
	if err != nil {
		return err
	}
	before, err := readTrayLaunchd(filepath.Join(dir, trayTaskName+".plist"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	daemon, err := DaemonPath()
	if err != nil {
		return err
	}
	config, err := userdirs.Config()
	if err != nil {
		return err
	}
	want, err := trayLaunchdContent(daemon, home, config)
	if err != nil {
		return err
	}
	if before != string(want) {
		return errors.New("preserving tray agent bound to another installation")
	}
	return unloadTrayLaunchd()
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
	return filepath.EvalSymlinks(path)
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
	current, err := readTrayLaunchd(p.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if current != p.Before {
		return fmt.Errorf("launchd tray agent changed since planning")
	}
	if !p.Pending() {
		return nil
	}
	if p.enabled {
		if err := os.MkdirAll(filepath.Dir(p.path), 0o700); err != nil {
			return err
		}
	}
	if err := unloadTrayLaunchd(); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(p.path))
	if err != nil {
		return err
	}
	defer root.Close()
	if current, err = readTrayLaunchdRoot(root); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	} else if current != p.Before {
		return fmt.Errorf("launchd tray agent changed while unloading")
	}
	if !p.enabled {
		if err := root.Remove(filepath.Base(p.path)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return syncTrayLaunchdDirectory(root)
	}
	if err := writeTrayLaunchd(root, []byte(p.After)); err != nil {
		return err
	}
	if err := userTrayLaunchctl("enable", trayLaunchdService()); err != nil {
		return err
	}
	return userTrayLaunchctl("bootstrap", trayLaunchdDomain(), p.path)
}

func PlanAutostart(_ RunRegistry, daemon, home, config string, enabled bool) (*AutostartPlan, error) {
	dir, err := userdirs.LaunchAgents()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, trayTaskName+".plist")
	before, err := readTrayLaunchd(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	content, err := trayLaunchdContent(daemon, home, config)
	if err != nil {
		return nil, err
	}
	after := ""
	if enabled {
		after = string(content)
	}
	if before != "" && before != string(content) {
		return nil, fmt.Errorf("autostart entry is not managed by Obituary: %s", path)
	}
	return &AutostartPlan{Name: path, Before: before, After: after, path: path, content: string(content), enabled: enabled}, nil
}

func trayLaunchdContent(daemon, home, config string) ([]byte, error) {
	return trayDefinition(daemon, home, config, trayTaskName)
}

func trayDefinition(daemon, home, config, label string) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(xml.Header + trayLaunchdOwner + "<plist version=\"1.0\"><dict>\n<key>Label</key><string>" + label + "</string>\n<key>ProgramArguments</key><array>\n")
	args := []string{daemon, "--tray", "--home", home, "--config-dir", config}
	if label == trayTaskName {
		args = append(args, "--managed")
	}
	for _, arg := range args {
		b.WriteString("<string>")
		if err := xml.EscapeText(&b, []byte(arg)); err != nil {
			return nil, err
		}
		b.WriteString("</string>\n")
	}
	b.WriteString("</array>\n<key>RunAtLoad</key><true/>\n")
	if label == trayTaskName {
		b.WriteString("<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>\n<key>ThrottleInterval</key><integer>60</integer>\n")
	}
	b.WriteString("<key>ProcessType</key><string>Interactive</string>\n</dict></plist>\n")
	return b.Bytes(), nil
}

func readTrayLaunchd(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("preserving non-regular tray job: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if err := verifyTrayLaunchd(data); err != nil {
		return "", err
	}
	return string(data), nil
}

func readTrayLaunchdRoot(root *os.Root) (string, error) {
	data, err := root.ReadFile(trayTaskName + ".plist")
	if err != nil {
		return "", err
	}
	if err := verifyTrayLaunchd(data); err != nil {
		return "", err
	}
	return string(data), nil
}

func verifyTrayLaunchd(data []byte) error {
	var plist struct {
		Args []string `xml:"dict>array>string"`
	}
	if err := xml.Unmarshal(data, &plist); err != nil {
		return err
	}
	if len(plist.Args) != 7 || plist.Args[1] != "--tray" || plist.Args[2] != "--home" || plist.Args[4] != "--config-dir" || plist.Args[6] != "--managed" {
		return errors.New("unowned launchd tray agent")
	}
	want, err := trayLaunchdContent(plist.Args[0], plist.Args[3], plist.Args[5])
	if err != nil {
		return err
	}
	if !bytes.Equal(data, want) {
		return errors.New("preserving edited or unowned launchd tray agent")
	}
	return nil
}

func trayLaunchdDomain() string  { return "gui/" + strconv.Itoa(os.Getuid()) }
func trayLaunchdService() string { return trayLaunchdDomain() + "/" + trayTaskName }

var trayLaunchctlExecutable = "launchctl"

func userTrayLaunchctl(args ...string) error {
	out, err := exec.Command(trayLaunchctlExecutable, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

func unloadTrayLaunchd() error {
	err := userTrayLaunchctl("print", trayLaunchdService())
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 113 {
		return nil
	}
	if err != nil {
		return err
	}
	return userTrayLaunchctl("bootout", trayLaunchdService())
}

func writeTrayLaunchd(root *os.Root, data []byte) error {
	name := trayTaskName + ".plist"
	tmp := name + ".tmp"
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	_, writeErr := f.Write(data)
	if err := errors.Join(writeErr, f.Sync(), f.Close()); err != nil {
		return err
	}
	if err := root.Rename(tmp, name); err != nil {
		return err
	}
	return syncTrayLaunchdDirectory(root)
}

func syncTrayLaunchdDirectory(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
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
	dir, err := userdirs.LaunchAgents()
	if err != nil {
		return ""
	}
	path := filepath.Join(dir, trayTaskName+".plist")
	if _, err := readTrayLaunchd(path); err != nil {
		return ""
	}
	return path
}
