//go:build windows

package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
)

func TestDoctorGUICompanion(t *testing.T) {
	for _, tc := range []struct {
		name, guiVersion, daemonVersion, status, detail string
	}{
		{"matching", "v1", "v1", "PASS", "matching version v1"},
		{"mismatch", "v1", "v2", "WARN", "same release"},
		{"no metadata", "", "", "PASS", "path check only"},
		{"one version absent", "v1", "", "PASS", "path check only"},
		{"missing", "", "", "WARN", "same directory"},
		{"directory", "", "", "WARN", "not a regular file"},
		{"daemon absent", "", "", "WARN", "cannot verify"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			daemon, gui := filepath.Join(root, "obitd.exe"), filepath.Join(root, "obit-gui.exe")
			clitest.WriteFixture(t, daemon, "never execute")
			var daemonErr error
			switch tc.name {
			case "missing":
				clitest.WriteFixture(t, filepath.Join(root, "elsewhere", "obit-gui.exe"), "wrong directory")
			case "directory":
				if err := os.Mkdir(gui, 0o700); err != nil {
					t.Fatal(err)
				}
			default:
				clitest.WriteFixture(t, gui, "never execute")
			}
			if tc.name == "daemon absent" {
				daemonErr = errors.New("missing obitd")
			}
			before := clitest.TreeSnapshot(t, root)
			got := diagnoseGUI(daemon, daemonErr, func(path string) string {
				if path == daemon {
					return tc.daemonVersion
				}
				return tc.guiVersion
			})
			if got.Status != tc.status || !strings.Contains(got.Detail, tc.detail) || got.GUI.Path != gui || got.GUI.DaemonPath != daemon {
				t.Fatalf("GUI companion diagnosis: %+v %+v", got, got.GUI)
			}
			if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
				t.Fatal("GUI diagnosis changed executables")
			}
		})
	}
}

func TestBuildVersion(t *testing.T) {
	for _, tc := range []struct {
		name, version, revision, modified, want string
	}{
		{"release", "v1.2.3", "revision", "false", "v1.2.3"},
		{"clean revision", "(devel)", "revision", "false", "revision"},
		{"dirty revision", "(devel)", "revision", "true", ""},
		{"missing metadata", "(devel)", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &debug.BuildInfo{Main: debug.Module{Version: tc.version}, Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: tc.revision}, {Key: "vcs.modified", Value: tc.modified},
			}}
			if got := buildVersion(info); got != tc.want {
				t.Fatalf("version = %q, want %q", got, tc.want)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "obit-gui.exe")
	clitest.WriteFixture(t, path, "no Go build information")
	if got := binaryVersion(path); got != "" {
		t.Fatalf("invented version: %q", got)
	}
}
