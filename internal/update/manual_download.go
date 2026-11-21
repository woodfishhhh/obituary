package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/buildinfo"
)

const archiveLimit = 256 << 20
const checksumsLimit = 1 << 20
const githubAnswerTimeout = 10 * time.Second

// sources are the places a release comes from, as directory URLs ending in "/".
// Like the install scripts, GitHub comes first and the mainland China mirror
// serves the same files when GitHub does not answer within ten seconds.
type sources struct {
	github, mirror string
	client         *http.Client
}

// releaseSources reads OBIT_UPDATE_BASE_URL only for a loopback HTTP address,
// the same exception install.ps1 makes for local snapshot servers.
func releaseSources(version string) sources {
	tag := "v" + strings.TrimPrefix(version, "v")
	if base := os.Getenv("OBIT_UPDATE_BASE_URL"); base != "" && loopbackHTTP(base) {
		dir := strings.TrimRight(base, "/") + "/" + tag + "/"
		return sources{github: dir, mirror: dir}
	}
	return sources{
		github: "https://github.com/woodfishhhh/obituary/releases/download/" + tag + "/",
		mirror: "https://cn.obituary.woodfish.site/download/" + tag + "/",
	}
}

func loopbackHTTP(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "http" || u.User != nil {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && ip.IsLoopback()
}

// noAnswer marks a request that got no HTTP response at all (timeout, DNS,
// refused). Only that moves the download to the mirror: an HTTP error from
// GitHub means GitHub is reachable and the error is the answer.
type noAnswer struct{ error }

func (e noAnswer) Unwrap() error { return e.error }

func (s sources) get(ctx context.Context, dir, name string, limit int64, progress func(done, total int64)) ([]byte, error) {
	endpoint := dir + name
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Obituary/"+buildinfo.Version)
	client := s.client
	if client == nil {
		client = &http.Client{}
	}
	c := *client
	c.Jar = nil
	c.CheckRedirect = followSecure
	resp, err := c.Do(req)
	if err != nil {
		return nil, noAnswer{fmt.Errorf("download %s: %w", endpoint, err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", endpoint, resp.StatusCode)
	}
	data, err := readCounting(resp.Body, resp.ContentLength, limit, progress)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", endpoint, err)
	}
	return data, nil
}

// followSecure keeps every hop on HTTPS (GitHub hands downloads to its own
// object host), apart from the loopback servers tests use.
func followSecure(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	if req.URL.Scheme != "https" && !loopbackHTTP(req.URL.String()) {
		return fmt.Errorf("refusing redirect to %s", req.URL.Redacted())
	}
	return nil
}

func readCounting(r io.Reader, size, limit int64, progress func(done, total int64)) ([]byte, error) {
	if size > limit {
		return nil, fmt.Errorf("the file is larger than %d bytes", limit)
	}
	var data []byte
	buf := make([]byte, 64<<10)
	for {
		n, err := r.Read(buf)
		data = append(data, buf[:n]...)
		if int64(len(data)) > limit {
			return nil, fmt.Errorf("the file is larger than %d bytes", limit)
		}
		if progress != nil {
			progress(int64(len(data)), size)
		}
		if errors.Is(err, io.EOF) {
			return data, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

// checksums returns checksums.txt and the directory it came from; the archive
// is downloaded from the same directory.
func (s sources) checksums(ctx context.Context, log io.Writer) ([]byte, string, error) {
	first, cancel := context.WithTimeout(ctx, githubAnswerTimeout)
	defer cancel()
	data, err := s.get(first, s.github, "checksums.txt", checksumsLimit, nil)
	var silent noAnswer
	if err == nil || !errors.As(err, &silent) || ctx.Err() != nil {
		return data, s.github, err
	}
	fmt.Fprintf(log, "GitHub did not answer (%v); downloading from the mirror instead.\n", err)
	data, err = s.get(ctx, s.mirror, "checksums.txt", checksumsLimit, nil)
	return data, s.mirror, err
}

var checksumLine = regexp.MustCompile(`^([0-9a-fA-F]{64})\s+\*?(\S+)$`)

// checksumFor requires exactly one entry for name, as install.ps1 does.
func checksumFor(list []byte, name string) (string, error) {
	var found []string
	for line := range strings.SplitSeq(string(list), "\n") {
		if m := checksumLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil && m[2] == name {
			found = append(found, strings.ToLower(m[1]))
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("expected exactly one SHA-256 entry for %s in checksums.txt; found %d", name, len(found))
	}
	return found[0], nil
}

func verifySum(data []byte, want, name string) error {
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != want {
		return fmt.Errorf("SHA-256 mismatch for %s; refusing to install", name)
	}
	return nil
}
