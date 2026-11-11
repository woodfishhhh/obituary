//go:build !windows

package desktop

import "fmt"

const WindowClass = "ObituaryManagementWindow"

func SetTaskbar(bool) error { return fmt.Errorf("taskbar visibility is Windows-only") }
