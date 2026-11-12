package auto

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func shortPath(t *testing.T, path string) string { return testenv.ShortPath(t, path) }

type repo struct {
	t      *testing.T
	root   string
	engine *Engine
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	r := newEngineFixture(t)
	testenv.InitRepo(t, r.root)
	return r
}

func newEngineFixture(t *testing.T) *repo {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "repo")
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	r := &repo{t: t, root: root}
	svc := &core.Service{
		Store: core.Store{Home: home},
		Guard: core.Guard{System: []string{home}},
		FS:    winfs.FS{},
	}
	r.engine = &Engine{Svc: svc, Trust: rules.TrustStore{Path: filepath.Join(home, "trusted_rules.json")}}
	svc.Owner, svc.Recheck = Owner, r.engine.RecheckRules
	return r
}

func (r *repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.root, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "core.autocrlf=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}

func (r *repo) write(rel, content string) string {
	r.t.Helper()
	path := filepath.Join(r.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		r.t.Fatal(err)
	}
	return path
}

func (r *repo) commit(rels ...string) {
	r.t.Helper()
	r.git(append([]string{"add", "-f", "--"}, rels...)...)
	r.git("commit", "-q", "-m", "c")
}

func (r *repo) apply(paths ...string) map[string]Decision {
	r.t.Helper()
	decisions, err := r.engine.Apply(paths, Options{Session: "s1"})
	if err != nil {
		r.t.Fatal(err)
	}
	out := map[string]Decision{}
	for _, d := range decisions {
		out[d.Path] = d
	}
	return out
}

func checkOwnerLedgerLock(t *testing.T, svc *core.Service, locked bool) {
	t.Helper()
	// Probe ownership without consuming a Git query's deadline while waiting.
	release, available, err := core.TryLockExclusive(filepath.Join(svc.Store.Home, "store.lock"))
	if available {
		release()
	}
	if err != nil || available == locked {
		t.Fatalf("Git verification used the wrong ledger lock boundary: locked=%t available=%t err=%v", locked, available, err)
	}
}

type unreadableObservationFS struct {
	core.FS
	path string
}

func (f *unreadableObservationFS) RealPath(path string) (string, error) {
	if path == f.path {
		return "", errors.New("temporary read failure")
	}
	return f.FS.RealPath(path)
}

type unavailableObservationFS struct {
	core.FS
	path        string
	volumeError string
	onlineOnce  bool
}

func (f *unavailableObservationFS) Identify(path string) (core.Identity, error) {
	if path == f.path {
		if f.onlineOnce {
			f.onlineOnce = false
			return f.FS.Identify(path)
		}
		return core.Identity{}, core.ErrNotFound
	}
	if filepath.Clean(path) == filepath.Clean(pathutil.VolumeRoot(f.path)) {
		if f.volumeError == "volume serial changed" {
			id, err := f.FS.Identify(path)
			id.Serial++
			return id, err
		}
		return core.Identity{}, core.ErrNotFound
	}
	return f.FS.Identify(path)
}

type changedObservationFS struct {
	core.FS
	path string
}

func (f changedObservationFS) Identify(path string) (core.Identity, error) {
	id, err := f.FS.Identify(path)
	if err == nil && path == f.path {
		id.Serial++
	}
	return id, err
}

type interleaveAddFS struct {
	winfs.FS
	calls     int
	beforeAdd func()
}

func (f *interleaveAddFS) Identify(path string) (core.Identity, error) {
	f.calls++
	if f.calls == 2 {
		f.beforeAdd() // vet passed on its snapshot; Add has not taken the write lock
	}
	return f.FS.Identify(path)
}

type timedBurialFS struct {
	core.FS
	checked *time.Time
	gap     time.Duration
	move    time.Duration
}

func (f *timedBurialFS) MovePinned(path string, want core.Identity, to string) error {
	f.gap = time.Since(*f.checked)
	start := time.Now()
	err := f.FS.MovePinned(path, want, to)
	f.move = time.Since(start)
	return err
}

type lookupFS struct {
	winfs.FS
	failID core.Identity
	err    error
	calls  map[core.Identity]int
}

func (f *lookupFS) Locate(id core.Identity) (string, error) {
	f.calls[id]++
	if id == f.failID && f.err != nil {
		return "", f.err
	}
	return f.FS.Locate(id)
}

func denyEnumeration(t *testing.T, path string) { testenv.DenyEnumeration(t, path) }

type approvalFS struct {
	winfs.FS
	first func()
	err   error
	calls int
}

func (f *approvalFS) Identify(path string) (core.Identity, error) {
	f.calls++
	if f.calls == 2 && f.err != nil {
		return core.Identity{}, f.err
	}
	id, err := f.FS.Identify(path)
	if f.calls == 1 && err == nil && f.first != nil {
		f.first()
	}
	return id, err
}
