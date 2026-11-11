package gui

import (
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/runkey"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// Open assembles production dependencies. Tests use New with isolated service
// paths and fake registries; opening the API itself never changes a registry.
func Open() (*API, error) {
	svc, err := app.Open()
	if err != nil {
		return nil, err
	}
	dir, err := userdirs.Config()
	if err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return New(Dependencies{Service: svc, Engine: app.Engine(svc), ConfigDir: dir,
		Executable: filepath.Join(filepath.Dir(exe), "obit.exe"), Daemon: filepath.Join(filepath.Dir(exe), "obitd.exe"),
		ContextMenu: contextmenu.UserRegistry(), RunKey: runkey.Current})
}
