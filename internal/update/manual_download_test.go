package update

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestReleaseSources(t *testing.T) {
	t.Setenv("OBIT_UPDATE_BASE_URL", "")
	got := releaseSources("1.4.4")
	if got.github != "https://github.com/woodfishhhh/obituary/releases/download/v1.4.4/" || got.mirror != "https://cn.obituary.woodfish.site/download/v1.4.4/" {
		t.Fatalf("sources %+v", got)
	}
	t.Setenv("OBIT_UPDATE_BASE_URL", "http://127.0.0.1:9/releases/")
	if got := releaseSources("v1.4.4"); got.github != "http://127.0.0.1:9/releases/v1.4.4/" || got.mirror != got.github {
		t.Fatalf("local sources %+v", got)
	}
	// Anything but loopback HTTP is ignored: the override cannot point
	// the updater at another machine.
	for _, base := range []string{"http://example.com/", "https://127.0.0.1/", "http://user@127.0.0.1/", "http://localhost/", "ftp://127.0.0.1/", "http://10.0.0.5/"} {
		t.Setenv("OBIT_UPDATE_BASE_URL", base)
		if got := releaseSources("1.4.4"); !strings.HasPrefix(got.github, "https://github.com/") {
			t.Errorf("%s was accepted: %+v", base, got)
		}
	}
}

func TestChecksumFor(t *testing.T) {
	hash := strings.Repeat("ab", 32)
	other := strings.Repeat("cd", 32)
	for name, tc := range map[string]struct {
		list, want string
		ok         bool
	}{
		"plain":       {hash + "  a.zip\n" + other + "  b.zip\n", hash, true},
		"binary mark": {hash + " *a.zip\n", hash, true},
		"crlf":        {hash + "  a.zip\r\n", hash, true},
		"upper":       {strings.ToUpper(hash) + "  a.zip\n", hash, true},
		"absent":      {other + "  b.zip\n", "", false},
		"twice":       {hash + "  a.zip\n" + other + "  a.zip\n", "", false},
		"short hash":  {"abcd  a.zip\n", "", false},
		"prefix only": {hash + "  a.zip.sig\n", "", false},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := checksumFor([]byte(tc.list), "a.zip")
			if (err == nil) != tc.ok || got != tc.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestVerifySum(t *testing.T) {
	data := []byte("release")
	sum := sha256.Sum256(data)
	good := hex.EncodeToString(sum[:])
	if err := verifySum(data, good, "a.zip"); err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string][]byte{"changed": []byte("releasX"), "longer": append(data, 'x'), "empty": nil} {
		if err := verifySum(got, good, "a.zip"); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func serveBytes(t *testing.T, data []byte) (sources, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return sources{client: srv.Client()}, srv.URL + "/"
}

func TestGetEnforcesTheSizeLimitAndReportsProgress(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 300<<10)
	s, dir := serveBytes(t, data)
	var last, total int64
	got, err := s.get(t.Context(), dir, "a", int64(len(data)), func(done, size int64) { last, total = done, size })
	if err != nil || len(got) != len(data) || last != int64(len(data)) || total != int64(len(data)) {
		t.Fatalf("len=%d last=%d total=%d err=%v", len(got), last, total, err)
	}
	if _, err := s.get(t.Context(), dir, "a", int64(len(data))-1, nil); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("a file over the limit was read: %v", err)
	}
	// A server that does not say how long the body is is held to the limit too.
	chunked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.(http.Flusher).Flush()
		io.Copy(w, bytes.NewReader(data))
	}))
	t.Cleanup(chunked.Close)
	if _, err := (sources{client: chunked.Client()}).get(t.Context(), chunked.URL+"/", "a", 1024, nil); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("an unbounded body was read: %v", err)
	}
}

func TestGetFollowsOnlySecureRedirects(t *testing.T) {
	data := []byte("moved")
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(data) }))
	t.Cleanup(target.Close)
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/real", http.StatusFound)
	}))
	t.Cleanup(redirect.Close)
	// Loopback HTTP hops are how these tests run; GitHub's own hop is HTTPS.
	if got, err := (sources{client: redirect.Client()}).get(t.Context(), redirect.URL+"/", "a", 1<<10, nil); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("%q %v", got, err)
	}
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com/a", http.StatusFound)
	}))
	t.Cleanup(insecure.Close)
	if _, err := (sources{client: insecure.Client()}).get(t.Context(), insecure.URL+"/", "a", 1<<10, nil); err == nil || !strings.Contains(err.Error(), "refusing redirect") {
		t.Fatalf("a plain-HTTP redirect to another host was followed: %v", err)
	}
}

func TestGetTreatsNoResponseAsNoAnswer(t *testing.T) {
	var silent noAnswer
	_, err := (sources{}).get(t.Context(), closedServerURL(t)+"/", "a", 1<<10, nil)
	if !errors.As(err, &silent) {
		t.Fatalf("err=%v", err)
	}
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	_, err = (sources{client: srv.Client()}).get(t.Context(), srv.URL+"/", "a", 1<<10, nil)
	if err == nil || errors.As(err, &silent) {
		t.Fatalf("an HTTP 404 counted as no answer: %v", err)
	}
}
