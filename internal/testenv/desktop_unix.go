//go:build !windows

package testenv

func realDesktopSnapshot() (desktopFootprint, error) { return desktopFootprint{}, nil }
func isolateRegistry() (func() error, error)         { return func() error { return nil }, nil }
