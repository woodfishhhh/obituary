//go:build windows || ((linux || darwin) && desktop)

package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/woodfishhhh/obituary/internal/gui"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func windowOptions(api *gui.API, width, height int) (*options.App, error) {
	opts := platformOptions(api, width, height)
	const legacy = "c36c42b1-df66-43e4-b909-obituary-gui"
	opts.SingleInstanceLock.UniqueId = legacy
	home, err := userdirs.Data()
	if err != nil {
		return nil, err
	}
	def, err := userdirs.DefaultData()
	if err != nil {
		return nil, err
	}
	if home == def {
		return opts, nil
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return nil, err
	}
	real, err := windowPath(home)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(real)
	if err != nil {
		return nil, err
	}
	normal, err := os.Stat(def)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if normal != nil && os.SameFile(info, normal) {
		return opts, nil
	}
	// The open API uses a path, not an inode: replacing a directory must not change its routing key.
	key := sha256.Sum256([]byte(real))
	opts.SingleInstanceLock.UniqueId = fmt.Sprintf("%s-%x", legacy, key[:16])
	opts.Title += " (" + real + ")"
	return opts, nil
}
