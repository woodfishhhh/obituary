package doctor

import (
	"debug/buildinfo"
	"fmt"
	"path/filepath"
	"runtime"
	"runtime/debug"
)

type GUICheck struct {
	Path          string `json:"path"`
	DaemonPath    string `json:"daemonPath"`
	Version       string `json:"version,omitempty"`
	DaemonVersion string `json:"daemonVersion,omitempty"`
}

func diagnoseGUI(daemon string, daemonErr error, version func(string) string) DoctorCheck {
	if runtime.GOOS != "windows" {
		return DoctorCheck{Name: "gui", Status: "INFO", Detail: "the GUI is Windows-only"}
	}
	g := &GUICheck{Path: filepath.Join(filepath.Dir(daemon), "obit-gui.exe"), DaemonPath: daemon}
	c := DoctorCheck{Name: "gui", Status: "WARN", GUI: g}
	if daemonErr != nil {
		c.Detail = "cannot verify GUI companion: " + daemonErr.Error() + "; reinstall obit-gui.exe alongside obitd.exe"
		return c
	}
	if err := regularFile(g.Path); err != nil {
		c.Detail = err.Error() + "; place obit-gui.exe in the same directory as obitd.exe: " + filepath.Dir(daemon)
		return c
	}
	g.Version, g.DaemonVersion = version(g.Path), version(daemon)
	if g.Version != "" && g.DaemonVersion != "" && g.Version != g.DaemonVersion {
		c.Detail = fmt.Sprintf("GUI version %s differs from obitd %s; reinstall both executables from the same release", g.Version, g.DaemonVersion)
		return c
	}
	c.Status, c.Detail = "PASS", "GUI companion exists alongside obitd: "+g.Path
	if g.Version == "" || g.DaemonVersion == "" {
		c.Detail += "; comparable version metadata unavailable (path check only)"
	} else {
		c.Detail += "; matching version " + g.Version
	}
	c.Detail += "; keep both executables together when upgrading"
	return c
}

// Inspect build information with the standard library so checking a version
// cannot launch a GUI or daemon.
func binaryVersion(path string) string {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return ""
	}
	return buildVersion(info)
}

func buildVersion(info *debug.BuildInfo) string {
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var revision string
	for _, setting := range info.Settings {
		if setting.Key == "vcs.modified" && setting.Value == "true" {
			return "" // the revision cannot identify uncommitted build inputs
		}
		if setting.Key == "vcs.revision" {
			revision = setting.Value
		}
	}
	return revision
}
