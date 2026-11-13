package lifecycle_test

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

// Limit native filesystem reads to this fixture even if the JSON mutates into
// an absolute path. Paths outside the graveyard but inside the fixture still
// reach the real containment/identity checks; no move or cleanup is dispatched.
type manifestFuzzFS struct {
	testFS
	base string
}

func (fs *manifestFuzzFS) RealPath(path string) (string, error) {
	rel, err := filepath.Rel(fs.base, path)
	if err != nil || !filepath.IsLocal(rel) {
		return "", core.ErrUnsafe
	}
	return fs.testFS.RealPath(path)
}

func FuzzManifest(f *testing.F) {
	base := f.TempDir()
	fs := &manifestFuzzFS{base: base}
	svc := &core.Service{Store: core.Store{Home: filepath.Join(base, "home")}, FS: fs}
	root := filepath.Join(svc.Store.Home, "graveyard")
	const name = "1-20261107T120000Z-aabbcc"
	dir := filepath.Join(root, name)
	outside := filepath.Join(base, "outside", name)
	for _, path := range []string{dir, outside} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			f.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "keep.txt"), []byte("keep"), 0o600); err != nil {
			f.Fatal(err)
		}
	}
	quote := func(v any) string {
		data, err := json.Marshal(v)
		if err != nil {
			f.Fatal(err)
		}
		return string(data)
	}
	identity := func(path string) core.Identity {
		id, err := fs.Identify(path)
		if err != nil {
			f.Fatal(err)
		}
		return id
	}
	grave := filepath.Join(dir, "keep.txt")
	outsideGrave := filepath.Join(outside, "keep.txt")
	replacements := strings.NewReplacer(
		`"@path@"`, quote(filepath.Join(base, "keep.txt")),
		`"@grave@"`, quote(grave), `"@outside@"`, quote(outsideGrave),
		`"@identity@"`, quote(identity(grave)), `"@graveID@"`, quote(identity(dir)),
		`"@outsideIdentity@"`, quote(identity(outsideGrave)), `"@outsideID@"`, quote(identity(outside)),
	)
	rootID := identity(root)
	manifest := dir + ".json"
	f.Fuzz(func(t *testing.T, input []byte) {
		data := []byte(replacements.Replace(string(input)))
		inspect := func(data []byte) core.PendingGrave {
			t.Helper()
			if err := os.WriteFile(manifest, data, 0o600); err != nil {
				t.Fatal(err)
			}
			rows, err := core.ScanGraveyardRoot(svc, root)
			if err != nil || len(rows) != 1 {
				t.Fatalf("manifest scan: %v, %+v", err, rows)
			}
			return rows[0]
		}
		p := inspect(data)
		if p.Error == "" {
			if p.Burial == nil || p.Status == "unverifiable" {
				t.Fatalf("accepted manifest has no burial: %+v", p)
			}
			parent := filepath.Dir(filepath.Dir(p.Burial.Grave))
			id, err := fs.Identify(parent)
			if err != nil || id != rootID {
				t.Fatalf("manifest accepted a grave outside its root: %q, %v", p.Burial.Grave, err)
			}
			var value any
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.UseNumber()
			if err := dec.Decode(&value); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			again := inspect(encoded)
			if again.Error != "" || again.Status != p.Status || !reflect.DeepEqual(again.Burial, p.Burial) {
				t.Fatalf("manifest decision changed on round trip: %+v -> %+v", p, again)
			}
		}
		if read(t, grave) != "keep" || read(t, outsideGrave) != "keep" || len(fs.recycled) != 0 {
			t.Fatal("manifest inspection modified a payload")
		}
	})
}

func BenchmarkScanManifests(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			svc, st := benchmarkGraveyard(b, n)
			b.ResetTimer()
			for b.Loop() {
				if _, err := core.ScanManifests(svc, st); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestConfirmedBurialAuditBeforeDirectTerminalAction(t *testing.T) {
	core.RequireFsync(t)
	for _, action := range []string{"resurrect", "purge"} {
		t.Run(action, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("payload.txt")
			moved := false
			step := core.SetBurialStep(f.svc, func(stage string) error { moved = moved || stage == "moved"; return nil })
			defer step()
			reset := core.SetReplacementSync(func(file *os.File) error {
				if moved && filepath.Base(file.Name()) == "store.json.tmp" {
					return errors.New("commit flush interrupted")
				}
				return file.Sync()
			})
			defer reset()
			if _, err := f.svc.Bury([]string{path}, false, ""); err == nil {
				t.Fatal("fixture did not interrupt burial")
			}
			reset()
			step()
			b := f.burials()[0]
			var rows []core.GraveResult
			var err error
			if action == "resurrect" {
				rows, err = f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			} else {
				rows, err = f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
			}
			if err != nil || core.GraveFailed(rows) {
				t.Fatalf("terminal action: %+v %v", rows, err)
			}
			events, err := f.svc.Store.History(20)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, e := range events {
				found = found || e.Action == core.ActionBuried && e.ID == b.ID && strings.Contains(e.Detail, "confirmed")
			}
			if !found {
				t.Fatalf("direct terminal action lost confirmed burial audit: %+v", events)
			}
			if exists(filepath.Dir(b.Grave) + ".json") {
				t.Fatal("confirmed manifest retained after terminal action")
			}
		})
	}
}

func TestForeignManifestCannotConfirmUnicodeRoot(t *testing.T) {
	testenv.RequireDistinctUnicodeNames(t)
	f := newFixture(t)
	f.svc.Store.Home = filepath.Join(filepath.Dir(f.dir), "K")
	stop := core.SetBurialStep(f.svc, func(stage string) error {
		if stage == "committed" {
			return errors.New("interrupted")
		}
		return nil
	})
	defer stop()
	if _, err := f.svc.Bury([]string{f.file("keep.txt")}, false, ""); err == nil {
		t.Fatal("fixture did not interrupt")
	}
	stop()
	state, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	b := state.Burials[0]
	other := *f.svc
	other.Store.Home = filepath.Join(filepath.Dir(f.dir), "K")
	root := filepath.Join(other.Store.Home, "graveyard")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := other.Store.Update(func(st *core.State) error { *st = state; return nil }); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Dir(b.Grave) + ".json"
	if err := os.WriteFile(filepath.Join(root, filepath.Base(manifest)), []byte(read(t, manifest)), 0600); err != nil {
		t.Fatal(err)
	}
	pending, err := other.PendingGraves()
	if err != nil || len(pending) != 1 || pending[0].Status != "unverifiable" || !strings.Contains(pending[0].Error, "escapes this ledger") {
		t.Fatalf("foreign manifest authorized Unicode root: %+v %v", pending, err)
	}
	if read(t, b.Grave) != "keep.txt" {
		t.Fatal("foreign manifest changed payload")
	}
}

func TestBurialAuditFailureRetainsRecoveryEvidence(t *testing.T) {
	for _, phase := range []string{"moved", "committed"} {
		t.Run(phase, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("payload.txt")
			audit := filepath.Join(f.svc.Store.Home, "audit.jsonl")
			if err := os.MkdirAll(audit, 0700); err != nil {
				t.Fatal(err)
			}
			stop := core.SetBurialStep(f.svc, func(stage string) error {
				if stage == phase {
					return errors.New("interrupted")
				}
				return nil
			})
			defer stop()
			if _, err := f.svc.Bury([]string{path}, false, ""); err == nil {
				t.Fatal("fixture did not interrupt")
			}
			stop()
			if _, err := f.svc.Sweep(false); err == nil {
				t.Fatal("audit failure missing")
			}
			b := f.burials()[0]
			st, err := f.svc.Store.ReadSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if !exists(filepath.Dir(b.Grave)+".json") && len(st.PendingAudit) == 0 {
				t.Fatal("audit failure discarded burial evidence")
			}
			if phase == "committed" && !exists(filepath.Dir(b.Grave)+".json") {
				t.Fatal("recorded recovery removed manifest before audit confirmation")
			}
			if err := os.Remove(audit); err != nil {
				t.Fatal(err)
			}
			if _, err := f.svc.Sweep(false); err != nil {
				t.Fatal(err)
			}
			if exists(filepath.Dir(b.Grave) + ".json") {
				t.Fatal("successful audit retained burial evidence")
			}
			events, err := f.svc.Store.History(20)
			found := false
			for _, e := range events {
				found = found || e.Action == core.ActionBuried && e.ID == b.ID && e.Path == b.Path
			}
			after, readErr := f.svc.Store.ReadSnapshot()
			if err != nil || readErr != nil || !found || after.Stats.Buried != 1 || len(after.PendingAudit) != 0 || len(f.fs.recycled) != 1 {
				t.Fatalf("burial audit retry lost event or repeated operation: %+v %+v %v %v", events, after.Stats, err, readErr)
			}
		})
	}
}

func TestExtendedHomeInterruptedBurial(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("extended drive and UNC home spellings are Windows-only")
	}
	for _, unc := range []bool{false, true} {
		t.Run(fmt.Sprintf("unc=%v", unc), func(t *testing.T) {
			f := newFixture(t)
			path := f.file("payload.txt")
			home := f.svc.Store.Home
			if unc {
				prefix := `\\localhost\` + strings.TrimSuffix(filepath.VolumeName(path), ":") + "$"
				path, home = prefix+path[2:], prefix+home[2:]
				if _, err := os.Stat(path); err != nil {
					t.Skipf("existing administrative share unavailable: %v", err)
				}
				f.svc.Store.Home = `\\?\UNC\` + home[2:]
			} else {
				f.svc.Store.Home = `\\?\` + home
			}
			// Exercise already-written verbatim manifests, independently of the
			// new userdirs normalization; both namespaces stay on scratch's volume.
			wantRoot := filepath.Join(f.svc.Store.Home, "graveyard")
			if root := core.GraveyardRoot(f.svc, path); root != wantRoot {
				t.Fatalf("refusing unsafe test graveyard: %s", root)
			}
			stop := core.SetBurialStep(f.svc, func(stage string) error {
				if stage == "moved" {
					return errors.New("interrupted move")
				}
				return nil
			})
			defer stop()
			if _, err := f.svc.Bury([]string{path}, false, ""); err == nil {
				t.Fatal("fixture did not interrupt move")
			}
			stop()
			rows, err := f.svc.Bury([]string{path}, false, "")
			if err != nil || core.GraveFailed(rows) || len(rows) != 1 || rows[0].Burial == nil || rows[0].Action != core.ActionBuried {
				t.Fatalf("extended Home manifest could not recover: %+v %v", rows, err)
			}
			restored, err := f.svc.Resurrect([]string{fmt.Sprint(rows[0].Burial.ID)})
			if err != nil || core.GraveFailed(restored) || read(t, path) != "payload.txt" {
				t.Fatalf("extended Home burial could not resurrect: %+v %v", restored, err)
			}
			if pending, err := f.svc.PendingGraves(); err != nil || len(pending) != 0 {
				t.Fatalf("extended Home cleanup did not converge: %+v %v", pending, err)
			}
		})
	}
}

func BenchmarkPendingGraves(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			svc, st := benchmarkGraveyard(b, n)
			b.ResetTimer()
			for b.Loop() {
				if _, err := core.PendingGraveSnapshot(svc, st); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

type countingGraveRootFS struct {
	core.FS
	root  string
	calls int
}

func (f *countingGraveRootFS) RealPath(path string) (string, error) {
	if samePath(path, f.root) {
		f.calls++
	}
	return f.FS.RealPath(path)
}

func TestPendingGravesResolvesRootsOnce(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "buried.txt")
	for i := range 12 {
		f.add(f.file(fmt.Sprintf("live-%d", i)), time.Hour)
	}
	fs := &countingGraveRootFS{FS: f.fs, root: core.GraveyardRoot(f.svc, b.Path)}
	f.svc.FS = fs
	view, err := f.svc.ReadGraveyard()
	if err != nil || len(view.Burials) != 1 || len(view.Pending) != 0 {
		t.Fatal(view, err)
	}
	if fs.calls != 1 {
		t.Fatalf("graveyard root resolved repeatedly for one listing: %d", fs.calls)
	}
}

type failedRestorePreparationFS struct{ core.FS }

func (failedRestorePreparationFS) OpenRestoreParent(string) (core.RestoreParent, error) {
	return nil, errors.New("injected restore preparation failure")
}

func TestRecordedManifestRequiresDurableConfirmation(t *testing.T) {
	core.RequireFsync(t)
	for _, failure := range []string{"parent-refused", "prepare-failed", "intent-failed"} {
		t.Run(failure, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("payload.txt")
			moved := false
			var oldLedger []byte
			step := core.SetBurialStep(f.svc, func(stage string) error {
				if stage == "moved" {
					moved = true
					var err error
					oldLedger, err = os.ReadFile(filepath.Join(f.svc.Store.Home, "store.json"))
					return err
				}
				return nil
			})
			defer step()
			stop := core.SetReplacementSync(func(file *os.File) error {
				if moved && filepath.Base(file.Name()) == "store.json.tmp" {
					return errors.New("injected post-rename sync failure")
				}
				return file.Sync()
			})
			defer stop()
			if _, err := f.svc.Bury([]string{path}, false, ""); err == nil {
				t.Fatal("fixture did not interrupt burial commit")
			}
			b := f.burials()[0]
			manifest := filepath.Dir(b.Grave) + ".json"
			switch failure {
			case "parent-refused":
				f.svc.Guard.Keep = append(f.svc.Guard.Keep, path)
			case "prepare-failed":
				f.svc.FS = failedRestorePreparationFS{FS: f.fs}
			case "intent-failed":
				if err := os.Mkdir(manifest+".tmp", 0700); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			if !core.GraveFailed(rows) || !exists(manifest) || !exists(b.Grave) {
				t.Fatalf("rejected resurrection discarded unconfirmed burial evidence: %+v %v", rows, err)
			}
			stop()
			step()
			f.svc.FS = f.fs
			// Simulate losing the unconfirmed rename: only the independent v1
			// manifest can now recover this otherwise unindexed grave.
			if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "store.json"), oldLedger, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := f.svc.Sweep(false)
			if err != nil || len(f.burials()) != 1 || !exists(b.Grave) {
				t.Fatalf("unconfirmed burial could not recover: %+v %v", out, err)
			}
		})
	}
}

func TestUnreadableManifestStillBlocksFilenameParent(t *testing.T) {
	for _, data := range []string{"{broken", strings.Repeat("x", (1<<20)+1)} {
		f := newFixture(t)
		path := f.file("keep.txt")
		item, grave := interruptBurial(t, f, path, "bury")
		manifest := filepath.Dir(grave) + ".json"
		if err := os.Rename(grave, path); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifest, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := f.svc.Sweep(false)
		if err != nil || len(out) != 1 || out[0].Item.ID != item.ID || out[0].Action != core.ActionBlocked || !exists(path) || len(f.burials()) != 0 {
			t.Fatalf("undecodable manifest lost filename block: %+v %v", out, err)
		}
	}
}

func TestManifestEmptyGraveRemainsRecoverable(t *testing.T) {
	f := newFixture(t)
	path := f.file("payload.txt")
	stop := core.SetBurialStep(f.svc, func(at string) error {
		if at == "manifest" {
			return errors.New("interrupted")
		}
		return nil
	})
	t.Cleanup(stop)
	_, _ = f.svc.Bury([]string{path}, false, "")
	stop()
	pending, err := f.svc.PendingGraves()
	if err != nil || len(pending) != 1 || pending[0].Status != "empty" || pending[0].Error != "" {
		t.Fatalf("empty manifest lost recoverable state: %+v %v", pending, err)
	}
	if _, err := f.svc.Sweep(false); err != nil || exists(pending[0].Manifest) || exists(filepath.Dir(pending[0].Burial.Grave)) || read(t, path) != "payload.txt" {
		t.Fatalf("empty manifest did not converge safely: %v", err)
	}
}

func TestManifestRejectsProposalID(t *testing.T) {
	f := newFixture(t)
	it, grave := interruptBurial(t, f, f.file("payload.txt"), "bury")
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.Proposals = append(st.Proposals, core.Proposal{ID: it.ID})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	pending, err := f.svc.PendingGraves()
	if err != nil || len(pending) != 1 || pending[0].Status != "unverifiable" || !strings.Contains(pending[0].Error, "belongs to a proposal") {
		t.Fatalf("proposal ID accepted: %+v %v", pending, err)
	}
	if read(t, grave) != "payload.txt" || !exists(filepath.Dir(grave)+".json") {
		t.Fatal("proposal conflict changed content or evidence")
	}
}

func TestManifestTamperingPreservesObjectsAndWarns(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, kind := range []string{"original-path", "original-in-grave", "wrong-id", "bad-time", "bad-suffix", "extra-object", "object-identity", "grave-identity", "object-type", "version", "unknown-field", "trailing-json", "unreadable"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("keep.txt")
			original := f.add(path, time.Hour)
			_, grave := interruptBurial(t, f, path, "bury")
			manifest := filepath.Dir(grave) + ".json"
			originalLocation := grave
			extra := ""
			switch kind {
			case "original-path":
				extra = f.file("precious.txt")
				rewriteManifest(t, manifest, func(m map[string]any) { m["path"] = extra })
			case "original-in-grave":
				rewriteManifest(t, manifest, func(m map[string]any) { m["path"] = grave })
			case "wrong-id":
				rewriteManifest(t, manifest, func(m map[string]any) { m["id"] = 999 })
			case "bad-time", "bad-suffix":
				name := fmt.Sprintf("%d-not-a-time-abcdef.json", original.ID)
				if kind == "bad-suffix" {
					name = fmt.Sprintf("%d-%s-nothex.json", original.ID, f.clock.UTC().Format("20060102T150405Z"))
				}
				renamed := filepath.Join(filepath.Dir(manifest), name)
				if err := os.Rename(manifest, renamed); err != nil {
					t.Fatal(err)
				}
				manifest = renamed
			case "extra-object":
				extra = filepath.Join(filepath.Dir(grave), "extra.txt")
				if err := os.WriteFile(extra, []byte("keep extra"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "object-identity":
				originalLocation = filepath.Join(f.dir, "kept-original.txt")
				if err := os.Rename(grave, originalLocation); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(grave, []byte("replacement"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "grave-identity":
				id, err := f.fs.Identify(f.dir)
				if err != nil {
					t.Fatal(err)
				}
				rewriteManifest(t, manifest, func(m map[string]any) { m["graveId"] = id })
			case "object-type":
				rewriteManifest(t, manifest, func(m map[string]any) { m["isDir"] = true })
			case "version":
				rewriteManifest(t, manifest, func(m map[string]any) { m["version"] = 2 })
			case "unknown-field":
				rewriteManifest(t, manifest, func(m map[string]any) { m["surprise"] = true })
			case "trailing-json":
				data, err := os.ReadFile(manifest)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(manifest, append(data, []byte(" {}")...), 0o600); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if err := os.Rename(manifest, manifest+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(manifest, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			pending, err := f.svc.PendingGraves()
			if err != nil || len(pending) != 1 || pending[0].Status != "unverifiable" || pending[0].Error == "" {
				t.Fatalf("damaged manifest not diagnosable: %+v, %v", pending, err)
			}
			wantError := map[string]string{
				"bad-suffix": "manifest name does not match", "bad-time": "manifest name does not match",
				"original-path": "manifest paths do not match", "original-in-grave": "manifest original path is inside",
				"grave-identity": "grave directory identity does not match",
			}[kind]
			if !strings.Contains(pending[0].Error, wantError) {
				t.Fatalf("damaged manifest not diagnosable: want %q, got %+v", wantError, pending)
			}
			if _, _, err := f.svc.Tick(); err != nil {
				t.Fatal(err)
			}
			if len(f.burials()) != 0 || len(f.items()) != 1 || !reflect.DeepEqual(f.items()[0], original) || read(t, originalLocation) != "keep.txt" || !exists(manifest) {
				t.Fatalf("unverifiable recovery changed objects or ledger: %+v", f.items())
			}
			if extra != "" && !exists(extra) || kind == "object-identity" && read(t, grave) != "replacement" {
				t.Fatal("an unrelated object was changed")
			}
			events, err := f.svc.Store.History(20)
			if err != nil || len(events) == 0 || events[len(events)-1].Action != "recovery warning" {
				t.Fatalf("missing recovery warning: %+v, %v", events, err)
			}
		})
	}
}

func TestManifestCannotCleanOutsideRoot(t *testing.T) {
	f := newFixture(t)
	path := f.file("keep.txt")
	original := f.add(path, time.Hour)
	restore := core.SetBurialStep(f.svc, func(step string) error {
		if step == "manifest" {
			return errors.New("interrupted before move")
		}
		return nil
	})
	t.Cleanup(restore)
	_, _ = f.svc.Bury([]string{path}, false, "")
	restore()
	pending, err := f.svc.PendingGraves()
	if err != nil || len(pending) != 1 || pending[0].Status != "empty" {
		t.Fatalf("empty grave fixture: %+v, %v", pending, err)
	}
	b := pending[0].Burial
	dir := filepath.Dir(b.Grave)
	outside := filepath.Join(f.dir, filepath.Base(dir))
	if err := os.Rename(dir, outside); err != nil {
		t.Fatal(err)
	}
	rewriteManifest(t, pending[0].Manifest, func(m map[string]any) { m["grave"] = filepath.Join(outside, filepath.Base(path)) })
	if _, _, err := f.svc.Tick(); err != nil {
		t.Fatal(err)
	}
	if !exists(outside) || !exists(pending[0].Manifest) || read(t, path) != "keep.txt" || len(f.burials()) != 0 || !reflect.DeepEqual(f.items()[0], original) {
		t.Fatal("manifest directed cleanup outside this ledger's graveyard")
	}
	rows, err := f.svc.PendingGraves()
	if err != nil || len(rows) != 1 || rows[0].Status != "unverifiable" {
		t.Fatalf("escaping manifest not warned: %+v, %v", rows, err)
	}
}

func TestManifestRenameBeforeMoveCanRetry(t *testing.T) {
	for _, operation := range []string{"sweep", "bury"} {
		t.Run(operation, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("keep.txt")
			f.add(path, time.Hour)
			f.clock = f.clock.Add(time.Hour)
			renamed := path + ".renamed"
			restore := core.SetAfterManifest(f.svc, func() {
				if err := os.Rename(path, renamed); err != nil {
					t.Fatal(err)
				}
			})
			t.Cleanup(restore)
			if operation == "sweep" {
				_, _ = f.svc.Sweep(false)
			} else {
				_, _ = f.svc.Bury([]string{path}, false, "")
			}
			restore()
			if !exists(renamed) || len(f.burials()) != 0 {
				t.Fatal("renamed object must stay put until retried")
			}
			out := f.sweepAt(time.Minute)
			if runtime.GOOS != "windows" {
				requireRetainedRetry(t, f, out)
				if read(t, renamed) != "keep.txt" {
					t.Fatal("unlocated source changed")
				}
				return
			}
			if len(f.burials()) != 1 || f.burials()[0].Path != renamed || len(f.items()) != 0 {
				t.Fatal("rename after manifest permanently stranded the item")
			}
		})
	}
}

func TestManifestTimestampSurvivesSecondBoundary(t *testing.T) {
	f := newFixture(t)
	path := f.file("keep.txt")
	restore := core.SetBurialStep(f.svc, func(step string) error {
		if step == "prepared" {
			f.clock = f.clock.Add(time.Second)
		}
		if step == "moved" {
			return errors.New("interrupted")
		}
		return nil
	})
	t.Cleanup(restore)
	_, _ = f.svc.Bury([]string{path}, false, "")
	restore()
	pending, err := f.svc.PendingGraves()
	if err != nil || len(pending) != 1 || pending[0].Status != "pending" {
		t.Fatalf("crossing a clock second made an invalid manifest: %+v, %v", pending, err)
	}
	if _, _, err := f.svc.Tick(); err != nil || len(f.burials()) != 1 {
		t.Fatalf("timestamp prevented recovery: %v", err)
	}
}

func TestRecordedManifestMustMatchBurial(t *testing.T) {
	for _, mismatch := range []string{"grave", "grave-id", "identity"} {
		t.Run(mismatch, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("keep.txt")
			_, grave := interruptBurial(t, f, path, "bury")
			pending, err := f.svc.PendingGraves()
			if err != nil || len(pending) != 1 {
				t.Fatal(err)
			}
			b := *pending[0].Burial
			switch mismatch {
			case "grave":
				b.Grave += ".other"
			case "grave-id":
				b.GraveID.FileID = strings.Repeat("b", 32)
			case "identity":
				b.Identity.FileID = strings.Repeat("c", 32)
			}
			if err := f.svc.Store.Update(func(st *core.State) error { st.Burials = append(st.Burials, b); return nil }); err != nil {
				t.Fatal(err)
			}
			if _, err := f.svc.Sweep(false); err != nil {
				t.Fatal(err)
			}
			pending, err = f.svc.PendingGraves()
			if err != nil || len(pending) != 1 || pending[0].Status != "unverifiable" || !exists(filepath.Dir(grave)+".json") || read(t, grave) != "keep.txt" {
				t.Fatalf("ID alone authorized manifest removal: %+v, %v", pending, err)
			}
		})
	}
}

func TestManifestMetadataConflictBlocksDestruction(t *testing.T) {
	for _, field := range []string{"path", "isDir", "buriedAt", "purgeAt", "reason", "size"} {
		t.Run(field, func(t *testing.T) {
			f := newFixture(t)
			b := committedManifest(t, f)
			f.clock = b.BuriedAt.Add(time.Hour)
			if err := f.svc.Store.Update(func(st *core.State) error {
				current := &st.Burials[0]
				switch field {
				case "path":
					current.Path = filepath.Join(f.dir, "other", "keep.txt")
				case "isDir":
					current.IsDir = !current.IsDir
				case "buriedAt":
					current.BuriedAt = current.BuriedAt.Add(-time.Hour)
				case "purgeAt":
					current.PurgeAt = f.clock
				case "reason":
					current.Reason = core.ReasonExpired
				case "size":
					current.Size = 42
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			out, purged, err := f.svc.Tick()
			if field == "size" {
				if err != nil || exists(filepath.Dir(b.Grave)+".json") || !exists(b.Grave) {
					t.Fatalf("size-only difference rejected: %v", err)
				}
				return
			}
			if err != nil || len(out) != 1 || out[0].Action != core.ActionBlocked || len(purged) != 0 || !exists(filepath.Dir(b.Grave)+".json") || read(t, b.Grave) != "keep.txt" {
				t.Fatalf("conflicting manifest authorized destruction: %+v, %+v, %v", out, purged, err)
			}
			pending, err := f.svc.PendingGraves()
			if err != nil || len(pending) != 1 || pending[0].Status != "unverifiable" {
				t.Fatalf("conflict not diagnosable: %+v, %v", pending, err)
			}
			rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			if err != nil || !core.GraveFailed(rows) || !exists(b.Grave) {
				t.Fatalf("conflict allowed resurrection: %+v, %v", rows, err)
			}
		})
	}
}

func TestOversizedChildManifestRefusesBeforeMove(t *testing.T) {
	if testing.Short() {
		t.Skip("constructs thousands of child identities to reach the manifest limit")
	}
	f := newFixture(t)
	path := f.file("output/first.txt")
	parent := f.add(filepath.Dir(path), time.Hour)
	var children []core.Item
	manifest := struct {
		Version int `json:"version"`
		core.Burial
	}{Version: 1, Burial: core.Burial{ID: parent.ID, Path: parent.Path, Identity: parent.Identity, IsDir: true}}
	var encoded []byte
	for len(encoded)+1 <= 1<<20 {
		// Batch real files to avoid re-encoding an ever-growing list per child.
		for range 256 {
			i := len(children)
			path := f.file(fmt.Sprintf("output/%d.txt", i))
			id, err := f.fs.Identify(path)
			if err != nil {
				t.Fatal(err)
			}
			child := core.Item{ID: int64(i + 2), Rev: 1, Path: path, Identity: &id, CreatedAt: f.clock, ExpiresAt: parent.ExpiresAt}
			children = append(children, child)
			manifest.Children = append(manifest.Children, core.BuriedChild{ID: child.ID, Identity: child.Identity})
			manifest.Contains = append(manifest.Contains, child.ID)
		}
		var err error
		encoded, err = json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
	}
	// The actual grave metadata only adds bytes to this serialized lower bound.
	if len(encoded)+1 <= 1<<20 {
		t.Fatalf("fixture does not exceed the manifest limit: %d bytes", len(encoded)+1)
	}
	t.Logf("manifest lower bound: %d bytes with %d children", len(encoded)+1, len(children))
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.Items = append(st.Items, children...)
		st.NextID = int64(len(children) + 2)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.clock = parent.ExpiresAt
	moves := 0
	f.fs.onRecycle = func(string) { moves++ }
	f.fs.fail = errors.New("move must not be reached")
	// An oversized recovery record must be refused before the final directory
	// scan can exhaust its work budget, regardless of filesystem latency.
	prepared := false
	f.svc.Guard.Canonical = func(path string) (string, error) {
		if prepared {
			return "", context.DeadlineExceeded
		}
		return path, nil
	}
	t.Cleanup(core.SetAfterManifest(f.svc, func() { prepared = true }))
	rows, err := f.svc.Bury([]string{parent.Path}, false, "")
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRefused || !strings.Contains(rows[0].Error, "too many registered") || moves != 0 {
		t.Fatalf("oversized manifest reached move: moves=%d rows=%+v err=%v", moves, rows, err)
	}
	if len(f.items()) != len(children)+1 || !exists(path) || len(f.burials()) != 0 {
		t.Fatal("refusal changed objects or lifespans")
	}
	if entries, err := os.ReadDir(core.GraveyardRoot(f.svc, parent.Path)); err != nil || len(entries) != 0 {
		t.Fatalf("refusal left a manifest or grave: %d %v", len(entries), err)
	}
}

func TestChildManifestKeepsOnlyIdentityAndReadsLegacySnapshot(t *testing.T) {
	f := newFixture(t)
	path := f.file("output/child.txt")
	parent := f.add(filepath.Dir(path), time.Hour)
	child := f.add(path, time.Hour)
	f.clock = parent.ExpiresAt
	stop := errors.New("interrupted move")
	restore := core.SetBurialStep(f.svc, func(step string) error {
		if step == "moved" {
			return stop
		}
		return nil
	})
	t.Cleanup(restore)
	if _, err := f.svc.Bury([]string{parent.Path}, false, ""); !errors.Is(err, stop) {
		t.Fatal(err)
	}
	restore()
	pending, err := f.svc.PendingGraves()
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending: %+v %v", pending, err)
	}
	manifest := pending[0].Manifest
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	var snapshots []map[string]json.RawMessage
	if err := json.Unmarshal(doc["children"], &snapshots); err != nil || len(snapshots) != 1 || len(snapshots[0]) != 2 || snapshots[0]["id"] == nil || snapshots[0]["identity"] == nil {
		t.Fatalf("snapshot carries lifespan data: %s %v", doc["children"], err)
	}
	// Model an older manifest whose child was canceled before recovery.
	doc["children"], err = json.Marshal([]core.Item{child})
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, data, 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(f.dir, "escaped.txt")
	if err := os.Rename(filepath.Join(pending[0].Burial.Grave, "child.txt"), outside); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.Items = st.Items[:1]
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Bury([]string{parent.Path}, false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if len(f.items()) != 1 || f.items()[0].ID != parent.ID || len(f.burials()) != 0 || !exists(manifest) || read(t, outside) != "output/child.txt" {
			t.Fatal("unknown child location lost evidence or rebuilt its canceled lifespan")
		}
		return
	}
	if len(f.items()) != 0 || len(f.burials()) != 1 || read(t, outside) != "output/child.txt" {
		t.Fatal("legacy snapshot rebuilt canceled lifespan")
	}
}
