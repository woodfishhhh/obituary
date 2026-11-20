// Package i18n contains shared human-facing text for the CLI and native UI;
// agent protocol output remains English and byte-stable.
package i18n

import (
	"os"
	"strings"

	"golang.org/x/text/language"
)

type Language string

const (
	Auto             = "auto"
	Chinese Language = "zh-CN"
	English Language = "en"
)

func Valid(language string) bool {
	return language == Auto || language == string(Chinese) || language == string(English)
}

var supportedTags = []language.Tag{language.English, language.MustParse("zh-CN")}
var supportedMatcher = language.NewMatcher(supportedTags)

// Resolve applies the shared language precedence used by CLI and native UI.
// The system callback is only consulted after explicit and locale settings.
func Resolve(language string, systemLanguage func() string) Language {
	return ResolveWith(language, os.Getenv, systemLanguage)
}

// ResolveWith is the injectable form used by tests and platform adapters.
func ResolveWith(setting string, getenv func(string) string, systemLanguage func() string) Language {
	if raw := getenv("OBIT_LANG"); raw != "" {
		if resolved, ok := match(raw); ok {
			return resolved
		}
		return English
	}
	if setting != "" && setting != Auto {
		if resolved, ok := match(setting); ok {
			return resolved
		}
		return English
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		raw := getenv(name)
		if raw == "" || isPOSIXLocale(raw) {
			continue
		}
		if resolved, ok := match(raw); ok {
			return resolved
		}
	}
	if systemLanguage != nil {
		if resolved, ok := match(systemLanguage()); ok {
			return resolved
		}
	}
	return English
}

func match(raw string) (Language, bool) {
	raw = strings.TrimSpace(strings.Trim(raw, "\"'"))
	if raw == "" || isPOSIXLocale(raw) {
		return "", false
	}
	if cut := strings.IndexAny(raw, ".@"); cut >= 0 {
		raw = raw[:cut]
	}
	raw = strings.ReplaceAll(raw, "_", "-")
	tag, err := language.Parse(raw)
	if err != nil {
		return "", false
	}
	base, _ := tag.Base()
	if strings.EqualFold(base.String(), "zh") {
		script, _ := tag.Script()
		region, _ := tag.Region()
		if script.String() == "Hant" || slicesContains([]string{"TW", "HK", "MO"}, region.String()) {
			return "", false
		}
	}
	_, index, confidence := supportedMatcher.Match(tag)
	if confidence == language.No {
		return "", false
	}
	return []Language{English, Chinese}[index], true
}

func isPOSIXLocale(raw string) bool {
	base := strings.ToUpper(strings.TrimSpace(strings.Trim(raw, "\"'")))
	return base == "C" || base == "POSIX" || strings.HasPrefix(base, "C.") || strings.HasPrefix(base, "POSIX.")
}

func slicesContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
