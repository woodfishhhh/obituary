//go:build windows || ((linux || darwin) && desktop)

// Windows native build, without Wails CLI, Node or a C compiler:
// CGO_ENABLED=0 go build -tags desktop,production -ldflags "-H windowsgui" -o bin/obit-gui.exe ./cmd/obit-gui
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/woodfishhhh/obituary/internal/gui"
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
	opts, err := windowOptions(api, *width, *height)
	if err != nil {
		return err
	}
	return wails.Run(opts)
}
