// Package i18n contains native UI text only; CLI and agent output stay English.
package i18n

import "strings"

type Language string

const (
	Auto             = "auto"
	Chinese Language = "zh-CN"
	English Language = "en"
)

func Valid(language string) bool {
	return language == Auto || language == string(Chinese) || language == string(English)
}

// Resolve uses the first Windows UI language, not the locale or keyboard layout.
// An unavailable system language selects English, like any non-Chinese language.
func Resolve(language string, systemLanguage func() string) Language {
	if language != Auto {
		return Language(language)
	}
	if strings.HasPrefix(strings.ToLower(systemLanguage()), "zh") {
		return Chinese
	}
	return English
}
