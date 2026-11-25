//go:build darwin && desktop

package main

// #include <fcntl.h>
// #include <sys/param.h>
// static int windowPathForFD(int fd, char *buf) { return fcntl(fd, F_GETPATH, buf); }
import "C"

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/gui"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// EvalSymlinks alone preserves user-supplied case on a case-insensitive APFS volume.
func windowPath(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var buf [C.MAXPATHLEN]C.char
	if rc, err := C.windowPathForFD(C.int(f.Fd()), &buf[0]); rc < 0 {
		return "", err
	}
	return C.GoString(&buf[0]), nil
}

func platformOptions(api *gui.API, width, height int) *options.App {
	var mu sync.Mutex
	var window context.Context
	gui.SetFolderPicker(api, func() (string, error) {
		mu.Lock()
		ctx := window
		mu.Unlock()
		if ctx == nil {
			return "", fmt.Errorf("window is not ready")
		}
		return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{})
	})
	return &options.App{
		Mac:   &mac.Options{TitleBar: mac.TitleBarDefault()},
		Title: config.UILanguage().Text(i18n.AppName), Width: width, Height: height, MinWidth: 600, MinHeight: 400,
		AssetServer: &assetserver.Options{Assets: assets}, Bind: []interface{}{api},
		HideWindowOnClose: false,
		OnStartup: func(ctx context.Context) {
			mu.Lock()
			window = ctx
			mu.Unlock()
			go func() {
				home, err := userdirs.Data()
				if err == nil {
					err = app.Start(home)
				}
				if err != nil {
					log.Printf("start menu bar: %v", err)
				}
			}()
			app.ObserveUsage()
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				mu.Lock()
				defer mu.Unlock()
				if window == nil {
					return
				}
				runtime.WindowShow(window)
				go app.ObserveUsage()
				runtime.WindowUnminimise(window)
				runtime.WindowSetAlwaysOnTop(window, true)
				runtime.WindowSetAlwaysOnTop(window, false)
			},
		},
	}
}
