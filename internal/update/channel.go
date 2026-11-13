package update

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
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
	if strings.Contains(exe, "/caskroom/obituary/") {
		return "homebrew"
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
	// The Homebrew cask links bin/obit to its Caskroom copy; os.Executable
	// reports the link. Windows channels are read from the path as launched.
	if runtime.GOOS != "windows" {
		if real, err := filepath.EvalSymlinks(executable); err == nil {
			executable = real
		}
	}
	packages, _ := userdirs.WinGetPackages()
	info, err := os.Stat(filepath.Join(filepath.Dir(executable), ".obituary-install.json"))
	return Channel(executable, packages, err == nil && info.Mode().IsRegular())
}

type receipt struct{ Owner, Source string }

// readReceipt returns the zero receipt when it is missing or unreadable.
func readReceipt(executable string) receipt {
	var r receipt
	file, err := os.Open(filepath.Join(filepath.Dir(executable), ".obituary-install.json"))
	if err != nil {
		return r
	}
	defer file.Close()
	if json.NewDecoder(io.LimitReader(file, 1<<20)).Decode(&r) != nil {
		return receipt{}
	}
	return r
}

// DetectSource defaults legacy and unreadable receipts to the original source.
func DetectSource(executable string) string {
	switch source := readReceipt(executable).Source; source {
	case "mirror", "custom":
		return source
	}
	return "github"
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
	case "homebrew":
		return "brew upgrade --cask obituary"
	case "script":
		dir := path.Dir(strings.ReplaceAll(executable, `\`, "/"))
		r := readReceipt(executable)
		if r.Owner == "obituary-install-sh-v1" {
			return shellCommand(latest, dir, r.Source == "mirror")
		}
		script, flags := "https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1", ""
		if r.Source == "mirror" {
			script, flags = "https://cn.obituary.woodfish.site/install.ps1", " -Mirror cn"
		}
		return "irm " + script + " -OutFile install.ps1; .\\install.ps1 -Version " + strings.TrimPrefix(latest, "v") + " -InstallDir '" + strings.ReplaceAll(dir, "'", "''") + "'" + flags
	default:
		return ""
	}
}

// shellCommand reruns install.sh, the documented upgrade for its installs.
func shellCommand(latest, dir string, mirror bool) string {
	script, flags := "https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh", ""
	if mirror {
		script, flags = "https://cn.obituary.woodfish.site/install.sh", " --mirror cn"
	}
	return "curl -fsSL " + script + " -o install.sh && bash install.sh --version " + strings.TrimPrefix(latest, "v") + " --install-dir '" + strings.ReplaceAll(dir, "'", `'\''`) + "'" + flags
}
