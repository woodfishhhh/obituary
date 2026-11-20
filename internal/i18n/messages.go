package i18n

import (
	"embed"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Message files are embedded so installed binaries do not depend on the
// working directory or a separately deployed locale directory.
//
//go:embed messages/active.en.json messages/active.zh-CN.json
var messageFS embed.FS

var messageBundle = func() *goi18n.Bundle {
	bundle := goi18n.NewBundle(language.English)
	if _, err := bundle.LoadMessageFileFS(messageFS, "messages/active.en.json"); err != nil {
		panic(err)
	}
	if _, err := bundle.LoadMessageFileFS(messageFS, "messages/active.zh-CN.json"); err != nil {
		panic(err)
	}
	return bundle
}()

// defaultMessages are deliberately kept in Go source. goi18n extract can
// regenerate active.en.json from these literals, while the runtime still has
// a safe English fallback if a translation file is incomplete.
var defaultMessages = map[string]*goi18n.Message{
	"telemetry.help":           &goi18n.Message{ID: "telemetry.help", Other: "Usage statistics: a random installation ID, version, OS, architecture, interface language and installation channel help measure installation counts and version use. No files, paths or commands are sent; IP addresses and user agents are not stored. Disable with obit config telemetry off or OBIT_TELEMETRY=0."},
	"telemetry.syntax":         &goi18n.Message{ID: "telemetry.syntax", Other: "Use obit config telemetry on or obit config telemetry off."},
	"cli.list.short":           &goi18n.Message{ID: "cli.list.short", Other: "Show tracked paths and Docker containers, soonest expiry first"},
	"cli.list.json":            &goi18n.Message{ID: "cli.list.json", Other: "print JSON"},
	"cli.list.empty":           &goi18n.Message{ID: "cli.list.empty", Other: "nothing is tracked"},
	"cli.list.header":          &goi18n.Message{ID: "cli.list.header", Other: "ID\tEXPIRES\tLEFT\tPATH\tNOTE"},
	"cli.list.stopped":         &goi18n.Message{ID: "cli.list.stopped", Other: "stopped retrying after {{.Attempts}} attempts: {{.Error}}; obit extend #{{.ID}} 1m tries again, obit spare #{{.ID}} stops tracking"},
	"cli.list.retry":           &goi18n.Message{ID: "cli.list.retry", Other: "retry {{.Attempts}}: {{.Error}}"},
	"cli.list.quiet":           &goi18n.Message{ID: "cli.list.quiet", Other: "quiet for {{.QuietFor}} more (stable_window={{.StableWindow}}); next check {{.NextCheck}}"},
	"cli.bury.short":           &goi18n.Message{ID: "cli.bury.short", Other: "Move paths using the configured deletion mode; recovery depends on that mode"},
	"cli.bury.short.graveyard": &goi18n.Message{ID: "cli.bury.short.graveyard", Other: "Move paths into the graveyard now (restore with obit resurrect)"},
	"cli.bury.short.recycle":   &goi18n.Message{ID: "cli.bury.short.recycle", Other: "Move paths to the system Recycle Bin (restore from the Recycle Bin)"},
	"cli.bury.force":           &goi18n.Message{ID: "cli.bury.force", Other: "allow folders that contain a .git repository"},
	"cli.bury.note":            &goi18n.Message{ID: "cli.bury.note", Other: "free-text note kept with the burial"},
	"cli.bury.json":            &goi18n.Message{ID: "cli.bury.json", Other: "print JSON"},
	"cli.bury.recycled":        &goi18n.Message{ID: "cli.bury.recycled", Other: "{{.Action}}  {{.Ref}}  (sent to the system Recycle Bin; restore from the Recycle Bin)"},
	"cli.bury.resurrected":     &goi18n.Message{ID: "cli.bury.resurrected", Other: "#{{.ID}} resurrected  {{.Path}} (recovered)"},
	"cli.bury.buried":          &goi18n.Message{ID: "cli.bury.buried", Other: "#{{.ID}} buried   {{.Path}}  ({{.Size}}, restorable until {{.PurgeAt}})"},
	"cli.resurrect.short":      &goi18n.Message{ID: "cli.resurrect.short", Other: "Restore files without overwriting; start buried Docker containers by ID"},
	"cli.resurrect.json":       &goi18n.Message{ID: "cli.resurrect.json", Other: "print JSON"},
	"cli.resurrect.last":       &goi18n.Message{ID: "cli.resurrect.last", Other: "restore the latest deletion batch in the current session or Git project"},
	"cli.resurrect.result":     &goi18n.Message{ID: "cli.resurrect.result", Other: "#{{.ID}} resurrected  {{.Path}}"},
	"cli.warning":              &goi18n.Message{ID: "cli.warning", Other: "warning: {{.Text}}"},
	"cli.error":                &goi18n.Message{ID: "cli.error", Other: "error: {{.Label}}: {{.Text}}"},
	"cli.blocked":              &goi18n.Message{ID: "cli.blocked", Other: "blocked: {{.Label}}: {{.Text}}"},
	"cli.not_executed":         &goi18n.Message{ID: "cli.not_executed", Other: "{{.Ref}}: {{.Text}}"},
	"cli.bury.error":           &goi18n.Message{ID: "cli.bury.error", Other: "{{.Action}}  {{.Ref}}: {{.Text}}"},
	"cli.reported":             &goi18n.Message{ID: "cli.reported", Other: "one or more items failed"},

	"tray.app_name":             &goi18n.Message{ID: "tray.app_name", Other: "Obituary"},
	"tray.open_gui":             &goi18n.Message{ID: "tray.open_gui", Other: "Open Obituary"},
	"tray.update_available":     &goi18n.Message{ID: "tray.update_available", Other: "New version available: %s"},
	"tray.missing_gui":          &goi18n.Message{ID: "tray.missing_gui", Other: "No usable obit-gui.exe; place it beside obitd.exe: %s. Run obit doctor to check."},
	"tray.recent":               &goi18n.Message{ID: "tray.recent", Other: "Recently buried (click one to resurrect)"},
	"tray.recent_hint":          &goi18n.Message{ID: "tray.recent_hint", Other: "Select a record to resurrect it"},
	"tray.upcoming":             &goi18n.Message{ID: "tray.upcoming", Other: "Expiring soon (within 24 hours)"},
	"tray.pause":                &goi18n.Message{ID: "tray.pause", Other: "Pause"},
	"tray.resume":               &goi18n.Message{ID: "tray.resume", Other: "Resume"},
	"tray.pause_hint":           &goi18n.Message{ID: "tray.pause_hint", Other: "Pause automatic burial and purging; operations already committing will finish"},
	"tray.paused_tooltip":       &goi18n.Message{ID: "tray.paused_tooltip", Other: "Obituary (paused)"},
	"tray.open_data":            &goi18n.Message{ID: "tray.open_data", Other: "Open data directory"},
	"tray.quit":                 &goi18n.Message{ID: "tray.quit", Other: "Quit"},
	"tray.quit_hint":            &goi18n.Message{ID: "tray.quit_hint", Other: "The scheduled task remains available"},
	"tray.incomplete":           &goi18n.Message{ID: "tray.incomplete", Other: "Not completed: %s"},
	"tray.read_graves_failed":   &goi18n.Message{ID: "tray.read_graves_failed", Other: "Could not read graveyard: %s"},
	"tray.read_upcoming_failed": &goi18n.Message{ID: "tray.read_upcoming_failed", Other: "Could not read upcoming items: %s"},
	"tray.no_graves":            &goi18n.Message{ID: "tray.no_graves", Other: "No graveyard records"},
	"tray.no_upcoming":          &goi18n.Message{ID: "tray.no_upcoming", Other: "No items expire within 24 hours"},
	"tray.resurrected":          &goi18n.Message{ID: "tray.resurrected", Other: "Resurrected"},
	"tray.not_completed":        &goi18n.Message{ID: "tray.not_completed", Other: "Not completed"},
	"tray.summary":              &goi18n.Message{ID: "tray.summary", Other: "Next 24 hours: %d to bury · %d in the graveyard"},
	"tray.summary_paused":       &goi18n.Message{ID: "tray.summary_paused", Other: "Paused, nothing is buried · %d in the graveyard"},
	"tray.today_at":             &goi18n.Message{ID: "tray.today_at", Other: "Today %s"},
	"tray.tomorrow_at":          &goi18n.Message{ID: "tray.tomorrow_at", Other: "Tomorrow %s"},
	"tray.yesterday_at":         &goi18n.Message{ID: "tray.yesterday_at", Other: "Yesterday %s"},
}

// Localizer is the application-facing wrapper around go-i18n. Keeping the
// message lookup here makes CLI and native UI use the same embedded catalogs.
type Localizer struct {
	language  Language
	localizer *goi18n.Localizer
}

func NewLocalizer(language Language) *Localizer {
	if language != English && language != Chinese {
		panic("invalid localizer language: " + string(language))
	}
	return &Localizer{language: language, localizer: goi18n.NewLocalizer(messageBundle, string(language))}
}

func (l *Localizer) Language() Language { return l.language }

func (l *Localizer) Text(id string, data any) string {
	defaultMessage, ok := defaultMessages[id]
	if !ok {
		panic("missing default message: " + id)
	}
	return l.localizer.MustLocalize(&goi18n.LocalizeConfig{DefaultMessage: defaultMessage, TemplateData: data})
}
