package init

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"

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
	exe, err := self.Path()
	if err != nil {
		return p, err
	}
	if !noMenu {
		p.menu, err = planDesktopMenu(out, p.registry, exe, uninstall)
		if err != nil {
			return p, err
		}
	}
	if !noShortcuts && runtime.GOOS == "windows" {
		p.shortcuts, err = planShortcuts(out, exe, uninstall)
	}
	return p, err
}

func planDesktopMenu(out io.Writer, reg contextmenu.Registry, exe string, uninstall bool) ([]contextmenu.Change, error) {
	fmt.Fprintln(out, cmdutil.Text("cli.init.context_menu"))
	if !uninstall && runtime.GOOS == "windows" {
		if err := checkMenuDaemon(exe); errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(out, cmdutil.Text("cli.init.unavailable_obitd_exe_is_missing"))
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
			fmt.Fprintln(out, cmdutil.Text("cli.init.unavailable_obit_gui_exe_is_missing"))
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, ch := range plan {
			fmt.Fprintf(out, "  %-9s %s %s\n", cmdutil.Column(cmdutil.Localizer(), cmdutil.Localizer().Label(ch.Action)), ch.Path, ch.Detail)
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
		if runtime.GOOS == "windows" {
			fmt.Fprintln(out, cmdutil.Text("cli.init.classic_context_menu_updated_windows_show_more"))
		}
	}
	return integrate.Apply(p.shortcuts)
}

func showDesktop(out io.Writer, self cmdutil.Executable, verbose bool) error {
	exe, err := self.Path()
	if err != nil {
		return err
	}
	menu, err := contextmenu.InspectDisplay(ContextMenuRegistry(), exe, cmdutil.Language(), cmdutil.Localizer())
	if err != nil {
		return err
	}
	detail := menu.Detail
	if !verbose { // the per-entry registration rows follow the first line
		detail, _, _ = strings.Cut(detail, "\n")
	}
	fmt.Fprintf(out, cmdutil.Text("cli.init.context_menu_showdesktop"), detail)
	if runtime.GOOS != "windows" {
		return nil
	}
	for _, place := range []shortcut.Location{shortcut.StartMenu, shortcut.Desktop} {
		path, err := shortcut.Inspect(ShortcutCodec, exe, place)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, cmdutil.Text("cli.init.installed"), place, path != "", path)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, cmdutil.Text("cli.init.tray_icon_taskbar"), cfg.UI.Tray, cfg.UI.Taskbar)
	startup, err := readStartupPlan(self, false)
	if err == nil && startup != nil {
		fmt.Fprintf(out, cmdutil.Text("cli.init.tray_startup"), startup.Before != "")
	}
	return err
}
