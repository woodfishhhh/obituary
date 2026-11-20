package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestUsageControls(t *testing.T) {
	testenv.EnableUsage(t)
	if _, on := usageLanguage(); !on {
		t.Fatal("expected on by default")
	}
	for key, val := range map[string]string{"OBIT_TELEMETRY": "0", "DO_NOT_TRACK": "1", "CI": "true", "GITHUB_ACTIONS": "true"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, val)
			if _, on := usageLanguage(); on {
				t.Fatal("should be off")
			}
		})
	}
	p, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p, []byte("[telemetry]\nenabled=false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, on := usageLanguage(); on {
		t.Fatal("config ignored")
	}
}

func TestFirstUsageSendsOnlyAllowedFields(t *testing.T) {
	testenv.EnableUsage(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || len(payload) != 6 || len(payload["install_id"]) != 32 {
			t.Error("invalid usage report", err)
		}
		for _, key := range strings.Fields("version os arch language channel") {
			if payload[key] == "" {
				t.Errorf("missing %s", key)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	var stamp string
	observeUsage(os.Getenv("OBIT_HOME"), time.Now(), func(s string) error { stamp = s; return nil })
	sendReservedUsage(stamp, srv.URL)
	sendReservedUsage(stamp, srv.URL)
	if calls != 1 {
		t.Fatalf("first use sent %d reports", calls)
	}
}

func TestFailedUsageAttemptConsumesRollingInterval(t *testing.T) {
	testenv.EnableUsage(t)
	home := t.TempDir()
	now := time.Now().UTC()
	starts := 0
	start := func(string) error { starts++; return errors.New("helper unavailable") }
	for _, elapsed := range []time.Duration{0, time.Second, time.Hour, 24*time.Hour - time.Nanosecond} {
		observeUsage(home, now.Add(elapsed), start)
	}
	if starts != 1 {
		t.Fatal(starts)
	}
	observeUsage(home, now.Add(24*time.Hour), start)
	if starts != 2 {
		t.Fatal("rolling interval never reopened")
	}
}

func TestUsageSharedClaim(t *testing.T) {
	testenv.EnableUsage(t)
	home := t.TempDir()
	now := time.Now().UTC()
	var starts atomic.Int32
	var stamp string
	start := func(s string) error { starts.Add(1); stamp = s; return nil }
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() { observeUsage(home, now, start) })
	}
	wg.Wait()
	if starts.Load() != 1 {
		t.Fatal(starts.Load())
	}
	id := claimUsage(home, stamp, now)
	if len(id) != 32 || claimUsage(home, stamp, now) != "" {
		t.Fatal("missing or repeated dispatch")
	}
	var next string
	observeUsage(home, now.Add(24*time.Hour), func(s string) error { next = s; return nil })
	if claimUsage(home, next, now.Add(24*time.Hour)) != id {
		t.Fatal("identity changed across uses")
	}
}

func TestUsageOptOutClearsPendingReport(t *testing.T) {
	testenv.EnableUsage(t)
	home := t.TempDir()
	t.Setenv("OBIT_HOME", home)
	now := time.Now().UTC()
	stamp := ""
	start := func(s string) error { stamp = s; return nil }
	observeUsage(home, now, start)
	if stamp == "" {
		t.Fatal("no reservation after actual use")
	}
	t.Setenv("OBIT_TELEMETRY", "0")
	ObserveUsage()
	s, e := readUsage(filepath.Join(home, "usage.json"))
	if e != nil || s.ID != "" || s.Pending || s.Attempt.IsZero() {
		t.Fatal("opt out did not clear identity/reservation", e)
	}
	if claimUsage(home, stamp, now) != "" {
		t.Fatal("sent after opt out")
	}
}

func TestUsageHTTPFailureDoesNotRetryOrRedirect(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Redirect(w, r, "/must-not-follow", http.StatusTemporaryRedirect)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer s.Close()
	sendUsage(s.URL, nil)
	sendUsage(s.URL, nil)
	if calls != 2 {
		t.Fatal(calls)
	}
}
