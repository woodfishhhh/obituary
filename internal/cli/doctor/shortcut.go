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

func diagnoseShortcut(executable string, target func(string) (string, error)) DoctorCheck {
	return diagnoseShortcutAt(executable, shortcut.StartMenu, target)
}

func diagnoseDesktop(executable string, target func(string) (string, error)) DoctorCheck {
	return diagnoseShortcutAt(executable, shortcut.Desktop, target)
}

func diagnoseShortcutAt(executable string, place shortcut.Location, target func(string) (string, error)) DoctorCheck {
	c := DoctorCheck{Name: string(place), Status: "WARN"}
	if runtime.GOOS != "windows" {
		c.Status, c.Detail = "INFO", "desktop shortcuts are provided by the Windows edition"
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
			c.Detail = "Shortcut targets a different installation: " + got
			continue
		}
		if err := shortcut.CheckReceipt(executable, path); err != nil {
			c.Detail = "Shortcut ownership receipt requires attention: " + err.Error()
			return c
		}
		c.Status, c.Detail = "PASS", path+" -> "+got
		return c
	}
	if c.Detail == "" {
		c.Detail = "Shortcut is absent; obit init creates it; Settings can turn it off"
	}
	return c
}

func shortcutTarget(path string) (string, error) { return shortcut.ReadTarget(path) }

func diagnoseIcons() []DoctorCheck {
	if runtime.GOOS != "windows" {
		return []DoctorCheck{
			{Name: "tray-icon", Status: "INFO", Detail: "tray icon is provided by the Windows edition"},
			{Name: "taskbar", Status: "INFO", Detail: "taskbar window is provided by the Windows edition"},
		}
	}
	cfg, err := config.Load()
	checks := []DoctorCheck{{Name: "tray-icon", Status: "PASS", Detail: "visible"}, {Name: "taskbar", Status: "PASS", Detail: "visible"}}
	if !cfg.UI.Tray {
		checks[0].Detail = "hidden; resident cleanup and update checks continue"
	}
	if !cfg.UI.Taskbar {
		checks[1].Detail = "hidden; shortcuts can still open the management window"
	}
	if err != nil {
		for i := range checks {
			checks[i].Status, checks[i].Detail = "FAIL", err.Error()
		}
	}
	return checks
}
