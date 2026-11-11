//go:build windows

package testenv_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"golang.org/x/sys/windows"
)

func TestUpdateChecksDisabledInTestEnvironment(t *testing.T) {
	if os.Getenv("OBIT_TEST_NO_UPDATE") != "1" {
		t.Fatal("public update checks are enabled")
	}
}

func TestRegistryLocksStayInScratch(t *testing.T) {
	data, err := userdirs.Data()
	if err != nil {
		t.Fatal(err)
	}
	locks, err := userdirs.RegistryLocks()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(filepath.Dir(data), locks)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		t.Fatalf("registry locks escaped test isolation: %s %v", locks, err)
	}
}

func TestGraveyardLeakDetection(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"new-ledger", "existing-ledger", "missing", "unreadable"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), "graveyard")
			if mode == "existing-ledger" {
				if err := os.MkdirAll(filepath.Join(root, "user-ledger"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "unreadable" {
				if err := os.WriteFile(root, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			added := testenv.GraveyardChanges(root, func() {
				var path string
				switch mode {
				case "new-ledger":
					path = filepath.Join(root, "test-ledger")
				case "existing-ledger":
					path = filepath.Join(root, "user-ledger", "legitimate-burial")
				case "unreadable":
					if err := os.Remove(root); err != nil {
						t.Fatal(err)
					}
					path = filepath.Join(root, "user-ledger")
				default:
					return
				}
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			})
			if mode == "new-ledger" {
				if len(added) != 1 || added[0] != filepath.Join(root, "test-ledger") {
					t.Fatalf("new ledger leak was not identified exactly: %v", added)
				}
			} else if len(added) != 0 {
				t.Fatalf("unrelated user content reported as a test leak: %v", added)
			}
		})
	}
}

func TestGraveyardOwnerFileLeakDetection(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{".owner", ".owner.lock", ".owner.tmp"} {
		t.Run(suffix, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, "ledger"+suffix)
			added := testenv.GraveyardChanges(root, func() {
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			})
			if len(added) != 1 || added[0] != path {
				t.Fatalf("owner file leak not detected: %v", added)
			}
		})
	}
}

func TestIsolationSurvivesHostileInheritedEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestIsolationChild$")
	cmd.Env = append(os.Environ(), "OBIT_ISOLATION_CHILD=1", "GIT_DIR=Z:\\missing", "GIT_WORK_TREE=Z:\\missing", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=user.name", "GIT_CONFIG_VALUE_0=leaked", "GIT_CONFIG_GLOBAL=Z:\\missing")
	cmd.Env = append(cmd.Env, "OBIT_GUARD_RM=0", "OBIT_DISABLED=1", "OBIT_HOOK_AUDIT=1", "OBIT_SESSION_ID=foreign-session")
	cmd.Env = append(cmd.Env, "CLAUDE_CODE_SESSION_ID=foreign-claude", "CODEX_THREAD_ID=foreign-codex")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child failed: %v\n%s", err, out)
	}
	var scratch string
	if err := json.Unmarshal([]byte(strings.SplitN(string(out), "\n", 2)[0]), &scratch); err != nil {
		t.Fatalf("child output: %s: %v", out, err)
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Fatalf("scratch survived TestMain: %s: %v", scratch, err)
	}
}

func TestIsolationExpandsShortTEMP(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root, err := os.MkdirTemp("", "short-temp-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 32768)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n >= uint32(len(buf)) {
		t.Fatalf("short TEMP: %d %v", n, err)
	}
	short := windows.UTF16ToString(buf[:n])
	if short == root {
		t.Skip("volume does not generate 8.3 aliases")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestIsolationChild$")
	cmd.Env = append(os.Environ(), "OBIT_ISOLATION_CHILD=1", "TEMP="+short, "TMP="+short)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("short TEMP child: %s %v", out, err)
	}
	var scratch string
	if err := json.Unmarshal([]byte(strings.SplitN(string(out), "\n", 2)[0]), &scratch); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(scratch, root+`\`) {
		t.Fatalf("scratch kept short TEMP: %s", scratch)
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Fatalf("short TEMP scratch survived: %s %v", scratch, err)
	}
}

func TestIsolationChild(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	if os.Getenv("OBIT_ISOLATION_CHILD") != "1" {
		t.Skip("subprocess assertion")
	}
	data, err := userdirs.Data()
	if err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Dir(data)
	if expanded, err := pathutil.LongAbs(scratch); err != nil || expanded != scratch {
		t.Fatalf("scratch is not a long path: %s => %s %v", scratch, expanded, err)
	}
	b, _ := json.Marshal(scratch)
	fmt.Println(string(b))
	for _, resolve := range []func() (string, error){userdirs.Data, userdirs.Config, userdirs.Home, userdirs.ClaudeDir, userdirs.CodexDir} {
		path, err := resolve()
		if err != nil || filepath.Dir(path) != scratch {
			t.Fatalf("directory escaped scratch: %s: %v", path, err)
		}
	}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(name), "GIT_") && name != "GIT_CONFIG_GLOBAL" && name != "GIT_CONFIG_NOSYSTEM" && name != "GIT_CEILING_DIRECTORIES" {
			t.Fatalf("inherited Git variable: %s", name)
		}
	}
	for _, name := range []string{"OBIT_GUARD_RM", "OBIT_DISABLED", "OBIT_HOOK_AUDIT", "OBIT_SESSION_ID", "CLAUDE_CODE_SESSION_ID", "CODEX_THREAD_ID"} {
		if value, ok := os.LookupEnv(name); ok {
			t.Errorf("inherited hook switch: %s=%s", name, value)
		}
	}
	repo := t.TempDir()
	if rel, err := filepath.Rel(scratch, repo); err != nil || strings.HasPrefix(rel, "..") {
		t.Fatalf("t.TempDir escaped scratch: %s: %v", repo, err)
	}
	probe := exec.Command("git", "-C", repo, "rev-parse", "--show-toplevel")
	if out, err := probe.CombinedOutput(); err == nil {
		t.Fatalf("non-repository fixture inherited a parent checkout: %s", out)
	}
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	if got := git("symbolic-ref", "--short", "HEAD"); got != "main" {
		t.Fatalf("branch = %s", got)
	}
	for key, want := range map[string]string{"user.name": "Obituary Tests", "user.email": "tests@obituary.invalid", "core.autocrlf": "false"} {
		if got := git("config", "--get", key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	git("commit", "--allow-empty", "-qm", "isolated")
	assertGitTemplateCopies(t)
}
