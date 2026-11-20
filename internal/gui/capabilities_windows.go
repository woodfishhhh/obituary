//go:build windows

package gui

func platformCapabilities() Capabilities {
	return Capabilities{Recycle: true, TrayVisibility: true, ContextMenu: true, Taskbar: true, StartMenu: true, DesktopShortcut: true}
}
