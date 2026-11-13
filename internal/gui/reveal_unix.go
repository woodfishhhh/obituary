//go:build linux || darwin

package gui

import "errors"

func revealPath(string) error { return errors.New("showing a file in its folder is Windows-only") }
