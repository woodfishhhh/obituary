//go:build windows

package tray

import (
	"fyne.io/systray"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

// The presentation boundary lets tests exercise refreshes without a real tray.
type menuItem interface {
	SetTitle(string)
	SetTooltip(string)
	Disable()
	Remove()
	AddSubMenuItem(string, string) menuItem
	clicks() <-chan struct{}
}

type nativeItem struct{ *systray.MenuItem }

func (n nativeItem) clicks() <-chan struct{} { return n.ClickedCh }
func (n nativeItem) AddSubMenuItem(title, tooltip string) menuItem {
	return nativeItem{n.MenuItem.AddSubMenuItem(title, tooltip)}
}

type menuUI struct {
	addItem func(string, string) menuItem
	tooltip func(string)
}

func makeMenu(r *app.Resident, gui guiAction, ui menuUI, language func() i18n.Language) *menu {
	m := &menu{resident: r, openGUI: gui, ui: ui, loadLanguage: language, actions: make(chan func() error, 16)}
	m.gui = ui.addItem("", "")
	if !gui.enabled {
		m.gui.Disable()
	}
	m.status = ui.addItem("", "")
	m.status.Disable()
	m.recent = ui.addItem("", "")
	m.upcoming = ui.addItem("", "")
	m.pause = ui.addItem("", "")
	m.open = ui.addItem("", "")
	m.quit = ui.addItem("", "")
	m.localize()
	return m
}

func (m *menu) localize() {
	language := m.loadLanguage()
	if language != m.language {
		m.status.SetTitle(language.Text(i18n.AppName))
	}
	m.language = language
	m.openGUI.localize(language)
	m.gui.SetTitle(m.openGUI.title)
	m.gui.SetTooltip(m.openGUI.tooltip)
	m.recent.SetTitle(language.Text(i18n.Recent))
	m.recent.SetTooltip(language.Text(i18n.RecentHint))
	m.upcoming.SetTitle(language.Text(i18n.Upcoming))
	m.pause.SetTooltip(language.Text(i18n.PauseHint))
	m.open.SetTitle(language.Text(i18n.OpenData))
	m.quit.SetTitle(language.Text(i18n.Quit))
	m.quit.SetTooltip(language.Text(i18n.QuitHint))
	m.showPaused(false)
}

func (m *menu) showPaused(paused bool) {
	title, tooltip := i18n.Pause, i18n.AppName
	if paused {
		title, tooltip = i18n.Resume, i18n.PausedTooltip
	}
	m.pause.SetTitle(m.language.Text(title))
	m.ui.tooltip(m.language.Text(tooltip))
}
