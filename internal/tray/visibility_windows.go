package tray

import (
	"context"
	"log"
	"time"
	"unsafe"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/desktop"
	"golang.org/x/sys/windows"
)

// fyne.io/systray has no visibility API. NIF_STATE changes only visibility of
// its current-process icon (the pinned library's window class and ID).
// The native message loop and resident retain their lifetime independently.
type iconState struct {
	Size                uint32
	Window              windows.Handle
	ID, Flags, Callback uint32
	Icon                windows.Handle
	Tip                 [128]uint16
	State, Mask         uint32
	Info                [256]uint16
	TimeoutOrVersion    uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	GUID                windows.GUID
	BalloonIcon         windows.Handle
}

func setVisible(enabled bool) error {
	hwnd, err := desktop.OwnWindow("SystrayClass")
	if err != nil {
		return err
	}
	data := iconState{Window: hwnd, ID: 100, Flags: 8, Mask: 1}
	data.Size = uint32(unsafe.Sizeof(data))
	if !enabled {
		data.State = 1
	}
	result, _, err := windows.NewLazySystemDLL("shell32.dll").NewProc("Shell_NotifyIconW").Call(1, uintptr(unsafe.Pointer(&data)))
	if result == 0 {
		return err
	}
	return nil
}

func watchVisibility(ctx context.Context, ticks <-chan time.Time, load func() (bool, error), apply func(bool) error, logger *log.Logger) {
	refresh := func() {
		enabled, err := load()
		if err == nil {
			err = apply(enabled)
		}
		if err != nil {
			logger.Printf("tray visibility: %v", err)
		}
	}
	refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			refresh()
		}
	}
}

func followVisibility(ctx context.Context, logger *log.Logger) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	watchVisibility(ctx, ticker.C, func() (bool, error) { cfg, err := config.Load(); return cfg.UI.Tray, err }, setVisible, logger)
}
