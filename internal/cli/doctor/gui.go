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

func (lang diagnostics) diagnoseGUI(daemon string, daemonErr error, version func(string) string) DoctorCheck {
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		if daemonErr == nil {
			path := filepath.Join(filepath.Dir(daemon), "obit-gui")
			if lang.regularFile(path) == nil {
				return DoctorCheck{Name: "gui", Status: "PASS", Detail: lang.Text("cli.doctor.desktop_companion", nil) + path}
			}
		}
		return DoctorCheck{Name: "gui", Status: "INFO", Detail: lang.Text("cli.doctor.the_optional_ubuntu_desktop_package_provides_the", nil)}
	}
	if runtime.GOOS != "windows" {
		return DoctorCheck{Name: "gui", Status: "INFO", Detail: lang.Text("cli.doctor.use_the_cli_on_this_platform", nil)}
	}
	g := &GUICheck{Path: filepath.Join(filepath.Dir(daemon), "obit-gui.exe"), DaemonPath: daemon}
	c := DoctorCheck{Name: "gui", Status: "WARN", GUI: g}
	if daemonErr != nil {
		c.Detail = lang.Text("cli.doctor.cannot_verify_gui_companion", nil) + daemonErr.Error() + lang.Text("cli.doctor.reinstall_obit_gui_exe_alongside_obitd_exe", nil)
		return c
	}
	if err := lang.regularFile(g.Path); err != nil {
		c.Detail = err.Error() + lang.Text("cli.doctor.place_obit_gui_exe_in_the_same", nil) + filepath.Dir(daemon)
		return c
	}
	g.Version, g.DaemonVersion = version(g.Path), version(daemon)
	if g.Version != "" && g.DaemonVersion != "" && g.Version != g.DaemonVersion {
		c.Detail = fmt.Sprintf(lang.Text("cli.doctor.gui_version_differs_from_obitd_reinstall_both", nil), g.Version, g.DaemonVersion)
		return c
	}
	c.Status, c.Detail = "PASS", lang.Text("cli.doctor.gui_companion_exists_alongside_obitd", nil)+g.Path
	if g.Version == "" || g.DaemonVersion == "" {
		c.Detail += lang.Text("cli.doctor.comparable_version_metadata_unavailable_path_check_only", nil)
	} else {
		c.Detail += lang.Text("cli.doctor.matching_version", nil) + g.Version
	}
	c.Detail += lang.Text("cli.doctor.keep_both_executables_together_when_upgrading", nil)
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
