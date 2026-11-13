//go:build linux || darwin

package winfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

func TestPOSIXRapidRecreationHasDistinctIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload")
	seen := map[corefs.Identity]bool{}
	for i := 0; i < 256; i++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		id := posixID(t, path)
		if seen[id] {
			t.Fatalf("rapid recreation reused a durable identity at iteration %d: %+v", i, id)
		}
		seen[id] = true
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPOSIXRecreatedObjectIsRetained(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "directory"}[directory], func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "payload")
			makePOSIXPayload(t, path, directory, "old")
			now := time.Now()
			moves := 0
			svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: FS{beforeRename: func() { moves++ }}, Now: func() time.Time { return now }}
			rows, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: now.Add(time.Hour)})
			if err != nil || core.Failed(rows) {
				t.Fatalf("add: %+v %v", rows, err)
			}
			old := *rows[0].Item.Identity
			if directory {
				if err := os.Remove(filepath.Join(path, "child")); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			makePOSIXPayload(t, path, directory, "replacement")
			got := posixID(t, path)
			if got == old || strings.HasPrefix(got.FileID, "0000000000000000") {
				t.Fatalf("recreated object lacks distinct durable identity: old=%+v new=%+v", old, got)
			}
			now = now.Add(2 * time.Hour)
			out, err := svc.Sweep(false)
			if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry {
				t.Fatalf("replacement was not retained: %+v %v", out, err)
			}
			if moves != 0 {
				t.Fatalf("replacement reached a rename: %d", moves)
			}
			assertPOSIXPayload(t, path, directory, got, "replacement")
			st, err := svc.Store.ReadSnapshot()
			if err != nil || len(st.Items) != 1 || len(st.Burials) != 0 {
				t.Fatalf("replacement ledger: %+v %v", st, err)
			}
		})
	}
}

func TestPOSIXPersistentIdentitySurvivesEdits(t *testing.T) {
	dir := t.TempDir()
	for _, directory := range []bool{false, true} {
		path := filepath.Join(dir, map[bool]string{false: "file", true: "directory"}[directory])
		makePOSIXPayload(t, path, directory, "old")
		want := posixID(t, path)
		if strings.HasPrefix(want.FileID, "0000000000000000") {
			t.Fatal("missing persistent component")
		}
		content := path
		if directory {
			content = filepath.Join(path, "child")
		}
		if err := os.WriteFile(content, []byte("edited"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		if got := posixID(t, path); got != want {
			t.Fatalf("chmod changed identity: %+v %+v", want, got)
		}
		if err := os.Chmod(path, 0700); err != nil {
			t.Fatal(err)
		}
		moved := path + "-renamed"
		mustPOSIXRename(t, path, moved)
		assertPOSIXPayload(t, moved, directory, want, "edited")
		svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: FS{}}
		buried, err := svc.Bury([]string{moved}, false, "")
		if err != nil || core.GraveFailed(buried) {
			t.Fatalf("edited object burial: %+v %v", buried, err)
		}
		assertPOSIXPayload(t, buried[0].Burial.Grave, directory, want, "edited")
		restored, err := svc.Resurrect([]string{moved})
		if err != nil || core.GraveFailed(restored) {
			t.Fatalf("edited object restore: %+v %v", restored, err)
		}
		assertPOSIXPayload(t, moved, directory, want, "edited")
	}
}

func TestPOSIXIdentityAndLinks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	fs := FS{}
	want, err := fs.Identify(path)
	if err != nil || len(want.FileID) != 32 {
		t.Fatal(want, err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	linkID, err := fs.Identify(link)
	if err != nil || linkID == want {
		t.Fatalf("symlink followed: %+v %v", linkID, err)
	}
	followed, err := fs.IdentifyFollowing(link)
	if err != nil || followed != want {
		t.Fatal(followed, err)
	}
	renamed := filepath.Join(dir, "renamed")
	if err := os.Rename(path, renamed); err != nil {
		t.Fatal(err)
	}
	got, err := fs.Identify(renamed)
	if err != nil || got != want {
		t.Fatalf("rename changed identity: %+v %v", got, err)
	}
	if _, err := fs.Locate(want); !errors.Is(err, corefs.ErrUnsupported) || errors.Is(err, corefs.ErrNotFound) {
		t.Fatalf("unknown inode was declared missing: %v", err)
	}
	if err := os.Link(renamed, path); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Identify(path); !errors.Is(err, corefs.ErrUnsafe) {
		t.Fatalf("hard link accepted: %v", err)
	}
	if got, err := fs.IdentifyFollowing(renamed); err != nil || got != want {
		t.Fatalf("following identity lost its metadata-only contract: %+v %v", got, err)
	}
	if _, err := fs.Identify(filepath.Join(dir, "absent")); !errors.Is(err, corefs.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestPOSIXCanonicalPreservesFinalLinkAndCase(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "Actual")
	if err := os.Mkdir(actual, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(actual, alias); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(actual, "link")
	if err := os.Symlink("missing", leaf); err != nil {
		t.Fatal(err)
	}
	got, err := Canonical(filepath.Join(alias, "link"))
	if err != nil || got != leaf {
		t.Fatal(got, err)
	}
	if same, err := (FS{}).SameName("A", "a"); err != nil || same {
		t.Fatal(same, err)
	}
	id := corefs.Identity{Volume: "/CaseSensitive", Serial: 1, FileID: "1"}
	if id.Normalized() != id {
		t.Fatal("mount name casing changed")
	}
}

func changedBirth(id corefs.Identity) corefs.Identity {
	if id.FileID[0] == '1' {
		id.FileID = "2" + id.FileID[1:]
	} else {
		id.FileID = "1" + id.FileID[1:]
	}
	return id
}
