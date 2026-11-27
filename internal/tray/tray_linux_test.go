//go:build linux && desktop

package tray

import (
	"context"
	"io"
	"log"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

type fakeLinuxItem struct {
	title, tooltip            string
	disabled, removed, hidden bool
	children                  []*fakeLinuxItem
	clicked                   chan struct{}
}

func (f *fakeLinuxItem) SetTitle(title string)     { f.title = title }
func (f *fakeLinuxItem) SetTooltip(tooltip string) { f.tooltip = tooltip }
func (f *fakeLinuxItem) Disable()                  { f.disabled = true }
func (f *fakeLinuxItem) Remove()                   { f.removed = true }
func (f *fakeLinuxItem) Hide()                     { f.hidden = true }
func (f *fakeLinuxItem) Show()                     { f.hidden = false }
func (f *fakeLinuxItem) clicks() <-chan struct{}   { return f.clicked }
func (f *fakeLinuxItem) AddSubMenuItem(title, tooltip string) menuItem {
	child := &fakeLinuxItem{title: title, tooltip: tooltip, clicked: make(chan struct{})}
	f.children = append(f.children, child)
	return child
}

func TestActivateOpensGUI(t *testing.T) {
	home := t.TempDir()
	items := &fakeLinuxItem{}
	r := &app.Resident{Service: &lifecycle.Service{Store: ledger.Store{Home: home}}}
	opened := make(chan string, 1)
	gui := guiAction{enabled: true, path: "fixture-gui", start: func(path string) error { opened <- path; return nil }}
	m := makeMenu(r, gui, menuUI{addItem: items.AddSubMenuItem, addSeparator: func() {}, tooltip: func(string) {}, notify: func(string, string) error { return nil }}, func() i18n.Language { return i18n.English })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); m.run(ctx, make(chan struct{}), log.New(io.Discard, "", 0)) }()
	defer func() { cancel(); <-done }()
	m.triggerOpenGUI()
	select {
	case path := <-opened:
		if path != "fixture-gui" {
			t.Fatal(path)
		}
	case <-time.After(time.Second):
		t.Fatal("tray activation did not open GUI")
	}
}

func TestActivateMissingGUINotifies(t *testing.T) {
	root := t.TempDir()
	items, told := &fakeLinuxItem{}, make(chan string, 1)
	r := &app.Resident{Service: &lifecycle.Service{Store: ledger.Store{Home: root}}}
	gui := newGUIAction(filepath.Join(root, "obitd"), func(string) error {
		t.Error("missing executable launched")
		return nil
	})
	m := makeMenu(r, gui, menuUI{addItem: items.AddSubMenuItem, addSeparator: func() {}, tooltip: func(string) {},
		notify: func(title, text string) error { told <- title + " | " + text; return nil }}, func() i18n.Language { return i18n.English })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); m.run(ctx, make(chan struct{}), log.New(io.Discard, "", 0)) }()
	defer func() { cancel(); <-done }()
	m.triggerOpenGUI()
	select {
	case got := <-told:
		if !strings.Contains(got, gui.path) || !strings.Contains(got, "obit doctor") {
			t.Fatal(got)
		}
	case <-time.After(time.Second):
		t.Fatal("missing GUI was not reported on activation")
	}
}
