//go:build windows

package lifecycle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestRegistrationScopeSelectsPossibleConflicts(t *testing.T) {
	f := newFixture(t)
	path := f.file("fresh.log")
	id, err := f.fs.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	other := f.file("other.txt")
	otherID, err := f.fs.Identify(other)
	if err != nil {
		t.Fatal(err)
	}
	parentID, err := f.fs.Identify(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	scope := f.svc.RegistrationScope(path, id)
	if scope == nil {
		t.Fatal("ordinary local file did not get an ancestry proof")
	}
	defer scope.Close()
	partial := otherID
	partial.FileID = "incomplete"
	remote := otherID
	remote.Volume = `\\server\share\`
	for _, tc := range []struct {
		name string
		item core.Item
		want bool
	}{
		{"unrelated", core.Item{Path: other, Identity: &otherID}, false},
		{"same-identity-old-name", core.Item{Path: other, Identity: &id}, true},
		{"moved-parent", core.Item{Path: other, Identity: &parentID}, true},
		{"same-name-different-object", core.Item{Path: path, Identity: &otherID}, true},
		{"case-alias", core.Item{Path: strings.ToUpper(path), Identity: &otherID}, true},
		{"path-parent", core.Item{Path: filepath.Dir(path), Identity: &otherID}, true},
		{"path-child", core.Item{Path: filepath.Join(path, "child"), Identity: &otherID}, true},
		{"legacy", core.Item{Path: other}, true},
		{"partial", core.Item{Path: other, Identity: &partial}, true},
		{"remote-volume", core.Item{Path: other, Identity: &remote}, true},
		{"short-name", core.Item{Path: filepath.Join(f.dir, "OTHER~1.TXT"), Identity: &otherID}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := scope.Related(tc.item); got != tc.want {
				t.Fatalf("candidate relevance=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestRegistrationScopeKeepsFullLookupForUnprovenFiles(t *testing.T) {
	f := newFixture(t)
	path := f.file("file.log")
	id, err := f.fs.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		path string
		id   core.Identity
	}{
		{"missing", path + ".missing", id},
		{"wrong-identity", f.file("replacement.log"), id},
		{"legacy", path, core.Identity{}},
		{"directory", filepath.Dir(path), id},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scope := f.svc.RegistrationScope(tc.path, tc.id)
			defer scope.Close()
			if scope != nil || !scope.Related(core.Item{Identity: &id}) {
				t.Fatal("unverifiable ancestry excluded a recorded object")
			}
		})
	}
	if err := os.Link(path, path+".alias"); err != nil {
		t.Fatal(err)
	}
	scope := f.svc.RegistrationScope(path, id)
	defer scope.Close()
	if scope != nil {
		t.Fatal("hard-linked candidate obtained a registration proof")
	}
	// A provider with no proof capability must retain the original path.
	svc := &core.Service{FS: struct{ core.FS }{winfs.FS{}}}
	if scope := svc.RegistrationScope(path, id); scope != nil {
		scope.Close()
		t.Fatal("unsupported filesystem obtained a registration proof")
	}
}
