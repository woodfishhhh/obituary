package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

const releaseJSON = `{"tag_name":"v1.0.1","name":"Release 1.0.1","body":"Fixes and improvements","html_url":"https://github.com/woodfishhhh/obituary/releases/tag/v1.0.1","published_at":"2026-11-11T00:00:00Z"}`

func TestCheckCachesReleaseAndETag(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "GET" || r.URL.Path != releasePath || r.Header.Get("Accept") != "application/vnd.github+json" || r.Header.Get("User-Agent") == "" || r.Header.Get("Authorization") != "" || r.ContentLength > 0 {
			t.Errorf("unexpected request: %+v", r)
		}
		w.Header().Set("ETag", `"release-1"`)
		if requests > 1 {
			if r.Header.Get("If-None-Match") != `"release-1"` {
				t.Error("missing cached ETag")
			}
			w.WriteHeader(http.StatusNotModified)
			return
		}
		fmt.Fprint(w, releaseJSON)
	}))
	defer server.Close()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	c := Checker{Home: t.TempDir(), BaseURL: server.URL, Now: func() time.Time { return now }}
	first, err := c.Check(context.Background(), true, false)
	if err != nil || first.Latest.Version != "1.0.1" || first.LastError != "" || first.ETag == "" {
		t.Fatalf("first: %+v %v", first, err)
	}
	cached, err := Read(c.Home)
	if err != nil || cached != first {
		t.Fatalf("cache: %+v %v", cached, err)
	}
	now = now.Add(Interval - time.Second)
	second, err := c.Check(context.Background(), true, false)
	if err != nil || requests != 1 || second != first {
		t.Fatalf("fresh cache: %+v %v calls=%d", second, err, requests)
	}
	now = now.Add(time.Second)
	third, err := c.Check(context.Background(), true, false)
	if err != nil || requests != 2 || third.Latest != first.Latest || !third.CheckedAt.Equal(now) || third.LastError != "" {
		t.Fatalf("304: %+v %v", third, err)
	}
}

func TestCheckedReleaseVersionComparison(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, releaseJSON) }))
	defer server.Close()
	state, err := (Checker{Home: t.TempDir(), BaseURL: server.URL}).Check(context.Background(), true, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		current string
		newer   bool
	}{
		{"1.0.0", true}, {"1.0.1", false}, {"1.1.0", false},
		{"1.0.1-dev.abc", false}, {"1.0.0-dev.abc", true},
	} {
		if got := Newer(tc.current, state.Latest.Version); got != tc.newer {
			t.Fatalf("current=%s latest=%s newer=%t", tc.current, state.Latest.Version, got)
		}
	}
}

func TestCheckDisabledAndManual(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; fmt.Fprint(w, releaseJSON) }))
	defer server.Close()
	c := Checker{Home: filepath.Join(t.TempDir(), "absent"), BaseURL: server.URL}
	state, err := c.Check(context.Background(), false, false)
	if err != nil || requests != 0 || !state.CheckedAt.IsZero() {
		t.Fatalf("disabled: %+v %v", state, err)
	}
	if _, err := os.Stat(c.Home); !os.IsNotExist(err) {
		t.Fatalf("disabled check wrote directory: %v", err)
	}
	for range 2 {
		state, err = c.Check(context.Background(), false, true)
		if err != nil || state.Latest.Version != "1.0.1" {
			t.Fatalf("manual: %+v %v", state, err)
		}
	}
	if requests != 2 {
		t.Fatalf("manual requests=%d", requests)
	}
}

func TestCheckNetworkFailureKeepsRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, releaseJSON) }))
	defer server.Close()
	now := time.Now()
	c := Checker{Home: t.TempDir(), BaseURL: server.URL, Now: func() time.Time { return now }}
	before, err := c.Check(context.Background(), true, false)
	if err != nil {
		t.Fatal(err)
	}
	server.Close()
	now = now.Add(Interval)
	state, err := c.Check(context.Background(), true, false)
	if err != nil || state.LastError == "" || state.Latest != before.Latest || !state.CheckedAt.Equal(now) {
		t.Fatalf("failure: %+v %v", state, err)
	}
	cached, err := Read(c.Home)
	if err != nil || cached.LastError != state.LastError {
		t.Fatalf("saved error: %+v %v", cached, err)
	}
}

func TestCheckRejectsInvalidResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"draft", `{"tag_name":"v1.0.1","draft":true}`, 200},
		{"prerelease", `{"tag_name":"v1.0.1","prerelease":true}`, 200},
		{"semver-prerelease", `{"tag_name":"v1.0.1-rc.1"}`, 200},
		{"external-url", `{"tag_name":"v1.0.1","html_url":"https://example.org/download"}`, 200},
		{"bad-json", `{`, 200}, {"invalid-tag", `{"tag_name":"hello"}`, 200},
		{"rate-limit", `{}`, 403}, {"no-release", `{}`, 404}, {"uncached-304", ``, 304},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			c := Checker{Home: t.TempDir(), BaseURL: server.URL}
			state, err := c.Check(context.Background(), true, false)
			if err != nil || state.LastError == "" || state.Latest.Version != "" {
				t.Fatalf("state=%+v err=%v", state, err)
			}
			_, _ = c.Check(context.Background(), true, false)
			if calls != 1 {
				t.Fatal("failed request ignored daily cache")
			}
		})
	}
}

func TestCheckConcurrentCallersShareCache(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); fmt.Fprint(w, releaseJSON) }))
	defer server.Close()
	c := Checker{Home: t.TempDir(), BaseURL: server.URL}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := c.Check(context.Background(), true, false); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if requests.Load() != 1 {
		t.Fatalf("requests=%d", requests.Load())
	}
}

func TestManualCheckWaitsForConcurrentCheck(t *testing.T) {
	home := t.TempDir()
	unlock, ok, err := core.TryLockExclusive(filepath.Join(home, "update.lock"))
	if err != nil || !ok {
		t.Fatalf("lock: %t %v", ok, err)
	}
	defer func() {
		if unlock != nil {
			unlock()
		}
	}()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, releaseJSON) }))
	defer server.Close()
	c := Checker{Home: home, BaseURL: server.URL}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Check(ctx, true, true); err == nil {
		t.Fatal("busy manual check silently returned stale data")
	}
	unlock()
	unlock = nil
	state, err := c.Check(context.Background(), true, true)
	if err != nil || state.Latest.Version != "1.0.1" || calls != 1 {
		t.Fatalf("manual: %+v %v", state, err)
	}
}

func TestCheckDoesNotFollowRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("redirect followed") }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer server.Close()
	state, err := (Checker{Home: t.TempDir(), BaseURL: server.URL}).Check(context.Background(), true, true)
	if err != nil || state.LastError == "" {
		t.Fatalf("redirect: %+v %v", state, err)
	}
}

func TestCheckRequestHasTenSecondDeadline(t *testing.T) {
	c := Checker{Home: t.TempDir(), BaseURL: "http://127.0.0.1", Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > 10*time.Second || time.Until(deadline) <= 0 {
			t.Errorf("deadline: %v %t", deadline, ok)
		}
		return nil, context.DeadlineExceeded
	})}}
	state, err := c.Check(context.Background(), true, true)
	if err != nil || state.LastError == "" {
		t.Fatalf("timeout: %+v %v", state, err)
	}
}

func TestCheckFallsBackToMirrorWithoutGitHubResponse(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	github.Close()
	var calls int
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/download/latest.json" || r.Header.Get("Accept") != "application/vnd.github+json" || r.Header.Get("User-Agent") == "" || r.Header.Get("Authorization") != "" || r.Header.Get("If-None-Match") != "" || r.ContentLength > 0 {
			t.Errorf("unexpected mirror request: %+v", r)
		}
		w.Header().Set("ETag", `"mirror-only"`)
		fmt.Fprint(w, releaseJSON)
	}))
	defer mirror.Close()
	c := Checker{Home: t.TempDir(), BaseURL: github.URL, MirrorURL: mirror.URL + "/download/latest.json"}
	if err := save(c.Home, Cache{ETag: `"github-only"`, Latest: Release{Version: "1.0.0"}}); err != nil {
		t.Fatal(err)
	}
	state, err := c.Check(context.Background(), true, true)
	if err != nil || state.LastError != "" || state.Latest.Version != "1.0.1" || state.ETag != "" || calls != 1 {
		t.Fatalf("mirror: %+v %v calls=%d", state, err, calls)
	}
	cached, err := Read(c.Home)
	if err != nil || cached != state {
		t.Fatalf("cache: %+v %v", cached, err)
	}
}

func TestCheckFallsBackAfterGitHubTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the GitHub request timeout")
	}
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer github.Close()
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, releaseJSON) }))
	defer mirror.Close()
	c := Checker{Home: t.TempDir(), BaseURL: github.URL, MirrorURL: mirror.URL}
	state, err := c.Check(context.Background(), true, true)
	if err != nil || state.LastError != "" || state.Latest.Version != "1.0.1" {
		t.Fatalf("timeout fallback: %+v %v", state, err)
	}
}

func TestCheckGitHubHTTPErrorDoesNotUseMirror(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError, http.StatusFound} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			mirror := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("mirror requested after GitHub responded")
			}))
			defer mirror.Close()
			github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", mirror.URL)
				w.WriteHeader(status)
			}))
			defer github.Close()
			c := Checker{Home: t.TempDir(), BaseURL: github.URL, MirrorURL: mirror.URL}
			state, err := c.Check(context.Background(), true, true)
			if err != nil || state.LastError != fmt.Sprintf("GitHub releases: HTTP %d", status) || state.Latest.Version != "" {
				t.Fatalf("HTTP error: %+v %v", state, err)
			}
		})
	}
}

func TestCheckGitHubInvalidBodyDoesNotUseMirror(t *testing.T) {
	mirror := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("mirror requested after GitHub responded") }))
	defer mirror.Close()
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		fmt.Fprint(w, `{`)
	}))
	defer github.Close()
	state, err := (Checker{Home: t.TempDir(), BaseURL: github.URL, MirrorURL: mirror.URL}).Check(context.Background(), true, true)
	if err != nil || state.LastError == "" || state.Latest.Version != "" {
		t.Fatalf("incomplete GitHub response: %+v %v", state, err)
	}
}

func TestCheckMirrorRejectsInvalidResponses(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	github.Close()
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"external-url", strings.Replace(releaseJSON, "https://github.com/woodfishhhh/obituary/releases/tag/v1.0.1", "https://example.org/download", 1), 200},
		{"draft", `{"tag_name":"v1.0.1","draft":true}`, 200},
		{"prerelease", `{"tag_name":"v1.0.1","prerelease":true}`, 200},
		{"oversized", strings.Repeat(" ", 1<<20) + releaseJSON, 200},
		{"bad-json", `{`, 200}, {"not-modified", "", 304}, {"http-error", "", 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer mirror.Close()
			c := Checker{Home: t.TempDir(), BaseURL: github.URL, MirrorURL: mirror.URL}
			if err := save(c.Home, Cache{Latest: Release{Version: "1.0.0"}}); err != nil {
				t.Fatal(err)
			}
			state, err := c.Check(context.Background(), true, true)
			if err != nil || state.LastError == "" || state.Latest.Version != "1.0.0" {
				t.Fatalf("invalid mirror: %+v %v", state, err)
			}
		})
	}
}

func TestCheckMirrorDoesNotFollowRedirect(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	github.Close()
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("redirect followed") }))
	defer target.Close()
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer mirror.Close()
	state, err := (Checker{Home: t.TempDir(), BaseURL: github.URL, MirrorURL: mirror.URL}).Check(context.Background(), true, true)
	if err != nil || state.LastError == "" {
		t.Fatalf("mirror redirect: %+v %v", state, err)
	}
}

func TestCheckMirrorSharesTotalDeadline(t *testing.T) {
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, releaseJSON) }))
	defer mirror.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	totalDeadline, _ := ctx.Deadline()
	var githubDeadline time.Time
	calls := 0
	c := Checker{Home: t.TempDir(), BaseURL: "http://127.0.0.1", MirrorURL: mirror.URL}
	c.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > requestTimeout {
			t.Errorf("deadline: %v %t", deadline, ok)
		}
		if calls == 1 {
			githubDeadline = deadline
			return nil, context.DeadlineExceeded
		}
		if !deadline.After(githubDeadline) || !deadline.Equal(totalDeadline) {
			t.Errorf("mirror deadline=%v GitHub deadline=%v total=%v", deadline, githubDeadline, totalDeadline)
		}
		return http.DefaultTransport.RoundTrip(r)
	})}
	state, err := c.Check(ctx, true, true)
	if err != nil || state.LastError != "" || state.Latest.Version != "1.0.1" || calls != 2 {
		t.Fatalf("timeout fallback: %+v %v calls=%d", state, err, calls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCheckTestEnvironmentNeverUsesPublicAPI(t *testing.T) {
	t.Setenv("OBIT_TEST_NO_UPDATE", "1")
	for _, base := range []string{"", APIBase, APIBase + "/"} {
		c := Checker{Home: filepath.Join(t.TempDir(), "absent"), BaseURL: base}
		state, err := c.Check(context.Background(), true, true)
		if err != nil || !state.CheckedAt.IsZero() {
			t.Fatalf("test check: %+v %v", state, err)
		}
		if _, err := os.Stat(c.Home); !os.IsNotExist(err) {
			t.Fatalf("test check wrote directory: %v", err)
		}
	}
}

func TestCheckTestEnvironmentNeverUsesPublicMirror(t *testing.T) {
	t.Setenv("OBIT_TEST_NO_UPDATE", "1")
	for _, endpoint := range []string{"", MirrorURL, MirrorURL + "/"} {
		calls := 0
		c := Checker{Home: t.TempDir(), BaseURL: "http://127.0.0.1", MirrorURL: endpoint}
		c.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.URL.Host != "127.0.0.1" {
				t.Errorf("public endpoint requested: %s", r.URL)
			}
			return nil, context.DeadlineExceeded
		})}
		state, err := c.Check(context.Background(), true, true)
		if err != nil || state.LastError == "" || calls != 1 {
			t.Fatalf("test mirror check: %+v %v calls=%d", state, err, calls)
		}
	}
}
