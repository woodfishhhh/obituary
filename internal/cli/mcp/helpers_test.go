package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

type approvalFaultFS struct {
	core.FS
	beforeRealPath func(string)
	beforeIdentify func(string)
}

func (fs approvalFaultFS) RealPath(path string) (string, error) {
	if fs.beforeRealPath != nil {
		fs.beforeRealPath(path)
	}
	return fs.FS.RealPath(path)
}

func (fs approvalFaultFS) Identify(path string) (core.Identity, error) {
	if fs.beforeIdentify != nil {
		fs.beforeIdentify(path)
	}
	return fs.FS.Identify(path)
}

type concurrentBurialFS struct {
	core.FS
	before func(string)
}

func (f concurrentBurialFS) MovePinned(path string, id core.Identity, to string) error {
	f.before(path)
	return f.FS.MovePinned(path, id, to)
}

func (f concurrentBurialFS) MovePinnedInto(path string, id core.Identity, parent core.RestoreParent, name string) (string, error) {
	f.before(path)
	return f.FS.MovePinnedInto(path, id, parent, name)
}

// Restore the v1 migration removed from 7be5fff, including its per-component
// enumeration. This must grant the unique Kelvin neighbor and fail the actual
// trust assertion; merely keeping the folded key would fail closed instead.
const legacyTrustMigrationMutant = `
legacyRulePath := func(path string) (string, bool) {
	abs, err := pathutil.Abs(path)
	if err != nil { return "", false }
	parent := pathutil.VolumeRoot(abs)
	for _, part := range strings.Split(strings.TrimPrefix(abs, parent), string(filepath.Separator)) {
		entries, err := os.ReadDir(parent)
		if err != nil { return "", false }
		match := ""
		for _, entry := range entries {
			if strings.ToLower(entry.Name()) == strings.ToLower(part) {
				if match != "" { return "", false }
				match = entry.Name()
			}
		}
		if match == "" { return "", false }
		parent = filepath.Join(parent, match)
	}
	real, err := rulePath(parent)
	return real, err == nil
}
migrated, conflicts := make(map[string]trustEntry), make(map[string]bool)
for old, entry := range f.Trusted {
	if real, unique := legacyRulePath(old); unique {
		if previous, ok := migrated[real]; ok && previous.SHA256 != entry.SHA256 { conflicts[real] = true }
		migrated[real] = entry
	}
}
for path := range conflicts { delete(migrated, path) }
f.Version, f.Trusted = 2, migrated
`

type mcpRecycleFS struct {
	core.FS
	destination string
}

func (f mcpRecycleFS) Recycle(path string, want core.Identity) error {
	return f.MovePinned(path, want, f.destination)
}

func mcpFixture(t *testing.T) (*mcp.ClientSession, string, *[]string) {
	t.Helper()
	starts := clitest.IsolateRunner(t)
	repo, _ := clitest.CommandDir(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	return clitest.ConnectMCP(t, NewServer(svc, "0.1.0-dev")), repo, starts
}

type cancelMCPTempFS struct {
	core.FS
	cancel context.CancelFunc
}

func mcpGitFixture(t *testing.T) (*mcp.ClientSession, string, *[]string) {
	t.Helper()
	session, repo, starts := mcpFixture(t)
	testenv.InitRepo(t, repo)
	return session, repo, starts
}

func (f cancelMCPTempFS) Identify(path string) (core.Identity, error) {
	id, err := f.FS.Identify(path)
	if err == nil && strings.Contains(path, "rollback-") {
		err = os.WriteFile(filepath.Join(path, "keep.txt"), []byte("keep"), 0o600)
		f.cancel()
	}
	return id, err
}
