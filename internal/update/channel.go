package update

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// Channel uses path components and a receipt observation supplied by its caller.
// It remains independent of the host filesystem for Windows path tests.
func Channel(executable, wingetPackages string, hasReceipt bool) string {
	exe := cleanPath(executable)
	if strings.Contains(exe, "/scoop/apps/obituary/") {
		return "scoop"
	}
	if wingetPackages != "" {
		prefix := cleanPath(wingetPackages) + "/"
		if rest, ok := strings.CutPrefix(exe, prefix); ok {
			pkg, _, nested := strings.Cut(rest, "/")
			if nested && strings.HasPrefix(pkg, "woodfishhhh.obituary_") {
				return "winget"
			}
		}
	}
	if hasReceipt {
		return "script"
	}
	return "other"
}

func cleanPath(value string) string {
	value = strings.TrimPrefix(strings.ReplaceAll(value, `\`, "/"), "//?/")
	return strings.ToLower(path.Clean(value))
}

func DetectChannel(executable string) string {
	packages, _ := userdirs.WinGetPackages()
	info, err := os.Stat(filepath.Join(filepath.Dir(executable), ".obituary-install.json"))
	return Channel(executable, packages, err == nil && info.Mode().IsRegular())
}

// Command is display-only. A version is validated before it enters shell text.
func Command(channel, latest, executable string) string {
	if !stable(latest) {
		return ""
	}
	switch channel {
	case "scoop":
		return "scoop update obituary"
	case "winget":
		return "winget upgrade woodfishhhh.Obituary"
	case "script":
		dir := path.Dir(strings.ReplaceAll(executable, `\`, "/"))
		return "irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1; .\\install.ps1 -Version " + strings.TrimPrefix(latest, "v") + " -InstallDir '" + strings.ReplaceAll(dir, "'", "''") + "'"
	default:
		return ""
	}
}
