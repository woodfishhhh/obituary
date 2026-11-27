package app

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"time"

	"github.com/woodfishhhh/obituary/internal/buildinfo"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/update"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

const usageEndpoint = "https://blog.woodfish.site/api/obituary/ping"

var reportVersion = regexp.MustCompile(`^v?(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})(-(dev|(alpha|beta|rc)\.?[0-9]{1,4}))?$`)

type usageState struct {
	ID      string
	Attempt time.Time
	Pending bool
}

func usageLanguage() (i18n.Language, bool) {
	if os.Getenv("OBIT_TELEMETRY") == "0" {
		_ = ForgetUsage()
		return "", false
	}
	if os.Getenv("DO_NOT_TRACK") == "1" {
		return "", false
	}
	for _, key := range []string{"CI", "CONTINUOUS_INTEGRATION", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "TF_BUILD", "TEAMCITY_VERSION", "BUILDKITE"} {
		if os.Getenv(key) != "" {
			return "", false
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return "", false
	}
	if !cfg.Telemetry.Enabled {
		_ = ForgetUsage()
		return "", false
	}
	return i18n.Resolve(cfg.UI.Language, i18n.SystemLanguage), true
}

func ObserveUsage() {
	_, enabled := usageLanguage()
	if !enabled {
		return
	}
	home, err := userdirs.Data()
	if err != nil {
		return
	}
	observeUsage(home, time.Now(), func(stamp string) error { return runDaemonFlag(home, "--usage-report="+stamp) })
}

// A busy lock, unavailable storage or a failed helper costs a sample, never a foreground command.
func observeUsage(home string, now time.Time, start func(string) error) {
	path := filepath.Join(home, "usage.json")
	if os.MkdirAll(home, 0700) != nil {
		return
	}
	release, ok, err := ledger.TryLockExclusive(filepath.Join(home, "usage.lock"))
	if err != nil || !ok {
		return
	}
	defer release()
	s, err := readUsage(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) || (!s.Attempt.IsZero() && now.Sub(s.Attempt) < 24*time.Hour) {
		return
	}
	// An opt-out may have committed between the foreground check and this lock.
	if cfg, err := config.Load(); err != nil || !cfg.Telemetry.Enabled {
		return
	}
	if s.ID == "" {
		var id [16]byte
		rand.Read(id[:])
		s.ID = hex.EncodeToString(id[:])
	}
	s.Attempt = now.UTC()
	s.Pending = true
	if saveUsage(path, s) == nil {
		_ = start(s.Attempt.Format(time.RFC3339Nano))
	}
}

func readUsage(path string) (usageState, error) {
	var s usageState
	b, err := lifecycle.ReadMetadata(path)
	if err == nil {
		err = json.Unmarshal(b, &s)
	}
	return s, err
}
func saveUsage(path string, s usageState) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return lifecycle.WriteMetadata(path, b)
}

// Clearing the identity is local. Keep the attempt timestamp across opt-out/in.
func ForgetUsage() error {
	home, err := userdirs.Data()
	if err != nil {
		return err
	}
	path := filepath.Join(home, "usage.json")
	current, err := readUsage(path)
	if errors.Is(err, os.ErrNotExist) || (err == nil && !current.Pending && current.ID == "") {
		return nil
	}
	if err != nil {
		return err
	}
	release, err := ledger.LockFile(filepath.Join(home, "usage.lock"), true)
	if err != nil {
		return err
	}
	defer release()
	s, err := readUsage(path)
	if err != nil {
		return err
	}
	return saveUsage(path, usageState{Attempt: s.Attempt})
}

// Only a fresh reservation from an actual use can authorize one detached attempt.
func claimUsage(home, stamp string, now time.Time) string {
	release, err := ledger.LockFile(filepath.Join(home, "usage.lock"), true)
	if err != nil {
		return ""
	}
	defer release()
	path := filepath.Join(home, "usage.json")
	s, err := readUsage(path)
	if err != nil || !s.Pending || s.Attempt.Format(time.RFC3339Nano) != stamp || now.Sub(s.Attempt) > time.Minute {
		return ""
	}
	raw, err := hex.DecodeString(s.ID)
	if err != nil || len(raw) != 16 {
		return ""
	}
	s.Pending = false
	s.Attempt = now.UTC()
	if saveUsage(path, s) != nil {
		return ""
	}
	return s.ID
}

func SendUsage(stamp string) {
	sendReservedUsage(stamp, usageEndpoint)
}

func sendReservedUsage(stamp, endpoint string) {
	lang, enabled := usageLanguage()
	if !enabled {
		return
	}
	home, err := userdirs.Data()
	if err != nil {
		return
	}
	id := claimUsage(home, stamp, time.Now())
	if id == "" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	version := buildinfo.Version
	if !reportVersion.MatchString(version) {
		version = "unknown"
	}
	arch := runtime.GOARCH
	if !slices.Contains([]string{"amd64", "386", "arm", "arm64"}, arch) {
		arch = "unknown"
	}
	p := map[string]string{"install_id": id, "version": version, "os": runtime.GOOS, "arch": arch, "language": string(lang), "channel": update.DetectChannel(exe)}
	sendUsage(endpoint, p)
}

func sendUsage(endpoint string, p map[string]string) {
	body, _ := json.Marshal(p)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "")
	client := http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}
