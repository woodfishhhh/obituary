//go:build linux || darwin

package lifecycle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestResurrectReportsRenamedParentPath(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "output/payload.txt")
	oldParent := filepath.Dir(b.Path)
	currentParent := oldParent + "-renamed"
	if err := os.Rename(oldParent, currentParent); err != nil {
		t.Fatal(err)
	}

	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || len(got) != 1 || !core.GraveFailed(got) || !exists(b.Grave) || exists(b.Path) {
		t.Fatalf("renamed parent was not retained: %+v %v", got, err)
	}
	if !strings.Contains(got[0].Error, oldParent) || !strings.Contains(got[0].Error, currentParent) {
		t.Fatalf("renamed parent hint missing verified paths: %s", got[0].Error)
	}
}

func TestResurrectRejectsReplacedParent(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "output/payload.txt")
	parent := filepath.Dir(b.Path)
	if err := os.Remove(parent); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || len(got) != 1 || !core.GraveFailed(got) || !exists(b.Grave) || exists(b.Path) {
		t.Fatalf("replaced parent was accepted: %+v %v", got, err)
	}
	if !strings.Contains(got[0].Error, "changed or cannot be verified") || !strings.Contains(got[0].Error, "graveyard") {
		t.Fatalf("replaced parent error is unclear: %s", got[0].Error)
	}
}

func TestResurrectRejectsDeletedParentWithoutGuessing(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "output/deep/payload.txt")
	for _, dir := range []string{filepath.Dir(b.Path), filepath.Dir(filepath.Dir(b.Path))} {
		if err := os.Remove(dir); err != nil {
			t.Fatal(err)
		}
	}

	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || len(got) != 1 || !core.GraveFailed(got) || !exists(b.Grave) || exists(b.Path) {
		t.Fatalf("deleted parent was not retained: %+v %v", got, err)
	}
	if !strings.Contains(got[0].Error, "cannot be verified") || !strings.Contains(got[0].Error, "graveyard") {
		t.Fatalf("deleted parent error is unclear: %s", got[0].Error)
	}
}

func TestResurrectReportsNestedRenamedParentPath(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "output/deep/nested/payload.txt")
	oldRoot := filepath.Join(f.dir, "output")
	currentRoot := oldRoot + "-renamed"
	if err := os.Rename(oldRoot, currentRoot); err != nil {
		t.Fatal(err)
	}
	currentParent := filepath.Join(currentRoot, "deep", "nested")

	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || len(got) != 1 || !core.GraveFailed(got) || !exists(b.Grave) || exists(b.Path) {
		t.Fatalf("nested renamed parent was not retained: %+v %v", got, err)
	}
	if !strings.Contains(got[0].Error, currentParent) || exists(oldRoot) {
		t.Fatalf("nested renamed parent hint missing or old path rebuilt: %s", got[0].Error)
	}
}
