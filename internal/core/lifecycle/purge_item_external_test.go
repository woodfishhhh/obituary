//go:build linux || darwin

package lifecycle_test

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"golang.org/x/sys/unix"
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

func TestPurgeSupportsPOSIXSpecialEntries(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*testing.T, *fixture, string) func(*testing.T)
	}{
		{
			name: "symlink",
			prepare: func(t *testing.T, f *fixture, payload string) func(*testing.T) {
				outside := f.dir + "-outside"
				sentinel := filepath.Join(outside, "sentinel")
				if err := os.MkdirAll(outside, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(sentinel, []byte("outside sentinel"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(sentinel, filepath.Join(payload, "link")); err != nil {
					t.Fatal(err)
				}
				return func(t *testing.T) {
					if got, err := os.ReadFile(sentinel); err != nil || string(got) != "outside sentinel" {
						t.Fatalf("outside sentinel changed: %q %v", got, err)
					}
				}
			},
		},
		{
			name: "closed-unix-socket",
			prepare: func(t *testing.T, _ *fixture, payload string) func(*testing.T) {
				t.Chdir(payload)
				const name = "s.sock" // Keep the bind address short even under a long TMPDIR.
				listener, err := net.Listen("unix", name)
				if err != nil {
					t.Fatal(err)
				}
				listener.(*net.UnixListener).SetUnlinkOnClose(false)
				if err := listener.Close(); err != nil {
					t.Fatal(err)
				}
				graveSocket := filepath.Join(payload, name)
				if _, err := os.Lstat(graveSocket); err != nil {
					t.Fatalf("closed socket node missing: %v", err)
				}
				return func(t *testing.T) {
					if exists(graveSocket) {
						t.Fatal("socket node survived purge")
					}
				}
			},
		},
	}

	for _, tc := range cases {
		for _, operation := range []string{"PurgeRefs", "Tick"} {
			t.Run(tc.name+"/"+operation, func(t *testing.T) {
				f := newFixture(t)
				payload := filepath.Join(f.dir, "payload")
				if err := os.Mkdir(payload, 0o700); err != nil {
					t.Fatal(err)
				}
				check := tc.prepare(t, f, payload)
				rows, err := f.svc.Bury([]string{payload}, true, "")
				if err != nil || len(rows) != 1 || rows[0].Burial == nil {
					t.Fatal(rows, err)
				}
				burial := rows[0].Burial
				f.clock = burial.PurgeAt.Add(time.Hour)
				var purged []core.GraveResult
				if operation == "PurgeRefs" {
					purged, err = f.svc.PurgeRefs([]string{fmt.Sprint(burial.ID)}, false)
				} else {
					_, purged, err = f.svc.Tick()
				}
				if err != nil || len(purged) != 1 || purged[0].Action != core.ActionPurged || exists(burial.Grave) || len(f.burials()) != 0 {
					t.Fatalf("%s failed: %+v %v", operation, purged, err)
				}
				check(t)
			})
		}
	}
}

func TestPurgeProtectsActiveMovedSymlink(t *testing.T) {
	for _, operation := range []string{"PurgeRefs", "Tick"} {
		t.Run(operation, func(t *testing.T) {
			f := newFixture(t)
			original := f.file("tree/sub/original")
			sub := filepath.Dir(original)
			fd, err := unix.Open(sub, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer unix.Close(fd)

			outside := f.dir + "-outside"
			target := filepath.Join(outside, "target")
			if err := os.MkdirAll(outside, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("active target"), 0o600); err != nil {
				t.Fatal(err)
			}
			active := filepath.Join(f.dir, "active-link")
			if err := os.Symlink(target, active); err != nil {
				t.Fatal(err)
			}
			identity, err := f.fs.Identify(active)
			if err != nil {
				t.Fatal(err)
			}
			var itemID int64
			if err := f.svc.Store.Update(func(st *core.State) error {
				itemID = st.NextID
				identityCopy := identity
				st.Items = append(st.Items, core.Item{ID: itemID, Rev: 1, Path: active, Identity: &identityCopy, CreatedAt: f.clock, ExpiresAt: f.clock.Add(24 * time.Hour), Source: "cli"})
				st.NextID++
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			rows, err := f.svc.Bury([]string{filepath.Dir(sub)}, true, "")
			if err != nil || len(rows) != 1 || rows[0].Burial == nil {
				t.Fatal(rows, err)
			}
			burial := rows[0].Burial
			if err := unix.Renameat(unix.AT_FDCWD, active, fd, "arrived"); err != nil {
				t.Fatal(err)
			}
			f.clock = burial.PurgeAt.Add(time.Hour)
			var purged []core.GraveResult
			if operation == "PurgeRefs" {
				purged, err = f.svc.PurgeRefs([]string{fmt.Sprint(burial.ID)}, false)
			} else {
				_, purged, err = f.svc.Tick()
			}
			link := filepath.Join(burial.Grave, "sub", "arrived")
			gotTarget, targetErr := os.Readlink(link)
			if err != nil || len(purged) != 1 || purged[0].Error == "" || !strings.Contains(purged[0].Error, "protected") || len(f.burials()) != 1 || len(f.items()) != 1 || f.items()[0].ID != itemID || targetErr != nil || gotTarget != target {
				t.Fatalf("%s removed protected symlink: %+v %v target=%q targetErr=%v", operation, purged, err, gotTarget, targetErr)
			}
		})
	}
}
