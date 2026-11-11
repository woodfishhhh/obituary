package i18n

import "fmt"

type Key int

const (
	AppName Key = iota
	OpenGUI
	UpdateAvailable
	MissingGUI
	Recent
	RecentHint
	Upcoming
	Pause
	Resume
	PauseHint
	PausedTooltip
	OpenData
	Quit
	QuitHint
	Incomplete
	ReadGravesFailed
	ReadUpcomingFailed
	NoGraves
	Restored
	NoUpcoming
	Mark
	ResurrectLatest
	FolderPolicy
	OneHour
	OneDay
	SevenDays
	ThirtyDays
	OperationComplete
	OperationIncomplete
	OperationDone
	MenuFiles
	MenuFolders
	MenuBackground
	MenuSubmenu
	MenuNotifyName
	MenuProgramMoved
	TrayIcon
	TaskbarIcon
	StartMenuShortcut
	DesktopShortcut
	Enabled
	Disabled
	IconLive
	IconNextWindow
	ShortcutPreserved
	keyCount
)

var catalogs = map[Language]map[Key]string{Chinese: chinese, English: english}

// Text deliberately has no fallback: catalog omissions are programming errors,
// checked for both languages by TestCatalogsComplete.
func (l Language) Text(key Key) string {
	text, ok := catalogs[l][key]
	if !ok || text == "" {
		panic(fmt.Sprintf("missing native UI text: %s/%d", l, key))
	}
	return text
}
