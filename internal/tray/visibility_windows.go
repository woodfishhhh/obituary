package tray

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unsafe"

	"github.com/fsnotify/fsnotify"
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
	data := iconState{Flags: 8, Mask: 1} // NIF_STATE, NIS_HIDDEN
	if !enabled {
		data.State = 1
	}
	return modifyIcon(data)
}

// showBalloon reports a menu action's result from the icon. Windows 10 and 11
// show it as a toast, which needs no registered AppUserModelID this way: the
// header carries obitd.exe's FileDescription and icon (scripts/winres), and
// NIIF_USER with no balloon icon puts the tray's own icon in the body.
func showBalloon(title, text string) error {
	data := iconState{Flags: 0x10, InfoFlags: 0x34} // NIF_INFO; NIIF_USER | NIIF_NOSOUND | NIIF_LARGE_ICON
	copy(data.InfoTitle[:len(data.InfoTitle)-1], utf16.Encode([]rune(title)))
	copy(data.Info[:len(data.Info)-1], utf16.Encode([]rune(text)))
	return modifyIcon(data)
}

func modifyIcon(data iconState) error {
	hwnd, err := desktop.OwnWindow("SystrayClass")
	if err != nil {
		return err
	}
	data.Window, data.ID = hwnd, 100
	data.Size = uint32(unsafe.Sizeof(data))
	result, _, err := windows.NewLazySystemDLL("shell32.dll").NewProc("Shell_NotifyIconW").Call(1, uintptr(unsafe.Pointer(&data))) // NIM_MODIFY
	if result == 0 {
		return err
	}
	return nil
}

// watchVisibility applies [ui] tray now and whenever the file at path
// changes. The OS reports the change, so an idle tray reads nothing.
func watchVisibility(ctx context.Context, path string, load func() (bool, error), apply func(bool) error, logger *log.Logger) {
	refresh := func() {
		enabled, err := load()
		if err == nil {
			err = apply(enabled)
		}
		if err != nil {
			logger.Printf("tray visibility: %v", err)
		}
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		refresh()
		logger.Printf("tray visibility: %v", err)
		return
	}
	defer w.Close()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		logger.Printf("tray visibility: %v", err)
	} else if err := w.Add(filepath.Dir(path)); err != nil {
		logger.Printf("tray visibility: %v", err)
	}
	refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-w.Events:
			if strings.EqualFold(filepath.Base(event.Name), filepath.Base(path)) {
				refresh()
			}
		case err := <-w.Errors:
			logger.Printf("tray visibility: %v", err)
			refresh() // events may have been lost
		}
	}
}

func followVisibility(ctx context.Context, logger *log.Logger) {
	path, err := config.Path()
	if err != nil {
		logger.Printf("tray visibility: %v", err)
		return
	}
	watchVisibility(ctx, path, func() (bool, error) { cfg, err := config.Load(); return cfg.UI.Tray, err }, setVisible, logger)
}
