package tray

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

func TestUpdateMenuLocalizedAndOpensGUI(t *testing.T) {
	home := t.TempDir()
	items := &fakeItem{}
	r := &app.Resident{Service: &core.Service{Store: core.Store{Home: home}}}
	opened := make(chan string, 1)
	gui := guiAction{enabled: true, path: "fixture-gui", start: func(path string) error { opened <- path; return nil }}
	language := i18n.Chinese
	m := makeMenu(r, gui, menuUI{addItem: items.AddSubMenuItem, addSeparator: func() {}, tooltip: func(string) {}}, func() i18n.Language { return language })
	m.refreshUpdate()
	if items.children[1] != m.update || !m.update.(*fakeItem).hidden {
		t.Fatal("empty cache displayed update, or the update item is not under the window item")
	}
	cache := filepath.Join(home, "update.json")
	if err := os.WriteFile(cache, []byte(`{"latest":{"version":"9.0.0"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m.refreshUpdate()
	if m.update.(*fakeItem).title != "发现新版本 9.0.0" || m.update.(*fakeItem).hidden {
		t.Fatal(m.update)
	}
	language = i18n.English
	m.localize()
	m.refreshUpdate()
	if m.update.(*fakeItem).title != "New version available: 9.0.0" {
		t.Fatal(m.update)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); m.run(ctx, cancel, make(chan struct{}), log.New(io.Discard, "", 0)) }()
	select {
	case m.update.(*fakeItem).clicked <- struct{}{}:
	case <-time.After(time.Second):
		t.Fatal("menu did not accept click")
	}
	select {
	case path := <-opened:
		if path != "fixture-gui" {
			t.Fatal(path)
		}
	case <-time.After(time.Second):
		t.Fatal("menu did not open GUI")
	}
	cancel()
	<-done
	if err := os.WriteFile(cache, []byte(`{"latest":{"version":"0.0.0"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m.refreshUpdate()
	if !m.update.(*fakeItem).hidden {
		t.Fatal("obsolete update menu retained")
	}
}

func TestRowTitleNamesDayFileAndFolder(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 20, 0, 0, time.Local)
	for _, tc := range []struct {
		at   time.Time
		want string
	}{
		{time.Date(2026, 11, 14, 0, 39, 0, 0, time.Local), `今天 00:39   debug.log  ·  scratch`},
		{time.Date(2026, 11, 15, 23, 59, 0, 0, time.Local), `明天 23:59   debug.log  ·  scratch`},
		{time.Date(2026, 11, 13, 9, 5, 0, 0, time.Local), `昨天 09:05   debug.log  ·  scratch`},
		{time.Date(2026, 11, 16, 0, 0, 0, 0, time.Local), `11-16 00:00   debug.log  ·  scratch`},
	} {
		if got := rowTitle(i18n.Chinese, `C:\work\scratch\debug.log`, tc.at, now); got != tc.want {
			t.Fatalf("rowTitle(%s) = %q, want %q", tc.at, got, tc.want)
		}
	}
	if got := rowTitle(i18n.English, "debug.log", now, now); got != "Today 00:20   debug.log" {
		t.Fatalf("bare name: %q", got)
	}
}
