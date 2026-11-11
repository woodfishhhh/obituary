//go:build windows

package e2e_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestDeleteGuardDemoRestoresBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	repo := demoRepo(t)
	demoRun(t, "init", "--yes", "--guard-rm")
	settingsPath := filepath.Join(repo, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	var settings struct {
		Hooks map[string][]struct {
			Matcher string
			Hooks   []struct {
				Type, Command string
				Args          []string
			}
		}
	}
	if err != nil || json.Unmarshal(data, &settings) != nil {
		t.Fatalf("project hook settings: %s %v", data, err)
	}
	groups := settings.Hooks["PreToolUse"]
	if len(groups) != 1 || groups[0].Matcher != ".*" || len(groups[0].Hooks) != 1 || groups[0].Hooks[0].Type != "command" || !filepath.IsAbs(groups[0].Hooks[0].Command) || strings.Join(groups[0].Hooks[0].Args, " ") != "hook claude" {
		t.Fatalf("project PreToolUse guard not installed: %s", data)
	}

	build := filepath.Join(repo, "build")
	want := demoBuildTree(t, build)
	payload, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse", "cwd": repo, "tool_name": "Bash",
		"session_id": "demo-agent", "tool_use_id": "demo-delete",
		"tool_input": map[string]string{"command": "rm -rf build"},
	})
	if err != nil {
		t.Fatal(err)
	}
	probe := exec.Command(binaryPath(t, "obit"), "hook", "claude")
	var stdout, stderr bytes.Buffer
	probe.Stdin, probe.Stdout, probe.Stderr = bytes.NewReader(payload), &stdout, &stderr
	if err := probe.Run(); err != nil {
		t.Fatalf("PreToolUse: stdout=%s stderr=%s %v", &stdout, &stderr, err)
	}
	var response struct {
		Hook struct {
			Event    string `json:"hookEventName"`
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil || response.Hook.Event != "PreToolUse" || response.Hook.Decision != "deny" {
		t.Fatalf("PreToolUse denial: %s %v", &stdout, err)
	}
	parts := strings.Split(response.Hook.Reason, "`")
	// demoRepo puts this same executable first on PATH.
	wantCommand := "obit rm -rf build"
	if len(parts) < 3 || parts[1] != wantCommand {
		t.Fatalf("equivalent command: %s; want %s", response.Hook.Reason, wantCommand)
	}
	demoAssertTree(t, build, want)
	demoGraves(t)
	demoRunSuggestion(t, parts[1])
	demoAbsent(t, build)
	grave := demoGraves(t, build)[build]
	demoAssertTree(t, grave.Grave, want)

	demoRun(t, "resurrect", "--last")
	demoAssertTree(t, build, want)
	demoGraves(t)
	demoNoLast(t)
	demoAssertTree(t, build, want)
}

func TestDeleteGuardDemoLastStaysInProject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	repoA := demoRepo(t)
	repoB := filepath.Join(filepath.Dir(repoA), "repo-b")
	testenv.InitRepo(t, repoB)
	buildA, buildB := filepath.Join(repoA, "build"), filepath.Join(repoB, "build")
	wantA, wantB := demoBuildTree(t, buildA), demoBuildTree(t, buildB)
	// No delay or second-resolution comparison separates the two commands.
	demoRun(t, "rm", "-rf", "build")
	t.Chdir(repoB)
	demoRun(t, "rm", "-rf", "build")
	demoAbsent(t, buildA, buildB)
	graves := demoGraves(t, buildA, buildB)
	first, second := graves[buildA].Operation, graves[buildB].Operation
	if first.Project != repoA || second.Project != repoB || first.ID == second.ID {
		t.Fatalf("project batches: A=%+v B=%+v", first, second)
	}
	t.Logf("A operation=%s startedAt=%s; B operation=%s startedAt=%s", first.ID, first.StartedAt.Format(time.RFC3339Nano), second.ID, second.StartedAt.Format(time.RFC3339Nano))

	t.Chdir(repoA)
	demoRun(t, "resurrect", "--last")
	demoAssertTree(t, buildA, wantA)
	demoAbsent(t, buildB)
	remaining := demoGraves(t, buildB)[buildB]
	if !reflect.DeepEqual(remaining, graves[buildB]) {
		t.Fatalf("project B burial changed: got %+v; want %+v", remaining, graves[buildB])
	}
	demoAssertTree(t, remaining.Grave, wantB)
	demoNoLast(t)
	demoAbsent(t, buildB)
	demoGraves(t, buildB)

	t.Chdir(repoB)
	demoRun(t, "resurrect", "--last")
	demoAssertTree(t, buildB, wantB)
	demoGraves(t)
}

func TestDeleteGuardDemoLastRestoresAllOperands(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	repo := demoRepo(t)
	paths := []string{filepath.Join(repo, "a"), filepath.Join(repo, "b"), filepath.Join(repo, "c")}
	want := demoBuildTree(t, paths[0])
	writeFixture(t, paths[1], "second operand\x00\xff\r\n")
	writeFixture(t, paths[2], "")
	demoRun(t, "rm", "-rf", "a", "b", "c")
	demoAbsent(t, paths...)
	graves := demoGraves(t, paths...)
	batch := graves[paths[0]].Operation
	for _, path := range paths {
		operation := graves[path].Operation
		if operation.ID != batch.ID || !operation.StartedAt.Equal(batch.StartedAt) || operation.Project != repo {
			t.Fatalf("rm operands have different operations: %s %+v; want %+v", path, operation, batch)
		}
	}
	demoRun(t, "resurrect", "--last")
	demoAssertTree(t, paths[0], want)
	for path, want := range map[string]string{paths[1]: "second operand\x00\xff\r\n", paths[2]: ""} {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("restored operand %s: %q %v; want %q", path, got, err, want)
		}
	}
	demoGraves(t)
	demoNoLast(t)
}

func TestDeleteGuardDemoLastOrdersConsecutiveBatches(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	repo := demoRepo(t)
	older, newer := filepath.Join(repo, "older"), filepath.Join(repo, "newer")
	writeFixture(t, older, "older bytes")
	writeFixture(t, newer, "newer bytes")
	// Keep real start times intact: sleeping or rewriting the ledger would
	// conceal a loss of sub-second precision across CLI process boundaries.
	demoRun(t, "rm", "-rf", "older")
	demoRun(t, "rm", "-rf", "newer")
	graves := demoGraves(t, older, newer)
	first, second := graves[older].Operation, graves[newer].Operation
	if first.ID == second.ID || !second.StartedAt.After(first.StartedAt) {
		t.Fatalf("consecutive batches cannot be ordered: older=%+v newer=%+v", first, second)
	}
	t.Logf("older operation=%s startedAt=%s; newer operation=%s startedAt=%s; same second=%t", first.ID, first.StartedAt.Format(time.RFC3339Nano), second.ID, second.StartedAt.Format(time.RFC3339Nano), first.StartedAt.Unix() == second.StartedAt.Unix())
	for _, path := range []string{newer, older} {
		demoRun(t, "resurrect", "--last")
		if got, err := os.ReadFile(path); err != nil || string(got) != filepath.Base(path)+" bytes" {
			t.Fatalf("restored batch %s: %q %v", path, got, err)
		}
		if path == newer {
			demoAbsent(t, older)
			demoGraves(t, older)
		}
	}
	demoGraves(t)
	demoNoLast(t)
}

func demoRepo(t *testing.T) string {
	t.Helper()
	// Isolate agent settings so real-process protocol and exit-code checks
	// cannot change the developer's configuration.
	root := t.TempDir()
	for name, dir := range map[string]string{"OBIT_USER_HOME": "user", "CLAUDE_CONFIG_DIR": "claude", "CODEX_HOME": "codex"} {
		path := filepath.Join(root, dir)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(name, path)
	}
	for _, name := range []string{"OBIT_SESSION_ID", "CLAUDE_CODE_SESSION_ID", "CODEX_THREAD_ID"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("BASH_ENV", "")
	t.Setenv("OBIT_DISABLED", "0")
	t.Setenv("OBIT_GUARD_RM", "1")
	t.Setenv("OBIT_HOOK_AUDIT", "0")
	t.Setenv("PATH", filepath.Dir(binaryPath(t, "obit"))+string(os.PathListSeparator)+os.Getenv("PATH"))
	// commandRepo isolates OBIT_HOME/OBIT_CONFIG_DIR, starts a disposable
	// resident before CLI calls, and stops it with obit daemon stop at cleanup.
	repo, _ := commandRepo(t)
	repo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	return repo
}

func demoRun(t *testing.T, command string, args ...string) string {
	t.Helper()
	out, stderr, err := execute(t, command, args...)
	if err != nil {
		t.Fatalf("obit %s %q: stdout=%s stderr=%s %v", command, args, out, stderr, err)
	}
	return out
}

func demoRunSuggestion(t *testing.T, suggestion string) {
	t.Helper()
	var candidates []string
	if git, err := exec.LookPath("git"); err == nil {
		root := filepath.Dir(filepath.Dir(git))
		candidates = append(candidates, filepath.Join(root, "bin", "bash.exe"), filepath.Join(root, "usr", "bin", "bash.exe"))
	}
	candidates = append(candidates, `C:\Program Files\Git\bin\bash.exe`)
	for _, bash := range candidates {
		if info, err := os.Stat(bash); err == nil && !info.IsDir() {
			cmd := exec.Command(bash, "--noprofile", "--norc", "-c", suggestion)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("Git Bash suggestion: %s %v", out, err)
			}
			t.Logf("executed exact hook suggestion through %s", bash)
			return
		}
	}
	// The assertion above fixes the entire literal argv, so direct execution
	// is equivalent on machines where Git Bash is unavailable.
	t.Log("Git Bash bash.exe unavailable; executing obit.exe rm -rf build directly")
	demoRun(t, "rm", "-rf", "build")
}

func demoNoLast(t *testing.T) {
	t.Helper()
	out, stderr, err := execute(t, "resurrect", "--last")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(stderr, "no deletion batch") || !strings.Contains(stderr, "obit graveyard") {
		t.Fatalf("expected exhausted --last error: stdout=%s stderr=%s %v", out, stderr, err)
	}
}

func demoGraves(t *testing.T, paths ...string) map[string]core.Burial {
	t.Helper()
	out := demoRun(t, "graveyard", "--json")
	var listing struct {
		Burials      []core.Burial
		Pending      []json.RawMessage
		PendingCount int
		Warning      string
	}
	if err := json.Unmarshal([]byte(out), &listing); err != nil || len(listing.Burials) != len(paths) || len(listing.Pending) != 0 || listing.PendingCount != 0 || listing.Warning != "" {
		t.Fatalf("graveyard: %s %v; want paths %q", out, err, paths)
	}
	byPath := make(map[string]core.Burial)
	for _, burial := range listing.Burials {
		if burial.ID <= 0 || burial.Operation == nil || burial.Operation.ID == "" || burial.Operation.StartedAt.IsZero() || burial.Operation.Session != "" {
			t.Fatalf("missing project-only deletion operation: %+v", burial)
		}
		byPath[burial.Path] = burial
	}
	for _, path := range paths {
		if _, ok := byPath[path]; !ok {
			t.Fatalf("missing grave %s: %s", path, out)
		}
	}
	return byPath
}

func demoAbsent(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("deleted path still present: %s %v", path, err)
		}
	}
}

type demoTreeEntry struct {
	Directory bool
	Data      string
}

func demoBuildTree(t *testing.T, root string) map[string]demoTreeEntry {
	t.Helper()
	for _, dir := range []string{"assets/中文/深层", "empty-dir", "nested/empty"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for path, data := range map[string]string{
		"index.html": "<html>demo\r\n</html>\n", "assets/中文/深层/数据.bin": "\x00\xff\x80\r\n\x00",
		"assets/中文/说明.txt": "产物保留原样\n", "empty-file": "", "nested/empty/空文件": "",
	} {
		writeFixture(t, filepath.Join(root, path), data)
	}
	return demoTree(t, root)
}

func demoTree(t *testing.T, root string) map[string]demoTreeEntry {
	t.Helper()
	tree := make(map[string]demoTreeEntry)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		value := demoTreeEntry{Directory: entry.IsDir()}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value.Data = string(data)
		}
		tree[filepath.ToSlash(rel)] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func demoAssertTree(t *testing.T, root string, want map[string]demoTreeEntry) {
	t.Helper()
	if got := demoTree(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("tree %s differs: got %#v; want %#v", root, got, want)
	}
}
