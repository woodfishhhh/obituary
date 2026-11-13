package gui

import (
	"errors"
	"os"

	"github.com/woodfishhhh/obituary/internal/pathutil"
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

// ShowInFolder opens the folder holding path, with it selected, so a
// resurrected item can be found. It shows the item; it never opens or runs it.
func (a *API) ShowInFolder(path string) error {
	abs, err := pathutil.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(abs); err != nil {
		return err
	}
	return revealPath(abs)
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
