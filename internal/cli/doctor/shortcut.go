package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/shortcut"
)

func (lang diagnostics) diagnoseShortcut(executable string, target func(string) (string, error)) DoctorCheck {
	return lang.diagnoseShortcutAt(executable, shortcut.StartMenu, target)
}

func (lang diagnostics) diagnoseDesktop(executable string, target func(string) (string, error)) DoctorCheck {
	return lang.diagnoseShortcutAt(executable, shortcut.Desktop, target)
}

func (lang diagnostics) diagnoseShortcutAt(executable string, place shortcut.Location, target func(string) (string, error)) DoctorCheck {
	c := DoctorCheck{Name: string(place), Status: "WARN"}
	if runtime.GOOS != "windows" {
		c.Status, c.Detail = "INFO", lang.Text("cli.doctor.desktop_shortcuts_are_provided_by_the_windows", nil)
		return c
	}
	paths, err := shortcut.Paths(place)
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
		return c
	}
	for _, path := range paths {
		got, err := target(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			c.Status, c.Detail = "FAIL", err.Error()
			return c
		}
		if !strings.EqualFold(filepath.Clean(got), filepath.Clean(shortcut.Target(executable))) {
			c.Detail = lang.Text("cli.doctor.shortcut_targets_a_different_installation", nil) + got
			continue
		}
		if err := shortcut.CheckReceipt(executable, path); err != nil {
			c.Detail = lang.Text("cli.doctor.shortcut_ownership_receipt_requires_attention", nil) + err.Error()
			return c
		}
		c.Status, c.Detail = "PASS", path+" -> "+got
		return c
	}
	if c.Detail == "" {
		c.Detail = lang.Text("cli.doctor.shortcut_is_absent_obit_init_creates_it", nil)
	}
	return c
}

func shortcutTarget(path string) (string, error) { return shortcut.ReadTarget(path) }

func (lang diagnostics) diagnoseIcons() []DoctorCheck {
	if runtime.GOOS != "windows" {
		return []DoctorCheck{
			{Name: "tray-icon", Status: "INFO", Detail: lang.Text("cli.doctor.tray_icon_is_provided_by_the_windows", nil)},
			{Name: "taskbar", Status: "INFO", Detail: lang.Text("cli.doctor.taskbar_window_is_provided_by_the_windows", nil)},
		}
	}
	cfg, err := config.Load()
	checks := []DoctorCheck{{Name: "tray-icon", Status: "PASS", Detail: lang.Text("cli.doctor.visible", nil)}, {Name: "taskbar", Status: "PASS", Detail: lang.Text("cli.doctor.visible", nil)}}
	if !cfg.UI.Tray {
		checks[0].Detail = lang.Text("cli.doctor.hidden_resident_cleanup_and_update_checks_continue", nil)
	}
	if !cfg.UI.Taskbar {
		checks[1].Detail = lang.Text("cli.doctor.hidden_shortcuts_can_still_open_the_management", nil)
	}
	if err != nil {
		for i := range checks {
			checks[i].Status, checks[i].Detail = "FAIL", err.Error()
		}
	}
	return checks
}
