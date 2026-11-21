//go:build linux && desktop

package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/userdirs"
)

const trayUnit = "obituary-tray.service"

func graphicalLogin() bool {
	return os.Getenv("SSH_CONNECTION") == "" && os.Getenv("SSH_TTY") == "" && os.Getenv("SSH_CLIENT") == "" && (os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "")
}

func graphicalManager() bool {
	return graphicalLogin() && userSystemctl("is-active", "--quiet", "graphical-session.target") == nil
}

func trayUnitContent(daemon, home, config string) (string, error) {
	args := []string{daemon, "--managed", "--tray", "--home", home, "--config-dir", config}
	for i, arg := range args {
		q, err := unitArgument(arg)
		if err != nil {
			return "", err
		}
		args[i] = q
	}
	return "# obituary-user-tray-v1\n[Unit]\nDescription=Obituary tray\nPartOf=graphical-session.target\nStartLimitIntervalSec=900\nStartLimitBurst=4\nConditionEnvironment=|DISPLAY\nConditionEnvironment=|WAYLAND_DISPLAY\n\n[Service]\nType=exec\nExecStart=:" + strings.Join(args, " ") + "\nRestart=on-failure\nRestartSec=60\nRestartForceExitStatus=SIGTERM SIGHUP SIGINT SIGPIPE\n\n[Install]\nWantedBy=graphical-session.target\n", nil
}

func readTrayUnit(daemon, home, config string) (path, content, before string, err error) {
	dir, err := userdirs.SystemdUser()
	if err != nil {
		return
	}
	path = filepath.Join(dir, trayUnit)
	content, err = trayUnitContent(daemon, home, config)
	if err != nil {
		return
	}
	before, err = readAutostart(path)
	if err != nil {
		return
	}
	if before != "" && before != content {
		err = fmt.Errorf("preserving edited or foreign unit %s", path)
		return
	}
	return
}

func systemdProperty(name string) (string, error) {
	out, err := exec.Command("systemctl", "--user", "show", trayUnit, "-p", name, "--value").Output()
	return strings.TrimSpace(string(out)), err
}

func verifyTrayUnitSource(path string) error {
	fragment, err := systemdProperty("FragmentPath")
	if err != nil {
		return err
	}
	drops, err := systemdProperty("DropInPaths")
	if err != nil {
		return err
	}
	if (fragment != "" && fragment != path) || drops != "" {
		return fmt.Errorf("preserving foreign unit or drop-ins: %s %s", fragment, drops)
	}
	return nil
}

func (p *AutostartPlan) applyTrayUnit() error {
	if err := verifyTrayUnitSource(p.path); err != nil {
		return err
	}
	current, err := readAutostart(p.path)
	if err != nil {
		return err
	}
	if current != p.Before {
		return errors.New("tray unit changed since preview")
	}
	if !p.Pending() {
		return nil
	}
	if !p.enabled {
		if err := userSystemctl("disable", "--now", trayUnit); err != nil {
			return err
		}
		now, err := readAutostart(p.path)
		if err != nil {
			return err
		}
		if now != current {
			return errors.New("tray unit changed while stopping; preserved")
		}
		if err := os.Remove(p.path); err != nil {
			return err
		}
		return userSystemctl("daemon-reload")
	}
	// Publish exclusively; the file that appeared after preview may belong to the user.
	if err := p.writeNewEntry(); err != nil {
		return err
	}
	if err := userSystemctl("daemon-reload"); err != nil {
		return err
	}
	if err := verifyTrayUnitSource(p.path); err != nil {
		return err
	}
	return userSystemctl("enable", "--now", trayUnit)
}

func startManagedUnit(daemon, home string) (bool, error) {
	config, err := userdirs.Config()
	if err != nil {
		return false, err
	}
	path, _, before, err := readTrayUnit(daemon, home, config)
	if err != nil || before == "" {
		return false, err
	}
	if err := verifyTrayUnitSource(path); err != nil {
		return true, err
	}
	if !graphicalManager() {
		return true, errors.New("graphical systemd user session is unavailable")
	}
	if err := userSystemctl("start", trayUnit); err != nil {
		return true, err
	}
	return true, waitFor(func() bool { return Running(home) }, 5*time.Second, "supervised tray did not start")
}
