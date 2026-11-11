//go:build windows

package core_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestAddApprovalCheckPreventsCommit(t *testing.T) {
	f := newFixture(t)
	path := f.file("candidate.log")
	changed := errors.New("rules changed")
	checked := false
	out, err := f.svc.Add([]string{path}, core.AddOptions{
		ExpiresAt: f.clock.Add(time.Hour), OnlyNew: true,
		BeforeCommit: func() error { checked = true; return changed },
	})
	if !checked || !errors.Is(err, changed) || len(out) != 0 || len(f.items()) != 0 || !exists(path) {
		t.Fatalf("approval check allowed registration: %+v %v checked=%v", out, err, checked)
	}
}

type registrationIdentifyFailureFS struct {
	core.FS
	path  string
	calls int
	fault error
}

func (f *registrationIdentifyFailureFS) Identify(path string) (core.Identity, error) {
	if samePath(path, f.path) {
		f.calls++
		if f.calls > 1 {
			return core.Identity{}, f.fault
		}
	}
	return f.FS.Identify(path)
}

func TestAddObservationRejectsChangedObjects(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		for _, changed := range []string{"observed", "unit", "lookup"} {
			t.Run(fmt.Sprintf("automatic=%v/%s", automatic, changed), func(t *testing.T) {
				if automatic && testing.Short() {
					t.Skip("automatic registration uses a real Git repository")
				}
				f := newFixture(t)
				if automatic {
					if out, err := exec.Command("git", "init", f.dir).CombinedOutput(); err != nil {
						t.Fatalf("git init: %v %s", err, out)
					}
				}
				observed, unit := f.file("write.log"), f.file("unit.log")
				captured, err := f.svc.CaptureObservations([]string{observed}, "write")
				if err != nil || len(captured) != 1 {
					t.Fatalf("capture: %+v %v", captured, err)
				}
				want := error(core.ErrUnsafe)
				swap := func(path string) {
					if err := os.Rename(path, path+".saved"); err != nil {
						t.Fatal(err)
					}
					f.file(filepath.Base(path))
				}
				switch changed {
				case "observed":
					swap(observed)
				case "unit":
					once := false
					f.svc.FS = &identifyHookFS{testFS: f.fs, after: func(path string) {
						if !once && samePath(path, unit) {
							once = true
							swap(unit)
						}
					}}
				case "lookup":
					want = errors.New("registration identity unavailable")
					f.svc.FS = &registrationIdentifyFailureFS{FS: f.fs, path: unit, fault: want}
				}
				rows, err := f.svc.Add([]string{unit}, core.AddOptions{
					ExpiresAt: f.clock.Add(time.Hour), OnlyNew: automatic, Observed: &captured[0],
				})
				if err != nil || len(rows) != 1 || !core.Failed(rows) || !errors.Is(rows[0].Err, want) || len(f.items()) != 0 || read(t, unit) != "unit.log" {
					t.Fatalf("changed observation registered or lost cause: %+v %v", rows, err)
				}
			})
		}
	}
}

func TestAddBatchKeepsInspectionFailuresAndLaterSuccess(t *testing.T) {
	f := newFixture(t)
	missing, valid := filepath.Join(f.dir, "missing.txt"), f.file("valid.txt")
	rows, err := f.svc.Add([]string{missing, valid}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil || len(rows) != 2 || rows[0].Ref != missing || rows[0].Error == "" || !errors.Is(rows[0].Err, core.ErrNotFound) || rows[1].Action != core.ActionAdded || rows[1].Item == nil || len(f.items()) != 1 {
		t.Fatalf("registration batch lost failure or later success: %+v %v", rows, err)
	}
}

func TestExtendedPathNeverSelectsNormalizedNeighbor(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, spelling := range []string{"dot", "space", "parent"} {
		for _, operation := range []string{"purge", "resurrect", "add", "bury", "propose"} {
			t.Run(spelling+"/"+operation, func(t *testing.T) {
				f := newFixture(t)
				b := buryOne(t, f, "neighbor.txt")
				path := b.Path
				if operation != "purge" && operation != "resurrect" {
					path = f.file("neighbor.txt")
				}
				input := `\\?\` + path
				switch spelling {
				case "dot":
					input += "."
				case "space":
					input += " "
				case "parent":
					input = `\\?\` + filepath.Dir(path) + `\missing\..\neighbor.txt`
				}
				var rejected bool
				switch operation {
				case "purge":
					rows, err := f.svc.PurgeRefs([]string{input}, false)
					rejected = err != nil || core.GraveFailed(rows)
				case "resurrect":
					rows, err := f.svc.Resurrect([]string{input})
					rejected = err != nil || core.GraveFailed(rows)
				case "bury":
					rows, err := f.svc.Bury([]string{input}, false, "")
					rejected = err != nil || core.GraveFailed(rows)
				case "add":
					rows, err := f.svc.Add([]string{input}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
					rejected = err != nil || core.Failed(rows)
				case "propose":
					_, _, _, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "session", []core.Candidate{{Path: input}})
					rejected = err != nil
				}
				if !rejected || len(f.burials()) != 1 || !exists(b.Grave) || len(f.items()) != 0 {
					t.Fatalf("extended spelling selected normalized neighbor: rejected=%v", rejected)
				}
				if operation != "purge" && operation != "resurrect" && read(t, path) != "neighbor.txt" {
					t.Fatal("neighbor file changed")
				}
			})
		}
	}
}

func TestTrailingSeparatorSelectsOnlyDirectories(t *testing.T) {
	for _, extended := range []bool{false, true} {
		for _, operation := range []string{"purge", "resurrect", "add", "bury", "propose"} {
			t.Run(fmt.Sprintf("%v/%s", extended, operation), func(t *testing.T) {
				f := newFixture(t)
				b := buryOne(t, f, "neighbor.txt")
				if operation != "purge" && operation != "resurrect" {
					f.file("neighbor.txt")
				}
				input := b.Path + `\`
				if extended {
					input = `\\?\` + input
				}
				if !rejectPath(t, f, operation, input) || !exists(b.Grave) || len(f.items()) != 0 || len(f.burials()) != 1 {
					t.Fatal("directory-only spelling selected a file")
				}
			})
		}
	}
}

func TestDirectoryTrailingSeparatorWorks(t *testing.T) {
	for _, extended := range []bool{false, true} {
		t.Run(fmt.Sprint(extended), func(t *testing.T) {
			f := newFixture(t)
			path := filepath.Join(f.dir, "build")
			f.file("build/payload")
			input := path + `\`
			if extended {
				input = `\\?\` + input
			}
			f.add(input, time.Hour)
			rows, err := f.svc.Bury([]string{input}, false, "")
			if err != nil || core.GraveFailed(rows) || !rows[0].Burial.IsDir {
				t.Fatalf("directory spelling was refused: %+v %v", rows, err)
			}
			rows, err = f.svc.Resurrect([]string{input})
			if err != nil || core.GraveFailed(rows) || !exists(filepath.Join(path, "payload")) {
				t.Fatalf("directory spelling could not restore: %+v %v", rows, err)
			}
		})
	}
}

func TestShortAliasTrailingDotNeverSelectsNeighbor(t *testing.T) {
	for _, extended := range []bool{false, true} {
		for _, operation := range []string{"add", "bury", "purge", "resurrect"} {
			t.Run(fmt.Sprintf("%v/%s", extended, operation), func(t *testing.T) {
				f := newFixture(t)
				b := buryOne(t, f, "Long Document Name.txt")
				ordinary := f.file("Long Document Name.txt")
				dotted := `\\?\` + ordinary + "."
				if err := os.WriteFile(dotted, []byte("dotted"), 0600); err != nil {
					t.Fatal(err)
				}
				input := strings.TrimPrefix(shortPath(t, dotted), `\\?\`)
				if extended {
					input = `\\?\` + input
				}
				if !rejectPath(t, f, operation, input) || read(t, ordinary) != "Long Document Name.txt" || read(t, dotted) != "dotted" || !exists(b.Grave) || len(f.items()) != 0 {
					t.Fatal("short trailing-dot alias selected its neighbor")
				}
			})
		}
	}
}

func TestNamedStreamEntrypointsReject(t *testing.T) {
	for _, extended := range []bool{false, true} {
		for _, operation := range []string{"add", "bury", "propose", "purge", "resurrect"} {
			t.Run(fmt.Sprintf("%v/%s", extended, operation), func(t *testing.T) {
				f := newFixture(t)
				path := f.file("document.txt")
				input := path + ":review"
				if extended {
					input = `\\?\` + input
				}
				if err := os.WriteFile(input, []byte("review"), 0600); err != nil {
					t.Fatal(err)
				}
				if !rejectPath(t, f, operation, input) || len(f.items()) != 0 || len(f.burials()) != 0 || read(t, path) != "document.txt" {
					t.Fatal("named stream entered whole-object operations")
				}
			})
		}
	}
}

func TestLifespanUnconfirmedCommitExplainsPossibleEffect(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	core.RequireFsync(t)
	for _, command := range []string{"add", "mark", "spare", "extend"} {
		t.Run(command, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("payload.txt")
			it := f.add(path, time.Hour)
			failed := false
			stop := core.SetReplacementSync(func(file *os.File) error {
				if !failed {
					failed = true
					return errors.New("injected post-rename sync failure")
				}
				return file.Sync()
			})
			defer stop()
			var err error
			switch command {
			case "add", "mark":
				_, err = f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(2 * time.Hour)})
			case "spare":
				_, err = f.svc.Cancel([]string{fmt.Sprint(it.ID)})
			case "extend":
				_, err = f.svc.Extend([]string{fmt.Sprint(it.ID)}, time.Hour)
			}
			if err == nil || !strings.Contains(err.Error(), "may already have taken effect") || !strings.Contains(err.Error(), "obit list before retrying") {
				t.Fatalf("unconfirmed lifespan commit lacks retry advice: %v", err)
			}
			st, readErr := f.svc.Store.ReadSnapshot()
			if readErr != nil || command == "spare" && len(st.Items) != 0 || command != "spare" && (len(st.Items) != 1 || !st.Items[0].ExpiresAt.Equal(f.clock.Add(2*time.Hour))) {
				t.Fatalf("fixture did not expose possibly applied mutation: %+v %v", st, readErr)
			}
		})
	}
}

func TestTrailingDotBoundaryIdentity(t *testing.T) {
	for _, isDir := range []bool{false, true} {
		for _, operation := range []string{"add", "bury"} {
			t.Run(fmt.Sprintf("%v/%s", isDir, operation), func(t *testing.T) {
				f := newFixture(t)
				// A normalization mutant may retain the extended namespace. Keep
				// Home in that same spelling so even a failed safety test stays
				// inside its own temporary directory instead of a volume graveyard.
				f.svc.Store.Home = `\\?\` + f.svc.Store.Home
				ordinary := filepath.Join(f.dir, "logs")
				selected := `\\?\` + ordinary + "."
				for _, path := range []string{ordinary, selected} {
					if isDir {
						if err := os.Mkdir(path, 0700); err != nil {
							t.Fatal(err)
						}
					} else if err := os.WriteFile(path, []byte(path), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if operation == "add" {
					rows, err := f.svc.Add([]string{selected}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
					if err != nil || !core.Failed(rows) || len(f.items()) != 0 {
						t.Fatalf("trailing-dot input selected another object: %+v %v", rows, err)
					}
				} else {
					rows, err := f.svc.Bury([]string{selected}, false, "")
					if err != nil || !core.GraveFailed(rows) || len(f.burials()) != 0 {
						t.Fatalf("trailing-dot input selected another object: %+v %v", rows, err)
					}
				}
				if !exists(ordinary) || !exists(selected) {
					t.Fatal("trailing-dot input moved an unselected object")
				}
			})
		}
	}
}

func TestNormalizeSession(t *testing.T) {
	for _, session := range []string{"", "short", strings.Repeat("s", 4096)} {
		if got := core.NormalizeSession(session); got != session {
			t.Fatal("bounded session changed")
		}
	}
	long := strings.Repeat("s", 4097)
	got := core.NormalizeSession(long)
	if len(got) != 71 || !strings.HasPrefix(got, "sha256:") || core.NormalizeSession(long) != got || core.NormalizeSession(got) != got || core.NormalizeSession(long+"s") == got {
		t.Fatal("long session is not stable and bounded")
	}
}

func TestAddRejectsOversizedMetadata(t *testing.T) {
	f := newFixture(t)
	path := f.file("note.txt")
	before, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"note", "session", "source"} {
		opts := core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)}
		switch field {
		case "note":
			opts.Note = strings.Repeat("n", 4097)
		case "session":
			opts.Session = strings.Repeat("s", 4097)
		case "source":
			opts.Source = strings.Repeat("s", 4097)
		}
		if _, err := f.svc.Add([]string{path}, opts); err == nil {
			t.Fatalf("oversized metadata accepted: %s", field)
		}
		after, err := f.svc.Store.Read()
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("rejected metadata changed ledger: %v", err)
		}
	}
	got, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Note: strings.Repeat("n", 4096)})
	if err != nil || core.Failed(got) {
		t.Fatalf("boundary note rejected: %v", err)
	}
}

func TestAddAuditsAfterCancellation(t *testing.T) {
	f := newFixture(t)
	path := f.file("mark.log")
	assertCommittedAudit(t, f, core.ActionAdded, func(ctx context.Context) error {
		_, err := f.svc.WithContext(ctx).Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
		return err
	}, func(st core.State) bool { return len(st.Items) == 1 })
}

func TestGraveyardsCannotBeRegistered(t *testing.T) {
	f := newFixture(t)
	path := f.file("a.log")
	f.add(path, time.Hour)
	f.sweepAt(time.Hour)

	results, err := f.svc.Add([]string{f.burials()[0].Grave}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil || results[0].Error == "" {
		t.Fatalf("a grave must not be registered, got %+v %v", results, err)
	}
	var refused *core.RefusedError
	if err := (core.Guard{}).Check(`D:\.obituary\graveyard\1-x\a.log`, true); err == nil || !errorsAs(err, &refused) {
		t.Fatalf("another volume's graveyard must be refused, got %v", err)
	}
}

func TestAddReusesOnlyDefinitelyGonePath(t *testing.T) {
	for _, kind := range []string{"gone", "no-identity", "lookup-error", "unsafe"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("debug.log")
			old := f.add(path, 30*24*time.Hour)
			if err := os.Rename(path, filepath.Join(f.dir, "keep.txt")); err != nil {
				t.Fatal(err)
			}
			f.file("debug.log")
			switch kind {
			case "gone":
				f.fs.locateErr = core.ErrNotFound
			case "no-identity":
				if err := f.svc.Store.Update(func(st *core.State) error { st.Items[0].Identity = nil; return nil }); err != nil {
					t.Fatal(err)
				}
			case "lookup-error":
				f.fs.locateErr = errors.New("volume unavailable")
			case "unsafe":
				f.fs.locateErr = core.ErrUnsafe
			}
			before := f.items()
			f.clock = f.clock.Add(time.Minute)
			got, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", Project: f.dir, Session: "new"})
			if err != nil || len(got) != 1 {
				t.Fatalf("Add = %+v, %v", got, err)
			}
			items := f.items()
			if kind == "lookup-error" || kind == "unsafe" {
				if !core.Failed(got) || got[0].Err != nil || !reflect.DeepEqual(items, before) {
					t.Fatalf("uncertain lookup must preserve ledger: %+v, %+v", got, items)
				}
				return
			}
			if core.Failed(got) || len(items) != 1 || items[0].ID != old.ID || items[0].Source != "rule" || items[0].Project != f.dir || items[0].Session != "new" || !items[0].CreatedAt.Equal(f.clock) || items[0].Rev <= old.Rev {
				t.Fatalf("replacement must be a fresh registration: %+v, %+v", got, items)
			}
		})
	}
}

func TestAddResolvesAmbiguousLedgerPaths(t *testing.T) {
	for _, kind := range []string{"gone", "alive", "uncertain"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("debug.log")
			first := f.add(path, time.Hour)
			if err := os.Rename(path, path+".one"); err != nil {
				t.Fatal(err)
			}
			f.file("debug.log")
			second := f.add(path, time.Hour)
			if err := os.Rename(path, path+".two"); err != nil {
				t.Fatal(err)
			}
			if err := f.svc.Store.Update(func(st *core.State) error {
				st.Items[st.Index(first.ID)].Path = path
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "gone":
				for _, suffix := range []string{".one", ".two"} {
					if err := os.Remove(path + suffix); err != nil {
						t.Fatal(err)
					}
				}
			case "uncertain":
				f.fs.locateErr = errors.New("offline")
			}
			f.file("debug.log")
			got, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(2 * time.Hour)})
			if err != nil || len(got) != 1 {
				t.Fatalf("add: %+v, %v", got, err)
			}
			items := f.items()
			if kind == "uncertain" {
				if !core.Failed(got) || len(items) != 2 || !strings.Contains(got[0].Error, "obit list") || !strings.Contains(got[0].Error, "obit spare") || !strings.Contains(got[0].Error, fmt.Sprintf("#%d", second.ID)) || strings.Contains(got[0].Error, "use an ID") {
					t.Fatalf("ambiguity needs an executable hint and preserved records: %+v, %+v", got, items)
				}
				return
			}
			if core.Failed(got) {
				t.Fatalf("resolvable ambiguity: %+v", got)
			}
			if kind == "gone" && len(items) != 1 {
				t.Fatalf("gone entries not cleaned: %+v", items)
			}
			if kind == "alive" && (len(items) != 3 || !exists(path+".one") || !exists(path+".two")) {
				t.Fatalf("living entries lost: %+v", items)
			}
			if kind == "alive" {
				for i, old := range []core.Item{first, second} {
					want := path + []string{".one", ".two"}[i]
					for _, item := range items {
						if item.ID == old.ID && (item.Path != want || item.Rev <= old.Rev) {
							t.Fatalf("living entry location or revision lost: %+v; want %s", item, want)
						}
					}
				}
			}
		})
	}
}

func TestAddRejectsObjectBuriedAfterInspection(t *testing.T) {
	for _, child := range []bool{false, true} {
		for _, onlyNew := range []bool{false, true} {
			t.Run(fmt.Sprintf("child=%v/onlyNew=%v", child, onlyNew), func(t *testing.T) {
				f := newFixture(t)
				path := f.file("output/keep.txt")
				target := path
				if child {
					target = filepath.Dir(path)
				}
				original := f.add(target, time.Hour)
				burier := *f.svc
				once := false
				f.svc.FS = &identifyHookFS{testFS: f.fs, after: func(found string) {
					if !once && samePath(found, path) {
						once = true
						rows, err := burier.Bury([]string{target}, false, "")
						if err != nil || core.GraveFailed(rows) {
							t.Fatalf("concurrent burial: %+v, %v", rows, err)
						}
					}
				}}
				rows, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(30 * 24 * time.Hour), OnlyNew: onlyNew})
				if err != nil || !once || !core.Failed(rows) || !strings.Contains(rows[0].Error, fmt.Sprintf("obit resurrect %d", original.ID)) {
					t.Fatalf("Add accepted buried identity or hid resurrection advice: %+v, %v", rows, err)
				}
				if len(f.items()) != 0 || len(f.burials()) != 1 {
					t.Fatal("buried object acquired an independent lifespan")
				}
			})
		}
	}
}

func TestReAddingResetsExpiryInsteadOfDuplicating(t *testing.T) {
	f := newFixture(t)
	path := f.file("x.log")
	first := f.add(path, time.Hour)
	second := f.add(path, 3*time.Hour)

	items := f.items()
	if len(items) != 1 || second.ID != first.ID || !items[0].ExpiresAt.Equal(f.clock.Add(3*time.Hour)) {
		t.Fatalf("want one item expiring in 3h, got %+v", items)
	}
}

func TestAddRefusesProtectedAndMissingPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	f := newFixture(t)
	keep := filepath.Join(filepath.Dir(f.dir), "keep")
	if err := os.MkdirAll(filepath.Join(keep, "inner"), 0o700); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(f.dir, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(f.dir, "link")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, keep).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}

	results, err := f.svc.Add([]string{keep, filepath.Dir(keep), repo, junction, filepath.Join(f.dir, "missing")},
		core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Error == "" {
			t.Errorf("%s was accepted", r.Ref)
		}
	}
	if len(f.items()) != 0 {
		t.Fatal("refused paths were tracked")
	}

	forced, err := f.svc.Add([]string{repo}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Force: true})
	if err != nil || forced[0].Error != "" {
		t.Fatalf("--force should allow a repository folder: %v %+v", err, forced)
	}
	// Recycling the inner folder of a kept location is fine.
	if inner, _ := f.svc.Add([]string{filepath.Join(keep, "inner")}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)}); inner[0].Error != "" {
		t.Fatalf("inner folder refused: %s", inner[0].Error)
	}
}

func TestRenameThenReAddUpdatesTheSameRecord(t *testing.T) {
	f := newFixture(t)
	a := f.file("a.txt")
	first := f.add(a, time.Hour)
	b := filepath.Join(f.dir, "b.txt")
	if err := os.Rename(a, b); err != nil {
		t.Fatal(err)
	}
	second := f.add(b, 24*time.Hour)

	items := f.items()
	if len(items) != 1 || second.ID != first.ID || !samePath(items[0].Path, b) {
		t.Fatalf("want one record following the file to b, got %+v", items)
	}
	if out := f.sweepAt(time.Hour); len(out) != 0 || !exists(b) {
		t.Fatalf("the old 1h lifespan must be gone, got %+v", out)
	}
}

func TestHardLinkedFileIsRefused(t *testing.T) {
	f := newFixture(t)
	important := f.file("important.txt")
	link := filepath.Join(f.dir, "temp-link.txt")
	if err := os.Link(important, link); err != nil {
		t.Fatal(err)
	}

	results, err := f.svc.Add([]string{link}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil || results[0].Error == "" {
		t.Fatalf("a hard-linked file must be refused, got %+v %v", results, err)
	}
}

func TestNestedRepositoryNeedsForce(t *testing.T) {
	f := newFixture(t)
	dir := filepath.Join(f.dir, "workspace")
	if err := os.MkdirAll(filepath.Join(dir, "projects", "app", ".git"), 0o700); err != nil {
		t.Fatal(err)
	}

	results, err := f.svc.Add([]string{dir}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil || results[0].Error == "" {
		t.Fatalf("a folder with a nested repository must need --force, got %+v %v", results, err)
	}
}

func shortPath(t *testing.T, path string) string {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	b := make([]uint16, 32768)
	n, err := windows.GetShortPathName(p, &b[0], uint32(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	short := windows.UTF16ToString(b[:n])
	if samePath(short, path) {
		t.Skip("volume does not generate 8.3 aliases")
	}
	return short
}

func TestRegistrationExpandsShortNames(t *testing.T) {
	for _, entry := range []string{"add", "bury", "proposal"} {
		t.Run(entry, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("Directory With A Long Name/payload.txt")
			short := shortPath(t, path)
			switch entry {
			case "add":
				it := f.add(short, time.Hour)
				if !samePath(it.Path, path) {
					t.Fatalf("short alias entered ledger: %s", it.Path)
				}
				f.sweepAt(time.Hour)
			case "bury":
				got, err := f.svc.Bury([]string{short}, false, "")
				if err != nil || core.GraveFailed(got) {
					t.Fatal(got, err)
				}
			case "proposal":
				p, _, _, err := f.svc.Propose(core.ProposalLarge, f.dir, "", []core.Candidate{{Path: short, Lifespan: "1d"}})
				if err != nil || !samePath(p.Candidates[0].Path, path) {
					t.Fatalf("short alias entered proposal: %+v %v", p, err)
				}
				return
			}
			b := f.burials()[0]
			if !samePath(b.Path, path) || !samePath(b.Parents[0].Path, filepath.Dir(path)) {
				t.Fatalf("short alias entered burial: %+v", b)
			}
			parent := filepath.Dir(path)
			if err := os.Remove(parent); err != nil {
				t.Fatal(err)
			}
			aliasParent := filepath.Join(f.dir, "Directory With Another Name")
			if err := os.Mkdir(aliasParent, 0700); err != nil {
				t.Fatal(err)
			}
			if filepath.Base(shortPath(t, aliasParent)) != filepath.Base(filepath.Dir(short)) {
				t.Fatal("NTFS did not reuse the short alias")
			}
			got, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			if err != nil || core.GraveFailed(got) || !exists(path) || exists(filepath.Join(aliasParent, "payload.txt")) {
				t.Fatalf("restore followed reused short alias: %+v %v", got, err)
			}
		})
	}
}
