//go:build windows

// Windows native build, without Wails CLI, Node or a C compiler:
// CGO_ENABLED=0 go build -tags desktop,production -ldflags "-H windowsgui" -o bin/obit-gui.exe ./cmd/obit-gui
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/gui"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("obit-gui", flag.ContinueOnError)
	smoke := flags.Bool("smoke", false, "print backend Summary as JSON without opening a window")
	width := flags.Int("width", 1040, "window width")
	height := flags.Int("height", 700, "window height")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *width < 600 || *height < 400 {
		return fmt.Errorf("window must be at least 600x400")
	}
	api, err := gui.Open()
	if err != nil {
		return err
	}
	if *smoke {
		summary, err := api.Summary()
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(summary)
	}
	return wails.Run(windowOptions(api, *width, *height))
}

// Reference: Wails v2 docs/reference/options (SingleInstanceLock) and the manual
// builds guide. SingleInstanceLock forwards launches to the existing process.
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
		Title: config.UILanguage().Text(i18n.AppName), Width: width, Height: height, MinWidth: 600, MinHeight: 400,
		AssetServer: &assetserver.Options{Assets: assets}, Bind: []interface{}{api},
		HideWindowOnClose: false,
		OnStartup:         func(ctx context.Context) { mu.Lock(); window = ctx; mu.Unlock() },
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "c36c42b1-df66-43e4-b909-obituary-gui",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				mu.Lock()
				defer mu.Unlock()
				if window == nil {
					return
				}
				runtime.WindowShow(window)
				runtime.WindowUnminimise(window)
				runtime.WindowSetAlwaysOnTop(window, true)
				runtime.WindowSetAlwaysOnTop(window, false)
			},
		},
	}
}
