package gui

import (
	"errors"
	"os"

	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// SetFolderPicker connects the window's native folder dialog. It is a
// function, not a method, so Wails does not bind it for the page.
func SetFolderPicker(a *API, pick func() (string, error)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pickFolder = pick
}

// PickFolder asks the user for a folder; an empty path means they cancelled.
func (a *API) PickFolder() (string, error) {
	a.mu.Lock()
	pick := a.pickFolder
	a.mu.Unlock()
	if pick == nil {
		return "", errors.New("no folder dialog in this window")
	}
	return pick()
}

// downloadsFolder is offered for folder policies when it exists.
func downloadsFolder() string {
	path, err := userdirs.Downloads()
	if err != nil {
		return ""
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return ""
	}
	return path
}
