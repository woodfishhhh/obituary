//go:build !windows

package gui

import "runtime"

func platformCapabilities() Capabilities { return Capabilities{Recycle: runtime.GOOS == "linux"} }
