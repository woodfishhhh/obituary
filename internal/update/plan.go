package update

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

type plan struct {
	version string
	channel string
	dir     string
	source  string
}

// CanApply reports channel support, independently of whether a release is newer.
func CanApply(channel string) bool {
	switch channel {
	case "scoop", "winget", "homebrew", "script":
		return true
	}
	return false
}

// Target validates the cached release before starting an update process.
func Target(home, exe, current string) (string, error) {
	exe, channel := installation(exe)
	if !CanApply(channel) {
		return "", errors.New("in-app update is not supported for this installation; see the release page")
	}
	state, err := Read(home)
	if err != nil {
		return "", err
	}
	p, err := newPlan(channel, exe, current, state.Latest.Version)
	return p.version, err
}

func newPlan(channel, exe, current, target string) (plan, error) {
	if !CanApply(channel) {
		return plan{}, fmt.Errorf("in-app update is not supported for channel %s", channel)
	}
	if !Newer(current, target) {
		return plan{}, errors.New("no newer stable release; run obit update --check to refresh release information")
	}
	return plan{strings.TrimPrefix(target, "v"), channel, filepath.Dir(exe), DetectSource(exe)}, nil
}

func (p plan) script() string {
	if p.channel != "script" {
		return ""
	}
	if runtime.GOOS == "windows" {
		return "install.ps1"
	}
	return "install.sh"
}

func (p plan) argv(script string) []string {
	switch p.channel {
	case "scoop":
		return []string{"scoop", "update", "obituary"}
	case "winget":
		return []string{"winget", "upgrade", "--id", "woodfishhhh.Obituary", "--exact", "--silent", "--accept-source-agreements", "--accept-package-agreements"}
	case "homebrew":
		return []string{"brew", "upgrade", "--cask", "obituary"}
	}
	return p.scriptArgs(script, runtime.GOOS == "windows")
}

func (p plan) scriptArgs(script string, windows bool) []string {
	if windows {
		args := []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script, "-Version", p.version, "-InstallDir", p.dir}
		if p.source == "mirror" {
			args = append(args, "-Mirror", "cn")
		}
		return args
	}
	args := []string{"bash", script, "--version", p.version, "--install-dir", p.dir}
	if p.source == "mirror" {
		args = append(args, "--mirror", "cn")
	}
	return args
}

func (p plan) installedExe(original string) string {
	switch p.channel {
	case "scoop":
		return filepath.Join(filepath.Dir(p.dir), "current", filepath.Base(original))
	case "homebrew":
		room := filepath.Dir(filepath.Dir(p.dir))
		if strings.EqualFold(filepath.Base(room), "Caskroom") {
			return filepath.Join(filepath.Dir(room), "bin", filepath.Base(original))
		}
	}
	return original
}
