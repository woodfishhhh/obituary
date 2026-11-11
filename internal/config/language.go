package config

import "github.com/woodfishhhh/obituary/internal/i18n"

// UILanguage keeps native UI available when configuration cannot be read.
// Configuration editing still reports the original error instead of overwriting it.
func UILanguage() i18n.Language {
	cfg, err := Load()
	if err != nil {
		cfg = Default()
	}
	return i18n.Resolve(cfg.UI.Language, i18n.SystemLanguage)
}
