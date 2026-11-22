package app

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

const loginReceipt = "obitd.login-task.xml"

func loginTaskName(home string) string { return startupName(home) + " Resident" }

func xmlText(text string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(text))
	return b.String()
}

func loginTaskXML(daemon, home, config, sid string) string {
	const head = "<?xml version=\"1.0\" encoding=\"UTF-16\"?>\n<Task version=\"1.4\" xmlns=\"http://schemas.microsoft.com/windows/2004/02/mit/task\">\n"
	account := "<UserId>" + xmlText(sid) + "</UserId>"
	args := windows.ComposeCommandLine([]string{"--supervise", "--home", home, "--config-dir", config})
	settings := strings.Replace(taskSettingsXML, "PT30M", "PT0S", 1)
	settings = strings.Replace(settings, "  </Settings>", "    <RestartOnFailure><Interval>PT1M</Interval><Count>3</Count></RestartOnFailure>\n  </Settings>", 1)
	return head + "<RegistrationInfo><URI>\\" + xmlText(loginTaskName(home)) + "</URI></RegistrationInfo>\n" +
		"<Triggers><LogonTrigger>" + account + "</LogonTrigger></Triggers>\n" +
		"<Principals><Principal id=\"Author\">" + account + "<LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal></Principals>\n" +
		settings + "<Actions Context=\"Author\"><Exec><Command>" + xmlText(daemon) + "</Command><Arguments>" + xmlText(args) + "</Arguments></Exec></Actions></Task>"
}

// Never adopt a same-named registration. Task Scheduler normalizes XML; keep its own export.
func installedLoginTask(home, daemon, config string) (string, error) {
	name := loginTaskName(home)
	current, err := registeredTaskXML(name)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	before, err := core.ReadMetadata(filepath.Join(home, loginReceipt))
	if err != nil || string(before) != current || !ownedTaskAction(current, daemon, "--supervise", home, config) {
		return "", fmt.Errorf("preserving unverified or edited login task %q", name)
	}
	return current, nil
}

func createLoginTask(home, daemon, config string) error {
	current, err := installedLoginTask(home, daemon, config)
	if err != nil || current != "" {
		return err
	}
	u, err := user.Current()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	name := loginTaskName(home)
	if err := registerTask(name, loginTaskXML(daemon, home, config, u.Uid)); err != nil {
		return err
	}
	actual, err := registeredTaskXML(name)
	if err != nil {
		return err
	}
	if !ownedTaskAction(actual, daemon, "--supervise", home, config) {
		return fmt.Errorf("login task %q changed during registration", name)
	}
	return core.WriteMetadata(filepath.Join(home, loginReceipt), []byte(actual))
}

func removeLoginTask(home, daemon, config string) error {
	current, err := installedLoginTask(home, daemon, config)
	if err != nil || current == "" {
		return err
	}
	name := loginTaskName(home)
	latest, err := registeredTaskXML(name)
	if err != nil {
		return err
	}
	if latest != current {
		return fmt.Errorf("login task %q changed before removal", name)
	}
	if out, err := schtasks("/Delete", "/TN", name, "/F", "/HRESULT"); err != nil {
		return fmt.Errorf("remove login task: %w: %s", err, out)
	}
	_, err = registeredTaskXML(name)
	if !errors.Is(err, os.ErrNotExist) {
		return errors.Join(err, errors.New("login task remains after removal"))
	}
	return nil
}
