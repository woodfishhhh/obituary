//go:build darwin

package i18n

import (
	"os/exec"
	"strings"
)

// AppleLanguages is the user's preferred UI language list. A failed defaults
// lookup simply leaves resolution to the English fallback.
func SystemLanguage() string {
	out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.Trim(line, " \t,()\"")
		if line != "" {
			return line
		}
	}
	return ""
}
