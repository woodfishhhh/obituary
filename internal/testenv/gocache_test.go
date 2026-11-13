package testenv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeGoCacheReadsBaseAndWritesLayer(t *testing.T) {
	base, layer := t.TempDir(), t.TempDir()
	hit, fresh := bytes.Repeat([]byte{0xab}, 32), bytes.Repeat([]byte{0xcd}, 32)
	output := bytes.Repeat([]byte{0x12}, 32)
	// An entry exactly as the go command writes it.
	writeFile(t, filepath.Join(base, "ab", fmt.Sprintf("%x-a", hit)), fmt.Sprintf("v1 %x %x %20d %20d\n", hit, output, 3, 7))
	writeFile(t, filepath.Join(base, "12", fmt.Sprintf("%x-d", output)), "old")
	before := snapshotFiles(t, base)

	var in bytes.Buffer
	enc := json.NewEncoder(&in)
	for _, req := range []goCacheRequest{
		{ID: 1, Command: "get", ActionID: hit},
		{ID: 2, Command: "get", ActionID: fresh},
		{ID: 3, Command: "put", ActionID: fresh, OutputID: output, BodySize: 5},
		{ID: 4, Command: "get", ActionID: fresh},
		{ID: 5, Command: "close"},
	} {
		if err := enc.Encode(req); err != nil {
			t.Fatal(err)
		}
		if req.Command == "put" {
			if err := enc.Encode([]byte("fresh")); err != nil {
				t.Fatal(err)
			}
		}
	}
	var out bytes.Buffer
	if err := serveGoCache(&in, &out, layer, base); err != nil {
		t.Fatal(err)
	}
	var res []goCacheResponse
	for dec := json.NewDecoder(&out); dec.More(); {
		var r goCacheResponse
		if err := dec.Decode(&r); err != nil {
			t.Fatal(err)
		}
		res = append(res, r)
	}
	if len(res) != 6 || len(res[0].KnownCommands) != 3 {
		t.Fatalf("responses: %+v", res)
	}
	if r := res[1]; r.Miss || r.Size != 3 || !bytes.Equal(r.OutputID, output) || !strings.HasPrefix(r.DiskPath, base) || r.Time == nil || r.Time.UnixNano() != 7 {
		t.Fatalf("base entry not served: %+v", r)
	}
	if !res[2].Miss || res[3].Err != "" || !strings.HasPrefix(res[3].DiskPath, layer) {
		t.Fatalf("miss or put: %+v %+v", res[2], res[3])
	}
	if r := res[4]; r.DiskPath != res[3].DiskPath || r.Size != 5 {
		t.Fatalf("put entry not served from layer: %+v", r)
	}
	if data, err := os.ReadFile(res[4].DiskPath); err != nil || string(data) != "fresh" {
		t.Fatalf("layer body: %q %v", data, err)
	}
	if after := snapshotFiles(t, base); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("base cache changed:\n%v\n%v", before, after)
	}
}

// A one-off build through the real go command: its entries land in the
// layer and never in the caller's cache, which still serves the runtime.
func TestMutantGoEnvKeepsBuildsOutOfCallerCache(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go command")
	}
	userCache := os.Getenv("GOCACHE")
	// Ordinary entries every build shares; a cold cache gets them once.
	if out, err := exec.Command("go", "build", "runtime").CombinedOutput(); err != nil {
		t.Fatalf("warm runtime: %v %s", err, out)
	}
	module := t.TempDir()
	writeFile(t, filepath.Join(module, "go.mod"), "module example.com/mutant\n\ngo 1.26\n")
	writeFile(t, filepath.Join(module, "main.go"), fmt.Sprintf("package main\n\nconst nonce = %q\n\nfunc main() { _ = nonce }\n", time.Now().String()))
	env := MutantGoEnv(t)
	layer := strings.TrimPrefix(env[0], "GOCACHE=")
	cmd := exec.Command("go", "build", "-o", filepath.Join(module, "mutant.exe"), ".")
	cmd.Dir = module
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build through GOCACHEPROG: %v %s", err, out)
	}
	var entries int
	for name := range snapshotFiles(t, layer) {
		if !strings.HasSuffix(name, "-a") && !strings.HasSuffix(name, "-d") {
			continue
		}
		entries++
		// Bodies are named by content and may match; actions are this build's.
		if _, err := os.Stat(filepath.Join(userCache, name)); err == nil && strings.HasSuffix(name, "-a") {
			t.Errorf("one-off build wrote %s into the caller's cache", name)
		}
	}
	// The runtime alone is about 90 bodies; only the main package is new.
	if entries == 0 || entries > 20 {
		t.Fatalf("layer has %d entries; the caller's cache was not used or not bypassed", entries)
	}
}

// Every one-off -overlay build goes through MutantGoEnv.
func TestOverlayBuildsUseMutantGoEnv(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	inherited := map[string]string{
		"cmd/obitd/main_test.go":                   "builds the overlay of a mutation suite, whose environment it inherits",
		"internal/e2e/main_test.go":                "builds the overlay of a mutation suite, whose environment it inherits",
		"internal/cli/graveyard/graveyard_test.go": "manual benchmark of a build someone chose to compare",
		"internal/testenv/gocache_test.go":         "this scan",
	}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "site" || d.Name() == "node_modules" || d.Name() == "docs-internal") {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if bytes.Contains(data, []byte(`"-overlay"`)) && !bytes.Contains(data, []byte("MutantGoEnv(")) && inherited[rel] == "" {
			t.Errorf("%s compiles an overlay into the caller's GOCACHE; use testenv.MutantGoEnv", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

// snapshotFiles maps slash paths under dir to size and modification time.
func snapshotFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		files[filepath.ToSlash(rel)] = fmt.Sprint(info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
