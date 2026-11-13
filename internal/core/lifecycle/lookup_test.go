package lifecycle_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestDirectoryReferenceSkipsNewerFileBurial(t *testing.T) {
	for _, extended := range []bool{false, true} {
		for _, operation := range []string{"purge", "resurrect"} {
			t.Run(fmt.Sprintf("%v/%s", extended, operation), func(t *testing.T) {
				if extended && runtime.GOOS != "windows" {
					t.Skip("extended path syntax is Windows-only")
				}
				f := newFixture(t)
				f.file("build/payload")
				path := filepath.Join(f.dir, "build")
				rows, err := f.svc.Bury([]string{path}, false, "")
				if err != nil || core.GraveFailed(rows) {
					t.Fatal(rows, err)
				}
				directory := *rows[0].Burial
				f.clock = f.clock.Add(time.Minute)
				file := buryOne(t, f, "build")
				ref := path + string(os.PathSeparator)
				if extended {
					ref = `\\?\` + ref
				}
				if operation == "purge" {
					rows, err = f.svc.PurgeRefs([]string{ref}, false)
				} else {
					rows, err = f.svc.Resurrect([]string{ref})
				}
				if err != nil || core.GraveFailed(rows) || rows[0].Burial == nil || rows[0].Burial.ID != directory.ID || !exists(file.Grave) {
					t.Fatalf("directory reference selected newer file burial: %+v %v", rows, err)
				}
				st := core.State{Items: []core.Item{{ID: 1, Path: path}, {ID: 2, Path: path, IsDir: true}}}
				if idx, err := st.FindPath(ref); err != nil || idx != 1 {
					t.Fatalf("directory entry reference lost its type constraint: %d %v", idx, err)
				}
				if operation == "resurrect" {
					if info, err := os.Stat(path); err != nil || !info.IsDir() {
						t.Fatal("restored directory is missing", err)
					}
				}
			})
		}
	}
}

func TestGraveNumericShapeNeverFallsBackToFilename(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	refs := []string{"9223372036854775807", "9223372036854775808", "#99999999999999999999", "-9223372036854775809", "007", "+5", "0", "-1", " 9001", "9001 "}
	for _, command := range []string{"purge", "resurrect"} {
		for _, ref := range refs {
			t.Run(command+"/"+ref, func(t *testing.T) {
				f := newFixture(t)
				b := buryOne(t, f, ref)
				t.Chdir(f.dir)
				var rows []core.GraveResult
				var err error
				if command == "purge" {
					rows, err = f.svc.PurgeRefs([]string{ref}, false)
				} else {
					rows, err = f.svc.Resurrect([]string{ref})
				}
				if err != nil || len(rows) != 1 || !core.GraveFailed(rows) || len(f.burials()) != 1 || !exists(b.Grave) {
					t.Fatalf("numeric-shaped reference selected a filename: %+v %v", rows, err)
				}
				if !strings.Contains(rows[0].Error, "filename use ./") {
					t.Fatalf("missing filename advice: %+v", rows)
				}
			})
		}
	}
	for _, ref := range []string{"007", "+7"} {
		st := core.State{Burials: []core.Burial{{ID: 7}}}
		if idx, err := st.FindBurial(ref); err != nil || idx != 0 {
			t.Fatalf("numeric spelling changed: %s %d %v", ref, idx, err)
		}
	}
}

func TestGraveExtendedSpellingMatchesOrdinaryPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("extended path syntax is Windows-only")
	}
	for _, command := range []string{"purge", "resurrect"} {
		for _, storedExtended := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", command, storedExtended), func(t *testing.T) {
				f := newFixture(t)
				b := buryOne(t, f, "payload.txt")
				ref := `\\?\` + b.Path
				if storedExtended {
					if err := f.svc.Store.Update(func(st *core.State) error {
						st.Burials[0].Path = ref
						for i := range st.Burials[0].Parents {
							st.Burials[0].Parents[i].Path = `\\?\` + st.Burials[0].Parents[i].Path
						}
						return nil
					}); err != nil {
						t.Fatal(err)
					}
					ref = b.Path
				}
				var rows []core.GraveResult
				var err error
				if command == "purge" {
					rows, err = f.svc.PurgeRefs([]string{ref}, false)
				} else {
					rows, err = f.svc.Resurrect([]string{ref})
				}
				if err != nil || core.GraveFailed(rows) || len(f.burials()) != 0 {
					t.Fatalf("equivalent extended path did not select burial: %+v %v", rows, err)
				}
			})
		}
	}
}

func TestGraveNumericReferenceDoesNotSelectFilename(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, command := range []string{"purge", "resurrect"} {
		for _, ref := range []string{"9001", "#9001", "./9001", "absolute"} {
			t.Run(command+"/"+ref, func(t *testing.T) {
				f := newFixture(t)
				b := buryOne(t, f, "9001")
				t.Chdir(f.dir)
				if ref == "absolute" {
					ref = b.Path
				}
				var rows []core.GraveResult
				var err error
				if command == "purge" {
					rows, err = f.svc.PurgeRefs([]string{ref}, false)
				} else {
					rows, err = f.svc.Resurrect([]string{ref})
				}
				if err != nil || len(rows) != 1 {
					t.Fatal(rows, err)
				}
				if ref == "9001" || ref == "#9001" {
					if !core.GraveFailed(rows) || len(f.burials()) != 1 || !exists(b.Grave) || !strings.Contains(rows[0].Error, "./9001") {
						t.Fatalf("numeric burial reference selected a filename or omitted path advice: %+v", rows)
					}
				} else if core.GraveFailed(rows) || len(f.burials()) != 0 {
					t.Fatalf("explicit numeric filename failed: %+v", rows)
				}
			})
		}
	}
}

func TestResurrectAmbiguousASCIIRequiresID(t *testing.T) {
	f := newFixture(t)
	for range 2 {
		buryOne(t, f, "case.txt")
	}
	got, err := f.svc.Resurrect([]string{strings.ToUpper(filepath.Join(f.dir, "case.txt"))})
	if err != nil || !core.GraveFailed(got) || !strings.Contains(got[0].Error, "use an ID") || len(f.burials()) != 2 {
		t.Fatalf("ambiguous burial path accepted: %+v %v", got, err)
	}
}
