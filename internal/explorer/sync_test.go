//go:build windows

package explorer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/project"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/windows"
)

func TestSyncScope(t *testing.T) {
	for _, tc := range []struct {
		name, parent, source              string
		watchID                           int64
		everywhere, git, inGit, wantLabel bool
	}{
		{name: "downloads", parent: "Downloads", wantLabel: true},
		{name: "downloads child", parent: "Downloads/child/nested", wantLabel: true},
		{name: "desktop", parent: "Desktop", wantLabel: true},
		{name: "desktop child", parent: "Desktop/child", wantLabel: true},
		{name: "download sibling", parent: "Downloads-old"},
		{name: "desktop sibling", parent: "Desktop-old"},
		{name: "scratch", parent: "scratch", source: "mktemp"},
		{name: "manual", parent: "work", source: "cli"},
		{name: "policy ID", parent: "work", watchID: 12, wantLabel: true},
		{name: "policy source", parent: "work", source: "watch", wantLabel: true},
		{name: "everywhere", parent: "work", everywhere: true, wantLabel: true},
		{name: "downloads git", parent: "Downloads/repo", git: true},
		{name: "downloads git opt in", parent: "Downloads/repo", git: true, inGit: true, wantLabel: true},
		{name: "desktop git", parent: "Desktop/repo", git: true},
		{name: "desktop git opt in", parent: "Desktop/repo", git: true, inGit: true, wantLabel: true},
		{name: "policy git", parent: "work", watchID: 12, git: true},
		{name: "policy git opt in", parent: "work", source: "watch", git: true, inGit: true, wantLabel: true},
		{name: "everywhere git", parent: "work", everywhere: true, git: true},
		{name: "everywhere git opt in", parent: "work", everywhere: true, git: true, inGit: true, wantLabel: true},
		{name: "in git does not widen scope", parent: "work", git: true, inGit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if testing.Short() && tc.git {
				t.Skip("checks scope and exclusions in a real Git repository")
			}
			svc, root := syncService(t), t.TempDir()
			folders := []string{filepath.Join(root, "Downloads"), filepath.Join(root, "Desktop")}
			for _, folder := range folders {
				if err := os.Mkdir(folder, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			dir := filepath.Join(root, filepath.FromSlash(tc.parent))
			item := trackedFile(t, dir, "report.log")
			item.WatchID, item.Source = tc.watchID, tc.source
			storeItems(t, svc, item)
			if tc.git {
				initLabelRepo(t, dir)
			}
			before, err := attributes(dir)
			if err != nil {
				t.Fatal(err)
			}
			options := Options{Enabled: true, InGit: tc.inGit, Everywhere: tc.everywhere, Folders: folders}
			if err := Sync(svc, options); err != nil {
				t.Fatal(err)
			}
			assertLabel(t, dir, "report.log", tc.wantLabel)
			if after, err := attributes(dir); !tc.wantLabel && (err != nil || before != after) {
				t.Fatalf("out-of-scope attributes: %x -> %x: %v", before, after, err)
			}
		})
	}
}

func initLabelRepo(t *testing.T, dir string) {
	t.Helper()
	testenv.InitRepo(t, dir)
}

func assertLabel(t *testing.T, dir, name string, want bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, iniName))
	if os.IsNotExist(err) && !want {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	text, err := decode(data)
	if err != nil || strings.Contains(text, name+"=") != want {
		t.Fatalf("label %s want=%v: %q %v", name, want, text, err)
	}
}

func TestSyncKnownFolderPathSpellings(t *testing.T) {
	svc := syncService(t)
	folder := filepath.Join(t.TempDir(), "Redirected Downloads With Spaces")
	item := trackedFile(t, filepath.Join(folder, strings.Repeat("long", 30), strings.Repeat("name", 30)), "report.log")
	storeItems(t, svc, item)
	path, err := windows.UTF16PtrFromString(folder)
	if err != nil {
		t.Fatal(err)
	}
	var short [windows.MAX_PATH]uint16
	if _, err := windows.GetShortPathName(path, &short[0], uint32(len(short))); err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{strings.ToUpper(folder), `\\?\` + folder, windows.UTF16ToString(short[:])} {
		if err := Sync(svc, Options{Enabled: true, Folders: []string{spelling}}); err != nil {
			t.Fatal(err)
		}
		assertLabel(t, filepath.Dir(item.Path), "report.log", true)
		if err := Sync(svc, Options{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSyncFolderPolicyDoesNotIncludeOtherItemsInDirectory(t *testing.T) {
	svc, dir := syncService(t), t.TempDir()
	manual := trackedFile(t, dir, "manual.log")
	policy := trackedFile(t, dir, "policy.log")
	policy.WatchID = 42
	storeItems(t, svc, manual, policy)
	if err := Sync(svc, Options{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	assertLabel(t, dir, "manual.log", false)
	assertLabel(t, dir, "policy.log", true)
}

type replaceLocatedDirectory struct {
	winfs.FS
	want    core.Identity
	replace func()
}

func (fs *replaceLocatedDirectory) Locate(id core.Identity) (string, error) {
	path, err := fs.FS.Locate(id)
	if err == nil && id == fs.want && fs.replace != nil {
		f := fs.replace
		fs.replace = nil
		f()
	}
	return path, err
}

func TestSyncRefusesDirectoryReplacedAfterLocate(t *testing.T) {
	for _, cleanup := range []bool{false, true} {
		t.Run(fmt.Sprint(cleanup), func(t *testing.T) { replacedLabelDirectory(t, cleanup, false) })
	}
}

func TestAuditItemLocationReplacedBeforeParent(t *testing.T) {
	svc, root := syncService(t), t.TempDir()
	dir, replacement := filepath.Join(root, "target"), filepath.Join(root, "replacement")
	item := trackedFile(t, dir, "report.log")
	storeItems(t, svc, item)
	trackedFile(t, replacement, "report.log")
	const user = "[User]\nImportant=preserve me\n"
	if err := os.WriteFile(filepath.Join(replacement, iniName), []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	svc.FS = &replaceLocatedDirectory{want: *item.Identity, replace: func() {
		if err := os.Rename(dir, filepath.Join(root, "old")); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(replacement, dir); err != nil {
			t.Fatal(err)
		}
	}}
	err := Sync(svc, Options{Enabled: true, Everywhere: true})
	if err == nil || string(readINI(t, dir)) != user {
		t.Fatalf("item location replacement was authorized: %v", err)
	}
}

func TestAuditDirectoryReplacedBeforeIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("discovers an actual Git repository during directory replacement")
	}
	svc, root := syncService(t), t.TempDir()
	dir, replacement := filepath.Join(root, "target"), filepath.Join(root, "replacement")
	storeItems(t, svc, trackedFile(t, dir, "report.log"))
	trackedFile(t, replacement, "report.log")
	initLabelRepo(t, replacement)
	const user = "[User]\nImportant=preserve me\n"
	if err := os.WriteFile(filepath.Join(replacement, iniName), []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	original, swapped, attempted := findProject, false, false
	findProject = func(ctx context.Context, path string) (project.Project, bool, error) {
		proj, found, err := original(ctx, path)
		attempted = true
		if moveErr := os.Rename(dir, filepath.Join(root, "old")); moveErr == nil {
			if err := os.Rename(replacement, dir); err != nil {
				t.Fatal(err)
			}
			swapped = true
		}
		return proj, found, err
	}
	t.Cleanup(func() { findProject = original })
	err := Sync(svc, Options{Enabled: true, Everywhere: true})
	if !attempted {
		t.Fatal("project discovery was not exercised")
	}
	if swapped && (err == nil || string(readINI(t, dir)) != user) {
		t.Fatalf("Git directory replacement was authorized: %v", err)
	}
	if !swapped && string(readINI(t, replacement)) != user {
		t.Fatal("unrelated Git INI changed")
	}
}

func TestSyncDropsNeverWrittenCandidates(t *testing.T) {
	svc, root := syncService(t), t.TempDir()
	for i := 0; i < 4; i++ {
		dir := filepath.Join(root, fmt.Sprint(i))
		storeItems(t, svc, trackedFile(t, dir, "report.log"))
		if err := os.WriteFile(filepath.Join(dir, iniName), []byte{0xff}, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Sync(svc, Options{Enabled: true, Everywhere: true}); err == nil {
			t.Fatal("invalid INI accepted")
		}
	}
	storeItems(t, svc)
	err := Sync(svc, Options{})
	index, readErr := loadDirectories(filepath.Join(svc.Store.Home, "explorer.json"))
	if err != nil || readErr != nil || len(index) != 0 {
		t.Fatalf("never-written cleanup targets retained: %v %v %v", index, err, readErr)
	}
}

func TestSyncKeepsWrittenTargetAfterPrepareFailure(t *testing.T) {
	svc, dir := syncService(t), t.TempDir()
	storeItems(t, svc, trackedFile(t, dir, "report.log"))
	if err := Sync(svc, Options{Enabled: true, Everywhere: true}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, iniName), []byte{0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	storeItems(t, svc)
	err := Sync(svc, Options{})
	index, readErr := loadDirectories(filepath.Join(svc.Store.Home, "explorer.json"))
	if err == nil || readErr != nil || len(index) != 1 || !bytes.Equal(readINI(t, dir), []byte{0xff}) {
		t.Fatalf("written cleanup target lost on planning failure: %v %v %v", index, err, readErr)
	}
}

func TestSyncRefusesGitDirectoryReplacedAfterLocate(t *testing.T) {
	if testing.Short() {
		t.Skip("creates a real Git repository")
	}
	replacedLabelDirectory(t, false, true)
}

func replacedLabelDirectory(t *testing.T, cleanup, git bool) {
	t.Helper()
	svc, root := syncService(t), t.TempDir()
	dir, replacement, old := filepath.Join(root, "target"), filepath.Join(root, "replacement"), filepath.Join(root, "old")
	storeItems(t, svc, trackedFile(t, dir, "report.log"))
	trackedFile(t, replacement, "report.log")
	const user = "[User]\nImportant=preserve me\n"
	if err := os.WriteFile(filepath.Join(replacement, iniName), []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	if git {
		initLabelRepo(t, replacement)
	}
	options := Options{Enabled: true, Everywhere: true}
	if cleanup {
		if err := Sync(svc, options); err != nil {
			t.Fatal(err)
		}
		options.Enabled = false
	}
	id, err := svc.FS.Identify(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc.FS = &replaceLocatedDirectory{want: id, replace: func() {
		if err := os.Rename(dir, old); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(replacement, dir); err != nil {
			t.Fatal(err)
		}
	}}
	err = Sync(svc, options)
	index, indexErr := loadDirectories(filepath.Join(svc.Store.Home, "explorer.json"))
	wantTargets := 0
	if cleanup {
		wantTargets = 1
	}
	if err == nil || string(readINI(t, dir)) != user || indexErr != nil || len(index) != wantTargets {
		t.Fatalf("replaced directory accepted or cleanup target lost: %v index=%v %v", err, index, indexErr)
	}
	if cleanup {
		assertLabel(t, old, "report.log", true)
	}
}

func TestSyncSkipsLabelInfrastructure(t *testing.T) {
	svc, dir := syncService(t), t.TempDir()
	items := []core.Item{trackedFile(t, dir, "report.log"), trackedFile(t, dir, "DESKTOP.INI"), trackedFile(t, dir, ".obit-labels-user.tmp")}
	if err := os.WriteFile(filepath.Join(dir, iniName), []byte("; user"), 0o600); err != nil {
		t.Fatal(err)
	}
	storeItems(t, svc, items...)
	if err := Sync(svc, Options{Enabled: true, Everywhere: true}); err != nil {
		t.Fatalf("infrastructure blocked other labels: %v", err)
	}
	text, _ := decode(readINI(t, dir))
	if !strings.Contains(text, "report.log=") || strings.Contains(text, ".obit-labels-user.tmp=") || strings.Contains(text, "DESKTOP.INI=") {
		t.Fatalf("infrastructure projected as a label: %q", text)
	}
}

func TestSyncTrackedFolderUsesParentScope(t *testing.T) {
	for _, child := range []string{"Downloads", "Downloads/cache", "scratch/obit-tmp"} {
		t.Run(child, func(t *testing.T) {
			svc, root := syncService(t), t.TempDir()
			downloads := filepath.Join(root, "Downloads")
			if err := os.Mkdir(downloads, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, filepath.FromSlash(child))
			if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
			identity, err := svc.FS.Identify(path)
			if err != nil {
				t.Fatal(err)
			}
			storeItems(t, svc, core.Item{Path: path, IsDir: true, Identity: &identity, Source: "mktemp", ExpiresAt: time.Now().Add(time.Hour)})
			if err := Sync(svc, Options{Enabled: true, Folders: []string{downloads}}); err != nil {
				t.Fatal(err)
			}
			assertLabel(t, filepath.Dir(path), filepath.Base(path), child == "Downloads/cache")
		})
	}
}

func TestSyncRemovesLegacyOutOfScopeLabels(t *testing.T) {
	for _, kind := range []string{"absent", "user", "readonly", "later edit"} {
		t.Run(kind, func(t *testing.T) {
			svc, dir := syncService(t), t.TempDir()
			storeItems(t, svc, trackedFile(t, dir, "report.log"))
			path := filepath.Join(dir, iniName)
			original := []byte("; user comment\n[.ShellClassInfo]\nIconResource=own.ico,0\n[LocalizedFileNames]\nkeep.txt=User name\n")
			if kind != "absent" {
				if err := os.WriteFile(path, original, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "readonly" {
				if err := setAttributes(dir, windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_SYSTEM); err != nil {
					t.Fatal(err)
				}
			}
			before, err := attributes(dir)
			if err != nil {
				t.Fatal(err)
			}
			// everywhere reproduces the previous default, including explorer.json.
			if err := Sync(svc, Options{Enabled: true, Everywhere: true}); err != nil {
				t.Fatal(err)
			}
			assertLabel(t, dir, "report.log", true)
			if kind == "later edit" {
				text, err := decode(readINI(t, dir))
				if err != nil {
					t.Fatal(err)
				}
				const added = "[User]\r\nNew=kept\r\n"
				if err := os.WriteFile(path, encode(text+added), 0o600); err != nil {
					t.Fatal(err)
				}
				original = encode(string(original) + added)
			}
			if err := Sync(svc, Options{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			assertLabel(t, dir, "report.log", false)
			if after, err := attributes(dir); err != nil || before != after {
				t.Fatalf("folder attributes: %x -> %x: %v", before, after, err)
			}
			if kind == "absent" {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("owned INI remains: %v", err)
				}
			} else if !bytes.Equal(readINI(t, dir), original) {
				t.Fatal("user INI changed during scope migration")
			}
			index, err := loadDirectories(filepath.Join(svc.Store.Home, "explorer.json"))
			if err != nil || len(index) != 0 {
				t.Fatalf("cleanup targets remain: %v %v", index, err)
			}
		})
	}
}

type countingFS struct {
	winfs.FS
	identifications int
}

func (fs *countingFS) Identify(path string) (core.Identity, error) {
	fs.identifications++
	return fs.FS.Identify(path)
}

func TestSyncCachesDirectoryDiscoveryWithinEachSync(t *testing.T) {
	if testing.Short() {
		t.Skip("rechecks discovery after creating a real Git repository")
	}
	assertDirectoryDiscovery(t, true)
}

func TestSyncCachesNonGitDirectoryDiscovery(t *testing.T) {
	assertDirectoryDiscovery(t, false)
}

func assertDirectoryDiscovery(t *testing.T, withGit bool) {
	t.Helper()
	svc, dir := syncService(t), t.TempDir()
	var items []core.Item
	for i := 0; i < 5; i++ {
		items = append(items, trackedFile(t, dir, fmt.Sprintf("report-%d.log", i)))
	}
	storeItems(t, svc, items...)
	fs := &countingFS{}
	svc.FS = fs
	finds, original := 0, findProject
	findProject = func(ctx context.Context, path string) (project.Project, bool, error) {
		finds++
		return original(ctx, path)
	}
	t.Cleanup(func() { findProject = original })
	steps := []string{"plain", "repeat"}
	if withGit {
		steps = append(steps, "git excluded", "git enabled")
	}
	for _, step := range steps {
		if step == "git excluded" {
			initLabelRepo(t, dir)
		}
		finds, fs.identifications = 0, 0
		options := Options{Enabled: true, Everywhere: true, InGit: step == "git enabled"}
		if err := Sync(svc, options); err != nil {
			t.Fatal(err)
		}
		wantIDs := 1
		if step == "git excluded" {
			wantIDs = 0
		}
		// Directory identities now come from the pinned handle, never Identify(path).
		if finds != 1 || fs.identifications != 0 {
			t.Fatalf("%s: directory discoveries=%d identifications=%d", step, finds, fs.identifications)
		}
		assertLabel(t, dir, "report-0.log", wantIDs == 1)
	}
}

func BenchmarkSync500(b *testing.B) {
	for _, directories := range []int{1, 10} {
		b.Run(fmt.Sprintf("%ddirs", directories), func(b *testing.B) {
			svc, root := syncService(b), b.TempDir()
			items := make([]core.Item, 500)
			for i := range items {
				dir := filepath.Join(root, fmt.Sprintf("dir-%d", i%directories))
				items[i] = trackedFile(b, dir, fmt.Sprintf("report-%d.log", i))
			}
			storeItems(b, svc, items...)
			options := Options{Enabled: true, Folders: []string{root}}
			if err := Sync(svc, options); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for b.Loop() {
				if err := Sync(svc, options); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
