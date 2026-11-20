//go:build !windows

package gui

func platformCapabilities() Capabilities { return Capabilities{} }
