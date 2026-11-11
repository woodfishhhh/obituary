//go:build windows

package auto

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/project"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"golang.org/x/sys/windows"
)

func TestMissingUNCAliasRevocationBlocksAutomaticRule(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	path := r.write(rules.FileName, "notes/** 1m\n")
	note := r.write("notes/private.md", "private")
	alias := `\\localhost\` + path[:1] + "$" + path[2:]
	if _, err := os.Stat(alias); err != nil {
		t.Skipf("existing administrative share unavailable: %v", err)
	}
	if _, err := r.engine.Trust.Trust(path, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatal(err)
	}
	was, revokeErr := r.engine.Trust.Untrust(alias)
	if err := os.Rename(path+".moved", path); err != nil {
		t.Fatal(err)
	}
	state, checkErr := r.engine.Trust.Check(path)
	rows, applyErr := r.engine.Apply([]string{note}, Options{})
	st, readErr := r.engine.Svc.Store.ReadSnapshot()
	if revokeErr != nil || !was || checkErr != nil || state.Status != rules.Untrusted || applyErr != nil || len(rows) != 1 || rows[0].Action != Skipped || readErr != nil || len(st.Items) != 0 {
		t.Fatalf("missing UNC alias retained automatic rule authority: revoked=%v state=%+v rows=%+v items=%d errors=%v/%v/%v/%v", was, state, rows, len(st.Items), revokeErr, checkErr, applyErr, readErr)
	}
}

func TestApplyTrustWarningsPerProject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	var paths []string
	for range 2 {
		other := newRepo(t)
		other.write(rules.FileName, "notes/** 1m\n")
		paths = append(paths, other.write("notes/a.md", "private"), other.write("notes/b.md", "private"))
	}
	if err := os.MkdirAll(filepath.Dir(r.engine.Trust.Path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.engine.Trust.Path, []byte(`{"version":1,"trusted":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	// The second batch uses cached rules; each project still gets one notice.
	for range 2 {
		rows, err := r.engine.Apply(paths, Options{})
		if err != nil || len(rows) != len(paths) {
			t.Fatal(rows, err)
		}
		for i, row := range rows {
			if row.Action != Skipped || (row.Warning != "") != (i%2 == 0) {
				t.Fatalf("trust upgrade warning lost or repeated across projects: %+v", rows)
			}
		}
	}
}

func TestApplyRejectsChangedExtendedSpelling(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"no-git", "git"} {
		t.Run(mode, func(t *testing.T) {
			if testing.Short() && mode == "git" {
				t.Skip("checks automatic registration in a real Git worktree")
			}
			r := newEngineFixture(t)
			if mode == "git" {
				testenv.InitRepo(t, r.root)
			}
			path := r.write("neighbor.log", "keep")
			input := `\\?\` + path + "."
			rows, err := r.engine.Apply([]string{input}, Options{Session: "s1"})
			// "No repository" or a failed Git query must not stand in for the
			// path rejection, even in the fast fixture without Git.
			if err != nil || len(rows) != 1 || rows[0].Action != Retry || !errors.Is(rows[0].Err, core.ErrUnsafe) {
				t.Fatalf("automatic registration selected normalized neighbor: %+v %v", rows, err)
			}
			st, err := r.engine.Svc.Store.ReadSnapshot()
			if err != nil || len(st.Items) != 0 {
				t.Fatalf("unsafe path entered automatic ledger: %+v %v", st, err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "keep" {
				t.Fatal("automatic input changed neighbor")
			}
		})
	}
}

func TestRuleSetShortRootMatchesCanonicalPaths(t *testing.T) {
	r := newEngineFixture(t)
	path := r.write("debug.log", "scratch")
	set, _, err := r.engine.RuleSet(shortPath(t, r.root))
	if err != nil {
		t.Fatal(err)
	}
	match, ok, err := set.Match(path, false)
	if err != nil || !ok || match.Unit != path {
		t.Fatalf("short rule base mismatched canonical path: %+v %v %v", match, ok, err)
	}
}

func TestApplyShortPathsPreservesRulesAndTrust(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := newRepo(t)
	root, err := filepath.EvalSymlinks(r.root)
	if err != nil {
		t.Fatal(err)
	}
	r.root = root
	short := shortPath(t, root)
	r.write("App.log", "tracked")
	r.commit("App.log")
	if err := os.Rename(filepath.Join(root, "App.log"), filepath.Join(root, "app.log")); err != nil {
		t.Fatal(err)
	}
	r.write("debug.log", "untracked")
	r.write(".playwright-mcp/page.png", "screenshot")
	r.write(rules.FileName, "notes/** 1m\n")
	r.write("notes/private.md", "private")
	if err := os.MkdirAll(filepath.Dir(r.engine.Trust.Path), 0700); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"version":1,"trusted":{}}`)
	if err := os.WriteFile(r.engine.Trust.Path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	inputs := []string{filepath.Join(short, "app.log"), filepath.Join(short, "debug.log"), filepath.Join(short, ".playwright-mcp", "page.png"), filepath.Join(short, "notes", "private.md")}
	rows, err := r.engine.Apply(inputs, Options{Session: "short"})
	if err != nil || len(rows) != len(inputs) {
		t.Fatalf("apply short paths: %+v %v", rows, err)
	}
	for i, want := range []string{Skipped, Registered, Registered, Skipped} {
		if rows[i].Action != want {
			t.Fatalf("decision %d: %+v", i, rows[i])
		}
	}
	if !strings.Contains(rows[0].Reason, "Git tracks") || rows[0].Warning == "" || rows[2].Unit != filepath.Join(root, ".playwright-mcp") {
		t.Fatalf("short paths changed tracking, migration warning or directory rule: %+v", rows)
	}
	for _, row := range rows[1:3] {
		if row.Item == nil || row.Item.Project != root || !strings.HasPrefix(row.Item.Path, root+`\`) {
			t.Fatalf("registration retained an alias: %+v", row)
		}
	}
	if got, err := os.ReadFile(r.engine.Trust.Path); err != nil || string(got) != string(legacy) {
		t.Fatalf("legacy trust changed during apply: %s %v", got, err)
	}
	if _, err := r.engine.Trust.Trust(filepath.Join(short, rules.FileName), ""); err != nil {
		t.Fatal(err)
	}
	r.engine.sets = nil
	rows, err = r.engine.Apply(inputs[3:], Options{})
	if err != nil || len(rows) != 1 || rows[0].Action != Registered || rows[0].Item.Project != root {
		t.Fatalf("reviewed trust did not apply across aliases: %+v %v", rows, err)
	}
}

func TestApplyRegistersUntrackedArtefactsOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	tracked := r.write("app.log", "committed log")
	r.commit("app.log")
	debug := r.write("debug.log", "x")
	shot := r.write(".playwright-mcp/page.png", "x")
	results := r.write("test-results/keep.xml", "committed")
	r.commit("test-results/keep.xml")
	r.write("test-results/new.xml", "x")
	source := r.write("src/main.go", "package main")

	got := r.apply(tracked, debug, shot, filepath.Join(r.root, "test-results", "new.xml"), source)

	if d := got[debug]; d.Action != Registered || d.Item.Source != "rule" || d.Item.Session != "s1" || !strings.EqualFold(d.Item.Project, r.root) {
		t.Errorf("debug.log: %+v", d)
	}
	if d := got[shot]; d.Action != Registered || d.Unit != filepath.Join(r.root, ".playwright-mcp") {
		t.Errorf(".playwright-mcp must be registered as one unit: %+v", d)
	}
	for _, path := range []string{tracked, filepath.Join(r.root, "test-results", "new.xml"), source} {
		if d := got[path]; d.Action != Skipped {
			t.Errorf("%s must be skipped (tracked, holds tracked files, or no rule), got %+v", path, d)
		}
	}
	_ = results
}

func TestApplyNeverShortensAnExistingLifespan(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	log := r.write("run.log", "x")
	if _, err := r.engine.Svc.Add([]string{log}, core.AddOptions{ExpiresAt: time.Now().Add(30 * 24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if d := r.apply(log)[log]; d.Action != Skipped {
		t.Fatalf("an explicit lifespan must be kept, got %+v", d)
	}
}

func TestUntrustedProjectRulesDoNothing(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	r.write(rules.FileName, "notes/** 1m\n")
	notes := r.write("notes/private.md", "my own notes")

	if d := r.apply(notes)[notes]; d.Action != Skipped {
		t.Fatalf("an untrusted .obitrules must not register anything, got %+v", d)
	}
	if _, err := r.engine.Trust.Trust(filepath.Join(r.root, rules.FileName), ""); err != nil {
		t.Fatal(err)
	}
	r.engine.sets = nil
	if d := r.apply(notes)[notes]; d.Action != Registered {
		t.Fatalf("once trusted the rule applies, got %+v", d)
	}
}

func TestApplyLegacyTrustRequiresReview(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, scenario := range []string{"original", "moved", "deleted"} {
		for _, version := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/v%d", scenario, version), func(t *testing.T) {
				t.Parallel()
				r := newRepo(t)
				base := filepath.Dir(r.root)
				original := filepath.Join(base, "K")
				r.root = original
				if err := os.Mkdir(original, 0700); err != nil {
					t.Fatal(err)
				}
				r.git("init", "-q")
				content := "notes/** 1m\n"
				path := r.write(rules.FileName, content)
				r.write("notes/private.md", "private")
				// Exact v1 writer schema/key from 579c56c internal/rules/trust.go.
				old := map[string]any{"trusted": map[string]any{strings.ToLower(filepath.Clean(path)): map[string]any{"sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(content))), "trustedAt": "2026-11-08T00:00:00Z"}}}
				if version != 0 {
					old["version"] = version
				}
				data, err := json.MarshalIndent(old, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(r.engine.Trust.Path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(r.engine.Trust.Path, append(data, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
				switch scenario {
				case "moved":
					if err := os.Rename(original, filepath.Join(base, "old")); err != nil {
						t.Fatal(err)
					}
				case "deleted":
					if err := os.RemoveAll(original); err != nil {
						t.Fatal(err)
					}
				}
				if scenario != "original" {
					r.root = filepath.Join(base, "K")
					if err := os.Mkdir(r.root, 0700); err != nil {
						t.Fatal(err)
					}
					r.git("init", "-q")
					r.write(rules.FileName, content)
					r.write("notes/private.md", "private")
				}
				for _, name := range []string{"K", "K"} {
					rule := filepath.Join(base, name, rules.FileName)
					state, err := r.engine.Trust.Check(rule)
					if err != nil || state.Status == rules.Trusted {
						t.Fatalf("legacy trust crossed repository boundary: %s %+v %v", name, state, err)
					}
					if state.Status != rules.Absent {
						note := filepath.Join(base, name, "notes", "private.md")
						if d := r.apply(note)[note]; d.Action != Skipped {
							t.Fatalf("legacy rules registered private file: %+v", d)
						}
					}
				}
				st, err := r.engine.Svc.Store.ReadSnapshot()
				if err != nil || len(st.Items) != 0 {
					t.Fatalf("legacy rules changed ledger: %+v %v", st, err)
				}
				if got, err := os.ReadFile(r.engine.Trust.Path); err != nil || string(got) != string(append(data, '\n')) {
					t.Fatal("checking legacy trust rewrote it")
				}
			})
		}
	}
}

func TestApplyDoesNotInheritUnicodeTrust(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	base := filepath.Dir(r.root)
	for _, name := range []string{"K", "K"} {
		r.root = filepath.Join(base, name)
		if err := os.Mkdir(r.root, 0700); err != nil {
			t.Fatal(err)
		}
		r.git("init", "-q")
		r.write(rules.FileName, "notes/** 1m\n")
		r.write("notes/private.md", "private")
	}
	if _, err := r.engine.Trust.Trust(filepath.Join(base, "K", rules.FileName), ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(r.root, "notes", "private.md")
	rows := r.apply(path)
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Items) != 0 || rows[path].Action == Registered {
		t.Fatalf("Unicode neighbor rules registered private file: %+v %v", rows, err)
	}
}

func TestOutsideAGitWorkTreeNothingIsRegistered(t *testing.T) {
	t.Parallel()
	r := newEngineFixture(t)
	stray := filepath.Join(filepath.Dir(r.root), "stray.log")
	if err := os.WriteFile(stray, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if d := r.apply(stray)[stray]; d.Action != Skipped {
		t.Fatalf("got %+v", d)
	}
	assertFastRuleRegistration(t)
}

// Complement the real-Git contracts in the slow tier without changing their
// assertions or test inventory. Each engine owns its injected query.
func assertFastRuleRegistration(t *testing.T) {
	t.Helper()
	for _, mode := range []string{"untracked", "tracked", "unavailable"} {
		r := newEngineFixture(t)
		if err := os.Mkdir(filepath.Join(r.root, ".git"), 0o700); err != nil {
			t.Fatal(err)
		}
		path := r.write("debug.log", "keep")
		r.engine.tracked = func(_ project.Project, _ string) (bool, error) {
			if mode == "unavailable" {
				return true, errors.New("Git unavailable")
			}
			return mode == "tracked", nil
		}
		rows, err := r.engine.Apply([]string{path}, Options{Session: "fast"})
		want := map[string]string{"untracked": Registered, "tracked": Skipped, "unavailable": Retry}[mode]
		if err != nil || len(rows) != 1 || rows[0].Action != want {
			t.Fatalf("%s registration: %+v %v", mode, rows, err)
		}
		state, err := r.engine.Svc.Store.ReadSnapshot()
		if err != nil || (len(state.Items) == 1) != (mode == "untracked") {
			t.Fatalf("%s ledger: %+v %v", mode, state, err)
		}
		if mode == "untracked" && r.apply(path)[path].Action != Skipped {
			t.Fatal("existing lifespan was replaced")
		}
		if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
			t.Fatalf("registration modified input: %s %v", data, err)
		}
	}
}

func TestMkTempLivesInTheProjectAndStaysOutOfGit(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	r.write("README.md", "x")
	r.commit("README.md")

	dir, item, err := r.engine.MkTemp(24*time.Hour, "scratch space", r.root, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.ToLower(dir), strings.ToLower(filepath.Join(r.root, ".obit", "tmp", "scratch-space-"))) || item.Source != "mktemp" {
		t.Fatalf("mktemp dir %s item %+v", dir, item)
	}
	if err := os.WriteFile(filepath.Join(dir, "probe.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if status := r.git("status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf(".obit must not show up in git status, got %q", status)
	}
}

func TestTrackedCheckIgnoresCaseOnWindows(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	r.write("App.log", "x")
	r.commit("App.log")
	lower := filepath.Join(r.root, "app.log")
	if err := os.Rename(filepath.Join(r.root, "App.log"), lower); err != nil { // the index still says App.log
		t.Fatal(err)
	}
	if d := r.apply(lower)[lower]; d.Action != Skipped {
		t.Fatalf("a tracked file renamed only in case must stay tracked, got %+v", d)
	}
}

func TestAJunctionIntoAnotherProjectIsJudgedByTheRealProject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	a, b := newRepo(t), newRepo(t)
	b.write("logs/build.log", "tracked in B")
	b.commit("logs/build.log")
	// The junction targets a folder inside B, so nothing on the lexical path
	// through A reveals B's .git: only the real location does.
	link := filepath.Join(a.root, "link")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, filepath.Join(b.root, "logs")).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}
	via := filepath.Join(link, "build.log")
	decisions, err := a.engine.Apply([]string{via}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if decisions[0].Action != Skipped || !strings.Contains(decisions[0].Reason, "Git tracks") {
		t.Fatalf("B's tracked file reached through A's junction must be skipped, got %+v", decisions[0])
	}
}

func TestAutomaticRegistrationNeverShortensALifespan(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	old := r.write("keep.log", "x")
	if _, err := r.engine.Svc.Add([]string{old}, core.AddOptions{ExpiresAt: time.Now().Add(30 * 24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(r.root, "renamed.log")
	if err := os.Rename(old, renamed); err != nil {
		t.Fatal(err)
	}
	inner := r.write("test-results/report.xml", "x")
	if _, err := r.engine.Svc.Add([]string{inner}, core.AddOptions{ExpiresAt: time.Now().Add(30 * 24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	sibling := r.write("test-results/other.xml", "x")

	got := r.apply(renamed, sibling)
	if d := got[renamed]; d.Action != Skipped {
		t.Errorf("the renamed object keeps its 30-day lifespan, got %+v", d)
	}
	if d := got[sibling]; d.Action != Skipped || !strings.Contains(d.Reason, "own lifespan") {
		t.Errorf("test-results holds an item with its own lifespan and must not be registered whole, got %+v", d)
	}
}

func TestMovedChildKeepsLongerLifespan(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, beforeRegistration := range []bool{true, false} {
		t.Run(fmt.Sprintf("before-registration=%v", beforeRegistration), func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			now := time.Now()
			r.engine.Svc.Now = func() time.Time { return now }
			keep := r.write("keep.txt", "keep for thirty days")
			added, err := r.engine.Svc.Add([]string{keep}, core.AddOptions{ExpiresAt: now.Add(30 * 24 * time.Hour)})
			if err != nil || core.Failed(added) {
				t.Fatalf("add: %+v, %v", added, err)
			}
			r.write("test-results/report.xml", "report")
			dir := filepath.Join(r.root, "test-results")
			moved := filepath.Join(dir, "keep.txt")
			move := func() {
				if err := os.Rename(keep, moved); err != nil {
					t.Fatal(err)
				}
			}
			if beforeRegistration {
				move()
				if d := r.apply(dir)[dir]; d.Action != Skipped {
					t.Fatalf("directory holding moved child must be skipped: %+v", d)
				}
				return
			}
			d := r.apply(dir)[dir]
			if d.Action != Registered {
				t.Fatalf("register parent: %+v", d)
			}
			move()
			now = d.Item.ExpiresAt.Add(time.Second)
			out, err := r.engine.Svc.Sweep(false)
			if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry {
				t.Errorf("parent must retry while child has a longer lifespan: %+v, %v", out, err)
			}
			if data, err := os.ReadFile(moved); err != nil || string(data) != "keep for thirty days" {
				t.Errorf("child must remain in place: %q, %v", data, err)
			}
			st, err := r.engine.Svc.Store.Read()
			if err != nil || len(st.Items) != 2 || len(st.Burials) != 0 {
				t.Fatalf("both lifespans must remain: %+v, %v", st, err)
			}
			parent := st.Items[st.Index(d.Item.ID)]
			if parent.Attempts != 1 || !parent.NextAttempt.After(now) {
				t.Errorf("parent must back off: %+v", parent)
			}
		})
	}
}

func TestOwnerReportsDeniedProjectDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	path := r.write("middle/child/debug.log", "keep")
	original, err := windows.GetNamedSecurityInfo(r.root, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	originalACL, _, err := original.DACL()
	if err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(r.root, ".git")
	gitOriginal, err := windows.GetNamedSecurityInfo(gitDir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	gitACL, _, err := gitOriginal.DACL()
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := user.User.Sid.String()
	// Denials apply to exactly these two directories. Inheritable deny ACEs
	// outlive a root-only DACL restore on some inheritance configurations.
	denied, err := windows.SecurityDescriptorFromString("D:(D;;0x1;;;" + sid + ")(A;OICI;FA;;;" + sid + ")")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := denied.DACL()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := windows.SetNamedSecurityInfo(r.root, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, originalACL, nil); err != nil {
			t.Error(err)
		}
		if err := windows.SetNamedSecurityInfo(gitDir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, gitACL, nil); err != nil {
			t.Error(err)
		}
		if _, err := os.ReadDir(gitDir); err != nil {
			t.Errorf("restored .git is unreadable before TempDir cleanup: %v", err)
		}
		if _, err := os.ReadFile(filepath.Join(gitDir, "config")); err != nil {
			t.Errorf("restored .git child is unreadable before TempDir cleanup: %v", err)
		}
	})
	gitDenied, err := windows.SecurityDescriptorFromString("D:(D;;GR;;;" + sid + ")(A;OICI;FA;;;" + sid + ")")
	if err != nil {
		t.Fatal(err)
	}
	gitDenyACL, _, err := gitDenied.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(gitDir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, gitDenyACL, nil); err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(r.root, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(r.root, ".git")); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("fixture must deny .git Lstat: %v", err)
	}
	if _, err := os.Stat(r.root); err != nil {
		t.Fatalf("fixture must allow stat of discovery start: %v", err)
	}
	if root, _, err := Owner(r.root); err == nil {
		t.Errorf("unreadable discovery must return an error, got root %q", root)
	}
	if d := r.apply(path)[path]; d.Action != Retry {
		t.Errorf("unreadable project must not register: %+v", d)
	}
}

func TestAutomaticAddPreservesConcurrentManualLifespan(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, route := range []string{"rule", "leftovers", "approve"} {
		for _, relation := range []string{"identity", "child", "parent", "old-path"} {
			t.Run(route+"/"+relation, func(t *testing.T) {
				t.Parallel()
				r := newRepo(t)
				now := time.Now()
				r.engine.Svc.Now = func() time.Time { return now }
				if err := r.engine.Snapshot(r.root, "s"); err != nil {
					t.Fatal(err)
				}
				unit := r.write("scratch/debug.log", "new")
				manualPath := unit
				switch relation {
				case "child":
					manualPath = r.write("test-results/keep.txt", "keep")
					unit = filepath.Dir(manualPath)
				case "parent":
					manualPath = filepath.Dir(unit)
				}
				var proposal core.Proposal
				if route != "rule" {
					var err error
					// Session leftovers and approved rule candidates share the approval gate.
					kind := core.ProposalOnboard
					if route == "leftovers" {
						kind = core.ProposalLeftovers
					}
					proposal, _, _, err = r.engine.Svc.Propose(kind, r.root, "s", []core.Candidate{{Path: unit, Lifespan: "3d"}})
					if err != nil {
						t.Fatal(err)
					}
				}
				manual := *r.engine.Svc
				var saved core.Item
				r.engine.Svc.FS = &interleaveAddFS{beforeAdd: func() {
					got, err := manual.Add([]string{manualPath}, core.AddOptions{ExpiresAt: now.Add(30 * 24 * time.Hour)})
					if err != nil || core.Failed(got) {
						t.Fatalf("manual add: %+v, %v", got, err)
					}
					saved = *got[0].Item
					if relation == "old-path" {
						if err := os.Rename(unit, unit+".kept"); err != nil {
							t.Fatal(err)
						}
						r.write("scratch/debug.log", "replacement")
					}
				}}
				if route == "rule" {
					got, err := r.engine.Apply([]string{unit}, Options{})
					if err != nil || len(got) != 1 || got[0].Action != Skipped || !strings.Contains(got[0].Reason, "already has a lifespan") {
						t.Errorf("race must refuse automatic add: %+v, %v", got, err)
					}
				} else {
					got, err := r.engine.Approve([]string{fmt.Sprint(proposal.ID)})
					if err != nil || len(got) != 1 || len(got[0].Results) != 1 || !strings.Contains(got[0].Results[0].Error, "already has a lifespan") {
						t.Errorf("approval race must settle without updating: %+v, %v", got, err)
					}
				}
				st, err := r.engine.Svc.Store.Read()
				if err != nil || len(st.Items) != 1 || st.Items[0].ID != saved.ID || st.Items[0].Rev != saved.Rev || st.Items[0].Source != "cli" || !st.Items[0].ExpiresAt.Equal(saved.ExpiresAt) || len(st.Proposals) != 0 {
					t.Fatalf("manual lifespan or resolved proposal changed: %+v, %v", st, err)
				}
			})
		}
	}
}

func TestBatchRechecksCachedConflictAfterMove(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, relation := range []string{"holds", "inside", "same"} {
		t.Run(relation, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			original := r.write("original/keep.txt", "keep")
			cached, unit := filepath.Join(r.root, "test-results", "keep.txt"), filepath.Join(r.root, "test-results")
			switch relation {
			case "inside":
				original = filepath.Dir(original)
				cached, unit = filepath.Join(r.root, "scratch"), filepath.Join(r.root, "scratch", "debug.log")
			case "same":
				cached = filepath.Join(r.root, "debug.log")
				unit = cached
			}
			old, err := r.engine.Svc.Add([]string{original}, core.AddOptions{ExpiresAt: time.Now().Add(30 * 24 * time.Hour)})
			if err != nil || core.Failed(old) {
				t.Fatalf("old add: %+v, %v", old, err)
			}
			if err := os.MkdirAll(filepath.Dir(cached), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(original, cached); err != nil {
				t.Fatal(err)
			}
			located := locations{}
			first := r.write("first.log", "first")
			state, err := r.engine.Svc.Store.Read()
			if err != nil {
				t.Fatal(err)
			}
			if d := r.engine.decide(first, &state, map[string]bool{}, located); d.Action != WouldRegister {
				t.Fatalf("first candidate: %+v", d)
			}
			if err := os.Rename(cached, filepath.Join(r.root, "saved")); err != nil {
				t.Fatal(err)
			}
			if relation == "holds" {
				r.write("test-results/new.xml", "new")
			} else {
				rel, err := filepath.Rel(r.root, unit)
				if err != nil {
					t.Fatal(err)
				}
				r.write(rel, "new")
			}
			got, err := r.engine.apply([]string{unit}, Options{}, located)
			if err != nil || len(got) != 1 || got[0].Action != Registered {
				t.Fatalf("stale location falsely refused candidate: %+v, %v", got, err)
			}
		})
	}
}

func TestReplacementLogKeepsBothIdentitiesAndRechecksGit(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	now := time.Now()
	r.engine.Svc.Now = func() time.Time { return now }
	path := r.write("debug.log", "old")
	old, err := r.engine.Svc.Add([]string{path}, core.AddOptions{ExpiresAt: now.Add(30 * 24 * time.Hour), Session: "old-session"})
	if err != nil || core.Failed(old) {
		t.Fatalf("add: %+v, %v", old, err)
	}
	keep := filepath.Join(r.root, "keep.txt")
	if err := os.Rename(path, keep); err != nil {
		t.Fatal(err)
	}
	r.write("debug.log", "new")
	now = now.Add(time.Minute)
	fs := &lookupFS{calls: map[core.Identity]int{}}
	r.engine.Svc.FS = fs
	d := r.apply(path)[path]
	if d.Action != Skipped || !strings.Contains(d.Reason, "already has a lifespan") {
		t.Fatalf("a live old-path record needs explicit reconciliation: %+v", d)
	}
	refreshed, err := r.engine.Svc.Add([]string{keep}, core.AddOptions{ExpiresAt: old[0].Item.ExpiresAt})
	if err != nil || core.Failed(refreshed) {
		t.Fatalf("refresh renamed record: %+v, %v", refreshed, err)
	}
	d = r.apply(path)[path]
	if d.Action != Registered || d.Item.Source != "rule" || d.Item.Session != "s1" || !d.Item.CreatedAt.Equal(now) {
		t.Fatalf("new log: %+v", d)
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Items) != 2 || st.Index(old[0].Item.ID) < 0 || st.Items[st.Index(old[0].Item.ID)].Source != "cli" || *st.Items[0].Identity == *st.Items[1].Identity {
		t.Fatalf("both objects must remain tracked: %+v, %v", st.Items, err)
	}
	r.commit("debug.log")
	now = d.Item.ExpiresAt.Add(time.Second)
	out, err := r.engine.Svc.Sweep(false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionRefused || !strings.Contains(out[0].Detail, "Git tracks") {
		t.Fatalf("new log must recheck Git: %+v, %v", out, err)
	}
	for _, p := range []string{path, keep} {
		if _, err := os.Stat(p); err != nil {
			t.Fatal(err)
		}
	}
	st, err = r.engine.Svc.Store.Read()
	if err != nil || len(st.Items) != 1 || st.Items[0].ID != old[0].Item.ID || len(st.Burials) != 0 {
		t.Fatalf("old object lost: %+v, %v", st, err)
	}
}

func TestMovedChildLookupFailureRetries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, stage := range []string{"apply", "approve", "sweep"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			now := time.Now()
			r.engine.Svc.Now = func() time.Time { return now }
			keep := r.write("keep.txt", "long lived")
			added, err := r.engine.Svc.Add([]string{keep}, core.AddOptions{ExpiresAt: now.Add(30 * 24 * time.Hour)})
			if err != nil || core.Failed(added) {
				t.Fatalf("add: %+v, %v", added, err)
			}
			r.write("test-results/report.xml", "report")
			dir := filepath.Join(r.root, "test-results")
			if stage == "sweep" {
				d := r.apply(dir)[dir]
				if d.Action != Registered {
					t.Fatalf("register: %+v", d)
				}
				now = d.Item.ExpiresAt.Add(time.Second)
			}
			moved := filepath.Join(dir, "keep.txt")
			if err := os.Rename(keep, moved); err != nil {
				t.Fatal(err)
			}
			fault := errors.New("temporary Locate failure")
			r.engine.Svc.FS = &lookupFS{failID: *added[0].Item.Identity, err: fault, calls: map[core.Identity]int{}}
			switch stage {
			case "apply":
				if d := r.apply(dir)[dir]; d.Action != Retry || !strings.Contains(d.Reason, fault.Error()) {
					t.Errorf("must skip unknown child location: %+v", d)
				}
			case "approve":
				p, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, r.root, "s", []core.Candidate{{Path: dir, Lifespan: "1d"}})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := r.engine.Approve([]string{fmt.Sprint(p.ID)}); !errors.Is(err, fault) {
					t.Errorf("must return retryable lookup error: %v", err)
				}
				st, err := r.engine.Svc.Store.Read()
				if err != nil || len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 1 {
					t.Errorf("retry candidate lost: %+v, %v", st.Proposals, err)
				}
			case "sweep":
				out, err := r.engine.Svc.Sweep(false)
				if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry || !strings.Contains(out[0].Detail, fault.Error()) {
					t.Errorf("must retry: %+v, %v", out, err)
				}
			}
			st, err := r.engine.Svc.Store.Read()
			want := 1
			if stage == "sweep" {
				want = 2
			}
			if err != nil || len(st.Items) != want || len(st.Burials) != 0 {
				t.Errorf("unsafe ledger result: %+v, %v", st, err)
			}
			if data, err := os.ReadFile(moved); err != nil || string(data) != "long lived" {
				t.Fatalf("child must remain: %q, %v", data, err)
			}
		})
	}
}

func TestRuleScanFailureRetriesAndSkipsRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, stage := range []string{"sweep", "apply", "approve", "leftovers"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			now := time.Now()
			r.engine.Svc.Now = func() time.Time { return now }
			file := r.write(rules.FileName, "!*.keep\n")
			if _, err := r.engine.Trust.Trust(file, ""); err != nil {
				t.Fatal(err)
			}
			if err := r.engine.Snapshot(r.root, "s"); err != nil {
				t.Fatal(err)
			}
			child := r.write("test-results/report.log", "keep")
			dir := filepath.Join(r.root, "test-results")
			if stage == "sweep" {
				d := r.apply(dir)[dir]
				if d.Action != Registered {
					t.Fatalf("register: %+v", d)
				}
				now = d.Item.ExpiresAt.Add(time.Second)
			}
			denyEnumeration(t, dir)
			if err := r.engine.RecheckRules(r.root, dir, true); err == nil || errors.Is(err, core.ErrProtected) {
				t.Errorf("scan error must stay retryable: %v", err)
			}
			switch stage {
			case "sweep":
				out, err := r.engine.Svc.Sweep(false)
				if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry {
					t.Errorf("scan failure must retry: %+v, %v", out, err)
				}
			case "apply":
				if d := r.apply(child)[child]; d.Action != Retry {
					t.Errorf("scan failure registered: %+v", d)
				}
			case "approve":
				p, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, r.root, "s", []core.Candidate{{Path: dir, Lifespan: "1d"}})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := r.engine.Approve([]string{fmt.Sprint(p.ID)}); err == nil {
					t.Error("approval must report retryable scan error")
				}
			case "leftovers":
				if _, err := r.engine.Leftovers(r.root, "s"); err == nil {
					t.Fatal("leftover rule scan failure must remain retryable")
				}
			}
			st, err := r.engine.Svc.Store.Read()
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if stage == "sweep" {
				want = 1
			}
			if len(st.Items) != want || len(st.Burials) != 0 {
				t.Errorf("scan failure lost or added a lifespan: %+v", st)
			}
			if stage == "sweep" && len(st.Items) == 1 && (st.Items[0].Attempts != 1 || !st.Items[0].NextAttempt.After(now)) {
				t.Errorf("retry backoff missing: %+v", st.Items[0])
			}
			if stage == "approve" && (len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 1) {
				t.Error("scan failure lost proposal")
			}
			if stage == "leftovers" && len(st.Proposals) != 0 {
				t.Errorf("unreadable directory proposed: %+v", st.Proposals)
			}
		})
	}
}

func TestBatchPreservesExistingLifespanAndRetriesLookupErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, stage := range []string{"apply", "observe", "leftovers", "lookup-error"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			keep := r.write("keep.txt", "long lived")
			added, err := r.engine.Svc.Add([]string{keep}, core.AddOptions{ExpiresAt: time.Now().Add(30 * 24 * time.Hour)})
			if err != nil || core.Failed(added) {
				t.Fatalf("add: %+v, %v", added, err)
			}
			if err := r.engine.Svc.Store.Update(func(st *core.State) error {
				st.Projects = map[string]core.ProjectState{core.ProjectKey(r.root): {Consent: core.ConsentApproved}}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := r.engine.Snapshot(r.root, "s"); err != nil {
				t.Fatal(err)
			}
			paths := []string{r.write("one.log", "1"), r.write("two.log", "2"), r.write("three.log", "3")}
			fs := &lookupFS{calls: map[core.Identity]int{}}
			r.engine.Svc.FS = fs
			switch stage {
			case "apply":
				_, err = r.engine.Apply(paths, Options{})
			case "observe":
				_, _, err = r.engine.Observe(paths, "s")
			case "leftovers":
				r.write("one.txt", "1")
				r.write("two.txt", "2")
				_, err = r.engine.Leftovers(r.root, "s")
			case "lookup-error":
				fs.failID, fs.err = *added[0].Item.Identity, errors.New("temporarily unavailable")
				var ds []Decision
				ds, err = r.engine.Apply(paths, Options{})
				for _, d := range ds {
					if d.Action != Registered {
						t.Errorf("unrelated lookup failure blocked independent file: %+v", d)
					}
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			st, err := r.engine.Svc.Store.Read()
			if err != nil {
				t.Fatal(err)
			}
			idx := st.Index(added[0].Item.ID)
			if idx < 0 || st.Items[idx].Source != "cli" || !st.Items[idx].ExpiresAt.Equal(added[0].Item.ExpiresAt) {
				t.Fatalf("batch changed existing lifespan: %+v", st.Items)
			}
			want := 4 // unrelated errors no longer stop ordinary-file registration
			if len(st.Items) != want {
				t.Fatalf("batch registration count: %+v", st.Items)
			}
			fs.err = nil
			fresh := r.write("next.log", "next batch")
			if d := r.apply(fresh)[fresh]; d.Action != Registered {
				t.Fatalf("new batch must refresh errors and locations: %+v", d)
			}
		})
	}
}

func TestRegistrationIgnoresUnrelatedHardLinkedIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("uses a real Git repository and Windows file identities")
	}
	r := newRepo(t)
	oldPath := r.write("keep.txt", "existing long-lived content")
	old, err := r.engine.Svc.Add([]string{oldPath}, core.AddOptions{ExpiresAt: time.Now().Add(30 * 24 * time.Hour)})
	if err != nil || core.Failed(old) {
		t.Fatalf("register old file: %+v, %v", old, err)
	}
	if err := os.Link(oldPath, filepath.Join(r.root, "keep-alias.txt")); err != nil {
		t.Fatal(err)
	}
	fresh := r.write("fresh.log", "new unrelated file")
	if d := r.apply(fresh)[fresh]; d.Action != Registered {
		t.Fatalf("unrelated unsafe identity blocked a proven independent file: %+v", d)
	}
	state, err := r.engine.Svc.Store.Read()
	idx := state.Index(old[0].Item.ID)
	if err != nil || len(state.Items) != 2 || idx < 0 || !state.Items[idx].ExpiresAt.Equal(old[0].Item.ExpiresAt) {
		t.Fatalf("lookup failure changed lifespans: %+v, %v", state.Items, err)
	}
	events, err := r.engine.Svc.RegistrationLocationDiagnostics()
	if err != nil || len(events) != 1 || events[0].ID != old[0].Item.ID || !strings.Contains(events[0].Detail, "hard links") {
		t.Fatalf("unrelated identity error disappeared from diagnostics: %+v, %v", events, err)
	}
}

func TestRegistrationMatchesFullLocate(t *testing.T) {
	if testing.Short() {
		t.Skip("compares registration using real Git and Windows file identities")
	}
	for _, kind := range []string{"unrelated", "renamed-file", "renamed-parent", "hardlink", "replacement", "case", "directory"} {
		for _, full := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/full=%v", kind, full), func(t *testing.T) {
				r := newRepo(t)
				path, kept, want := registrationConflictFixture(t, r, kind)
				if full {
					// Hide only the optional proof capability; all native safety
					// operations still run against the same real filesystem.
					r.engine.Svc.FS = struct{ core.FS }{r.engine.Svc.FS}
				}
				got, err := r.engine.Apply([]string{path}, Options{})
				if err != nil || len(got) != 1 || got[0].Action != want {
					t.Fatalf("scope missed registration conflict: %+v, %v; want %s", got, err, want)
				}
				st, err := r.engine.Svc.Store.Read()
				idx := st.Index(kept.ID)
				if err != nil || idx < 0 || st.Items[idx].Rev != kept.Rev || !st.Items[idx].ExpiresAt.Equal(kept.ExpiresAt) {
					t.Fatalf("registration changed an existing lifespan: %+v %v", st.Items, err)
				}
			})
		}
	}
}

func registrationConflictFixture(t *testing.T, r *repo, kind string) (string, core.Item, string) {
	t.Helper()
	old := r.write("keep.txt", "keep for thirty days")
	if kind == "renamed-parent" {
		old = filepath.Dir(r.write("kept/keep.txt", "keep"))
	}
	if kind == "replacement" || kind == "case" {
		old = r.write("fresh.log", "old content")
	}
	rows, err := r.engine.Svc.Add([]string{old}, core.AddOptions{ExpiresAt: time.Now().Add(30 * 24 * time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("seed lifespan: %+v %v", rows, err)
	}
	path, want := filepath.Join(r.root, "fresh.log"), Skipped
	switch kind {
	case "unrelated":
		path, want = r.write("fresh.log", "new"), Registered
	case "renamed-file":
		err = os.Rename(old, path)
	case "renamed-parent":
		err = os.Rename(old, filepath.Join(r.root, "moved"))
		path = r.write("moved/fresh.log", "new child")
	case "hardlink":
		err = os.Link(old, path)
		want = Retry // LongPath refuses hard links before rule matching, in both paths
	case "replacement":
		err = os.Rename(old, filepath.Join(r.root, "saved.txt"))
		path = r.write("fresh.log", "replacement")
	case "case":
		path = strings.ToUpper(old)
	case "directory":
		err = os.Link(old, old+".alias")
		path, want = filepath.Dir(r.write("test-results/report.xml", "report")), Retry
	}
	if err != nil {
		t.Fatal(err)
	}
	return path, *rows[0].Item, want
}

type unavailableFileAncestry struct{ core.FS }

func TestRegistrationRetainsUnverifiableRecord(t *testing.T) {
	if testing.Short() {
		t.Skip("uses a real Git repository and Windows file identities")
	}
	for _, kind := range []string{"legacy", "partial", "unavailable-volume"} {
		t.Run(kind, func(t *testing.T) {
			r := newRepo(t)
			old := r.write("keep.txt", "keep")
			rows, err := r.engine.Svc.Add([]string{old}, core.AddOptions{ExpiresAt: time.Now().Add(30 * 24 * time.Hour)})
			if err != nil || core.Failed(rows) {
				t.Fatalf("old lifespan: %+v %v", rows, err)
			}
			if err := r.engine.Svc.Store.Update(func(st *core.State) error {
				switch kind {
				case "legacy":
					st.Items[0].Identity = nil
				case "partial":
					st.Items[0].Identity.FileID = ""
				case "unavailable-volume":
					st.Items[0].Identity.Volume = filepath.Join(r.root, "absent-volume")
					st.Items[0].Identity.Serial++
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			path := r.write("fresh.log", "new")
			if d := r.apply(path)[path]; d.Action != Retry {
				t.Fatalf("unverifiable record bypassed registration: %+v", d)
			}
		})
	}
}

func (unavailableFileAncestry) OpenFileAncestry(string, core.Identity) (core.FileAncestry, error) {
	return nil, os.ErrPermission
}

func TestRegistrationAncestryFailureKeepsFullLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("uses a real Git repository and Windows file identities")
	}
	r := newRepo(t)
	old := r.write("keep.txt", "keep")
	rows, err := r.engine.Svc.Add([]string{old}, core.AddOptions{ExpiresAt: time.Now().Add(30 * 24 * time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("old lifespan: %+v %v", rows, err)
	}
	if err := os.Link(old, old+".alias"); err != nil {
		t.Fatal(err)
	}
	r.engine.Svc.FS = unavailableFileAncestry{r.engine.Svc.FS}
	path := r.write("fresh.log", "fresh")
	if d := r.apply(path)[path]; d.Action != Retry || !strings.Contains(d.Reason, "hard links") {
		t.Fatalf("unproven ancestry skipped full lookup: %+v", d)
	}
}

func TestExpiryRetriesWhenGitUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := newRepo(t)
	now := time.Now()
	r.engine.Svc.Now = func() time.Time { return now }
	path := r.write("debug.log", "keep this file")
	d := r.apply(path)[path]
	if d.Action != Registered || d.Item == nil || d.Item.Source != "rule" {
		t.Fatalf("automatic registration: %+v", d)
	}
	before := *d.Item
	now = before.ExpiresAt.Add(time.Second)
	t.Setenv("PATH", t.TempDir())

	out, err := r.engine.Svc.Sweep(false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry {
		t.Errorf("Git unavailable at expiry must retry: %+v, %v", out, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep this file" {
		t.Errorf("file must remain untouched at %s: %q, %v", path, data, err)
	}
	state, err := r.engine.Svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Items) != 1 || state.Items[0].ID != before.ID {
		t.Fatalf("automatic lifespan must stay in the ledger: %+v", state.Items)
	}
	after := state.Items[0]
	if after.Attempts != before.Attempts+1 || after.LastError == "" || !after.NextAttempt.After(now) || !after.ExpiresAt.Equal(before.ExpiresAt) {
		t.Fatalf("retry must record the error and back off without losing expiry: %+v", after)
	}
}

func TestExpiryRechecksRulesAndProject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	dir := filepath.Join(r.root, "test-results")
	r.write("test-results/a.xml", "x")
	r.write("nested/README.md", "x")
	if _, err := r.apply(dir)[dir], error(nil); err != nil {
		t.Fatal(err)
	}
	// Later the project protects something inside it.
	r.write(rules.FileName, "!test-results/keep.xml\n")
	if _, err := r.engine.Trust.Trust(filepath.Join(r.root, rules.FileName), ""); err != nil {
		t.Fatal(err)
	}
	r.write("test-results/keep.xml", "precious")
	if err := r.engine.Svc.Store.Update(func(st *core.State) error {
		st.Items[0].ExpiresAt = st.Items[0].CreatedAt
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	out, err := r.engine.Svc.Sweep(false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionRefused {
		t.Fatalf("a directory that now holds a protected file must not be buried, got %+v %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.xml")); err != nil {
		t.Fatal("keep.xml must stay")
	}
}

func TestExpiryRetriesUnreadableRules(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	now := time.Now()
	r.engine.Svc.Now = func() time.Time { return now }
	file := r.write(rules.FileName, "*.log 1d\n")
	if _, err := r.engine.Trust.Trust(file, ""); err != nil {
		t.Fatal(err)
	}
	path := r.write("debug.log", "keep")
	d := r.apply(path)[path]
	if d.Action != Registered {
		t.Fatalf("register: %+v", d)
	}
	if err := os.Rename(file, file+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0o700); err != nil {
		t.Fatal(err)
	}
	now = d.Item.ExpiresAt.Add(time.Second)
	out, err := r.engine.Svc.Sweep(false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry {
		t.Errorf("unreadable rules must retry: %+v, %v", out, err)
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Items) != 1 || st.Items[0].Attempts != 1 || !st.Items[0].NextAttempt.After(now) {
		t.Errorf("lifespan must remain with backoff: %+v, %v", st.Items, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Errorf("file must remain: %q, %v", data, err)
	}
}

func TestBurialRechecksGitAfterRuleInspection(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	now := time.Now()
	r.engine.Svc.Now = func() time.Time { return now }
	path := r.write("debug.log", "keep")
	d := r.apply(path)[path]
	if d.Action != Registered {
		t.Fatalf("register: %+v", d)
	}
	now = d.Item.ExpiresAt
	r.engine.Svc.Recheck = func(root, path string, isDir bool) error {
		err := r.engine.RecheckRules(root, path, isDir)
		r.git("add", "--", "debug.log")
		return err
	}
	out, err := r.engine.Svc.Sweep(false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionRefused {
		t.Fatalf("Git staging during the final rule check must prevent burial: %+v, %v", out, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Fatalf("staged object was moved: %q, %v", data, err)
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Burials) != 0 || len(st.Items) != 0 {
		t.Fatalf("refused object left a burial: %+v, %v", st, err)
	}
}

func TestAutomaticBurialGitToMoveWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := newRepo(t)
	now := time.Now()
	r.engine.Svc.Now = func() time.Time { return now }
	path := r.write("debug.log", "keep")
	d := r.apply(path)[path]
	if d.Action != Registered {
		t.Fatalf("register: %+v", d)
	}
	now = d.Item.ExpiresAt
	var checked time.Time
	r.engine.Svc.Owner = func(path string) (string, bool, error) {
		root, tracked, err := Owner(path) // real Git in a real disposable repository
		checked = time.Now()
		return root, tracked, err
	}
	fs := &timedBurialFS{FS: r.engine.Svc.FS, checked: &checked}
	r.engine.Svc.FS = fs
	out, err := r.engine.Svc.Sweep(false)
	st, readErr := r.engine.Svc.Store.Read()
	if err != nil || readErr != nil || len(out) != 1 || out[0].Action != core.ActionBuried || len(st.Burials) != 1 {
		t.Fatalf("automatic burial: %+v, %v, %v", out, err, readErr)
	}
	if data, err := os.ReadFile(st.Burials[0].Grave); err != nil || string(data) != "keep" {
		t.Fatalf("buried content: %q, %v", data, err)
	}
	t.Logf("sample only: Git return to MovePinned entry=%s; pinned open/identity/rename=%s; no scheduling upper bound", fs.gap, fs.move)
}

func TestSweepRefusesCommittedFileWithInheritedGitEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, key := range []string{"GIT_INDEX_FILE", "GIT_DIR"} {
		t.Run(key, func(t *testing.T) {
			r, other := newRepo(t), newRepo(t)
			path := r.write("debug.log", "must survive")
			d := r.apply(path)[path]
			if d.Action != Registered {
				t.Fatalf("registration: %+v", d)
			}
			r.commit("debug.log")
			value := filepath.Join(other.root, ".git")
			if key == "GIT_INDEX_FILE" {
				value = filepath.Join(other.root, "absent-index")
			}
			t.Setenv(key, value)
			r.engine.Svc.Now = func() time.Time { return d.Item.ExpiresAt }
			out, err := r.engine.Svc.Sweep(false)
			if err != nil || len(out) != 1 || out[0].Action != core.ActionRefused {
				t.Fatalf("committed file was not refused: %+v, %v", out, err)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "must survive" {
				t.Fatalf("committed content lost: %q, %v", data, err)
			}
		})
	}
}
