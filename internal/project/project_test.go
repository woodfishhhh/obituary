//go:build windows

package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/windows"
)

func TestRelShortAndLongPaths(t *testing.T) {
	root := t.TempDir()
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 32768)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n >= uint32(len(buf)) {
		t.Fatalf("short path: %d %v", n, err)
	}
	short := windows.UTF16ToString(buf[:n])
	if short == root {
		t.Skip("volume does not generate 8.3 aliases")
	}
	if err := os.WriteFile(filepath.Join(root, "debug.log"), []byte("scratch"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{root, short} {
		proj := Project{Root: base}
		for _, spelling := range []string{root, short} {
			for _, name := range []string{"debug.log", "missing/nested/debug.log"} {
				got, err := proj.Rel(filepath.Join(spelling, filepath.FromSlash(name)))
				if err != nil || got != name {
					t.Errorf("Rel(%q, %q) = %q %v", base, spelling, got, err)
				}
			}
		}
		if got, err := proj.Rel(filepath.Join(filepath.Dir(root), "outside.log")); err == nil {
			t.Errorf("outside path accepted: %q", got)
		}
	}
}

func TestFindContextHonorsCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _, err := FindContext(ctx, t.TempDir()+string(rune(0)))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled discovery inspected path: %v", err)
	}
}

func TestGitBatchDeadlineBoundsInheritedPipes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	pidFile := hangingGitPipes(t)
	root := t.TempDir()
	ctx, started := testenv.ReadyDeadline(t, pidFile, 250*time.Millisecond)
	tracked, err := (Project{Root: root}).TrackedBatchContext(ctx, []string{filepath.Join(root, "keep.log")})
	elapsed := testenv.ReadyElapsed(t, started)
	if !errors.Is(err, context.DeadlineExceeded) || len(tracked) != 1 || !tracked[0] {
		t.Fatalf("bounded Git failed open: %v %v", tracked, err)
	}
	if elapsed > time.Second {
		t.Fatalf("batch Git pipe outlived hook budget: %s", elapsed)
	}
}

func TestFindCeilingMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	ceiling := filepath.Join(repo, "ceiling")
	child := filepath.Join(ceiling, "child")
	testenv.InitRepo(t, repo)
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	slash := filepath.ToSlash(ceiling)
	for _, tc := range []struct{ name, value string }{
		{"empty", ""}, {"canonical", ceiling}, {"slash", slash},
		{"empty-backslash", ";" + ceiling}, {"empty-slash", ";" + slash},
		{"empty-dot", ";" + slash + "/."}, {"empty-parent", ";" + slash + "/child/.."},
		{"empty-double-slash", ";" + strings.ReplaceAll(slash, "/ceiling", "//ceiling")},
		{"empty-trailing-slash", ";" + slash + "/"}, {"empty-upper", ";" + strings.ToUpper(slash)},
		{"quoted", `"` + ceiling + `"`}, {"empty-quoted", `;"` + slash + `"`},
		{"invalid-star", filepath.Join(repo, "*")}, {"invalid-before-valid", filepath.Join(repo, "*") + ";" + ceiling},
		{"relative", "ceiling"}, {"missing", filepath.Join(repo, "missing")},
		{"empty-middle", filepath.Join(root, "missing") + ";;" + slash},
		{"canonical-dot", ceiling + `\.`}, {"canonical-parent", ceiling + `\child\..`},
		{"canonical-trailing", ceiling + `\`}, {"equal-start", child},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GIT_CEILING_DIRECTORIES", tc.value)
			out, gitErr := exec.Command("git", "-C", child, "rev-parse", "--show-toplevel").CombinedOutput()
			p, found, err := Find(child)
			if err != nil || found != (gitErr == nil) {
				t.Fatalf("ceiling matrix disagrees with native Git: ceiling=%q found=%v err=%v git=%v %s", tc.value, found, err, gitErr, out)
			}
			if found && !strings.EqualFold(filepath.Clean(p.Root), filepath.Clean(strings.TrimSpace(string(out)))) {
				t.Fatalf("ceiling matrix selected another project: %s git=%s", p.Root, out)
			}
		})
	}
}

func TestFindRootRelativeCeilingWithExtendedCWD(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	ceiling := filepath.Join(repo, "ceiling")
	child := filepath.Join(ceiling, "child")
	testenv.InitRepo(t, repo)
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	for _, unc := range []bool{false, true} {
		for _, slash := range []bool{false, true} {
			t.Run(fmt.Sprintf("unc=%v/slash=%v", unc, slash), func(t *testing.T) {
				cwd := `\\?\` + child
				if unc {
					cwd = `\\?\UNC\localhost\` + child[:1] + "$" + child[2:]
					if _, err := os.Stat(cwd); err != nil {
						t.Skipf("existing administrative share unavailable: %v", err)
					}
				}
				t.Chdir(cwd)
				value := ceiling[2:]
				if slash {
					value = filepath.ToSlash(value)
				}
				t.Setenv("GIT_CEILING_DIRECTORIES", value)
				gitCWD := child
				if unc {
					gitCWD = `\\localhost\` + child[:1] + "$" + child[2:]
				}
				// Compare native discovery after Git normalizes its cwd via -C;
				// Find must resolve the same ceiling from the inherited verbatim cwd.
				out, gitErr := exec.Command("git", "-C", gitCWD, "rev-parse", "--show-toplevel").CombinedOutput()
				var exit *exec.ExitError
				if !errors.As(gitErr, &exit) || exit.ExitCode() != 128 {
					t.Fatalf("native Git did not stop at root-relative ceiling: %s %v", out, gitErr)
				}
				p, found, err := Find(".")
				if err != nil || found {
					t.Fatalf("root-relative ceiling disagrees with native Git: %+v found=%v err=%v git=%s", p, found, err, out)
				}
			})
		}
	}
}

func TestFindMissingDescendantHonorsResolvedCeiling(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	ceiling := filepath.Join(repo, "ceiling")
	testenv.InitRepo(t, repo)
	if err := os.Mkdir(ceiling, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", alias, ceiling).CombinedOutput(); err != nil {
		t.Fatalf("junction: %s %v", out, err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", ceiling)
	if p, found, err := Find(filepath.Join(alias, "missing", "payload.txt")); err != nil || found {
		t.Fatalf("missing descendant bypassed resolved ceiling: %+v %v %v", p, found, err)
	}
}

func TestFindCeilingAliasesAgreeWithGit(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	ceiling := filepath.Join(repo, "Long Ceiling Directory")
	child := filepath.Join(ceiling, "child")
	testenv.InitRepo(t, repo)
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", alias, ceiling).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %s %v", out, err)
	}
	p, _ := windows.UTF16PtrFromString(ceiling)
	buf := make([]uint16, 32768)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	short := windows.UTF16ToString(buf[:n])
	for _, tc := range []struct{ name, limit, start string }{
		{"extended", `\\?\` + ceiling, child},
		{"short", short, child},
		{"junction", alias, child},
		{"junction start", alias, alias},
		{"empty disables resolution", string(os.PathListSeparator) + alias, child},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "short" && strings.EqualFold(short, ceiling) {
				t.Skip("8.3 names unavailable")
			}
			t.Setenv("GIT_CEILING_DIRECTORIES", tc.limit)
			_, gitErr := exec.Command("git", "-C", tc.start, "rev-parse", "--show-toplevel").CombinedOutput()
			p, found, err := Find(tc.start)
			if err != nil || found != (gitErr == nil) {
				t.Fatalf("ceiling alias disagrees with native git: %+v found=%v err=%v git=%v", p, found, err, gitErr)
			}
		})
	}
}

func TestFindRejectsChangedExtendedProject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	if os.Getenv("OBIT_TEST_CHANGED_PROJECT_CWD") == "1" {
		path, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(path) != "repo." {
			t.Skipf("Windows normalized the process cwd before Find: %q", path)
		}
		p, found, err := Find(path)
		if err == nil || found {
			t.Fatalf("changed project cwd selected neighbor: input=%q %+v %v %v", path, p, found, err)
		}
		if p, found, err := Find("."); err == nil || found {
			t.Fatalf("relative project cwd selected neighbor: %+v %v %v", p, found, err)
		}
		return
	}
	neighbor := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(neighbor, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	special := `\\?\` + neighbor + "."
	if err := os.Mkdir(special, 0700); err != nil {
		t.Fatal(err)
	}
	for _, current := range []bool{false, true} {
		t.Run(fmt.Sprint(current), func(t *testing.T) {
			path := special
			if current {
				exe, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command(exe, "-test.run=^TestFindRejectsChangedExtendedProject$", "-test.v")
				cmd.Dir, cmd.Env = special, append(os.Environ(), "OBIT_TEST_CHANGED_PROJECT_CWD=1")
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("changed cwd child: %v %s", err, out)
				}
				if strings.Contains(string(out), "--- SKIP:") {
					t.Skip(string(out))
				}
				return
			}
			p, found, err := Find(path)
			if err == nil || found {
				t.Fatalf("changed project spelling selected neighbor: input=%q %+v %v %v", path, p, found, err)
			}
		})
	}
}

func TestFindRejectsUnstableParentProject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	base := t.TempDir()
	neighbor := filepath.Join(base, "repo")
	special := `\\?\` + neighbor + " "
	staging := filepath.Join(base, "staging")
	for _, root := range []string{neighbor, staging} {
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		testenv.InitRepo(t, root)
	}
	if err := os.WriteFile(filepath.Join(staging, "precious.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", staging, "add", "precious.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	if out, err := exec.Command("git", "-C", staging, "ls-files").CombinedOutput(); err != nil || !strings.Contains(string(out), "precious.txt") {
		t.Fatalf("special repository did not track fixture: %v %s", err, out)
	}
	if err := os.Rename(staging, special); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(special, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	p, found, err := Find(child)
	if err == nil || found || p.Root != "" {
		t.Fatalf("unstable parent selected neighbor repository: %+v %v %v", p, found, err)
	}
}

func TestTrackedBatchChunksLongCommandLines(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root := t.TempDir()
	testenv.InitRepo(t, root)
	// Overlay tests nest their isolated TEMP roots. Permit long filesystem
	// names so this fixture measures CreateProcess argv, not Git's MAX_PATH.
	if out, err := exec.Command("git", "-C", root, "config", "core.longpaths", "true").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v %s", err, out)
	}
	var paths []string
	for i := range 360 {
		name := fmt.Sprintf("%03d %s.txt", i, strings.Repeat("long", 32))
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("candidate"), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
		if i%90 == 0 {
			if out, err := exec.Command("git", "-C", root, "add", "--", name).CombinedOutput(); err != nil {
				t.Fatalf("git add: %v %s", err, out)
			}
		}
	}
	p := Project{Root: root}
	want := make([]bool, len(paths))
	for i, path := range paths {
		var err error
		want[i], err = p.Tracked(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	old, calls := gitCommand, 0
	t.Cleanup(func() { gitCommand = old })
	gitCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls++
		return old(ctx, name, args...)
	}
	got, err := p.TrackedBatch(paths)
	if err != nil || calls < 2 || !reflect.DeepEqual(got, want) {
		t.Fatalf("long batch did not split into equivalent queries: calls=%d failed=%v", calls, err != nil)
	}
	t.Logf("%d candidates matched individual queries in %d batches", len(paths), calls)
	calls = 0
	gitCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls++
		if calls == 2 {
			return old(ctx, name, "-C", root, "not-a-git-command")
		}
		return old(ctx, name, args...)
	}
	got, err = p.TrackedBatch(paths)
	if err == nil {
		t.Fatal("failed later batch did not return an error")
	}
	for i, tracked := range got {
		if !tracked {
			t.Fatalf("failed later batch exposed partial untracked answer at %d", i)
		}
	}
}

func TestTrackedBatchUnicodeMatchesIndividualQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, pair := range [][2]string{{"ǅ", "ǆ"}, {"ẞ", "ß"}, {"K", "k"}} {
		t.Run(pair[0], func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			testenv.InitRepo(t, root)
			trackedPath, other := filepath.Join(root, pair[0]+".txt"), filepath.Join(root, pair[1]+".txt")
			if err := os.WriteFile(trackedPath, []byte("tracked"), 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command("git", "-C", root, "add", "--", trackedPath).CombinedOutput(); err != nil {
				t.Fatalf("git add: %v %s", err, out)
			}
			p := Project{Root: root}
			want, err := p.Tracked(other)
			if err != nil || want {
				t.Fatalf("Git icase unexpectedly folds Unicode: %v %v", want, err)
			}
			for _, batch := range [][]string{{other}, {other, trackedPath}, {trackedPath, other}} {
				got, err := p.TrackedBatch(batch)
				if err != nil {
					t.Fatal(err)
				}
				for i, path := range batch {
					one, err := p.Tracked(path)
					if err != nil || got[i] != one {
						t.Fatalf("batch membership changed Unicode tracking: %q batch=%v got=%v individual=%v err=%v", path, batch, got, one, err)
					}
				}
			}
		})
	}
}

func TestTrackedBatchUsesOneLiteralCaseInsensitiveQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	git("init", "-q")
	for _, name := range []string{"Dir/Tracked.txt", "[literal].txt", "a.txt", "new.txt"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", "--", "Dir/Tracked.txt", "[literal].txt", "a.txt")
	git("commit", "-qm", "tracked candidates")
	old := gitCommand
	t.Cleanup(func() { gitCommand = old })
	calls := 0
	gitCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls++
		return old(ctx, name, args...)
	}
	paths := []string{}
	for _, rel := range []string{"dir", "DIR/TRACKED.TXT", "[literal].txt", "[a].txt", "new.txt"} {
		paths = append(paths, filepath.Join(root, filepath.FromSlash(rel)))
	}
	got, err := (Project{Root: root}).TrackedBatch(paths)
	if err != nil || calls != 1 || !reflect.DeepEqual(got, []bool{true, true, true, false, false}) {
		t.Fatalf("batch tracking lost literal/icase or queried repeatedly: %v calls=%d %v", got, calls, err)
	}
	t.Setenv("PATH", t.TempDir())
	got, err = (Project{Root: root}).TrackedBatch(paths)
	if err == nil || !reflect.DeepEqual(got, []bool{true, true, true, true, true}) {
		t.Fatalf("failed batch treated candidates as untracked: %v %v", got, err)
	}
}

func TestFindWithoutGit(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	if p, ok, err := Find(root); err != nil || !ok || !strings.EqualFold(p.Root, root) {
		t.Fatalf("Find without git = %+v, %v, %v; want %s", p, ok, err, root)
	}
	assertTrackedQueriesWithoutProcess(t, root)
}

func assertTrackedQueriesWithoutProcess(t *testing.T, root string) {
	t.Helper()
	// The batching limit resolves the executable path before executing Git.
	// Only that lookup needs a file; the injected query never launches it.
	if err := os.WriteFile(filepath.Join(os.Getenv("PATH"), "git.exe"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	previous := gitOutput
	t.Cleanup(func() { gitOutput = previous })
	gitOutput = func(cmd *exec.Cmd) ([]byte, error) {
		if !strings.Contains(strings.Join(cmd.Args, " "), "ls-files") {
			t.Fatalf("unexpected fake Git query: %v", cmd.Args)
		}
		return []byte("kept.log\x00"), nil
	}
	paths := []string{filepath.Join(root, "kept.log"), filepath.Join(root, "free.log")}
	got, err := (Project{Root: root}).TrackedBatch(paths)
	if err != nil || !reflect.DeepEqual(got, []bool{true, false}) {
		t.Fatalf("tracked query: %v %v", got, err)
	}
	gitOutput = func(*exec.Cmd) ([]byte, error) { return nil, errors.New("Git unavailable") }
	got, err = (Project{Root: root}).TrackedBatch(paths)
	if err == nil || !reflect.DeepEqual(got, []bool{true, true}) {
		t.Fatalf("failed query must retain every candidate: %v %v", got, err)
	}
}

func TestFindCeilingDirectories(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	ceiling := filepath.Join(repo, "Ceiling")
	child := filepath.Join(ceiling, "child")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, ceilings, path string
		found                bool
	}{
		{"above repository", root, child, true},
		{"repository ceiling", repo, child, false},
		{"intermediate ceiling", ceiling, child, false},
		{"start at ceiling", repo, repo, true},
		{"start at intermediate ceiling", ceiling, ceiling, true},
		{"multiple ceilings", filepath.Join(root, "other") + string(os.PathListSeparator) + ceiling, child, false},
		{"case insensitive", strings.ToUpper(ceiling), child, false},
		{"clean ceiling", ceiling + string(os.PathSeparator) + ".", child, false},
		{"empty and relative entries", strings.Join([]string{"", ".", "repo", root, ""}, string(os.PathListSeparator)), child, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GIT_CEILING_DIRECTORIES", tc.ceilings)
			p, ok, err := Find(tc.path)
			if err != nil || ok != tc.found || ok && !strings.EqualFold(p.Root, repo) {
				t.Fatalf("Find(%s) with ceilings %q = %+v, %v, %v; want found=%v", tc.path, tc.ceilings, p, ok, err, tc.found)
			}
		})
	}
}

func TestFindAgreesWithGitWorktreeBoundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if p, ok, err := Find(root); err != nil || ok {
		t.Fatalf("outside Git's discovery boundary, found %+v, %v", p, err)
	}
	repo := filepath.Join(root, "repo")
	git("init", "-q", repo)
	git("-C", repo, "commit", "--allow-empty", "-qm", "initial")
	worktree := filepath.Join(root, "worktree")
	git("-C", repo, "worktree", "add", "-q", "-b", "linked", worktree)
	for _, dir := range []string{repo, worktree} {
		file := filepath.Join(dir, "sub", "file.log")
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
		p, ok, err := Find(file)
		if err != nil || !ok || !strings.EqualFold(filepath.Clean(p.Root), filepath.Clean(dir)) {
			t.Fatalf("Find(%s) = %+v, %v, %v", file, p, ok, err)
		}
	}
	bare := filepath.Join(root, "bare")
	git("init", "--bare", "-q", bare)
	if p, ok, err := Find(bare); err != nil || ok {
		t.Fatalf("bare repository is not a work tree: %+v, %v", p, err)
	}
}

func TestHangingGitHelper(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	mode := os.Getenv("OBIT_HANGING_GIT")
	if mode == "pipes" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^TestHangingGitHelper$")
		cmd.Env = append(os.Environ(), "OBIT_HANGING_GIT=pipe-holder")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(os.Getenv("OBIT_GIT_CHILD_PID"), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
			t.Fatal(err)
		}
		_ = cmd.Wait()
		os.Exit(0)
	}
	if mode == "pipe-holder" {
		// Stay alive beyond the setup limit; the parent test reaps this process.
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	if mode == "1" {
		time.Sleep(3 * time.Second)
		os.Exit(0)
	}
}

func hangingGitPipes(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	previousCommand, previousTimeout := gitCommand, gitTimeout
	gitTimeout = time.Minute
	t.Cleanup(func() {
		gitCommand, gitTimeout = previousCommand, previousTimeout
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(string(data)); err == nil {
				if proc, err := os.FindProcess(pid); err == nil {
					_ = proc.Kill()
					_, _ = proc.Wait()
				}
			}
		}
	})
	gitCommand = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestHangingGitHelper$")
		cmd.Env = append(os.Environ(), "OBIT_HANGING_GIT=pipes", "OBIT_GIT_CHILD_PID="+pidFile)
		return cmd
	}
	return pidFile
}

func TestGitTimeoutBoundsInheritedOutputPipes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	pidFile := hangingGitPipes(t)
	previous := gitWithTimeout
	t.Cleanup(func() { gitWithTimeout = previous })
	ctx, started := testenv.ReadyDeadline(t, pidFile, 500*time.Millisecond)
	gitWithTimeout = func(context.Context, time.Duration) (context.Context, context.CancelFunc) {
		return ctx, func() {}
	}
	_, err := (Project{Root: t.TempDir()}).Untracked()
	elapsed := testenv.ReadyElapsed(t, started)
	if !errors.Is(err, context.DeadlineExceeded) || elapsed > 2500*time.Millisecond {
		t.Fatalf("inherited output pipe stalled timeout: %s, %v", elapsed, err)
	}
}

func TestGitTimeoutRetriesAutomaticBurial(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root := t.TempDir()
	testenv.InitRepo(t, root)
	path := filepath.Join(root, "debug.log")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	originalCommand, originalTimeout := gitCommand, gitTimeout
	t.Cleanup(func() { gitCommand, gitTimeout = originalCommand, originalTimeout })
	gitTimeout = 100 * time.Millisecond
	var commands []*exec.Cmd
	gitCommand = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestHangingGitHelper$")
		cmd.Env = append(os.Environ(), "OBIT_HANGING_GIT=1")
		commands = append(commands, cmd)
		return cmd
	}
	p := Project{Root: root}
	for _, operation := range []string{"tracked", "untracked", "ensure", "sweep"} {
		t.Run(operation, func(t *testing.T) {
			started := time.Now()
			var err error
			switch operation {
			case "tracked":
				var tracked bool
				tracked, err = p.Tracked(path)
				if !tracked {
					t.Error("timeout treated as untracked")
				}
			case "untracked":
				_, err = p.Untracked()
			case "ensure":
				err = p.Ensure()
			case "sweep":
				now := time.Now()
				svc := &core.Service{Store: core.Store{Home: t.TempDir()}, FS: winfs.FS{}, Now: func() time.Time { return now },
					Owner: func(path string) (string, bool, error) { tracked, err := p.Tracked(path); return root, tracked, err }}
				added, addErr := svc.Add([]string{path}, core.AddOptions{ExpiresAt: now.Add(time.Hour), Source: "rule", Project: root})
				if addErr != nil || core.Failed(added) {
					t.Fatalf("add: %+v, %v", added, addErr)
				}
				now = now.Add(time.Hour)
				rows, sweepErr := svc.Sweep(false)
				st, readErr := svc.Store.Read()
				if sweepErr != nil || readErr != nil || len(rows) != 1 || rows[0].Action != core.ActionRetry || len(st.Items) != 1 || len(st.Burials) != 0 {
					t.Fatalf("timed out Git authorized burial: %+v, %v, %v", rows, sweepErr, readErr)
				}
				if _, statErr := os.Stat(path); statErr != nil {
					t.Fatal("timeout moved original", statErr)
				}
			}
			if operation != "sweep" && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Git timeout not returned: %v", err)
			}
			last := commands[len(commands)-1]
			if time.Since(started) > time.Second || last.ProcessState == nil || last.ProcessState.Success() {
				t.Fatalf("Git not killed and reaped promptly: duration=%s state=%v", time.Since(started), last.ProcessState)
			}
		})
	}
}

func TestTrackedIgnoresInheritedRepositoryEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root, other := t.TempDir(), t.TempDir()
	git := func(dir string, args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git(root, "init", "-q")
	git(other, "init", "-q")
	path := filepath.Join(root, "keep.log")
	if err := os.WriteFile(path, []byte("tracked"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(root, "add", "keep.log")
	for _, env := range []map[string]string{
		{"GIT_INDEX_FILE": filepath.Join(root, "absent-index")},
		{"GIT_DIR": filepath.Join(other, ".git"), "GIT_WORK_TREE": other},
		{"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "core.bare", "GIT_CONFIG_VALUE_0": "true", "GIT_CONFIG_PARAMETERS": "invalid"},
		{"GIT_LITERAL_PATHSPECS": "1", "GIT_GLOB_PATHSPECS": "1", "GIT_NOGLOB_PATHSPECS": "1", "GIT_ICASE_PATHSPECS": "1"},
	} {
		t.Run("inherited", func(t *testing.T) {
			for key, value := range env {
				t.Setenv(key, value)
			}
			p := Project{Root: root}
			if got, err := p.Tracked(path); err != nil || !got {
				t.Fatalf("tracked file escaped real index: %v, %v", got, err)
			}
			if got, err := p.Tracked(filepath.Join(root, "new.log")); err != nil || got {
				t.Fatalf("untracked file misclassified: %v, %v", got, err)
			}
		})
	}
}
