package update

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProgramsAndArchiveNames(t *testing.T) {
	for _, tc := range []struct {
		goos, goarch, archive string
		programs              []string
	}{
		{"windows", "amd64", "obituary_1.4.4_windows_amd64.zip", []string{"obit.exe", "obitd.exe", "obit-gui.exe"}},
		{"windows", "arm64", "obituary_1.4.4_windows_arm64.zip", []string{"obit.exe", "obitd.exe"}},
		{"linux", "amd64", "obituary_1.4.4_linux_amd64.tar.gz", []string{"obit", "obitd"}},
		{"linux", "arm64", "obituary_1.4.4_linux_arm64.tar.gz", []string{"obit", "obitd"}},
		{"darwin", "arm64", "obituary_1.4.4_darwin_arm64.tar.gz", []string{"obit", "obitd"}},
		{"darwin", "amd64", "obituary_1.4.4_darwin_amd64.tar.gz", []string{"obit", "obitd"}},
		{"windows", "386", "", nil},
		{"freebsd", "amd64", "", nil},
	} {
		name, ok := archiveName("1.4.4", tc.goos, tc.goarch)
		if name != tc.archive || ok != (tc.archive != "") {
			t.Errorf("%s/%s: archive %q, %v", tc.goos, tc.goarch, name, ok)
		}
		if tc.archive != "" {
			if got := programs(tc.goos, tc.goarch); strings.Join(got, " ") != strings.Join(tc.programs, " ") {
				t.Errorf("%s/%s: programs %v", tc.goos, tc.goarch, got)
			}
		}
	}
}

func TestManualBlocker(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	names := programs(goos, goarch)
	t.Run("every condition met", func(t *testing.T) {
		f := newInstall(t, "version=1.4.3")
		if reason, detail := manualBlocker(f.exe, "1.4.3", goos, goarch); reason != "" {
			t.Fatalf("blocked: %s %s", reason, detail)
		}
		// The probe leaves nothing behind in the shared folder.
		f.assertNoStray(t)
	})
	for _, current := range []string{"", "dev", "0.0.0-dev", "1.4.3-dev.abc1234", "1.4.3-rc.1", "latest"} {
		t.Run("not a release: "+current, func(t *testing.T) {
			f := newInstall(t, "version=1.4.3")
			if reason, _ := manualBlocker(f.exe, current, goos, goarch); reason != BlockedDev {
				t.Fatalf("reason=%q", reason)
			}
		})
	}
	t.Run("no archive for this system", func(t *testing.T) {
		f := newInstall(t, "version=1.4.3")
		if reason, detail := manualBlocker(f.exe, "1.4.3", "freebsd", "amd64"); reason != BlockedPlatform || detail != "freebsd/amd64" {
			t.Fatalf("reason=%q detail=%q", reason, detail)
		}
	})
	t.Run("inside an app bundle", func(t *testing.T) {
		exe := filepath.Join(t.TempDir(), "Obituary.app", "Contents", "MacOS", "obit")
		if reason, _ := manualBlocker(exe, "1.4.3", "darwin", "arm64"); reason != BlockedBundle {
			t.Fatalf("reason=%q", reason)
		}
	})
	t.Run("a program is missing", func(t *testing.T) {
		f := newInstall(t, "version=1.4.3")
		if err := os.Remove(filepath.Join(f.dir, names[1])); err != nil {
			t.Fatal(err)
		}
		if reason, detail := manualBlocker(f.exe, "1.4.3", goos, goarch); reason != BlockedFiles || detail != names[1] {
			t.Fatalf("reason=%q detail=%q", reason, detail)
		}
	})
	t.Run("a program is a folder", func(t *testing.T) {
		f := newInstall(t, "version=1.4.3")
		path := filepath.Join(f.dir, names[1])
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if reason, detail := manualBlocker(f.exe, "1.4.3", goos, goarch); reason != BlockedFiles || detail != names[1] {
			t.Fatalf("reason=%q detail=%q", reason, detail)
		}
	})
	t.Run("a program is a link", func(t *testing.T) {
		f := newInstall(t, "version=1.4.3")
		elsewhere := filepath.Join(t.TempDir(), "shared")
		write(t, elsewhere, fakeProgram("version=1.4.3"))
		path := filepath.Join(f.dir, names[1])
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(elsewhere, path); err != nil {
			t.Skipf("cannot make a link here: %v", err)
		}
		if reason, detail := manualBlocker(f.exe, "1.4.3", goos, goarch); reason != BlockedFiles || detail != names[1] {
			t.Fatalf("reason=%q detail=%q", reason, detail)
		}
	})
	t.Run("the running program was renamed", func(t *testing.T) {
		f := newInstall(t, "version=1.4.3")
		renamed := filepath.Join(f.dir, "ob"+filepath.Ext(names[0]))
		write(t, renamed, fakeProgram("version=1.4.3"))
		if reason, detail := manualBlocker(renamed, "1.4.3", goos, goarch); reason != BlockedFiles || detail != filepath.Base(renamed) {
			t.Fatalf("reason=%q detail=%q", reason, detail)
		}
	})
}

func TestWritableFailsWhereTheFolderCannotBeWritten(t *testing.T) {
	if err := writable(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("a missing folder is writable")
	}
	dir := t.TempDir()
	if err := writable(dir); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("the probe left %d entries", len(entries))
	}
}

// A folder that needs administrator rights (Program Files, /usr/local/bin)
// is reported as such instead of offering a button that cannot work.
func TestManualBlockerNeedsWriteAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("changes folder permissions with a child process")
	}
	f := newInstall(t, "version=1.4.3")
	if runtime.GOOS == "windows" {
		user := os.Getenv("USERDOMAIN") + `\` + os.Getenv("USERNAME")
		if out, err := exec.Command("icacls", f.dir, "/deny", user+":(WD,AD)").CombinedOutput(); err != nil {
			t.Skipf("icacls: %v %s", err, out)
		}
		t.Cleanup(func() { exec.Command("icacls", f.dir, "/remove:d", user).Run() })
	} else if err := os.Chmod(f.dir, 0o555); err != nil {
		t.Fatal(err)
	} else {
		t.Cleanup(func() { os.Chmod(f.dir, 0o755) })
	}
	if writable(f.dir) == nil {
		t.Skip("this account writes through a deny entry (administrator or root)")
	}
	if reason, detail := manualBlocker(f.exe, "1.4.3", runtime.GOOS, runtime.GOARCH); reason != BlockedWrite || detail != f.dir {
		t.Fatalf("reason=%q detail=%q", reason, detail)
	}
}

func TestInspectAndResolve(t *testing.T) {
	f := newInstall(t, "version=1.4.3")
	got := Inspect(f.exe, "1.4.3")
	if got.Channel != "other" || !got.Can || got.Blocked != "" {
		t.Fatalf("manual install: %+v", got)
	}
	exe, channel, err := resolve(f.exe, "1.4.3")
	if err != nil || channel != "manual" || !CanApply(channel) || filepath.Dir(exe) == "" {
		t.Fatalf("resolve: %q %q %v", exe, channel, err)
	}
	// A development build of the same files is not offered an update.
	if got := Inspect(f.exe, "0.0.0-dev"); got.Can || got.Blocked != BlockedDev || got.Channel != "other" {
		t.Fatalf("dev build: %+v", got)
	}
	_, _, err = resolve(f.exe, "0.0.0-dev")
	var blocked *BlockedError
	if !errors.As(err, &blocked) || !strings.Contains(err.Error(), "development build") {
		t.Fatalf("err=%v", err)
	}
}

// Package-managed installs keep their own channel and never become "manual",
// even though their folders hold the same programs.
func TestInspectKeepsPackageChannels(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scoop", "apps", "obituary", "current")
	for _, p := range programs(runtime.GOOS, runtime.GOARCH) {
		write(t, filepath.Join(dir, p), fakeProgram("version=1.4.3"))
	}
	if got := Inspect(filepath.Join(dir, programs(runtime.GOOS, runtime.GOARCH)[0]), "1.4.3"); got.Channel != "scoop" || !got.Can || got.Blocked != "" {
		t.Fatalf("scoop: %+v", got)
	}
	// A receipt from the install script keeps the script channel.
	f := newInstall(t, "version=1.4.3")
	write(t, filepath.Join(f.dir, ".obituary-install.json"), []byte(`{"owner":"obituary-install-v1"}`))
	if got := Inspect(f.exe, "1.4.3"); got.Channel != "script" || !got.Can {
		t.Fatalf("script: %+v", got)
	}
	if _, channel, err := resolve(f.exe, "0.0.0-dev"); err != nil || channel != "script" {
		t.Fatalf("resolve script: %q %v", channel, err)
	}
}

func TestTargetNamesTheFailingCondition(t *testing.T) {
	f := newInstall(t, "version=1.4.3")
	home := t.TempDir()
	if err := save(home, Cache{Latest: Release{Version: "1.4.4"}}); err != nil {
		t.Fatal(err)
	}
	if target, err := Target(home, f.exe, "1.4.3"); err != nil || target != "1.4.4" {
		t.Fatalf("target=%q err=%v", target, err)
	}
	if err := os.Remove(filepath.Join(f.dir, programs(runtime.GOOS, runtime.GOARCH)[1])); err != nil {
		t.Fatal(err)
	}
	_, err := Target(home, f.exe, "1.4.3")
	if err == nil || !strings.Contains(err.Error(), "not available") || !strings.Contains(err.Error(), programs(runtime.GOOS, runtime.GOARCH)[1]) {
		t.Fatalf("err=%v", err)
	}
}

func TestBlockedErrorWords(t *testing.T) {
	for _, tc := range []struct{ reason, detail, want string }{
		{BlockedDev, "0.0.0-dev", "development build (0.0.0-dev)"},
		{BlockedPlatform, "freebsd/amd64", "no archive for freebsd/amd64"},
		{BlockedBundle, "", "app bundle"},
		{BlockedFiles, "obitd.exe", "plain files: obitd.exe"},
		{BlockedWrite, `C:\Program Files\Obituary`, `C:\Program Files\Obituary cannot be written`},
	} {
		if got := (&BlockedError{tc.reason, tc.detail}).Error(); !strings.Contains(got, tc.want) {
			t.Errorf("%s: %q lacks %q", tc.reason, got, tc.want)
		}
	}
}
