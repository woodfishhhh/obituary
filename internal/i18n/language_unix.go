//go:build linux

package i18n

// Linux has no platform-specific language lookup here; locale environment
// variables are handled by Resolve before this callback is consulted.
func SystemLanguage() string { return "" }
