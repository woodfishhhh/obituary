//go:build windows

package tray

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
)

type fakeItem struct {
	title, tooltip    string
	disabled, removed bool
	children          []*fakeItem
	clicked           chan struct{}
}

func (f *fakeItem) SetTitle(title string)     { f.title = title }
func (f *fakeItem) SetTooltip(tooltip string) { f.tooltip = tooltip }
func (f *fakeItem) Disable()                  { f.disabled = true }
func (f *fakeItem) Remove()                   { f.removed = true }
func (f *fakeItem) clicks() <-chan struct{}   { return f.clicked }
func (f *fakeItem) AddSubMenuItem(title, tooltip string) menuItem {
	child := &fakeItem{title: title, tooltip: tooltip, clicked: make(chan struct{})}
	f.children = append(f.children, child)
	return child
}

func TestMenuRefreshUsesCurrentLanguage(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", root)
	items := &fakeItem{}
	var tooltip string
	r := &app.Resident{Service: &core.Service{Store: core.Store{Home: filepath.Join(root, "home")}}}
	gui := newGUIAction(filepath.Join(root, "obitd.exe"), func(string) error { t.Fatal("desktop launcher called"); return nil })
	m := makeMenu(r, gui, menuUI{addItem: items.AddSubMenuItem, tooltip: func(text string) { tooltip = text }}, config.UILanguage)
	defer func() {
		if m.rowCancel != nil {
			m.rowCancel()
		}
	}()
	for _, tc := range []struct {
		language                        string
		titles                          []string
		empty, upcoming, paused, resume string
	}{
		{"zh-CN", []string{"打开讣告", "讣告 Obituary", "最近下葬的", "即将下葬的（24 小时内）", "暂停", "打开墓地列表（数据目录）", "退出"}, "暂无墓地记录", "24 小时内没有要下葬的", "讣告 Obituary（已暂停）", "继续"},
		{"en", []string{"Open Obituary", "Obituary", "Recently buried", "Expiring soon (within 24 hours)", "Pause", "Open graveyard list (data directory)", "Quit"}, "No graveyard records", "No items expire within 24 hours", "Obituary (paused)", "Resume"},
	} {
		if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte("[ui]\nlanguage='"+tc.language+"'\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		prior := append([]menuItem(nil), m.rows...)
		m.refresh(context.Background())
		var got []string
		for _, item := range items.children {
			got = append(got, item.title)
		}
		if !reflect.DeepEqual(got, tc.titles) || tooltip != tc.titles[1] {
			t.Fatalf("%s menu: %q tooltip=%s", tc.language, got, tooltip)
		}
		if m.rows[0].(*fakeItem).title != tc.empty || m.rows[1].(*fakeItem).title != tc.upcoming {
			t.Fatalf("%s empty rows: %+v", tc.language, m.rows)
		}
		for _, old := range prior {
			if !old.(*fakeItem).removed {
				t.Fatal("stale row retained")
			}
		}
		if !items.children[0].disabled || !strings.Contains(items.children[0].tooltip, "obit-gui.exe") || items.children[2].tooltip == "" || items.children[4].tooltip == "" || items.children[6].tooltip == "" {
			t.Fatal("missing action state or hints")
		}
		if err := app.SetPaused(r.Service.Store.Home, true); err != nil {
			t.Fatal(err)
		}
		m.refresh(context.Background())
		if tooltip != tc.paused || m.pause.(*fakeItem).title != tc.resume {
			t.Fatal("paused text did not refresh")
		}
		if err := app.SetPaused(r.Service.Store.Home, false); err != nil {
			t.Fatal(err)
		}
	}
}
