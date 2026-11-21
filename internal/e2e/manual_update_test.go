//go:build windows

package e2e_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/core"
)

const versionVar = "github.com/woodfishhhh/obituary/internal/buildinfo.Version"

var (
	versionBuilds   = map[string]func() (string, error){}
	versionBuildsMu sync.Mutex
)

// versionedBinaries builds obit.exe and obitd.exe that report version, once per
// version. obitd has no tray, so no icon appears on the desktop of whoever runs this.
func versionedBinaries(t *testing.T, version string) string {
	t.Helper()
	versionBuildsMu.Lock()
	build, ok := versionBuilds[version]
	if !ok {
		build = sync.OnceValues(func() (string, error) {
			root, err := filepath.Abs("../..")
			if err != nil {
				return "", err
			}
			dir, err := os.MkdirTemp("", "obit-e2e-update-bin-")
			if err != nil {
				return "", err
			}
			for name, flags := range map[string]string{
				"obit":  "-X " + versionVar + "=" + version,
				"obitd": "-H=windowsgui -X main.defaultTray=false -X " + versionVar + "=" + version,
			} {
				cmd := exec.Command("go", "build", "-o", filepath.Join(dir, name+".exe"), "-ldflags="+flags, "./cmd/"+name)
				cmd.Dir = root
				if out, err := cmd.CombinedOutput(); err != nil {
					return "", fmt.Errorf("build %s %s: %w\n%s", name, version, err, out)
				}
			}
			return dir, nil
		})
		versionBuilds[version] = build
	}
	versionBuildsMu.Unlock()
	dir, err := build()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// releaseServer serves a zip like the Windows release archive of version, with
// the given program contents, and checksums.txt for it.
func releaseServer(t *testing.T, version string, programs map[string][]byte) *httptest.Server {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range map[string][]byte{"LICENSE": []byte("license"), "README.txt": []byte("readme")} {
		programs[name] = data
	}
	for name, data := range programs {
		f, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		f.Write(data)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	archive := "obituary_" + version + "_windows_amd64.zip"
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v" + version + "/checksums.txt":
			fmt.Fprintf(rw, "%s  %s\n", sum(buf.Bytes()), archive)
		case "/v" + version + "/" + archive:
			rw.Write(buf.Bytes())
		default:
			http.NotFound(rw, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

type manualInstall struct {
	bin, home, config string
	others            map[string][]byte
	oldObit           []byte
}

// newManualInstall unpacks version 1.4.3 into a folder shared with other tools,
// starts its obitd, and caches 1.4.4 as the latest release.
func newManualInstall(t *testing.T) manualInstall {
	t.Helper()
	root := t.TempDir()
	m := manualInstall{bin: filepath.Join(root, "bin"), home: filepath.Join(root, "data"), config: filepath.Join(root, "config"),
		others: map[string][]byte{"jq.exe": []byte("another tool"), "notes.txt": []byte("mine"), "LICENSE": []byte("not ours"),
			filepath.Join("backup-1.4.2", "obit.exe"): []byte("older copy")}}
	t.Setenv("OBIT_HOME", m.home)
	t.Setenv("OBIT_CONFIG_DIR", m.config)
	old := versionedBinaries(t, "1.4.3")
	m.oldObit = readFile(t, filepath.Join(old, "obit.exe"))
	for name, data := range m.others {
		writeFixture(t, filepath.Join(m.bin, name), string(data))
	}
	writeFixture(t, filepath.Join(m.bin, "obit.exe"), string(m.oldObit))
	writeFixture(t, filepath.Join(m.bin, "obitd.exe"), string(readFile(t, filepath.Join(old, "obitd.exe"))))
	writeFixture(t, filepath.Join(m.bin, "obit-gui.exe"), string(m.oldObit)) // stands in for the window; never started
	writeFixture(t, filepath.Join(m.home, "update.json"), `{"checkedAt":"2026-11-21T00:00:00Z","latest":{"version":"1.4.4"}}`)
	m.run(t, "--version")
	return m
}

func (m manualInstall) run(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command(filepath.Join(m.bin, "obit.exe"), args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("obit %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (m manualInstall) startResident(t *testing.T) {
	t.Helper()
	cmd := exec.Command(filepath.Join(m.bin, "obitd.exe"), "--home", m.home, "--config-dir", m.config)
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go cmd.Wait()
	waitUntil(t, "the resident to start", func() bool { return app.Running(m.home) })
	t.Cleanup(func() {
		stop := exec.Command(filepath.Join(m.bin, "obit.exe"), "daemon", "stop")
		stop.Env = os.Environ()
		if out, err := stop.CombinedOutput(); err != nil {
			t.Errorf("stop resident: %v\n%s", err, out)
		}
	})
}

func waitUntil(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

// update runs `obit update --apply` with the fake release server and waits for
// the background helper to record how it went.
func (m manualInstall) update(t *testing.T, srv *httptest.Server) (success bool, message string, log string) {
	t.Helper()
	cmd := exec.Command(filepath.Join(m.bin, "obit.exe"), "update", "--apply")
	cmd.Env = append(os.Environ(), "OBIT_UPDATE_BASE_URL="+srv.URL)
	if out, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(out), "scheduled") {
		t.Fatalf("obit update --apply: %v\n%s", err, out)
	}
	var last struct {
		Success bool
		Error   string
		LogPath string
		Version string
	}
	waitUntil(t, "the update helper to finish", func() bool {
		data, err := os.ReadFile(filepath.Join(m.home, "update-apply.json"))
		return err == nil && json.Unmarshal(data, &last) == nil && last.LogPath != ""
	})
	logData, _ := os.ReadFile(last.LogPath)
	return last.Success, last.Error, string(logData)
}

func (m manualInstall) assertOthersUntouched(t *testing.T) {
	t.Helper()
	for name, want := range m.others {
		if got := readFile(t, filepath.Join(m.bin, name)); !bytes.Equal(got, want) {
			t.Errorf("%s changed: %q", name, got)
		}
	}
}

func (m manualInstall) updateFolder(t *testing.T) string {
	t.Helper()
	entries, err := os.ReadDir(m.bin)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".obituary-update-") {
			found = append(found, filepath.Join(m.bin, e.Name()))
		}
	}
	if len(found) != 1 {
		t.Fatalf("update folders: %v", found)
	}
	return found[0]
}

func TestManualInstallUpdatesItselfInPlace(t *testing.T) {
	if testing.Short() {
		t.Skip("builds two versions and runs the real updater")
	}
	m := newManualInstall(t)
	if got := m.run(t, "--version"); got != "obit version 1.4.3" {
		t.Fatalf("before: %s", got)
	}
	newBinaries := versionedBinaries(t, "1.4.4")
	newObit := readFile(t, filepath.Join(newBinaries, "obit.exe"))
	srv := releaseServer(t, "1.4.4", map[string][]byte{"obit.exe": newObit,
		"obitd.exe": readFile(t, filepath.Join(newBinaries, "obitd.exe")), "obit-gui.exe": newObit})
	m.startResident(t)

	success, message, log := m.update(t, srv)
	if !success {
		t.Fatalf("update failed: %s\n%s", message, log)
	}
	if got := m.run(t, "--version"); got != "obit version 1.4.4" {
		t.Fatalf("after: %s", got)
	}
	if !bytes.Equal(readFile(t, filepath.Join(m.bin, "obit.exe")), newObit) || !bytes.Equal(readFile(t, filepath.Join(m.bin, "obit-gui.exe")), newObit) {
		t.Error("the programs are not the release's")
	}
	m.assertOthersUntouched(t)
	if !app.Running(m.home) {
		t.Error("the resident was not brought back")
	}

	// The previous programs wait in a folder that obit tracks with a lifespan of a day.
	folder := m.updateFolder(t)
	if got := readFile(t, filepath.Join(folder, "old", "obit.exe")); !bytes.Equal(got, m.oldObit) {
		t.Error("the previous obit.exe was not kept")
	}
	state, err := core.Store{Home: m.home}.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range state.Items {
		if strings.EqualFold(filepath.Clean(item.Path), folder) {
			if until := time.Until(item.ExpiresAt); until < 23*time.Hour || until > 25*time.Hour {
				t.Errorf("lifespan %s", until)
			}
			return
		}
	}
	t.Fatalf("the update folder %s is not tracked", folder)
}

func TestManualInstallUpdateRollsBackWhenTheNewProgramDoesNotStart(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a version and runs the real updater")
	}
	m := newManualInstall(t)
	oldBinaries := versionedBinaries(t, "1.4.3")
	oldDaemon := readFile(t, filepath.Join(oldBinaries, "obitd.exe"))
	srv := releaseServer(t, "1.4.4", map[string][]byte{"obit.exe": []byte("this is not a program"),
		"obitd.exe": oldDaemon, "obit-gui.exe": []byte("neither")})
	m.startResident(t)

	success, message, log := m.update(t, srv)
	if success || !strings.Contains(message, "put back") {
		t.Fatalf("success=%v message=%q\n%s", success, message, log)
	}
	if got := m.run(t, "--version"); got != "obit version 1.4.3" {
		t.Fatalf("after the rollback: %s", got)
	}
	if !bytes.Equal(readFile(t, filepath.Join(m.bin, "obit.exe")), m.oldObit) || !bytes.Equal(readFile(t, filepath.Join(m.bin, "obitd.exe")), oldDaemon) {
		t.Error("the previous programs are not back")
	}
	m.assertOthersUntouched(t)
	if !app.Running(m.home) {
		t.Error("the resident was not brought back")
	}
}
