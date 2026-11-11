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
	m := makeMenu(r, gui, menuUI{addItem: items.AddSubMenuItem, tooltip: func(string) {}}, func() i18n.Language { return language })
	m.refreshUpdate()
	if m.update != nil {
		t.Fatal("empty cache displayed update")
	}
	cache := filepath.Join(home, "update.json")
	if err := os.WriteFile(cache, []byte(`{"latest":{"version":"9.0.0"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m.refreshUpdate()
	if m.update.(*fakeItem).title != "发现新版本 9.0.0" {
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
	previous := m.update.(*fakeItem)
	if err := os.WriteFile(cache, []byte(`{"latest":{"version":"0.0.0"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m.refreshUpdate()
	if m.update != nil || !previous.removed {
		t.Fatal("obsolete update menu retained")
	}
}
