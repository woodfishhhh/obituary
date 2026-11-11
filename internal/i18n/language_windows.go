package i18n

import "golang.org/x/sys/windows"

// SystemLanguage reads the user's preferred Windows UI language through
// GetUserPreferredUILanguages with MUI_LANGUAGE_NAME. The x/sys wrapper
// handles the UTF-16 buffer.
func SystemLanguage() string {
	languages, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(languages) == 0 {
		return ""
	}
	return languages[0]
}
