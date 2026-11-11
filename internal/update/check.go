package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/buildinfo"
	"github.com/woodfishhhh/obituary/internal/core"
)

const APIBase = "https://api.github.com"
const MirrorURL = "https://cn.obituary.woodfish.site/download/latest.json"
const releasePath = "/repos/woodfishhhh/obituary/releases/latest"
const requestTimeout = 10 * time.Second

// Checker accepts loopback endpoints in tests. Production supplies no endpoint,
// credentials, cookies, request body, or machine-specific identifiers.
type Checker struct {
	Home      string
	BaseURL   string
	MirrorURL string
	Client    *http.Client
	Now       func() time.Time
}

func (c Checker) Check(ctx context.Context, enabled, force bool) (Cache, error) {
	state, err := Read(c.Home)
	if err != nil {
		state.LastError = err.Error()
	}
	if !force && !enabled {
		return state, nil
	}
	// Test child processes inherit this switch, including explicit CLI checks.
	if testEndpointDisabled(c.BaseURL, APIBase) {
		return state, nil
	}
	if err := os.MkdirAll(c.Home, 0o700); err != nil {
		return state, err
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	unlock, ok, err := c.lock(ctx, force)
	if err != nil || !ok {
		return state, err
	}
	defer unlock()
	state, err = Read(c.Home)
	if err != nil {
		state.LastError = err.Error()
	}
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	if !force && !state.CheckedAt.IsZero() && now.Sub(state.CheckedAt) < Interval {
		return state, nil
	}
	return c.check(ctx, state, now)
}

func (c Checker) lock(ctx context.Context, wait bool) (func(), bool, error) {
	for {
		unlock, ok, err := core.TryLockExclusive(filepath.Join(c.Home, "update.lock"))
		if err != nil || ok || !wait {
			return unlock, ok, err
		}
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (c Checker) check(ctx context.Context, state Cache, now time.Time) (Cache, error) {
	latest, etag, err := c.fetch(ctx, state)
	state.CheckedAt = now.UTC()
	state.LastError = ""
	if err != nil {
		state.LastError = err.Error()
	} else {
		state.Latest, state.ETag = latest, etag
	}
	// Failed requests count toward the interval, preserving the last release
	// while avoiding repeated requests when offline or rate limited.
	return state, save(c.Home, state)
}

func testEndpointDisabled(endpoint, public string) bool {
	return os.Getenv("OBIT_TEST_NO_UPDATE") == "1" && (endpoint == "" || strings.TrimRight(endpoint, "/") == public)
}

func (c Checker) fetch(ctx context.Context, state Cache) (Release, string, error) {
	base := c.BaseURL
	if base == "" {
		base = APIBase
	}
	// Reserve part of the shared ten-second budget for an unreachable GitHub.
	githubCtx, cancel := context.WithTimeout(ctx, requestTimeout/2)
	defer cancel()
	etag := ""
	if state.Latest.Version != "" {
		etag = state.ETag
	}
	resp, err := c.request(githubCtx, strings.TrimRight(base, "/")+releasePath, etag)
	if err != nil {
		if resp == nil && ctx.Err() == nil && !testEndpointDisabled(c.MirrorURL, MirrorURL) {
			return c.fetchMirror(ctx)
		}
		return Release{}, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified && state.Latest.Version != "" {
		etag := resp.Header.Get("ETag")
		if etag == "" {
			etag = state.ETag
		}
		return state.Latest, etag, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, "", fmt.Errorf("GitHub releases: HTTP %d", resp.StatusCode)
	}
	latest, err := decodeRelease(resp.Body)
	return latest, resp.Header.Get("ETag"), err
}

func (c Checker) fetchMirror(ctx context.Context) (Release, string, error) {
	endpoint := c.MirrorURL
	if endpoint == "" {
		endpoint = MirrorURL
	}
	resp, err := c.request(ctx, endpoint, "")
	if err != nil {
		return Release{}, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, "", fmt.Errorf("mirror releases: HTTP %d", resp.StatusCode)
	}
	latest, err := decodeRelease(resp.Body)
	// A mirror response cannot validate a later conditional GitHub request.
	return latest, "", err
}

func (c Checker) request(ctx context.Context, endpoint, etag string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Obituary/"+buildinfo.Version)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	client := http.Client{Timeout: requestTimeout}
	if c.Client != nil {
		client = *c.Client
	}
	// Follow-up requests must stay on the explicitly chosen public endpoint.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client.Jar = nil
	return client.Do(req)
}

func releaseURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Host == "github.com" && u.User == nil && strings.HasPrefix(u.Path, "/woodfishhhh/obituary/releases/")
}

func decodeRelease(body io.Reader) (Release, error) {
	var wire struct {
		Tag        string    `json:"tag_name"`
		Name       string    `json:"name"`
		Body       string    `json:"body"`
		URL        string    `json:"html_url"`
		Published  time.Time `json:"published_at"`
		Draft      bool      `json:"draft"`
		Prerelease bool      `json:"prerelease"`
	}
	data, err := io.ReadAll(io.LimitReader(body, (1<<20)+1))
	if err != nil {
		return Release{}, err
	}
	if len(data) > 1<<20 {
		return Release{}, errors.New("GitHub release metadata exceeds 1 MiB")
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return Release{}, fmt.Errorf("GitHub release metadata: %w", err)
	}
	if wire.Draft || wire.Prerelease || !stable(wire.Tag) {
		return Release{}, errors.New("GitHub did not return a stable release")
	}
	if wire.URL == "" {
		wire.URL = "https://github.com/woodfishhhh/obituary/releases/tag/" + version(wire.Tag)
	}
	if !releaseURL(wire.URL) {
		return Release{}, errors.New("GitHub release URL is outside this repository")
	}
	return Release{Version: strings.TrimPrefix(wire.Tag, "v"), Name: wire.Name, Notes: wire.Body,
		URL: wire.URL, PublishedAt: wire.Published}, nil
}
