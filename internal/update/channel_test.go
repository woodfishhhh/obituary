package update

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestChannel(t *testing.T) {
	const packages = `C:\Users\test\AppData\Local\Microsoft\WinGet\Packages`
	for _, tc := range []struct {
		exe     string
		receipt bool
		want    string
	}{
		{`C:\Users\test\scoop\apps\obituary\current\obit.exe`, false, "scoop"},
		{`D:\SCOOP\APPS\OBITUARY\1.0.0\obitd.exe`, true, "scoop"},
		{packages + `\woodfishhhh.Obituary_abc\obit-gui.exe`, false, "winget"},
		{packages + `\other.Obituary_abc\obit.exe`, false, "other"},
		{`D:\Microsoft\WinGet\Packages\woodfishhhh.Obituary_abc\obit.exe`, false, "other"},
		{`C:\scoop\apps\obituary-extra\obit.exe`, false, "other"},
		{`C:\scoop\apps\obituary\..\other\obit.exe`, false, "other"},
		{`C:\Programs\obituary\obit.exe`, true, "script"},
		{`D:\source\bin\obit.exe`, false, "other"},
		{"/opt/homebrew/Caskroom/obituary/1.1.1/obit", false, "homebrew"},
		{"/usr/local/Caskroom/obituary/1.1.1/obitd", false, "homebrew"},
		{"/opt/homebrew/Caskroom/obituary-extra/1.1.1/obit", false, "other"},
	} {
		t.Run(tc.exe, func(t *testing.T) {
			if got := Channel(tc.exe, packages, tc.receipt); got != tc.want {
				t.Fatalf("channel=%s want %s", got, tc.want)
			}
		})
	}
}

func TestScriptCommandReceiptSource(t *testing.T) {
	for _, tc := range []struct {
		name, receipt, source, script, suffix string
	}{
		{"legacy", `{"owner":"obituary-install-v1"}`, "github", "https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1", ""},
		{"github", `{"owner":"obituary-install-v1","source":"github"}`, "github", "https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1", ""},
		{"mirror", `{"owner":"obituary-install-v1","source":"mirror"}`, "mirror", "https://cn.obituary.woodfish.site/install.ps1", " -Mirror cn"},
		{"custom", `{"owner":"obituary-install-v1","source":"custom"}`, "custom", "https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1", ""},
		{"unknown", `{"owner":"obituary-install-v1","source":"untrusted"}`, "github", "https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1", ""},
		{"invalid", `{`, "github", "https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "O'Brien")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			exe := filepath.Join(dir, "obit.exe")
			if err := os.WriteFile(filepath.Join(dir, ".obituary-install.json"), []byte(tc.receipt), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := DetectSource(exe); got != tc.source {
				t.Fatalf("source=%q want %q", got, tc.source)
			}
			channel := DetectChannel(exe)
			if channel != "script" {
				t.Fatalf("channel=%s", channel)
			}
			want := "irm " + tc.script + " -OutFile install.ps1; .\\install.ps1 -Version 1.0.1 -InstallDir '" + strings.ReplaceAll(filepath.ToSlash(dir), "'", "''") + "'" + tc.suffix
			if got := Command(channel, "v1.0.1", exe); got != want {
				t.Fatalf("command=%q want %q", got, want)
			}
			if got := Command(channel, "1.0.1;bad", exe); got != "" {
				t.Fatalf("unsafe command=%q", got)
			}
		})
	}
}

func TestCommand(t *testing.T) {
	if got := Command("scoop", "1.0.1", ""); got != "scoop update obituary" {
		t.Fatal(got)
	}
	if got := Command("winget", "1.0.1", ""); got != "winget upgrade woodfishhhh.Obituary" {
		t.Fatal(got)
	}
	if got := Command("other", "1.0.1", ""); got != "" {
		t.Fatal(got)
	}
	got := Command("script", "v1.0.1", `D:\O'Brien\obit.exe`)
	if !strings.Contains(got, `-Version 1.0.1`) || !strings.Contains(got, `-InstallDir 'D:/O''Brien'`) {
		t.Fatal(got)
	}
	if got := Command("script", "1.0.1;bad", ""); got != "" {
		t.Fatal(got)
	}
}

func TestHomebrewCommand(t *testing.T) {
	if got := Command("homebrew", "v1.2.0", "/opt/homebrew/Caskroom/obituary/1.1.1/obit"); got != "brew upgrade --cask obituary" {
		t.Fatalf("command=%q", got)
	}
	if got := Command("homebrew", "v1.2.0-rc.1", ""); got != "" {
		t.Fatalf("prerelease command=%q", got)
	}
}

// The cask links bin/obit to its Caskroom copy; os.Executable reports the link.
func TestDetectChannelFollowsHomebrewLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Homebrew casks are macOS only")
	}
	root := t.TempDir()
	real := filepath.Join(root, "Caskroom", "obituary", "1.1.1", "obit")
	link := filepath.Join(root, "bin", "obit")
	for _, dir := range []string{filepath.Dir(real), filepath.Dir(link)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(real, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if got := DetectChannel(link); got != "homebrew" {
		t.Fatalf("channel=%s", got)
	}
}

// install.sh writes its own receipt; rerunning it is the upgrade.
func TestShellScriptCommand(t *testing.T) {
	for _, tc := range []struct{ source, script, suffix string }{
		{"github", "https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh", ""},
		{"mirror", "https://cn.obituary.woodfish.site/install.sh", " --mirror cn"},
		{"custom", "https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh", ""},
	} {
		t.Run(tc.source, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "O'Brien")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			exe := filepath.Join(dir, "obit")
			receipt := `{"owner":"obituary-install-sh-v1","source":"` + tc.source + `"}` + "\n"
			if err := os.WriteFile(filepath.Join(dir, ".obituary-install.json"), []byte(receipt), 0o600); err != nil {
				t.Fatal(err)
			}
			channel := DetectChannel(exe)
			if channel != "script" {
				t.Fatalf("channel=%s", channel)
			}
			want := "curl -fsSL " + tc.script + " -o install.sh && bash install.sh --version 1.2.0 --install-dir '" +
				strings.ReplaceAll(filepath.ToSlash(dir), "'", `'\''`) + "'" + tc.suffix
			if got := Command(channel, "v1.2.0", exe); got != want {
				t.Fatalf("command=%q want %q", got, want)
			}
			if got := Command(channel, "1.2.0;bad", exe); got != "" {
				t.Fatalf("unsafe command=%q", got)
			}
		})
	}
}
