//go:build windows

package core_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestGraveyardDirectoryBoundaries(t *testing.T) {
	f := newFixture(t)
	var paths []string
	for _, name := range []string{"a/one", "a/sub/two", "ab/neighbor", "K/latin", "K/kelvin", "Long directory name/leaf"} {
		paths = append(paths, f.file(name))
	}
	rows, err := f.svc.Bury(paths, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatal(rows, err)
	}
	for _, tc := range []struct {
		name, directory string
		want            []int64
	}{
		{"descendants and prefix boundary", filepath.Join(f.dir, "a"), []int64{rows[0].Burial.ID, rows[1].Burial.ID}},
		{"ASCII case", strings.ToUpper(filepath.Join(f.dir, "a")), []int64{rows[0].Burial.ID, rows[1].Burial.ID}},
		{"nested", filepath.Join(f.dir, "a", "sub"), []int64{rows[1].Burial.ID}},
		{"Unicode is not folded", filepath.Join(f.dir, "K"), []int64{rows[3].Burial.ID}},
		{"Kelvin stays separate", filepath.Join(f.dir, "K"), []int64{rows[4].Burial.ID}},
		{"no matches", filepath.Join(f.dir, "absent"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view, err := f.svc.ReadGraveyard()
			if err != nil {
				t.Fatal(err)
			}
			if err := f.svc.FilterGraveyard(view, tc.directory); err != nil {
				t.Fatal(err)
			}
			var ids []int64
			for _, b := range view.Burials {
				ids = append(ids, b.ID)
			}
			if !reflect.DeepEqual(ids, tc.want) {
				t.Fatalf("wrong directory filter: %v, want %v", ids, tc.want)
			}
		})
	}
	t.Run("short name", func(t *testing.T) {
		long := filepath.Join(f.dir, "Long directory name")
		p, err := windows.UTF16PtrFromString(long)
		if err != nil {
			t.Fatal(err)
		}
		buf := make([]uint16, 32768)
		n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
		if err != nil {
			t.Fatal(err)
		}
		short := windows.UTF16ToString(buf[:n])
		if !strings.Contains(short, "~") {
			t.Skip("volume has no short-name aliases")
		}
		view, err := f.svc.ReadGraveyard()
		if err != nil {
			t.Fatal(err)
		}
		if err := f.svc.FilterGraveyard(view, short); err != nil || len(view.Burials) != 1 || view.Burials[0].ID != rows[5].Burial.ID {
			t.Fatalf("short directory lost burial: %+v %v", view, err)
		}
	})
}

func TestGraveyardFilterIncludesPendingAndRecoveredOriginalPaths(t *testing.T) {
	f := newFixture(t)
	inside := core.Burial{ID: 1, Path: filepath.Join(f.dir, "a", "child"), LongNames: true}
	outside := core.Burial{ID: 2, Path: filepath.Join(f.dir, "ab", "child"), LongNames: true}
	view := &core.GraveyardView{
		Burials:  []core.Burial{inside, outside},
		Pending:  []core.PendingGrave{{Burial: &inside}, {Burial: &outside}, {Error: "unlocated"}},
		Restored: []core.GraveResult{{Burial: &inside, Path: outside.Path}, {Burial: &outside, Path: inside.Path}},
	}
	if err := f.svc.FilterGraveyard(view, filepath.Join(f.dir, "a")); err != nil {
		t.Fatal(err)
	}
	if len(view.Burials) != 1 || len(view.Pending) != 1 || len(view.Restored) != 1 || view.Restored[0].Burial.ID != 1 {
		t.Fatalf("filter leaked unrelated originals or used restored paths: %+v", view)
	}
}

func TestGraveyardFilterPrefersExactDirectorySpelling(t *testing.T) {
	f := newFixture(t)
	makeView := func() *core.GraveyardView {
		return &core.GraveyardView{Burials: []core.Burial{
			{ID: 1, Path: filepath.Join(f.dir, "Case", "one"), LongNames: true},
			{ID: 2, Path: filepath.Join(f.dir, "case", "two"), LongNames: true},
		}}
	}
	view := makeView()
	if err := f.svc.FilterGraveyard(view, filepath.Join(f.dir, "Case")); err != nil || len(view.Burials) != 1 || view.Burials[0].ID != 1 {
		t.Fatalf("exact directory did not win: %+v %v", view, err)
	}
	view = makeView()
	if err := f.svc.FilterGraveyard(view, filepath.Join(f.dir, "CASE")); err == nil {
		t.Fatal("ambiguous folded directory accepted")
	}
}
