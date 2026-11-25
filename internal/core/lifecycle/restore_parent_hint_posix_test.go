//go:build linux || darwin

package lifecycle_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestResurrectReportsRenamedParentPath(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "alpha-src/payload.txt")
	oldParent := filepath.Dir(b.Path)
	currentParent := filepath.Join(f.dir, "omega-dst")
	if err := os.Rename(oldParent, currentParent); err != nil {
		t.Fatal(err)
	}

	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || len(got) != 1 || !core.GraveFailed(got) || !exists(b.Grave) || exists(b.Path) {
		t.Fatalf("renamed parent was not retained: %+v %v", got, err)
	}
	if !strings.Contains(got[0].Error, oldParent+" is currently at "+currentParent) || exists(filepath.Join(currentParent, "payload.txt")) || read(t, b.Grave) != "alpha-src/payload.txt" || len(f.burials()) != 1 {
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
	b := buryOne(t, f, "alpha-src/deep/nested/payload.txt")
	oldRoot := filepath.Join(f.dir, "alpha-src")
	currentRoot := filepath.Join(f.dir, "omega-dst")
	if err := os.Rename(oldRoot, currentRoot); err != nil {
		t.Fatal(err)
	}
	currentParent := filepath.Join(currentRoot, "deep", "nested")

	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || len(got) != 1 || !core.GraveFailed(got) || !exists(b.Grave) || exists(b.Path) {
		t.Fatalf("nested renamed parent was not retained: %+v %v", got, err)
	}
	if !strings.Contains(got[0].Error, filepath.Dir(b.Path)+" is currently at "+currentParent) || exists(oldRoot) || exists(filepath.Join(currentParent, "payload.txt")) || read(t, b.Grave) != "alpha-src/deep/nested/payload.txt" || len(f.burials()) != 1 {
		t.Fatalf("nested renamed parent hint missing or old path rebuilt: %s", got[0].Error)
	}
}

func TestRestoreParentSiblingHints(t *testing.T) {
	for _, tc := range []struct {
		old, current string
		occupied     bool
	}{{"alpha-src", "Z-renamed", true}, {"Folder", "folder", false}} {
		t.Run(tc.current, func(t *testing.T) {
			f := newFixture(t)
			name := tc.old + "/payload"
			b := buryOne(t, f, name)
			old, current := filepath.Dir(b.Path), filepath.Join(f.dir, tc.current)
			if err := os.Rename(old, current); err != nil {
				t.Fatal(err)
			}
			if tc.occupied {
				f.file(tc.old + "/sentinel")
			}
			rows, err := f.svc.Resurrect([]string{b.Path})
			if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Error, old+" is currently at "+current) || read(t, b.Grave) != name || exists(filepath.Join(current, "payload")) || len(f.burials()) != 1 {
				t.Fatalf("sibling lookup: %+v %v", rows, err)
			}
			if tc.occupied && (read(t, filepath.Join(old, "sentinel")) != tc.old+"/sentinel" || exists(b.Path)) {
				t.Fatal("changed occupied original path")
			}
		})
	}
}

func TestRestoreParentLinkHint(t *testing.T) {
	f := newFixture(t)
	alias, real := filepath.Join(f.dir, "Alias"), filepath.Join(f.dir, "Real")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	b := buryOne(t, f, "Alias/payload")
	if err := os.Rename(alias, filepath.Join(f.dir, "RenamedAlias")); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Error, "link at "+alias) || strings.Contains(rows[0].Error, real) || read(t, b.Grave) != "Alias/payload" || exists(filepath.Join(real, "payload")) {
		t.Fatalf("link location misreported: %+v %v", rows, err)
	}
}

func TestRestoreParentEvidenceRequired(t *testing.T) {
	for _, mode := range []string{"weak", "missing-chain"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "alpha-src/payload")
			dir := filepath.Dir(b.Path)
			if mode == "missing-chain" {
				if err := os.Rename(dir, filepath.Join(f.dir, "omega-dst")); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.svc.Store.Update(func(s *core.State) error {
				p := s.Burials[0].Parents
				if mode == "weak" {
					weak := p[0].Identity
					weak.FileID = "0000000000000000" + weak.FileID[16:]
					p[0].Identity = weak
					f.svc.FS = weakRestoreFS{testFS: f.fs, path: dir, id: weak}
				} else {
					s.Burials[0].Parents = nil
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			rows, err := f.svc.Resurrect([]string{b.Path})
			if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Error, "cannot be verified") || read(t, b.Grave) != "alpha-src/payload" || exists(b.Path) || mode == "missing-chain" && exists(dir) {
				t.Fatalf("unsafe evidence accepted: %+v %v", rows, err)
			}
		})
	}
}

type weakRestoreFS struct {
	*testFS
	path string
	id   core.Identity
}

func (f weakRestoreFS) IdentifyFollowing(path string) (core.Identity, error) {
	if path == f.path {
		return f.id, nil
	}
	return f.testFS.IdentifyFollowing(path)
}

type lockCheckingHintFS struct {
	*testFS
	check func()
}

func (f lockCheckingHintFS) FindRestoreDirectory(p core.RestoreParent, id core.Identity) string {
	f.check()
	return f.testFS.FindRestoreDirectory(p, id)
}

func TestRestoreParentHintBudgetOutsideGraveyardLock(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "alpha-src/payload")
	old := filepath.Dir(b.Path)
	for i := range 2048 {
		f.file(fmt.Sprintf("A-unrelated/file-%04d", i))
	}
	// The old parent is below the search depth even if the budget is relaxed.
	deep := filepath.Join(f.dir, "other", "deeper", "omega-dst")
	if err := os.MkdirAll(filepath.Dir(deep), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(old, deep); err != nil {
		t.Fatal(err)
	}
	called := false
	f.svc.FS = lockCheckingHintFS{f.fs, func() {
		called = true
		unlock, ok, err := core.TryLockExclusive(filepath.Join(f.svc.Store.Home, "graveyard.lock"))
		if err != nil || !ok {
			t.Fatalf("hint holds the graveyard lock: %v", err)
		}
		unlock()
	}}
	rows, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || len(rows) != 1 || !called || !strings.Contains(rows[0].Error, "cannot be verified") || strings.Contains(rows[0].Error, deep) || read(t, b.Grave) != "alpha-src/payload" {
		t.Fatalf("unbounded or uncertain lookup: %+v %v", rows, err)
	}
}

func TestRestoreParentPreparationReasons(t *testing.T) {
	for _, mode := range []string{"permission", "missing"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "alpha-src/nested/payload")
			dir := filepath.Join(f.dir, "alpha-src")
			if mode == "permission" {
				if os.Geteuid() == 0 {
					t.Skip("root bypasses permissions")
				}
				if err := os.Chmod(dir, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.Chmod(dir, 0700) })
			} else {
				if err := os.Rename(dir, filepath.Join(f.dir, "omega-dst")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("missing-target", dir); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := f.svc.Resurrect([]string{b.Path})
			if err != nil || len(rows) != 1 {
				t.Fatal(rows, err)
			}
			issue, ok := errors.AsType[*corefs.RestoreParentError](rows[0].Err)
			if !ok || issue.Kind != mode || !strings.Contains(rows[0].Error, "graveyard") || read(t, b.Grave) != "alpha-src/nested/payload" {
				t.Fatalf("lost preparation reason: %+v", rows)
			}
		})
	}
}

func TestRestoreHintSkipsUnreadableSiblingAtDepthTwo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	f := newFixture(t)
	b := buryOne(t, f, "siblings/Old/payload")
	old := filepath.Dir(b.Path)
	root := filepath.Dir(old)
	parked := filepath.Join(f.dir, "parked")
	if err := os.Rename(old, parked); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one", "two", "three"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	dir, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	if err != nil || len(names) != 3 {
		t.Fatal(names, err)
	}
	first := filepath.Join(root, names[0])
	if err := os.Chmod(first, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(first, 0700) })
	current := filepath.Join(root, names[len(names)-1], "New")
	if err := os.Rename(parked, current); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Error, old+" is currently at "+current) || read(t, b.Grave) != "siblings/Old/payload" || exists(filepath.Join(current, "payload")) || len(f.burials()) != 1 {
		t.Fatal(rows, err)
	}
}

func TestRestoreFinalCheckRequiresRecordedLinkType(t *testing.T) {
	f := newFixture(t)
	real := filepath.Join(f.dir, "Real")
	alias := filepath.Join(f.dir, "Alias")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	directoryLink(t, alias, real)
	b := buryOne(t, f, "Alias/child/payload")
	chain, err := core.PrepareRestoreParent(f.svc, b)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()
	// Model the two snapshots deterministically: the only difference is the recorded link bit.
	if b.Parents[1].Path != alias || !b.Parents[1].Reparse {
		t.Fatal("link fixture was not recorded")
	}
	b.Parents[1].Reparse = false
	_, err = f.fs.MovePinnedInto(b.Grave, *b.Identity, chain.RestoreParent, "payload")
	issue, ok := errors.AsType[*corefs.RestoreParentError](err)
	if !ok || issue.Kind != "link" || issue.Path != alias || !errors.Is(err, core.ErrUnsafe) || read(t, b.Grave) != "Alias/child/payload" || exists(b.Path) || len(f.burials()) != 1 {
		t.Fatalf("ordinary ancestor accepted as a same-target link: %v", err)
	}
}
