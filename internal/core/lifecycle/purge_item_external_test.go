//go:build linux || darwin

package lifecycle_test

import (
	"crypto/rand"
	"fmt"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPurgeStageRecovery(t *testing.T) {
	for _, kind := range []string{"permission", "empty", "legacy-empty", "staged", "active", "stranger-file", "stranger-dir", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			orig := f.file("tree/sub/keep")
			dir := filepath.Dir(filepath.Dir(orig))
			rows, err := f.svc.Bury([]string{dir}, true, "")
			if err != nil || len(rows) != 1 || rows[0].Burial == nil {
				t.Fatal(rows, err)
			}
			b := rows[0].Burial
			member := filepath.Join(b.Grave, "sub", "keep")
			root := filepath.Dir(b.Grave)
			ref := []string{fmt.Sprint(b.ID)}
			token := ".obit-purge-" + rand.Text()
			stage := filepath.Join(root, token)
			recorded := token
			if kind == "legacy-empty" {
				recorded = ""
			}
			if kind != "permission" {
				if err := f.svc.Store.UpdateGraveyard(func(st *core.State) error {
					st.Burials[0].PurgeIntent = &core.PurgeIntent{VerifiedAt: f.clock, Basis: "explicit selection", Stage: recorded}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "permission":
				if os.Geteuid() == 0 {
					t.Skip("requires ordinary permissions")
				}
				sub := filepath.Dir(member)
				if err := os.Chmod(sub, 0); err != nil {
					t.Fatal(err)
				}
				rows, err = f.svc.PurgeRefs(ref, false)
				chmod := os.Chmod(sub, 0700)
				if chmod != nil || err != nil || len(rows) != 1 || !strings.Contains(rows[0].Error, "permission denied") {
					t.Fatal(rows, err, chmod)
				}
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 1 || read(t, member) != "tree/sub/keep" {
					t.Fatal(entries, err)
				}
			case "empty", "legacy-empty", "staged", "active":
				if err := os.Mkdir(stage, 0700); err != nil {
					t.Fatal(err)
				}
				if kind == "staged" || kind == "active" {
					origin := member
					if kind == "active" {
						origin = f.add(f.file("late"), 24*time.Hour).Path
					}
					id, err := f.fs.Identify(origin)
					if err != nil {
						t.Fatal(err)
					}
					to := filepath.Join(stage, id.FileID)
					if err := os.Rename(origin, to); err != nil {
						t.Fatal(err)
					}
					if kind == "active" {
						rows, err = f.svc.PurgeRefs(ref, false)
						if err != nil || len(rows) != 1 || rows[0].Error == "" || read(t, to) != "late" || read(t, member) != "tree/sub/keep" {
							t.Fatal(rows, err)
						}
						return
					}
				}
			default:
				foreign := filepath.Join(root, "foreign")
				switch kind {
				case "stranger-file":
					err = os.WriteFile(foreign, []byte("foreign"), 0600)
				case "stranger-dir":
					err = os.Mkdir(filepath.Join(root, ".obit-purge-"+rand.Text()), 0700)
				case "symlink":
					err = os.Symlink(dir, stage)
				}
				if err != nil {
					t.Fatal(err)
				}
				rows, err = f.svc.PurgeRefs(ref, false)
				if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Error, "holds something besides the buried object") || read(t, member) != "tree/sub/keep" {
					t.Fatal(rows, err)
				}
				return
			}
			rows, err = f.svc.PurgeRefs(ref, false)
			if err != nil || len(rows) != 1 || rows[0].Action != core.ActionPurged || exists(root) || len(f.burials()) != 0 {
				t.Fatal(rows, err)
			}
		})
	}
}

// A producer can retain a directory descriptor across a rename. Finish fixture
// setup before starting either purge so the retention result is deterministic.
func TestPublishedGraveWithRetainedDirectoryDescriptor(t *testing.T) {
	for _, entry := range []string{"PurgeRefs", "Tick"} {
		t.Run(entry, func(t *testing.T) {
			f := newFixture(t)
			orig := f.file("tree/sub/original")
			sub := filepath.Dir(orig)
			fd, err := unix.Open(sub, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer unix.Close(fd)
			f.file("tree/sibling")
			keep := f.add(f.file("long-lived"), 30*24*time.Hour)
			rows, err := f.svc.Bury([]string{filepath.Dir(sub)}, true, "")
			if err != nil || len(rows) != 1 || rows[0].Burial == nil {
				t.Fatal(rows, err)
			}
			b := rows[0].Burial
			if err := unix.Renameat(unix.AT_FDCWD, keep.Path, fd, "arrived"); err != nil {
				t.Fatal(err)
			}
			f.clock = b.PurgeAt.Add(time.Hour)
			if entry == "Tick" {
				_, rows, err = f.svc.Tick()
			} else {
				rows, err = f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
			}
			if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Error, "protected") || len(f.items()) != 1 || len(f.burials()) != 1 {
				t.Fatal(rows, err)
			}
			for name, want := range map[string]string{"sub/arrived": "long-lived", "sub/original": "tree/sub/original", "sibling": "tree/sibling"} {
				if got := read(t, filepath.Join(b.Grave, name)); got != want {
					t.Fatal(name, got)
				}
			}
		})
	}
}
