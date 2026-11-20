package gui

// Capabilities describes integrations exposed by the current desktop build.
// The frontend uses it to avoid rendering controls that cannot work on Unix.
type Capabilities struct {
	Recycle         bool `json:"recycle"`
	TrayVisibility  bool `json:"trayVisibility"`
	ContextMenu     bool `json:"contextMenu"`
	Taskbar         bool `json:"taskbar"`
	StartMenu       bool `json:"startMenu"`
	DesktopShortcut bool `json:"desktopShortcut"`
}
