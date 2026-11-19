package update

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestApplyArgv(t *testing.T) {
	for _, tc := range []struct {
		channel string
		want    []string
	}{
		{"scoop", []string{"scoop", "update", "obituary"}},
		{"winget", []string{"winget", "upgrade", "--id", "woodfishhhh.Obituary", "--exact", "--silent", "--accept-source-agreements", "--accept-package-agreements"}},
		{"homebrew", []string{"brew", "upgrade", "--cask", "obituary"}},
	} {
		t.Run(tc.channel, func(t *testing.T) {
			p, err := newPlan(tc.channel, "obit", "1.3.2", "v1.4.0")
			if err != nil || !reflect.DeepEqual(p.argv(""), tc.want) {
				t.Fatalf("plan=%+v argv=%q err=%v", p, p.argv(""), err)
			}
		})
	}
	dir := filepath.Join(t.TempDir(), "Obit & Tools")
	p, err := newPlan("script", filepath.Join(dir, "obit"), "1.3.2", "1.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.scriptArgs("install.sh", false); !reflect.DeepEqual(got, []string{"bash", "install.sh", "--version", "1.4.0", "--install-dir", dir}) {
		t.Fatal(got)
	}
	p.source = "mirror"
	if got := p.scriptArgs("install.sh", false); !reflect.DeepEqual(got, []string{"bash", "install.sh", "--version", "1.4.0", "--install-dir", dir, "--mirror", "cn"}) {
		t.Fatal(got)
	}
	if got := p.scriptArgs("install.ps1", true); !reflect.DeepEqual(got, []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", "install.ps1", "-Version", "1.4.0", "-InstallDir", dir, "-Mirror", "cn"}) {
		t.Fatal(got)
	}
}

func TestApplyRejectsNonUpdates(t *testing.T) {
	for _, target := range []string{"", "latest", "2.0", "2.0.0-rc.1", "1.2.0", "1.3.2", "1.3.2+build", "2.0.0; other"} {
		if _, err := newPlan("script", "obit", "1.3.2-dev.1", target); err == nil || !strings.Contains(err.Error(), "no newer stable release") {
			t.Fatalf("target=%q err=%v", target, err)
		}
	}
	if _, err := newPlan("other", "obit", "1.3.2", "1.4.0"); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatal(err)
	}
}

func TestApplyFindsUpdatedHomebrewLink(t *testing.T) {
	prefix := t.TempDir()
	exe := filepath.Join(prefix, "Caskroom", "obituary", "1.3.2", "obit")
	want := filepath.Join(prefix, "bin", "obit")
	for _, original := range []string{exe, want} {
		p, err := newPlan("homebrew", original, "1.3.2", "1.4.0")
		if err != nil || p.installedExe(original) != want {
			t.Fatalf("original=%q updated=%q err=%v", original, p.installedExe(original), err)
		}
	}
}
