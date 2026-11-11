package gui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

type fakeMenu struct {
	trees  map[string]contextmenu.Tree
	err    error
	mu     sync.Mutex
	writes int
	failAt int
}

func (f *fakeMenu) ReadTree(path string) (contextmenu.Tree, error) { return f.trees[path], f.err }
func (f *fakeMenu) WriteTree(path string, tree contextmenu.Tree) error {
	if f.err != nil {
		return f.err
	}
	f.trees[path] = tree
	return nil
}
func (f *fakeMenu) Lock() (func(), error) {
	f.mu.Lock()
	return f.mu.Unlock, nil
}

func (f *fakeMenu) CompareAndSwap(c contextmenu.Change) error {
	if !reflect.DeepEqual(f.trees[c.Path], c.Before) {
		return errors.New("registry changed after planning")
	}
	f.writes++
	if f.writes == f.failAt {
		return errors.New("injected write failure")
	}
	if c.After == nil {
		return f.DeleteTree(c.Path, c.Before)
	}
	return f.WriteTree(c.Path, c.After)
}

func (f *fakeMenu) DeleteTree(path string, _ contextmenu.Tree) error {
	if f.err != nil {
		return f.err
	}
	delete(f.trees, path)
	return nil
}

type safeFS struct{ winfs.FS }

func (safeFS) Recycle(string, core.Identity) error {
	return errors.New("test must not reach Recycle Bin")
}

func fixture(t *testing.T) (*API, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	svc := &core.Service{Store: core.Store{Home: home}, FS: safeFS{}, Guard: core.Guard{System: []string{home}}, Owner: auto.Owner}
	engine := app.Engine(svc)
	svc.Recheck = engine.RecheckRules
	d := Dependencies{Service: svc, Engine: engine, ConfigDir: filepath.Join(root, "config"),
		Executable: filepath.Join(root, "obit.exe"), Daemon: filepath.Join(root, "obitd.exe"),
		ContextMenu: &fakeMenu{trees: map[string]contextmenu.Tree{}}, RunKey: testenv.NewRunRegistry()}
	write(t, d.Executable, "fixture")
	write(t, d.Daemon, "fixture")
	a, err := New(d)
	if err != nil {
		t.Fatal(err)
	}
	return a, root
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func add(t *testing.T, a *API, path string, expiry time.Time) core.Item {
	t.Helper()
	write(t, path, "payload")
	results, err := a.deps.Service.Add([]string{path}, core.AddOptions{ExpiresAt: expiry})
	if err != nil || len(results) != 1 || results[0].Error != "" {
		t.Fatalf("add: %+v %v", results, err)
	}
	return *results[0].Item
}

func bury(t *testing.T, a *API, path string) core.Burial {
	t.Helper()
	write(t, path, "payload")
	results, err := a.deps.Service.Bury([]string{path}, false, "")
	if err != nil || len(results) != 1 || results[0].Error != "" {
		t.Fatalf("bury: %+v %v", results, err)
	}
	return *results[0].Burial
}

func ref(id int64) string { return strconv.FormatInt(id, 10) }

func breakStore(t *testing.T, a *API) {
	t.Helper()
	write(t, filepath.Join(a.deps.Service.Store.Home, "store.json"), "invalid")
}

func repository(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, "repo")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	testenv.InitRepo(t, path)
	return path
}
