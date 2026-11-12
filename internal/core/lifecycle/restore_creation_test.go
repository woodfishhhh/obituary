package lifecycle_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestResurrectPreflightCreatesNothingOnRefusal(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"new-junction", "retargeted", "missing-junction", "protected-real"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			link, real, other := filepath.Join(f.dir, "output"), filepath.Join(f.dir, "real"), filepath.Join(f.dir, "other")
			for _, dir := range []string{real, other} {
				if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			junction := func(target string) {
				if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
					t.Fatalf("junction: %v %s", err, out)
				}
			}
			if mode != "new-junction" {
				junction(real)
			}
			b := buryOne(t, f, "output/deep/nested/payload.txt")
			for _, dir := range []string{filepath.Dir(b.Path), filepath.Dir(filepath.Dir(b.Path))} {
				if err := os.Remove(dir); err != nil {
					t.Fatal(err)
				}
			}
			if mode != "protected-real" {
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
				if mode != "missing-junction" {
					junction(other)
				}
			} else {
				f.svc.Guard.System = append(f.svc.Guard.System, real)
			}
			f.svc.FS = observeRestoreCreationFS{FS: f.fs, observe: func(path string) {
				if exists(path) {
					t.Errorf("refused resurrection created a directory: %s", path)
				}
			}}
			got, err := f.svc.Resurrect([]string{b.Path})
			if err != nil || !core.GraveFailed(got) || !exists(b.Grave) || len(f.burials()) != 1 {
				t.Fatalf("unsafe restore was not refused: %+v %v", got, err)
			}
			for _, dir := range []string{filepath.Join(link, "deep"), filepath.Join(real, "deep"), filepath.Join(other, "deep")} {
				if exists(dir) {
					t.Fatalf("refused resurrection created a directory: %s", dir)
				}
			}
			if mode == "missing-junction" && exists(link) {
				t.Fatal("refused resurrection recreated a missing junction")
			}
		})
	}
}

func TestResurrectRecreatesMissingParents(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "output/deep/payload.txt")
	if err := os.Remove(filepath.Dir(b.Path)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(filepath.Dir(b.Path))); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || core.GraveFailed(got) || read(t, b.Path) != "output/deep/payload.txt" {
		t.Fatalf("missing parents not recreated: %+v %v", got, err)
	}
}

func TestResurrectRetriesAfterRecreatingParents(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "output/payload.txt")
	if err := os.Remove(filepath.Dir(b.Path)); err != nil {
		t.Fatal(err)
	}
	f.fs.fail = errors.New("file temporarily busy")
	first, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || !core.GraveFailed(first) || !exists(b.Grave) {
		t.Fatalf("failed restoration lost grave: %+v %v", first, err)
	}
	f.fs.fail = nil
	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || core.GraveFailed(got) || !exists(b.Path) {
		t.Fatalf("recreated parents blocked retry: %+v %v", got, err)
	}
}

func TestResurrectReportsRecreatedParentIdentityFailure(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "output/deep/payload.txt")
	nearest := filepath.Dir(b.Path)
	parent := filepath.Dir(nearest)
	for _, dir := range []string{nearest, parent} {
		if err := os.Remove(dir); err != nil {
			t.Fatal(err)
		}
	}
	injected := errors.New("recreated parent identity is temporarily unavailable")
	f.fs.identifyFollowingErr = func(path string) error {
		if path == parent {
			return injected
		}
		return nil
	}
	defer func() {
		if caught := recover(); caught != nil {
			t.Fatalf("parent identity failure panicked instead of reporting the ref: %v", caught)
		}
	}()
	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || len(got) != 1 || got[0].Ref != b.Path || got[0].Error != injected.Error() || got[0].Action != "" {
		t.Fatalf("parent identity failure did not reach its ref result: %+v %v", got, err)
	}
	if exists(nearest) || exists(parent) {
		t.Fatal("failed parent recheck left recreated directories")
	}
	if !exists(b.Grave) || exists(b.Path) || len(f.burials()) != 1 {
		t.Fatal("parent identity failure changed the buried object or its record")
	}
	f.fs.identifyFollowingErr = nil
	got, err = f.svc.Resurrect([]string{b.Path})
	if err != nil || core.GraveFailed(got) || !exists(b.Path) {
		t.Fatalf("recreated parent identity failure blocked retry: %+v %v", got, err)
	}
}

func TestResurrectSharedMissingParent(t *testing.T) {
	for _, batch := range []bool{true, false} {
		t.Run(fmt.Sprint(batch), func(t *testing.T) {
			f := newFixture(t)
			a := buryOne(t, f, "output/a.txt")
			b := buryOne(t, f, "output/b.txt")
			if err := os.Remove(filepath.Dir(a.Path)); err != nil {
				t.Fatal(err)
			}
			groups := [][]string{{a.Path, b.Path}}
			if !batch {
				groups = [][]string{{a.Path}, {b.Path}}
			}
			for _, refs := range groups {
				got, err := f.svc.Resurrect(refs)
				if err != nil || core.GraveFailed(got) {
					t.Fatalf("shared rebuilt parent blocked resurrection: %+v %v", got, err)
				}
			}
			if read(t, a.Path) != "output/a.txt" || read(t, b.Path) != "output/b.txt" || len(f.burials()) != 0 {
				t.Fatal("shared parent lost restored files")
			}
		})
	}
}

type beforeMkdirRestoreFS struct {
	core.FS
	before func()
}

func (fs beforeMkdirRestoreFS) MkdirRestoreParent(p core.RestoreParent, name string) (core.RestoreParent, error) {
	fs.before()
	return fs.FS.MkdirRestoreParent(p, name)
}

func TestResurrectRollsBackDirectoriesAfterAncestorMove(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "output/deep/nested/payload.txt")
			parent := filepath.Join(f.dir, "output")
			protected := filepath.Join(f.dir, "protected")
			elsewhere := filepath.Join(f.dir, "elsewhere")
			for _, p := range []string{filepath.Dir(b.Path), filepath.Dir(filepath.Dir(b.Path))} {
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(elsewhere, 0700); err != nil {
				t.Fatal(err)
			}
			f.svc.Guard.System = append(f.svc.Guard.System, protected)
			moved := false
			f.svc.FS = beforeMkdirRestoreFS{FS: f.fs, before: func() {
				if moved {
					return
				}
				moved = true
				if err := os.Rename(parent, protected); err != nil {
					t.Fatal(err)
				}
				if out, err := exec.Command("cmd", "/c", "mklink", "/J", parent, elsewhere).CombinedOutput(); err != nil {
					t.Fatalf("junction: %v %s", err, out)
				}
				if existing {
					if err := os.Mkdir(filepath.Join(protected, "deep"), 0700); err != nil {
						t.Fatal(err)
					}
				}
			}}
			got, err := f.svc.Resurrect([]string{b.Path})
			if !moved || err != nil || !core.GraveFailed(got) || !exists(b.Grave) || len(f.burials()) != 1 {
				t.Fatalf("ancestor race not refused: %+v %v", got, err)
			}
			if exists(filepath.Join(protected, "deep", "nested")) || exists(filepath.Join(elsewhere, "deep")) || !existing && exists(filepath.Join(protected, "deep")) {
				t.Fatal("refused resurrection left created directories")
			}
			if existing && !exists(filepath.Join(protected, "deep")) {
				t.Fatal("rollback deleted concurrently created directory")
			}
		})
	}
}

type observeRestoreCreationFS struct {
	core.FS
	observe func(string)
}

func (fs observeRestoreCreationFS) MkdirRestoreParent(p core.RestoreParent, name string) (core.RestoreParent, error) {
	child, err := fs.FS.MkdirRestoreParent(p, name)
	if err == nil {
		fs.observe(child.Path())
	}
	return child, err
}
