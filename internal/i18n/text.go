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
	TodayAt
	TomorrowAt
	YesterdayAt
	Resurrected
	NotCompleted
	Summary
	SummaryPaused
	StartDocker
	BuryNow
	MenuMarked
	MenuBuried
	MenuResultDetails
	MenuSelectionHandler
	ExplorerBurialLabel
	keyCount
)

var catalogs = map[Language]map[Key]string{Chinese: chinese, English: english}

var nativeMessageIDs = map[Key]string{
	AppName:            "tray.app_name",
	OpenGUI:            "tray.open_gui",
	UpdateAvailable:    "tray.update_available",
	MissingGUI:         "tray.missing_gui",
	Recent:             "tray.recent",
	RecentHint:         "tray.recent_hint",
	Upcoming:           "tray.upcoming",
	Pause:              "tray.pause",
	Resume:             "tray.resume",
	PauseHint:          "tray.pause_hint",
	PausedTooltip:      "tray.paused_tooltip",
	OpenData:           "tray.open_data",
	Quit:               "tray.quit",
	QuitHint:           "tray.quit_hint",
	Incomplete:         "tray.incomplete",
	ReadGravesFailed:   "tray.read_graves_failed",
	ReadUpcomingFailed: "tray.read_upcoming_failed",
	NoGraves:           "tray.no_graves",
	NoUpcoming:         "tray.no_upcoming",
	Resurrected:        "tray.resurrected",
	NotCompleted:       "tray.not_completed",
	Summary:            "tray.summary",
	SummaryPaused:      "tray.summary_paused",
	TodayAt:            "tray.today_at",
	TomorrowAt:         "tray.tomorrow_at",
	YesterdayAt:        "tray.yesterday_at",
}

// Text deliberately has no fallback: catalog omissions are programming errors,
// checked for both languages by TestCatalogsComplete.
func (l Language) Text(key Key) string {
	if id, ok := nativeMessageIDs[key]; ok {
		return NewLocalizer(l).Text(id, nil)
	}
	text, ok := catalogs[l][key]
	if !ok || text == "" {
		panic(fmt.Sprintf("missing native UI text: %s/%d", l, key))
	}
	return text
}
