//go:build windows

package gui

func platformCapabilities() Capabilities {
	return Capabilities{ContextMenu: true, Taskbar: true, StartMenu: true, DesktopShortcut: true}
}
