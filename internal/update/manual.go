package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// A manual install is a folder the user unpacked a release into. The folder is
// shared (it may sit on PATH next to other tools), so an in-app update replaces
// only the programs the release archive ships and never treats the folder as
// owned by Obituary: no receipt, no PATH or shortcut changes, no other file.

// The conditions a manual install must meet before it can update itself. Each
// is a stable code so the window can word it in the user's language.
const (
	BlockedDev      = "dev"      // not a stable release build
	BlockedPlatform = "platform" // the release has no archive for this system
	BlockedBundle   = "bundle"   // inside a signed app bundle
	BlockedFiles    = "files"    // the release's programs are not all here as plain files
	BlockedWrite    = "write"    // the folder needs more than the user's own rights
)

// Applicability tells whether this installation can update itself, and for a
// manual install which condition fails. Channel is the same name DetectChannel
// reports, so a manual install that can update is still "other".
type Applicability struct {
	Channel string
	Can     bool
	Blocked string
	Detail  string
}

// Inspect looks at the installation of executable. Writing is probed last, only
// once every other condition holds.
func Inspect(executable, current string) Applicability {
	exe, channel := installation(executable)
	if channel != "other" {
		return Applicability{Channel: channel, Can: CanApply(channel)}
	}
	reason, detail := manualBlocker(exe, current, runtime.GOOS, runtime.GOARCH)
	return Applicability{Channel: channel, Can: reason == "", Blocked: reason, Detail: detail}
}

// Review is Inspect for a release check: the folder of a manual install is
// probed only when latest is newer than current, and a manual install that is
// up to date reports no condition.
func Review(executable, current, latest string) Applicability {
	if Newer(current, latest) {
		return Inspect(executable, current)
	}
	channel := DetectChannel(executable)
	return Applicability{Channel: channel, Can: CanApply(channel)}
}

// BlockedError says why a manual install cannot update itself.
type BlockedError struct{ Reason, Detail string }

func (e *BlockedError) Error() string {
	why := ""
	switch e.Reason {
	case BlockedDev:
		why = "this is a development build (" + e.Detail + ")"
	case BlockedPlatform:
		why = "the release has no archive for " + e.Detail
	case BlockedBundle:
		why = "it is inside an app bundle; update the app from its disk image"
	case BlockedFiles:
		why = "the release programs are not all in this folder as plain files: " + e.Detail
	case BlockedWrite:
		why = e.Detail + " cannot be written by the current user"
	default:
		why = e.Reason
	}
	return "in-app update is not available for this installation: " + why + "; see the release page"
}

// resolve names the channel an update would use. A manual install becomes
// "manual" only when every condition of manualBlocker holds.
func resolve(executable, current string) (string, string, error) {
	exe, channel := installation(executable)
	if channel != "other" {
		return exe, channel, nil
	}
	if reason, detail := manualBlocker(exe, current, runtime.GOOS, runtime.GOARCH); reason != "" {
		return exe, channel, &BlockedError{reason, detail}
	}
	return exe, "manual", nil
}

func manualBlocker(exe, current, goos, goarch string) (reason, detail string) {
	if !stable(current) {
		return BlockedDev, current
	}
	if _, ok := archiveName("0.0.0", goos, goarch); !ok {
		return BlockedPlatform, goos + "/" + goarch
	}
	if strings.Contains(filepath.ToSlash(exe), ".app/Contents/") {
		return BlockedBundle, ""
	}
	if bad := unusablePrograms(exe, goos, goarch); len(bad) > 0 {
		return BlockedFiles, strings.Join(bad, ", ")
	}
	dir := filepath.Dir(exe)
	if err := writable(dir); err != nil {
		return BlockedWrite, dir
	}
	return "", ""
}

// programs are the files of the release archive that this update replaces.
func programs(goos, goarch string) []string {
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	names := []string{"obit" + ext, "obitd" + ext}
	if goos == "windows" && goarch == "amd64" {
		names = append(names, "obit-gui.exe")
	}
	return names
}

// archiveName is the release archive for a system, as the release names it.
func archiveName(version, goos, goarch string) (string, bool) {
	ext := ".tar.gz"
	switch goos {
	case "windows":
		ext = ".zip"
	case "linux", "darwin":
	default:
		return "", false
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", false
	}
	return "obituary_" + version + "_" + goos + "_" + goarch + ext, true
}

// unusablePrograms lists the programs missing next to exe or not plain files
// (a link would be replaced by a copy), plus the running program itself when it
// has been renamed: the update could not tell that it replaced it.
func unusablePrograms(exe, goos, goarch string) []string {
	names := programs(goos, goarch)
	var bad []string
	for _, name := range names {
		info, err := os.Lstat(filepath.Join(filepath.Dir(exe), name))
		if err != nil || !info.Mode().IsRegular() {
			bad = append(bad, name)
		}
	}
	if !slices.ContainsFunc(names, func(name string) bool { return strings.EqualFold(name, filepath.Base(exe)) }) {
		bad = append(bad, filepath.Base(exe))
	}
	return bad
}

// writable creates and removes one file of its own name in dir; a read-only
// attribute or an access control list that needs elevation fails here.
func writable(dir string) error {
	f, err := os.CreateTemp(dir, ".obituary-probe-*")
	if err != nil {
		return err
	}
	return errors.Join(f.Close(), os.Remove(f.Name()))
}

var errNoArchive = errors.New("the release has no archive for this system")

func (p plan) archive() (string, error) {
	name, ok := archiveName(p.version, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return "", fmt.Errorf("%w: %s/%s", errNoArchive, runtime.GOOS, runtime.GOARCH)
	}
	return name, nil
}
