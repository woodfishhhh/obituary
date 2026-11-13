package testenv

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Mutation suites compile source that exists for one run: each mutant and
// every package above it get build-cache entries no later build can hit.
// In the caller's GOCACHE they stay until Go trims them five days later (one
// MCP suite run wrote about 6 GB). Their go commands get a GOCACHEPROG
// instead: this test binary, serving hits from the caller's cache read-only
// and storing new entries in a directory removed with the test. The protocol
// package is internal to cmd/go and no library serves the go command's own
// cache layout read-only, so the small server is written here.
const goCacheProgArg = "obit-test-gocacheprog"

// MutantGoEnv returns environment entries for go commands that compile
// one-off source (-overlay mutants and fixtures). Call it once per suite:
// mutants then share the packages they rebuild without the mutation.
func MutantGoEnv(t testing.TB) []string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	layer := t.TempDir()
	var prog []string
	for _, arg := range []string{exe, goCacheProgArg, layer, os.Getenv("GOCACHE")} {
		if strings.Contains(arg, `"`) {
			t.Fatalf("GOCACHEPROG cannot quote %q", arg)
		}
		prog = append(prog, `"`+arg+`"`)
	}
	// GOCACHE also moves, so the go command's own files (README, fuzz
	// corpora) land in the layer, too.
	return []string{"GOCACHE=" + layer, "GOCACHEPROG=" + strings.Join(prog, " ")}
}

// Field names follow cmd/go/internal/cacheprog.
type goCacheRequest struct {
	ID       int64
	Command  string
	ActionID []byte
	OutputID []byte
	BodySize int64
}

type goCacheResponse struct {
	ID            int64
	Err           string     `json:",omitempty"`
	KnownCommands []string   `json:",omitempty"`
	Miss          bool       `json:",omitempty"`
	OutputID      []byte     `json:",omitempty"`
	Size          int64      `json:",omitempty"`
	Time          *time.Time `json:",omitempty"`
	DiskPath      string     `json:",omitempty"`
}

func serveGoCache(in io.Reader, out io.Writer, layer, base string) error {
	dec, enc := json.NewDecoder(in), json.NewEncoder(out)
	if err := enc.Encode(goCacheResponse{KnownCommands: []string{"get", "put", "close"}}); err != nil {
		return err
	}
	for {
		var req goCacheRequest
		if err := dec.Decode(&req); errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
		var res goCacheResponse
		switch req.Command {
		case "get":
			var ok bool
			if res, ok = lookupGoCache(layer, req.ActionID); !ok {
				if res, ok = lookupGoCache(base, req.ActionID); !ok {
					res = goCacheResponse{Miss: true}
				}
			}
		case "put":
			var body []byte // the body follows as a base64 JSON string
			if req.BodySize > 0 {
				if err := dec.Decode(&body); err != nil {
					return err
				}
			}
			if int64(len(body)) != req.BodySize {
				return fmt.Errorf("put body has %d bytes, want %d", len(body), req.BodySize)
			}
			path, err := putGoCache(layer, req.ActionID, req.OutputID, body)
			if err != nil {
				res.Err = err.Error()
			}
			res.DiskPath = path
		case "close":
			return enc.Encode(goCacheResponse{ID: req.ID})
		default:
			res.Err = "unknown command " + req.Command
		}
		res.ID = req.ID
		if err := enc.Encode(res); err != nil {
			return err
		}
	}
}

// lookupGoCache reads the go command's cache layout: <dir>/<aa>/<action>-a
// holds "v1 <action> <output> <size> <unixnano>\n" and the body is
// <dir>/<oo>/<output>-d. Anything unexpected is a miss, never a wrong hit.
func lookupGoCache(dir string, action []byte) (goCacheResponse, bool) {
	if !filepath.IsAbs(dir) || len(action) == 0 {
		return goCacheResponse{}, false
	}
	a := hex.EncodeToString(action)
	entry, err := os.ReadFile(filepath.Join(dir, a[:2], a+"-a"))
	fields := strings.Fields(string(entry))
	if err != nil || len(fields) != 5 || fields[0] != "v1" || fields[1] != a {
		return goCacheResponse{}, false
	}
	output, errOutput := hex.DecodeString(fields[2])
	size, errSize := strconv.ParseInt(fields[3], 10, 64)
	nanos, errTime := strconv.ParseInt(fields[4], 10, 64)
	if errOutput != nil || errSize != nil || errTime != nil || len(output) == 0 {
		return goCacheResponse{}, false
	}
	path := filepath.Join(dir, fields[2][:2], fields[2]+"-d")
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() || info.Size() != size {
		return goCacheResponse{}, false
	}
	at := time.Unix(0, nanos)
	return goCacheResponse{OutputID: output, Size: size, Time: &at, DiskPath: path}, true
}

func putGoCache(dir string, action, output, body []byte) (string, error) {
	if len(action) == 0 || len(output) == 0 {
		return "", errors.New("put without action or output ID")
	}
	o := hex.EncodeToString(output)
	path := filepath.Join(dir, o[:2], o+"-d")
	// Bodies are named by their hash: one already in place is this body.
	if info, err := os.Stat(path); err != nil || info.Size() != int64(len(body)) {
		if err := publishGoCacheFile(path, body); err != nil {
			return "", err
		}
	}
	a := hex.EncodeToString(action)
	entry := fmt.Sprintf("v1 %s %s %20d %20d\n", a, o, len(body), time.Now().UnixNano())
	return path, publishGoCacheFile(filepath.Join(dir, a[:2], a+"-a"), []byte(entry))
}

// Several go commands share one layer, so files appear complete or not at
// all. Windows refuses to replace a file another build has open; the file
// already there is then a complete entry for the same key.
func publishGoCacheFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "put-*")
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		_ = os.Remove(f.Name()) // our own temporary file
		if _, statErr := os.Stat(path); statErr == nil {
			return nil
		}
	}
	return err
}
