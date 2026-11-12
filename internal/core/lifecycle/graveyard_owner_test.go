package lifecycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

// Force the owner branch on a scratch-local root; never create volume-root
// graves, even under an owner-cache mutation.
type ownerBranchFS struct{ core.FS }

// Only name equivalence is simulated; both directories keep their real IDs.
type foldedOwnerFS struct {
	core.FS
	first, second string
}

func (fs foldedOwnerFS) SameName(a, b string) (bool, error) {
	if a == fs.first && b == fs.second || a == fs.second && b == fs.first {
		return true, nil
	}
	return fs.FS.SameName(a, b)
}

func TestOwnerIdentitySurvivesNameFolding(t *testing.T) {
	f := newFixture(t)
	a, b := *f.svc, *f.svc
	a.Store.Home, b.Store.Home = filepath.Join(f.dir, "first-home"), filepath.Join(f.dir, "second-home")
	for _, home := range []string{a.Store.Home, b.Store.Home} {
		if err := os.Mkdir(home, 0700); err != nil {
			t.Fatal(err)
		}
	}
	a.FS = foldedOwnerFS{FS: f.fs, first: a.Store.Home, second: b.Store.Home}
	b.FS = a.FS
	root := filepath.Join(f.dir, "external", "shared-key")
	if err := core.CheckGraveyardOwner(&a, root, true); err != nil {
		t.Fatal(err)
	}
	if err := core.CheckGraveyardOwner(&b, root, false); !errors.Is(err, core.ErrUnsafe) {
		t.Fatalf("folded name accepted another owner's identity: %v", err)
	}
	data, _ := json.Marshal(a.Store.Home)
	if err := os.WriteFile(root+".owner", data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := core.CheckGraveyardOwner(&b, root, false); !errors.Is(err, core.ErrUnsafe) {
		t.Fatalf("folded legacy name migrated another owner's identity: %v", err)
	}
}

func TestLegacyOwnerMigrationHonorsStoreDeadline(t *testing.T) {
	f := newFixture(t)
	f.svc.FS = ownerBranchFS{f.fs}
	b := buryOne(t, f, "keep.txt")
	marker := filepath.Dir(filepath.Dir(b.Grave)) + ".owner"
	data, _ := json.Marshal(f.svc.Store.Home)
	if err := os.WriteFile(marker, data, 0600); err != nil {
		t.Fatal(err)
	}
	release, err := core.LockFile(marker+".lock", true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	// The outer context bounds even the broken implementation, without masking
	// the earlier Store deadline assertion.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	svc := f.svc.WithContext(ctx)
	svc.Store = svc.Store.WithDeadline(time.Now().Add(100 * time.Millisecond))
	start := time.Now()
	_, err = svc.ReadGraveyard()
	if err == nil || time.Since(start) > time.Second || ctx.Err() != nil {
		t.Fatalf("owner migration ignored Store deadline: elapsed=%v err=%v", time.Since(start), err)
	}
	if read(t, marker) != string(data) || read(t, b.Grave) != "keep.txt" {
		t.Fatal("timed out owner migration changed evidence")
	}
}

func (fs ownerBranchFS) SameName(a, b string) (bool, error) {
	if strings.HasSuffix(a, `\graveyard`) && strings.HasSuffix(b, `\graveyard`) {
		return false, nil
	}
	return fs.FS.SameName(a, b)
}

func TestSweepConfirmsOwnerOncePerOperation(t *testing.T) {
	core.RequireFsync(t)
	f := newFixture(t)
	f.svc.FS = ownerBranchFS{f.fs}
	for _, name := range []string{"a", "b", "c"} {
		f.add(f.file(name), time.Hour)
	}
	claims := 0
	reset := core.SetReplacementSync(func(file *os.File) error {
		if strings.HasSuffix(file.Name(), ".owner.tmp") {
			claims++
		}
		return file.Sync()
	})
	defer reset()
	f.sweepAt(time.Hour)
	if claims != 1 || len(f.burials()) != 3 {
		t.Fatalf("owner persisted repeatedly during one sweep: claims=%d graves=%d", claims, len(f.burials()))
	}
	f.add(f.file("d"), time.Hour)
	f.sweepAt(time.Hour)
	if claims != 2 {
		t.Fatalf("new sweep did not reconfirm owner: %d", claims)
	}
}

func TestBuryConfirmsOwnerOncePerOperation(t *testing.T) {
	core.RequireFsync(t)
	f := newFixture(t)
	f.svc.FS = ownerBranchFS{f.fs}
	paths := []string{f.file("a"), f.file("b"), f.file("c")}
	claims := 0
	reset := core.SetReplacementSync(func(file *os.File) error {
		if strings.HasSuffix(file.Name(), ".owner.tmp") {
			claims++
		}
		return file.Sync()
	})
	defer reset()
	if rows, err := f.svc.Bury(paths, false, ""); err != nil || core.GraveFailed(rows) || claims != 1 {
		t.Fatalf("bury persisted owner repeatedly: claims=%d %+v %v", claims, rows, err)
	}
	if rows, err := f.svc.Bury([]string{f.file("d")}, false, ""); err != nil || core.GraveFailed(rows) || claims != 2 {
		t.Fatalf("new bury did not reconfirm owner: claims=%d %+v %v", claims, rows, err)
	}
}

func TestBuryRetriesUnconfirmedOwner(t *testing.T) {
	core.RequireFsync(t)
	f := newFixture(t)
	f.svc.FS = ownerBranchFS{f.fs}
	paths := []string{f.file("a"), f.file("b")}
	claims := 0
	reset := core.SetReplacementSync(func(file *os.File) error {
		if strings.HasSuffix(file.Name(), ".owner.tmp") {
			claims++
			if claims == 1 {
				return errors.New("owner flush failed")
			}
		}
		return file.Sync()
	})
	defer reset()
	rows, err := f.svc.Bury(paths, false, "")
	if err != nil || len(rows) != 2 || rows[0].Error == "" || rows[1].Action != core.ActionBuried || claims != 2 || !exists(paths[0]) {
		t.Fatalf("failed owner confirmation was cached: claims=%d %+v %v", claims, rows, err)
	}
}

func BenchmarkSweepOwnerClaims(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		base := b.TempDir()
		svc := &core.Service{Store: core.Store{Home: filepath.Join(base, "home")}, FS: ownerBranchFS{winfs.FS{}}}
		now := time.Now()
		st := core.State{NextID: 101}
		for j := range 100 {
			path := filepath.Join(base, fmt.Sprintf("payload-%d", j))
			if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
				b.Fatal(err)
			}
			id, err := svc.FS.Identify(path)
			if err != nil {
				b.Fatal(err)
			}
			st.Items = append(st.Items, core.Item{ID: int64(j + 1), Rev: 1, Path: path, Identity: &id, ExpiresAt: now.Add(-time.Hour)})
		}
		if err := svc.Store.Update(func(current *core.State) error { *current = st; return nil }); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		rows, err := svc.Sweep(false)
		b.StopTimer()
		if err != nil || len(rows) != 100 {
			b.Fatalf("sweep: rows=%d %v", len(rows), err)
		}
		for _, row := range rows {
			if row.Action != core.ActionBuried {
				b.Fatalf("sweep did not bury: %+v", row)
			}
		}
	}
}

func TestOwnerCaseSensitiveDataDirectories(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	f := newFixture(t)
	parent := filepath.Join(f.dir, "case-sensitive")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("fsutil", "file", "setCaseSensitiveInfo", parent, "enable").CombinedOutput(); err != nil {
		t.Skipf("case sensitive directories unavailable: %v %s", err, out)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(filepath.Join(parent, "Data")); err != nil {
			t.Error(err)
		}
		if err := os.RemoveAll(filepath.Join(parent, "data")); err != nil {
			t.Error(err)
		}
		if out, err := exec.Command("fsutil", "file", "setCaseSensitiveInfo", parent, "disable").CombinedOutput(); err != nil {
			t.Errorf("disable case sensitivity: %v %s", err, out)
		}
	})
	a, b := *f.svc, *f.svc
	a.Store.Home, b.Store.Home = filepath.Join(parent, "Data"), filepath.Join(parent, "data")
	for _, home := range []string{a.Store.Home, b.Store.Home} {
		if err := os.Mkdir(home, 0700); err != nil {
			t.Fatal(err)
		}
	}
	first, err := f.fs.IdentifyFollowing(a.Store.Home)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.fs.IdentifyFollowing(b.Store.Home)
	if err != nil || first.FileID == second.FileID {
		t.Fatalf("fixture identities: %+v %+v %v", first, second, err)
	}
	root := filepath.Join(f.dir, "external", "same-key")
	if err := core.CheckGraveyardOwner(&a, root, true); err != nil {
		t.Fatal(err)
	}
	if err := core.CheckGraveyardOwner(&b, root, false); err == nil {
		t.Fatal("case-sensitive Home accepted another owner's root")
	}
	// The old path-only marker must resolve its recorded spelling, too:
	// SameName alone cannot certify a legacy marker on a case-sensitive parent.
	data, _ := json.Marshal(a.Store.Home)
	if err := os.WriteFile(root+".owner", data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := core.CheckGraveyardOwner(&b, root, false); err == nil {
		t.Fatal("legacy owner migrated to another case-sensitive Home")
	}
}

func TestGraveyardOwnerWaitsForClaimLock(t *testing.T) {
	f := newFixture(t)
	if err := os.Mkdir(f.svc.Store.Home, 0700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(f.dir, "external", "key")
	if err := os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		t.Fatal(err)
	}
	release, err := core.LockFile(root+".owner.lock", true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	err = core.CheckGraveyardOwner(f.svc.WithContext(ctx), root, true)
	if err == nil || exists(root+".owner") {
		t.Fatalf("claim bypassed owner lock: %v", err)
	}
	release()
	winner := *f.svc
	winner.Store.Home = filepath.Join(f.dir, "winner-home")
	if err := os.Mkdir(winner.Store.Home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := core.CheckGraveyardOwner(&winner, root, true); err != nil {
		t.Fatal(err)
	}
	var owner struct {
		Path     string
		Identity core.Identity
	}
	id, err := f.fs.IdentifyFollowing(winner.Store.Home)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(read(t, root+".owner")), &owner); err != nil || owner.Path != winner.Store.Home || owner.Identity.Serial != id.Serial || owner.Identity.FileID != id.FileID {
		t.Fatalf("released claim did not persist successful owner's identity: %+v %v", owner, err)
	}
	if err := core.CheckGraveyardOwner(f.svc, root, false); err == nil {
		t.Fatal("timed out contender acquired ownership")
	}
}

func TestGraveyardOwnerMigratesLegacyAndRejectsReplacement(t *testing.T) {
	f := newFixture(t)
	if err := os.Mkdir(f.svc.Store.Home, 0700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(f.dir, "external", "key")
	if err := os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(strings.ToUpper(f.svc.Store.Home))
	if err := os.WriteFile(root+".owner", data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := core.CheckGraveyardOwner(f.svc, root, true); err != nil {
		t.Fatal(err)
	}
	var owner struct {
		Version  int
		Path     string
		Identity core.Identity
	}
	id, err := f.fs.IdentifyFollowing(f.svc.Store.Home)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(read(t, root+".owner")), &owner); err != nil || owner.Version != 1 || owner.Identity.Serial != id.Serial || owner.Identity.FileID != id.FileID {
		t.Fatalf("legacy marker was not upgraded: %+v %v", owner, err)
	}
	if err := os.Rename(f.svc.Store.Home, f.svc.Store.Home+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(f.svc.Store.Home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := core.CheckGraveyardOwner(f.svc, root, true); err == nil || !strings.Contains(err.Error(), "after confirming ownership, remove") {
		t.Fatalf("replacement Home reused owner identity: %v", err)
	}
}

func TestPendingGravesLeavesLegacyOwnerReadOnly(t *testing.T) {
	f := newFixture(t)
	f.svc.FS = ownerBranchFS{f.fs}
	b := buryOne(t, f, "keep.txt")
	root := filepath.Dir(filepath.Dir(b.Grave))
	data, _ := json.Marshal(f.svc.Store.Home)
	if err := os.WriteFile(root+".owner", data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(root + ".owner.lock"); err != nil {
		t.Fatal(err)
	}
	if pending, err := f.svc.PendingGraves(); err != nil || len(pending) != 0 {
		t.Fatalf("legacy diagnostic: %+v %v", pending, err)
	}
	if read(t, root+".owner") != string(data) || exists(root+".owner.lock") {
		t.Fatal("read-only diagnostic migrated owner marker")
	}
	if err := core.CheckGraveyardOwner(f.svc, root, false); err != nil {
		t.Fatal(err)
	}
	if read(t, root+".owner") == string(data) {
		t.Fatal("ordinary operation did not migrate legacy marker")
	}
}

func TestGraveyardOwnerRejectsUnicodeKeyCollision(t *testing.T) {
	f := newFixture(t)
	first, second := *f.svc, *f.svc
	first.Store.Home = filepath.Join(filepath.Dir(f.dir), "K")
	second.Store.Home = filepath.Join(filepath.Dir(f.dir), "K")
	for _, home := range []string{first.Store.Home, second.Store.Home} {
		if err := os.Mkdir(home, 0700); err != nil {
			t.Fatal(err)
		}
	}
	a, err := f.fs.Identify(first.Store.Home)
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.fs.Identify(second.Store.Home)
	if err != nil || a == b {
		t.Fatalf("fixture needs two distinct NTFS directories: %v", err)
	}
	key := filepath.Base(core.GraveyardRoot(&first, `Z:\payload.txt`))
	if key != filepath.Base(core.GraveyardRoot(&second, `Z:\payload.txt`)) {
		t.Fatal("existing Unicode-folded ledger key was changed")
	}
	// Creating an actual cross-volume root would touch the real .obituary.
	// Exercise the same ownership and scan boundaries on a root in scratch.
	root := filepath.Join(f.dir, "external", key)
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(root, "legacy-payload")
	if err := os.WriteFile(payload, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := core.CheckGraveyardOwner(&first, root, false); err != nil {
		t.Fatal("unmarked legacy root rejected", err)
	}
	if exists(root + ".owner") {
		t.Fatal("read-only ownership check claimed a legacy root")
	}
	if err := core.CheckGraveyardOwner(&first, root, true); err != nil {
		t.Fatal(err)
	}
	for _, create := range []bool{false, true} {
		if err := core.CheckGraveyardOwner(&second, root, create); err == nil || !strings.Contains(err.Error(), "ledger key collision") || !strings.Contains(err.Error(), "OBIT_HOME") {
			t.Fatalf("colliding Home accepted another ledger's graveyard: create=%v %v", create, err)
		}
	}
	if rows, err := core.ScanGraveyardRoot(&second, root); err == nil || len(rows) != 1 || rows[0].Status != "unverifiable" || !strings.Contains(rows[0].Error, "ledger key collision") {
		t.Fatalf("colliding ledger scanned another owner's root: %+v %v", rows, err)
	}
	var owner struct {
		Path     string
		Identity core.Identity
	}
	if err := json.Unmarshal([]byte(read(t, root+".owner")), &owner); err != nil || owner.Path != first.Store.Home || owner.Identity.FileID != a.FileID {
		t.Fatalf("owner marker lost exact Home spelling: %+v %v", owner, err)
	}
	first.Store.Home = `\\?\` + filepath.Join(filepath.Dir(first.Store.Home), "k")
	if err := core.CheckGraveyardOwner(&first, root, true); err != nil {
		t.Fatal("ordinary casing or extended spelling changed ownership", err)
	}
	if read(t, payload) != "keep" {
		t.Fatal("ownership check changed the legacy root")
	}
}

func TestGraveyardOwnerSerializesFirstClaim(t *testing.T) {
	f := newFixture(t)
	a, b := *f.svc, *f.svc
	a.Store.Home = filepath.Join(f.dir, "K")
	b.Store.Home = filepath.Join(f.dir, "K")
	for _, home := range []string{a.Store.Home, b.Store.Home} {
		if err := os.Mkdir(home, 0700); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(f.dir, "external", "same-key")
	start := make(chan struct{})
	done := make(chan error, 2)
	for _, svc := range []*core.Service{&a, &b} {
		go func() { <-start; done <- core.CheckGraveyardOwner(svc, root, true) }()
	}
	close(start)
	first, second := <-done, <-done
	if (first == nil) == (second == nil) {
		t.Fatalf("competing ledgers both claimed one root: %v %v", first, second)
	}
	var owner struct {
		Path     string
		Identity core.Identity
	}
	if err := json.Unmarshal([]byte(read(t, root+".owner")), &owner); err != nil || owner.Path != a.Store.Home && owner.Path != b.Store.Home {
		t.Fatalf("concurrent claim left a partial owner: %+v %v", owner, err)
	}
}
