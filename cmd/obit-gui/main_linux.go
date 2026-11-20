//go:build linux

package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/gui"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

func windowOptions(api *gui.API, width, height int) *options.App {
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
		Linux: &linux.Options{ProgramName: "obituary"},
		Title: config.UILanguage().Text(i18n.AppName), Width: width, Height: height, MinWidth: 600, MinHeight: 400,
		AssetServer: &assetserver.Options{Assets: assets}, Bind: []interface{}{api},
		HideWindowOnClose: false,
		OnStartup: func(ctx context.Context) {
			mu.Lock()
			window = ctx
			mu.Unlock()
			app.ObserveUsage()
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "c36c42b1-df66-43e4-b909-obituary-gui",
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
