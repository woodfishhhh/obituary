package hook

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func TestPOSIXNativePathRejectsRemoteURIs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native POSIX path namespace")
	}
	for _, value := range []string{"ssh://host/work/file.log", "file:///tmp/file.log", "sftp://host/file.log"} {
		if got, ok := NativePath(value); ok || got != "" {
			t.Fatalf("URI was treated as a native path: %q -> %q %v", value, got, ok)
		}
	}
	for _, value := range []string{"/tmp/a:b", "/mnt/c/repo/file.log", "relative.log"} {
		if got, ok := NativePath(value); !ok || got != value {
			t.Fatalf("native spelling changed: %q -> %q %v", value, got, ok)
		}
	}
}

func TestPOSIXNativePathsRemainNative(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX hook path contract")
	}
	for _, path := range []string{"/tmp/payload", "/mnt/d/kept-as-posix", "relative/file", `/tmp/colon:back\slash`} {
		if got, ok := NativePath(path); !ok || got != path {
			t.Fatalf("native path changed: %q %v", got, ok)
		}
	}
	if _, ok := NativePath(""); ok {
		t.Fatal("empty path accepted")
	}
}

func TestCodexWindowsShellDeclaration(t *testing.T) {
	testenv.WindowsOnly(t, "Windows host shell declarations and PowerShell commandWindows registration")
	// Canonical hook shape from openai/codex rust-v0.167.0,
	// core/src/tools/handlers/unified_exec/exec_command.rs: Bash + command.
	for _, tc := range []struct{ name, config, cwd, command, want string }{
		{"bash", `codex_windows_shell = "bash"`, testenv.NativeFixture(`C:\repo`), `rm -rf build`, `rm -rf build`},
		{"powershell", `codex_windows_shell = "powershell"`, testenv.NativeFixture(`C:\repo`), `Remove-Item -LiteralPath 'build dir' -Recurse`, `remove-item -LiteralPath 'build dir' -Recurse`},
		{"powershell-native-path", `codex_windows_shell = "powershell"`, testenv.NativeFixture(`C:\repo`), `Remove-Item -LiteralPath .\build -Recurse`, `remove-item -LiteralPath .\build -Recurse`},
		{"unset", "", testenv.NativeFixture(`C:\repo`), `rm -rf build`, ""},
		{"bash-table", "[agent_shells]\ncodex = 'bash'", testenv.NativeFixture(`C:\repo`), `rm -rf build`, `rm -rf build`},
		{"powershell-table", "[agent_shells]\ncodex = 'powershell'", testenv.NativeFixture(`C:\repo`), `Remove-Item -LiteralPath build`, `remove-item -LiteralPath build`},
		{"both", "codex_windows_shell = 'bash'\n[agent_shells]\ncodex = 'bash'", testenv.NativeFixture(`C:\repo`), `rm -rf build`, `rm -rf build`},
		{"conflict", "codex_windows_shell = 'bash'\n[agent_shells]\ncodex = 'powershell'", testenv.NativeFixture(`C:\repo`), `rm -rf build`, ""},
		{"invalid", `codex_windows_shell = "cmd"`, testenv.NativeFixture(`C:\repo`), `rm -rf build`, ""},
		{"ps-declared-bash-source", `codex_windows_shell = "powershell"`, testenv.NativeFixture(`C:\repo`), `rm -rf build`, noEquivalent},
		{"bash-declared-ps-source", `codex_windows_shell = "bash"`, testenv.NativeFixture(`C:\repo`), `rm -LiteralPath build`, noEquivalent},
		{"ps-declared-bash-escape", `codex_windows_shell = "powershell"`, testenv.NativeFixture(`C:\repo`), `rm build\ dir`, noEquivalent},
		{"ps-declared-bash-path", `codex_windows_shell = "powershell"`, testenv.NativeFixture(`C:\repo`), `rm .\build`, noEquivalent},
		{"bash-declared-ps-path", `codex_windows_shell = "bash"`, testenv.NativeFixture(`C:\repo`), `rm .\build`, noEquivalent},
		{"bash-declared-ps-array", `codex_windows_shell = "bash"`, testenv.NativeFixture(`C:\repo`), `rm a,b`, noEquivalent},
		{"bash-declared-ps-quote", `codex_windows_shell = "bash"`, testenv.NativeFixture(`C:\repo`), `rm 'a''b'`, noEquivalent},
		{"bash-declared-ps-git-quote", `codex_windows_shell = "bash"`, testenv.NativeFixture(`C:\repo`), `git clean -fd -- 'a''b'`, noEquivalent},
		{"bash-git-clean", `codex_windows_shell = "bash"`, testenv.NativeFixture(`C:\repo`), `git clean -fdx`, `git-clean -fdx`},
		{"wsl", `codex_windows_shell = "powershell"`, "/mnt/c/repo", `rm -rf build`, `rm -rf build`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("OBIT_CONFIG_DIR", dir)
			if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(tc.config), 0600); err != nil {
				t.Fatal(err)
			}
			got := guardSuggestion(t, "codex", tc.cwd, "Bash", map[string]any{"command": tc.command})
			want := tc.want
			if want != "" && want != noEquivalent {
				shell := guardrm.Bash
				if strings.HasPrefix(tc.name, "powershell") {
					shell = guardrm.PowerShell
				}
				executable := expectedGuardExecutable(t, shell)
				if tc.name == "wsl" {
					self, err := os.Executable()
					if err != nil {
						t.Fatal(err)
					}
					self, err = filepath.EvalSymlinks(self)
					if err != nil {
						t.Fatal(err)
					}
					executable = WSLExecutable(self)
				}
				want = executable + " " + want
			}
			if got != want {
				t.Fatalf("declared shell payload: got %q, want %q", got, want)
			}
		})
	}
}

func BenchmarkCodexHook(b *testing.B) { benchmarkHook(b, "codex", Codex) }

func FuzzCodexPayload(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		var input codexInput
		if err := json.Unmarshal(data, &input); err != nil {
			return
		}
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		var again codexInput
		if err := json.Unmarshal(encoded, &again); err != nil || !reflect.DeepEqual(input, again) {
			t.Fatalf("Codex envelope changed on round trip: %v", err)
		}
		before := PatchPaths(firstString(input.ToolInput, "command", "patch", "input"))
		after := PatchPaths(firstString(again.ToolInput, "command", "patch", "input"))
		if !slices.Equal(before, after) {
			t.Fatalf("Codex patch paths changed: %q -> %q", before, after)
		}
		cwd, ok := NativePath(input.Cwd)
		if got, reachable := NativePath(again.Cwd); cwd != got || ok != reachable {
			t.Fatal("Codex cwd changed on round trip")
		}
	})
}

func FuzzPatchPaths(f *testing.F) {
	for _, patch := range []string{
		"*** Begin Patch\n*** Environment ID: remote\n*** Add File: debug.log\n+x\n*** End Patch",
		"*** Begin Patch\r\n\t*** Environment ID:remote \t\r\n*** Update File: debug.log\r\n@@\r\n-x\r\n+y\r\n*** End Patch",
		"*** Begin Patch\n*** Environment ID:   \n*** Update File: old.log\n*** Move to: new.log\n*** End Patch",
		"*** Begin Patch\n*** Environment ID: local\n*** Environment ID: other\n*** Add File: debug.log\n+x\n*** End Patch",
		"*** Begin Patch\n*** Update File: debug.log\n@@\n *** Environment ID: remote\n-x\n+y\n*** End Patch",
	} {
		f.Add(patch, "/mnt/c/repo/debug.log")
	}
	f.Fuzz(func(t *testing.T, patch, candidate string) {
		paths := PatchPaths(patch)
		var canonical strings.Builder
		for _, p := range paths {
			if p == "" || p != strings.TrimSpace(p) || strings.Contains(p, "\n") {
				t.Fatalf("invalid patch path %q", p)
			}
			canonical.WriteString("*** Add File: " + p + "\n")
		}
		if got := PatchPaths(canonical.String()); !slices.Equal(paths, got) {
			t.Fatalf("patch paths changed on round trip: %q -> %q", paths, got)
		}
		for _, prefix := range []string{" ", "+", "-", "\t"} {
			hunk := prefix + strings.ReplaceAll(patch, "\n", "\n"+prefix)
			if got := PatchPaths(hunk); len(got) != 0 {
				t.Fatalf("non-column-zero header accepted: prefix=%q, paths=%q", prefix, got)
			}
		}
		mapped, ok := NativePath(candidate)
		if !ok {
			return
		}
		if again, ok := NativePath(mapped); !ok || again != mapped {
			t.Fatalf("native path mapping is not idempotent: %q -> %q", candidate, mapped)
		}
		if strings.HasPrefix(candidate, "/") && runtime.GOOS == "windows" {
			clean := path.Clean(candidate)
			parts := strings.Split(strings.TrimPrefix(clean, "/"), "/")
			if len(parts) < 2 || parts[0] != "mnt" || len(parts[1]) != 1 || !strings.Contains("abcdefghijklmnopqrstuvwxyz", strings.ToLower(parts[1])) {
				t.Fatalf("unreachable POSIX path accepted: %q -> %q", candidate, mapped)
			}
			want := strings.ToUpper(parts[1]) + `:\` + filepath.FromSlash(strings.Join(parts[2:], "/"))
			if mapped != want {
				t.Fatalf("WSL drive or suffix changed: %q -> %q, want %q", candidate, mapped, want)
			}
		} else if candidate != mapped {
			t.Fatalf("native/relative input changed: %q -> %q", candidate, mapped)
		}
	})
}

func TestPatchPathsPreservesInteriorCarriageReturn(t *testing.T) {
	t.Parallel()
	// PatchPaths extracts header text; it does not validate Windows filenames.
	// A CR inside a header is data, unlike the CR at the end of a CRLF line.
	if got := PatchPaths("*** Add File: 0\r0\r\n"); !slices.Equal(got, []string{"0\r0"}) {
		t.Fatalf("interior header bytes changed: %q", got)
	}
}

func TestCodexNormalizesLongSession(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	var sessions []string
	runner := &Runner{Finish: func(_, session string, _ bool) { sessions = append(sessions, session) }}
	for range 2 {
		payload, err := json.Marshal(map[string]any{"hook_event_name": "SessionEnd", "session_id": strings.Repeat("s", 4097), "cwd": e.root})
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		Codex(e.engine, bytes.NewReader(payload), &out, runner)
	}
	if len(sessions) != 2 || len(sessions[0]) != 71 || sessions[0] != sessions[1] || !strings.HasPrefix(sessions[0], "sha256:") {
		t.Fatalf("unstable or oversized Codex session: %v", sessions)
	}
}

func TestPatchPathsListsWrittenFiles(t *testing.T) {
	t.Parallel()
	patch := `*** Begin Patch
*** Add File: scratch/debug.py
+print("x")
*** Update File: src/app.go
@@
-a
+b
*** Delete File: old.txt
*** Update File: notes.md
@@
 *** Add File: quoted-in-a-document.log
+*** Add File: also-just-text.log
*** Move to: docs/notes.md
*** End Patch`
	want := []string{"scratch/debug.py", "src/app.go", "notes.md", "docs/notes.md"}
	if got := PatchPaths(patch); !reflect.DeepEqual(got, want) {
		t.Fatalf("PatchPaths = %v; want %v", got, want)
	}
}

func TestPatchPathsRejectsUnverifiedEnvironment(t *testing.T) {
	t.Parallel()
	// Match openai/codex rust-v0.167.0 parser.rs's environment preamble tests
	// and streaming_parser.rs's whitespace, duplicate and empty ID handling.
	for _, marker := range []string{
		"*** Environment ID: remote",
		"*** Environment ID:remote",
		" \t*** Environment ID: container \t\r",
		"\u2003*** Environment ID: remote\u2003",
		"*** Environment ID:",
		"*** Environment ID:   ",
		"*** Environment ID: local",
		"*** Environment ID: default",
		"*** Environment ID: first\n*** Environment ID: second",
	} {
		t.Run(marker, func(t *testing.T) {
			patch := "*** Begin Patch\n" + marker + "\n*** Update File: old.log\n*** Move to: new.log\n@@\n-x\n+y\n*** End Patch"
			if got := PatchPaths(patch); len(got) != 0 {
				t.Fatalf("environment patch exposed local paths: %q", got)
			}
		})
	}
	for _, patch := range []string{
		"*** Begin Patch\n*** Add File: debug.log\n+x\n*** Environment ID: remote\n*** End Patch",
		"*** Begin Patch\n*** Add File: first.log\n+x\n*** End Patch\n*** Begin Patch\n *** Environment ID: remote\n*** Add File: second.log\n+x\n*** End Patch",
		"*** Begin Patch\n*** Add File: debug.log\n+" + strings.Repeat("x", 4<<20) + "\n*** Environment ID: remote\n*** End Patch",
	} {
		if got := PatchPaths(patch); len(got) != 0 {
			t.Fatalf("partially scanned environment patch exposed local paths: %q", got)
		}
	}
}

func TestPatchPathsKeepsEnvironmentTextInHunks(t *testing.T) {
	t.Parallel()
	patch := `*** Begin Patch
*** Update File: debug.log
@@
 *** Begin Patch
 *** Environment ID: remote
-*** Environment ID: old
+*** Environment ID: new
*** Add File: other.log
+*** Environment ID: container
*** End Patch`
	if got := PatchPaths(patch); !slices.Equal(got, []string{"debug.log", "other.log"}) {
		t.Fatalf("hunk text suppressed local patch paths: %q", got)
	}
}

func TestNativePathBridgesWSL(t *testing.T) {
	testenv.WindowsOnly(t, "WSL-to-Windows path translation is only performed by the Windows host")
	t.Parallel()
	cases := map[string]string{
		"/mnt/d/Coding/x": testenv.NativeFixture(`D:\Coding\x`),
		"/mnt/c":          testenv.NativeFixture(`C:\`),
		"relative/a.log":  "relative/a.log",
	}
	for in, want := range cases {
		if got, ok := NativePath(in); !ok || got != want {
			t.Errorf("NativePath(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"/home/me/repo", "/mnt/dd/x", ""} {
		if got, ok := NativePath(in); ok {
			t.Errorf("NativePath(%q) = %q; want unreachable", in, got)
		}
	}
}

func TestCodexFromWSLProposesFromAnApplyPatch(t *testing.T) {
	testenv.WindowsOnly(t, "WSL-to-Windows path translation is only performed by the Windows host")
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	e.write("app.log")
	payload, _ := json.Marshal(map[string]any{
		"session_id": "c1", "cwd": wsl(e.root), "hook_event_name": "PostToolUse", "tool_name": "apply_patch",
		"tool_input": map[string]any{"command": "*** Begin Patch\n*** Add File: app.log\n+x\n*** End Patch"},
	})
	var out bytes.Buffer
	Codex(e.engine, bytes.NewReader(payload), &out, nil)

	st := e.state()
	if len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 1 ||
		!strings.EqualFold(st.Proposals[0].Candidates[0].Path, filepath.Join(e.root, "app.log")) {
		t.Fatalf("want a proposal for app.log, got %+v", st.Proposals)
	}
	var o claudeOutput
	if err := json.Unmarshal(out.Bytes(), &o); err != nil || !strings.Contains(o.HookSpecificOutput.AdditionalContext, "obit approve") {
		t.Fatalf("Codex must hear about the proposal, got %q", out.String())
	}
}

func TestCodexEnvironmentPatchLeavesLocalFilesUnregistered(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, tc := range []struct {
		name, field   string
		fallback, wsl bool
	}{
		{"canonical", "command", false, false},
		{"compat-patch", "patch", true, false},
		{"compat-input", "input", true, false},
		{"wsl", "command", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			local := e.write("debug.log")
			identity, err := e.engine.Svc.FS.Identify(local)
			if err != nil {
				t.Fatal(err)
			}
			if err := e.engine.Svc.Store.Update(func(st *core.State) error {
				st.Projects = map[string]core.ProjectState{core.ProjectKey(e.root): {Consent: core.ConsentApproved}}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			ledger := filepath.Join(e.engine.Svc.Store.Home, "store.json")
			before, err := os.ReadFile(ledger)
			if err != nil {
				t.Fatal(err)
			}
			patch := "*** Begin Patch\n*** Environment ID: remote\n*** Add File: debug.log\n+remote contents\n*** End Patch"
			input := map[string]any{tc.field: patch}
			if tc.fallback {
				input["file_path"] = local
			}
			cwd := e.root
			if tc.wsl {
				cwd = wsl(cwd)
			}
			payload, err := json.Marshal(map[string]any{
				"session_id": "c1", "cwd": cwd, "hook_event_name": "PostToolUse", "tool_name": "apply_patch",
				"tool_input": input,
			})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			Codex(e.engine, bytes.NewReader(payload), &out, nil)
			after, err := os.ReadFile(ledger)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("environment patch changed local ledger: %v\nbefore: %s\nafter: %s", err, before, after)
			}
			contents, err := os.ReadFile(local)
			if err != nil || string(contents) != "debug.log" {
				t.Fatalf("environment patch changed local file: %q %v", contents, err)
			}
			if got, err := e.engine.Svc.FS.Identify(local); err != nil || got != identity {
				t.Fatalf("environment patch replaced local file: %+v %v", got, err)
			}
			if out.Len() != 0 {
				t.Fatalf("environment patch prompted agent: %q", out.String())
			}
			log, err := os.ReadFile(filepath.Join(e.engine.Svc.Store.Home, "hook.log"))
			if err != nil || strings.Count(string(log), "\n") != 1 || !strings.Contains(string(log), "Environment ID") {
				t.Fatalf("environment patch diagnostic missing or repeated: %q %v", log, err)
			}
			// The same local path matches a rule and is registered once the header is absent.
			delete(input, "file_path")
			input[tc.field] = strings.Replace(patch, "*** Environment ID: remote\n", "", 1)
			payload, err = json.Marshal(map[string]any{
				"session_id": "c1", "cwd": cwd, "hook_event_name": "PostToolUse", "tool_name": "apply_patch",
				"tool_input": input,
			})
			if err != nil {
				t.Fatal(err)
			}
			Codex(e.engine, bytes.NewReader(payload), &out, nil)
			st := e.state()
			if len(st.Items) != 1 || st.Items[0].Identity == nil || *st.Items[0].Identity != identity || st.Items[0].Source != "rule" || len(st.Proposals) != 0 {
				t.Fatalf("local patch failed to register matching file: %+v", st)
			}
		})
	}
}

func TestCodexIgnoresPathsItCannotReach(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	payload := `{"session_id":"c1","cwd":"/home/me/repo","hook_event_name":"SessionStart","source":"startup"}`
	var out bytes.Buffer
	Codex(e.engine, strings.NewReader(payload), &out, nil)
	if out.Len() != 0 {
		t.Fatalf("got %q", out.String())
	}
}

func TestCodexUnreachablePatchLogsDiagnostic(t *testing.T) {
	testenv.WindowsOnly(t, "WSL-to-Windows path translation is only performed by the Windows host")
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	local := e.write("debug.log")
	payload, err := json.Marshal(map[string]any{
		"session_id": "c1", "cwd": "/home/remote/repo", "hook_event_name": "PostToolUse", "tool_name": "apply_patch",
		"tool_input": map[string]any{"command": "*** Begin Patch\n*** Add File: debug.log\n+x\n*** End Patch", "file_path": local},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	Codex(e.engine, bytes.NewReader(payload), &out, nil)
	st := e.state()
	if len(st.Items) != 0 || len(st.Proposals) != 0 || out.Len() != 0 {
		t.Fatalf("unreachable patch observed local path: %+v %q", st, out.String())
	}
	log, err := os.ReadFile(filepath.Join(e.engine.Svc.Store.Home, "hook.log"))
	if err != nil || strings.Count(string(log), "\n") != 1 || !strings.Contains(string(log), "cwd cannot be verified as local") {
		t.Fatalf("unreachable patch diagnostic missing: %q %v", log, err)
	}
}

func TestCodexRelativeWorkdirScope(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		cwd, workdir, shell string
		deny                bool
	}{
		{testenv.NativeFixture(`C:\repo\child`), `..\build`, "powershell.exe", true},
		{testenv.NativeFixture(`C:\repo`), "sub/../build", "powershell.exe", true},
		{"/mnt/c/repo/child", "../build", "bash", true},
		{"/mnt/c/repo", "../../home", "bash", false},
		{"/home/me", "../repo", "bash", false},
		{"/home/me", "/mnt/c/repo", "bash", true},
	} {
		t.Run(tc.cwd+"/"+tc.workdir, func(t *testing.T) {
			t.Parallel()
			if tc.shell == "powershell.exe" {
				testenv.WindowsOnly(t, "PowerShell Remove-Item integration is Windows-only")
			}
			if runtime.GOOS != "windows" {
				tc.deny = true // All absolute POSIX workdirs are in the native namespace.
			}
			command := "rm -rf build"
			if tc.shell == "powershell.exe" {
				command = `Remove-Item -Recurse -Force .\build`
			}
			got := guardSuggestion(t, "codex", tc.cwd, "exec_command", map[string]any{"cmd": command, "workdir": tc.workdir, "shell": tc.shell})
			if (got != "") != tc.deny {
				t.Fatalf("workdir changed shell/platform scope: %q, want deny=%v", got, tc.deny)
			}
		})
	}
}

func TestDeleteWSLExecutableWithApostrophe(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	copy := filepath.Join(t.TempDir(), "agent's hook.exe")
	if err := os.WriteFile(copy, data, 0o700); err != nil {
		t.Fatal(err)
	}
	out, err := runFreshExecutable(func() *exec.Cmd {
		return exec.Command(copy, "-test.run=^TestDeleteReview5MainScenarios/codex-wsl", "-test.count=1")
	})
	if err != nil {
		t.Fatalf("WSL expectation must use the original executable path: %s %v", out, err)
	}
}
