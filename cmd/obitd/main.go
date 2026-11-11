// Command obitd is the Obituary background process: it recycles items when
// their lifespan runs out. Build it with -ldflags=-H=windowsgui so it runs
// without a console window; `obit daemon` starts and stops it.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/tray"
)

const logLimit = 1 << 20

// Subprocess tests override this at link time, keeping all test runs headless.
var defaultTray = "true"

func main() {
	home := flag.String("home", "", "data directory (default: OBIT_HOME or %LOCALAPPDATA%\\obituary)")
	configDir := flag.String("config-dir", "", "configuration directory (default: OBIT_CONFIG_DIR or %APPDATA%\\obituary)")
	once := flag.Bool("once", false, "do one round of work (reap and purge) and exit")
	menu := flag.Bool("context-menu", false, "run an explicit Explorer action and show its result")
	withTray := flag.Bool("tray", defaultTray == "true", "show the resident tray icon")
	flag.Parse()
	for value, env := range map[*string]string{home: "OBIT_HOME", configDir: "OBIT_CONFIG_DIR"} {
		if *value == "" {
			continue
		}
		abs, err := pathutil.Abs(*value)
		if err != nil {
			os.Exit(2)
		}
		os.Setenv(env, abs)
	}
	if *menu {
		if err := contextmenu.Run(flag.Args(), contextmenu.Execute, contextmenu.Notify); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	svc, err := app.Open()
	if err != nil {
		os.Exit(1)
	}
	logger := log.New(&rotatingLog{path: filepath.Join(svc.Store.Home, "obitd.log")}, "", log.LstdFlags)
	if *once {
		outcomes, purged, err := app.Tick(svc)
		for _, o := range outcomes {
			logger.Printf("once: #%d %s %s %s", o.Item.ID, o.Action, o.Path, o.Detail)
		}
		for _, r := range purged {
			logger.Printf("once: %s %s %s", r.Ref, r.Action, r.Error)
		}
		if err != nil {
			logger.Printf("once: %v", err)
			os.Exit(1)
		}
		return
	}
	serve := app.Serve
	debug.SetMemoryLimit(12 << 20)
	if *withTray {
		serve = tray.Run
	}
	if err := serve(svc, logger); err != nil {
		logger.Printf("obitd: %v", err)
		if runtime.GOOS != "windows" {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

// rotatingLog appends to path and moves it to path.1 once it exceeds logLimit.
// Write errors are swallowed: the daemon must keep running without its log.
type rotatingLog struct {
	mu   sync.Mutex
	path string
}

func (r *rotatingLog) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		return len(p), nil
	}
	if info, err := os.Stat(r.path); err == nil && info.Size() > logLimit {
		_ = os.Rename(r.path, r.path+".1")
	}
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return len(p), nil
	}
	defer f.Close()
	_, _ = f.Write(p)
	return len(p), nil
}
