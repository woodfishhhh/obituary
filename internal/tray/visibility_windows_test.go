package tray

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"fyne.io/systray"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/config"
)

func TestHiddenTrayKeepsResidentRunning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_HOME", home)
	cfg := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", cfg)
	if err := os.WriteFile(filepath.Join(cfg, "config.toml"), []byte("[ui]\ntray=false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	logger := log.New(io.Discard, "", 0)
	r, err := app.NewResident(svc, logger)
	if err != nil || r == nil {
		t.Fatal(r, err)
	}
	defer r.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	ticks := make(chan time.Time)
	applied := make(chan bool, 1)
	visibleDone := make(chan struct{})
	go func() {
		defer close(visibleDone)
		watchVisibility(ctx, ticks, func() (bool, error) { cfg, err := config.Load(); return cfg.UI.Tray, err }, func(enabled bool) error { applied <- enabled; return nil }, logger)
	}()
	select {
	case enabled := <-applied:
		if enabled {
			t.Fatal("tray was enabled")
		}
	case <-time.After(time.Second):
		t.Fatal("no visibility update")
	}
	if !app.Running(home) {
		t.Fatal("hiding tray stopped resident")
	}
	if err := os.WriteFile(filepath.Join(cfg, "config.toml"), []byte("[ui]\ntray=true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ticks <- time.Now()
	select {
	case enabled := <-applied:
		if !enabled {
			t.Fatal("configuration was not reloaded")
		}
	case <-time.After(time.Second):
		t.Fatal("reload did not arrive")
	}
	if !app.Running(home) {
		t.Fatal("config reload stopped resident")
	}
	select {
	case err := <-done:
		t.Fatal("resident exited", err)
	default:
	}
	cancel()
	<-visibleDone
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestNotifyIconStateLayout(t *testing.T) {
	var data iconState
	if unsafe.Sizeof(uintptr(0)) == 8 && unsafe.Sizeof(data) != 976 {
		t.Fatalf("NOTIFYICONDATAW size: %d", unsafe.Sizeof(data))
	}
}

func TestNativeTrayVisibility(t *testing.T) {
	if os.Getenv("OBIT_TEST_DESKTOP") != "1" {
		t.Skip("set OBIT_TEST_DESKTOP=1 for isolated native tray tests")
	}
	result := make(chan error, 1)
	systray.Run(func() {
		systray.SetIcon(icon)
		var err error
		for _, visible := range []bool{false, true, false} {
			if err = setVisible(visible); err != nil {
				break
			}
		}
		result <- err
		systray.Quit()
	}, func() {})
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}
