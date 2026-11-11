package app

import (
	"encoding/json"
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

var TaskName = "obituary.timer"
var ErrScheduleDisabled = errors.New("user timer is disabled or inactive")

const Interval = 5 * time.Minute
const unitOwner = "# obituary-user-runner-v1\n"
const serviceName = "obituary.service"

// systemd/systemd man/systemd.service.xml and man/systemd.timer.xml: a
// oneshot service cannot overlap itself; Persistent catches missed calendars.
// Quoting follows man/systemd.syntax.xml, not shell quoting. Escape % and use
// ExecStart's ':' prefix to disable environment substitution of literal paths.
func unitArgument(value string) (string, error) {
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", errors.New("unit argument contains a control character")
	}
	value = strings.ReplaceAll(value, "%", "%%")
	return strconv.Quote(value), nil
}

func scheduleFiles(daemon, home, config string) (map[string]string, error) {
	args := []string{daemon, "--once", "--home", home, "--config-dir", config}
	quoted := make([]string, len(args))
	for i, arg := range args {
		q, err := unitArgument(arg)
		if err != nil {
			return nil, err
		}
		quoted[i] = q
	}
	metadata, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	service := unitOwner + "# argv: " + string(metadata) + "\n[Unit]\nDescription=Obituary maintenance\n\n[Service]\nType=oneshot\nExecStart=:" + strings.Join(quoted, " ") + "\n"
	timer := unitOwner + "[Unit]\nDescription=Obituary maintenance timer\n\n[Timer]\nOnCalendar=*:0/5\nPersistent=true\nUnit=" + serviceName + "\n\n[Install]\nWantedBy=timers.target\n"
	return map[string]string{serviceName: service, TaskName: timer}, nil
}

func userSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemd user manager unavailable (%s): %w; without systemd, schedule obitd --once --home <data> --config-dir <config> with a user crontab (*/5 * * * *)", strings.TrimSpace(string(out)), err)
	}
	return nil
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
	files, err := scheduleFiles(daemon, home, config)
	if err != nil {
		return err
	}
	dir, err := userdirs.SystemdUser()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	for name := range files {
		if err := ownedUnit(root, name); err != nil {
			return err
		}
	}
	for name, body := range files {
		if err := writeUnit(root, name, body); err != nil {
			return err
		}
	}
	if err := userSystemctl("daemon-reload"); err != nil {
		return err
	}
	return userSystemctl("enable", "--now", TaskName)
}

func ownedUnit(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("preserving non-regular unit %s", name)
	}
	data, err := root.ReadFile(name)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(data), unitOwner) {
		return fmt.Errorf("preserving unowned unit %s", name)
	}
	args := []string{"", "--once", "--home", "", "--config-dir", ""}
	if name == serviceName {
		args, err = unitArgs(data)
		if err != nil {
			return err
		}
	}
	files, err := scheduleFiles(args[0], args[3], args[5])
	if err != nil {
		return err
	}
	if string(data) != files[name] {
		return fmt.Errorf("preserving edited unit %s", name)
	}
	return nil
}

func writeUnit(root *os.Root, name, body string) error {
	tmp := name + ".obituary.tmp"
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	_, writeErr := f.WriteString(body)
	err = errors.Join(writeErr, f.Close())
	if err != nil {
		return err
	}
	return root.Rename(tmp, name)
}

func RemoveSchedule() error {
	dir, err := userdirs.SystemdUser()
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
	for _, name := range []string{TaskName, serviceName} {
		if err := ownedUnit(root, name); err != nil {
			return err
		}
	}
	if err := userSystemctl("disable", "--now", TaskName); err != nil {
		return err
	}
	for _, name := range []string{TaskName, serviceName} {
		if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return userSystemctl("daemon-reload")
}

func readSchedule() ([]string, error) {
	dir, err := userdirs.SystemdUser()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, serviceName))
	if err != nil {
		return nil, err
	}
	args, err := unitArgs(data)
	if err != nil {
		return nil, err
	}
	files, err := scheduleFiles(args[0], args[3], args[5])
	if err != nil {
		return nil, err
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if string(got) != want {
			return nil, fmt.Errorf("user unit %s has changed", name)
		}
	}
	return args, nil
}

func unitArgs(data []byte) ([]string, error) {
	line, _, ok := strings.Cut(strings.TrimPrefix(string(data), unitOwner), "\n")
	if !ok || !strings.HasPrefix(line, "# argv: ") {
		return nil, errors.New("unowned user service")
	}
	var args []string
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "# argv: ")), &args); err != nil {
		return nil, err
	}
	if len(args) != 6 || args[1] != "--once" || args[2] != "--home" || args[4] != "--config-dir" {
		return nil, errors.New("invalid user service arguments")
	}
	return args, nil
}

func ScheduleInstalled() bool { _, err := readSchedule(); return err == nil }

func ScheduleTarget() (string, error) {
	args, err := readSchedule()
	if err != nil {
		return "", err
	}
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = strconv.Quote(arg)
	}
	target := strings.Join(quoted, " ")
	if userSystemctl("is-enabled", "--quiet", TaskName) != nil || userSystemctl("is-active", "--quiet", TaskName) != nil {
		return target, ErrScheduleDisabled
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
