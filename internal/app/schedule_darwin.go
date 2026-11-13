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
	"time"

	"github.com/woodfishhhh/obituary/internal/userdirs"
)

var TaskName = "site.woodfish.obituary"
var ErrScheduleDisabled = errors.New("launchd user agent is not loaded; run obit daemon enable")

const Interval = 5 * time.Minute

const launchdOwner = "<!-- obituary user agent v1 -->\n"

func launchdContent(daemon, home, config string) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(xml.Header + launchdOwner + "<plist version=\"1.0\"><dict>\n<key>Label</key><string>" + TaskName + "</string>\n<key>ProgramArguments</key><array>\n")
	for _, arg := range []string{daemon, "--once", "--home", home, "--config-dir", config} {
		b.WriteString("<string>")
		if err := xml.EscapeText(&b, []byte(arg)); err != nil {
			return nil, err
		}
		b.WriteString("</string>\n")
	}
	fmt.Fprintf(&b, "</array>\n<key>StartInterval</key><integer>%d</integer>\n<key>RunAtLoad</key><true/>\n<key>ProcessType</key><string>Background</string>\n</dict></plist>\n", int(Interval.Seconds()))
	return b.Bytes(), nil
}

func launchdArguments(data []byte) ([]string, error) {
	var plist struct {
		Args []string `xml:"dict>array>string"`
	}
	if err := xml.Unmarshal(data, &plist); err != nil {
		return nil, err
	}
	a := plist.Args
	if len(a) != 6 || a[1] != "--once" || a[2] != "--home" || a[4] != "--config-dir" {
		return nil, errors.New("unowned launchd user agent")
	}
	want, err := launchdContent(a[0], a[3], a[5])
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, want) {
		return nil, errors.New("preserving edited or unowned launchd user agent")
	}
	return a, nil
}

func readLaunchd(root *os.Root) ([]byte, error) {
	name := TaskName + ".plist"
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("preserving non-regular launchd user agent")
	}
	data, err := root.ReadFile(name)
	if err == nil {
		_, err = launchdArguments(data)
	}
	return data, err
}

func launchdDomain() string  { return "gui/" + strconv.Itoa(os.Getuid()) }
func launchdService() string { return launchdDomain() + "/" + TaskName }

var launchctlExecutable = "launchctl"

func userLaunchctl(args ...string) error {
	out, err := exec.Command(launchctlExecutable, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

func unloadLaunchd() error {
	err := userLaunchctl("print", launchdService())
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 113 {
		return nil // launchctl reports no service with this label.
	}
	if err != nil {
		return err
	}
	return userLaunchctl("bootout", launchdService())
}

func InstallSchedule(home string) error {
	daemon, err := DaemonPath()
	if err != nil {
		return err
	}
	config, err := userdirs.Config()
	if err != nil {
		return err
	}
	return installLaunchd(daemon, home, config)
}

func installLaunchd(daemon, home, config string) error {
	units, err := PreviewSchedule(daemon, home, config, true)
	if err != nil {
		return err
	}
	unit := units[0]
	if err := os.MkdirAll(filepath.Dir(unit.Path), 0o700); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(unit.Path))
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := readLaunchd(root); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := writeLaunchd(root, []byte(unit.Content)); err != nil {
		return err
	}
	if err := unloadLaunchd(); err != nil {
		return err
	}
	if err := userLaunchctl("enable", launchdService()); err != nil {
		return err
	}
	return userLaunchctl("bootstrap", launchdDomain(), unit.Path)
}

func writeLaunchd(root *os.Root, data []byte) error {
	name := TaskName + ".plist"
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
	return syncLaunchdDirectory(root)
}

func syncLaunchdDirectory(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}

func RemoveSchedule() error {
	dir, err := userdirs.LaunchAgents()
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	before, err := readLaunchd(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := unloadLaunchd(); err != nil {
		return err
	}
	after, err := readLaunchd(root)
	if err != nil {
		return err
	}
	if !bytes.Equal(before, after) {
		return errors.New("launchd user agent changed while unloading; file preserved")
	}
	if err := root.Remove(TaskName + ".plist"); err != nil {
		return err
	}
	return syncLaunchdDirectory(root)
}

func readSchedule() ([]string, error) {
	dir, err := userdirs.LaunchAgents()
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	data, err := readLaunchd(root)
	if err != nil {
		return nil, err
	}
	return launchdArguments(data)
}

func ScheduleInstalled() bool { _, err := readSchedule(); return err == nil }

func ScheduleTarget() (string, error) {
	args, err := readSchedule()
	if err != nil {
		return "", err
	}
	for i := range args {
		args[i] = strconv.Quote(args[i])
	}
	target := strings.Join(args, " ")
	if err := userLaunchctl("print", launchdService()); err != nil {
		return target, errors.Join(ErrScheduleDisabled, err)
	}
	return target, nil
}

func ScheduleServes(home string) bool {
	args, err := readSchedule()
	if err != nil || filepath.Clean(args[3]) != filepath.Clean(home) {
		return false
	}
	_, err = ScheduleTarget()
	return err == nil
}
