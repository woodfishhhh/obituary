package cmdutil

import (
	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

// Language resolves the CLI language without opening the ledger. Hooks and
// MCP do not call this helper because their output is a stable English API.
func Language() i18n.Language {
	cfg, err := config.Load()
	if err != nil {
		return i18n.Resolve(i18n.Auto, i18n.SystemLanguage)
	}
	return i18n.Resolve(cfg.UI.Language, i18n.SystemLanguage)
}

func Localizer() *i18n.Localizer { return i18n.NewLocalizer(Language()) }

// OutputLocalizer keeps diagnostics accompanying machine output byte-stable.
func OutputLocalizer(asJSON bool) *i18n.Localizer {
	if asJSON {
		return i18n.NewProtocolLocalizer()
	}
	return Localizer()
}

// Text looks up a CLI label or printf format at display time, not during command registration.
func Text(id string) string { return Localizer().Text(id, nil) }

// SetLocalizedHelp updates fields at render time so command listings and
// direct help use the same setting without changing command names or flags.
func SetLocalizedHelp(cmd *cobra.Command, apply func(*i18n.Localizer, *cobra.Command)) {
	help := cmd.HelpFunc()
	cmd.SetHelpFunc(func(command *cobra.Command, args []string) {
		apply(Localizer(), command)
		help(command, args)
	})
}
