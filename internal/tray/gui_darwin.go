//go:build darwin && desktop

package tray

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/i18n"
)

type guiAction struct {
	path    string
	title   string
	tooltip string
	enabled bool
	detail  string
	start   func(string) error
}

// The launcher is injected so tray tests never create a desktop process.
func newGUIAction(daemon string, start func(string) error) guiAction {
	dir := filepath.Dir(daemon)
	if outer, ok := strings.CutSuffix(dir, "/Helpers/Obituary Menu.app/Contents/MacOS"); ok {
		dir = filepath.Join(outer, "MacOS")
	}
	path := filepath.Join(dir, "obit-gui")
	action := guiAction{path: path, enabled: true, start: start}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		action.enabled = false
		if err != nil {
			action.detail = err.Error()
		}
	}
	return action
}

func (a *guiAction) localize(language i18n.Language) {
	a.title, a.tooltip = language.Text(i18n.OpenGUI), a.path
	if !a.enabled {
		a.tooltip = fmt.Sprintf(language.Text(i18n.MissingGUI), a.path)
		if a.detail != "" {
			a.tooltip += " (" + a.detail + ")"
		}
	}
}

func (a guiAction) open() error {
	if !a.enabled {
		return fmt.Errorf("%s", a.tooltip)
	}
	return a.start(a.path)
}

func startGUI(path string) error {
	cmd := exec.Command(path)
	if bundle, ok := strings.CutSuffix(path, "/Contents/MacOS/obit-gui"); ok && filepath.Ext(bundle) == ".app" {
		cmd = exec.Command("/usr/bin/open", bundle)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
