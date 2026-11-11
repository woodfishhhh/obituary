package init

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/shortcut"
)

var ShortcutCodec shortcut.Codec = shortcut.Native{}

type desktopPlan struct {
	registry  contextmenu.Registry
	menu      []contextmenu.Change
	shortcuts []integrate.Change
}

func planDesktop(out io.Writer, self cmdutil.Executable, uninstall, noMenu, noShortcuts bool) (desktopPlan, error) {
	p := desktopPlan{registry: ContextMenuRegistry()}
	if runtime.GOOS != "windows" {
		return p, nil
	}
	exe, err := self.Path()
	if err != nil {
		return p, err
	}
	if uninstall || !noMenu {
		p.menu, err = planDesktopMenu(out, p.registry, exe, uninstall)
		if err != nil {
			return p, err
		}
	}
	if uninstall || !noShortcuts {
		p.shortcuts, err = planShortcuts(out, exe, uninstall)
	}
	return p, err
}

func planDesktopMenu(out io.Writer, reg contextmenu.Registry, exe string, uninstall bool) ([]contextmenu.Change, error) {
	fmt.Fprintln(out, "context-menu:")
	if !uninstall {
		if err := checkMenuDaemon(exe); errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(out, "  unavailable: obitd.exe is missing")
			return nil, nil
		} else if err != nil {
			return nil, err
		}
	}
	changes, err := contextmenu.Plan(reg, exe, uninstall)
	if err != nil {
		return nil, err
	}
	for _, change := range changes {
		printContextMenuChange(out, change)
	}
	return changes, nil
}

func planShortcuts(out io.Writer, exe string, uninstall bool) ([]integrate.Change, error) {
	changes := []integrate.Change{}
	for _, place := range []shortcut.Location{shortcut.StartMenu, shortcut.Desktop} {
		fmt.Fprintf(out, "%s:\n", place)
		plan, err := shortcut.Plan(ShortcutCodec, exe, place, !uninstall)
		if !uninstall && errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(out, "  unavailable: obit-gui.exe is missing")
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, ch := range plan {
			fmt.Fprintf(out, "  %-9s %s %s\n", ch.Action, ch.Path, ch.Detail)
		}
		// Other installers' links remain visible in the plan and untouched.
		plan = slices.DeleteFunc(plan, func(c integrate.Change) bool { return c.Action == "preserve" })
		changes = append(changes, plan...)
	}
	return changes, nil
}

func (p desktopPlan) pending() bool { return len(p.menu) > 0 || integrate.Pending(p.shortcuts) }
func (p desktopPlan) apply(out io.Writer) error {
	if len(p.menu) > 0 {
		if err := contextmenu.Apply(p.registry, p.menu); err != nil {
			return err
		}
		fmt.Fprintln(out, "classic context menu updated; Windows 11: 显示更多选项 (Show more options)")
	}
	return integrate.Apply(p.shortcuts)
}

func showDesktop(out io.Writer, self cmdutil.Executable) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	exe, err := self.Path()
	if err != nil {
		return err
	}
	menu, err := contextmenu.Inspect(ContextMenuRegistry(), exe)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "context-menu: %s\n", menu.Detail)
	for _, place := range []shortcut.Location{shortcut.StartMenu, shortcut.Desktop} {
		path, err := shortcut.Inspect(ShortcutCodec, exe, place)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s: installed=%t %s\n", place, path != "", path)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "tray-icon: %t; taskbar: %t\n", cfg.UI.Tray, cfg.UI.Taskbar)
	startup, err := readStartupPlan(self, false)
	if err == nil && startup != nil {
		fmt.Fprintf(out, "tray-startup: %t\n", startup.Before != "")
	}
	return err
}
