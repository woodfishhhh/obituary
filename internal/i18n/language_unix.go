//go:build linux || darwin

package i18n

// Native UI integration is Windows-only; an unavailable UI language resolves
// to English through the common resolver.
func SystemLanguage() string { return "" }
