package hook

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func setProtectedDiscoveryRoots(t *testing.T, roots ...string) {
	t.Helper()
	original := protectedDiscoveryRoots
	protectedDiscoveryRoots = func() []string { return roots }
	t.Cleanup(func() { protectedDiscoveryRoots = original })
}

type env struct {
	t      testing.TB
	root   string
	engine *auto.Engine
}

func newEnv(t testing.TB) *env {
	t.Helper()
	dir := t.TempDir()
	root, home := filepath.Join(dir, "repo"), filepath.Join(dir, "home")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	testenv.InitRepo(t, root)
	svc := &core.Service{Store: core.Store{Home: home}, Guard: core.Guard{System: []string{home}}, FS: winfs.FS{}}
	return &env{t: t, root: root, engine: &auto.Engine{Svc: svc, Trust: rules.TrustStore{Path: filepath.Join(home, "trusted_rules.json")}}}
}

// Each sample starts with the same ledger, so PostToolUse measures a first
// observation rather than the cheaper already-registered/notified path.
// The runner measures the adapter's handoff; real process costs are measured
// separately with obit.exe. Exclude setup to isolate hook execution time.
func benchmarkHook(b *testing.B, agent string, run func(*auto.Engine, io.Reader, io.Writer, *Runner)) {
	for _, name := range []string{"PreToolUse/delete", "PreToolUse/ordinary", "PostToolUse/approved", "PostToolUse/pending", "SessionStart/startup", "SessionStart/resume", "Stop/dispatch"} {
		b.Run(name, func(b *testing.B) {
			e := newEnv(b)
			e.engine.Svc.Guard = core.DefaultGuard(e.engine.Svc.Store.Home).WithCanonical(winfs.Canonical)
			e.engine.Svc.Owner = auto.Owner
			path := e.write("debug.log")
			event, variant, _ := strings.Cut(name, "/")
			payload := map[string]any{"hook_event_name": event, "session_id": "bench", "cwd": e.root, "source": variant}
			if event == "PreToolUse" {
				command := "git status --short"
				if variant == "delete" {
					command = "rm -rf build"
				}
				payload["tool_name"] = "Bash"
				payload["tool_input"] = map[string]any{"command": command, "shell": "bash"}
			}
			if event == "PostToolUse" {
				payload["tool_name"] = "Write"
				payload["tool_input"] = map[string]any{"file_path": path, "content": "x"}
				if agent == "codex" {
					payload["cwd"] = wsl(e.root)
					payload["tool_name"] = "apply_patch"
					payload["tool_input"] = map[string]any{"command": "*** Begin Patch\n*** Add File: " + wsl(path) + "\n+x\n*** End Patch"}
				}
			}
			data, err := json.Marshal(payload)
			if err != nil {
				b.Fatal(err)
			}
			if variant == "resume" || event == "Stop" {
				if err := e.engine.Snapshot(e.root, "bench"); err != nil {
					b.Fatal(err)
				}
			}
			finishes := 0
			runner := &Runner{Tick: func() {}, Finish: func(_, _ string, _ bool) { finishes++ }}
			var output bytes.Buffer
			var samples []float64
			b.ReportAllocs()
			for b.Loop() {
				b.StopTimer()
				if event == "PostToolUse" {
					if err := e.engine.Svc.Store.Update(func(st *core.State) error {
						*st = core.State{NextID: 1}
						if variant == "approved" {
							st.Projects = map[string]core.ProjectState{core.ProjectKey(e.root): {Consent: core.ConsentApproved}}
						}
						return nil
					}); err != nil {
						b.Fatal(err)
					}
				}
				if variant == "startup" {
					// These are only this benchmark's own snapshot files.
					if err := os.RemoveAll(filepath.Join(e.engine.Svc.Store.Home, "sessions")); err != nil {
						b.Fatal(err)
					}
				}
				output.Reset()
				b.StartTimer()
				started := time.Now()
				run(e.engine, bytes.NewReader(data), &output, runner)
				elapsed := time.Since(started)
				b.StopTimer()
				samples = append(samples, float64(elapsed)/float64(time.Millisecond))
				st := e.state()
				switch name {
				case "PreToolUse/delete":
					if !strings.Contains(output.String(), `"permissionDecision":"deny"`) {
						b.Fatalf("expected deletion denial: %s", output.String())
					}
				case "PreToolUse/ordinary":
					if output.Len() != 0 {
						b.Fatalf("expected ordinary command passthrough: %s", output.String())
					}
				case "PostToolUse/approved":
					if len(st.Items) != 1 || len(st.Proposals) != 0 || output.Len() != 0 {
						b.Fatalf("expected silent registration: %+v %s", st, output.String())
					}
				case "PostToolUse/pending":
					if len(st.Items) != 0 || len(st.Proposals) != 1 || !strings.Contains(output.String(), "obit approve") {
						b.Fatalf("expected onboard proposal: %+v %s", st, output.String())
					}
				case "Stop/dispatch":
					if finishes == 0 || output.Len() != 0 {
						b.Fatalf("expected silent background handoff: %d %s", finishes, output.String())
					}
				}
				if log, err := os.ReadFile(filepath.Join(e.engine.Svc.Store.Home, "hook.log")); !os.IsNotExist(err) {
					b.Fatalf("hook failed open: %s %v", log, err)
				}
				b.StartTimer()
			}
			slices.Sort(samples)
			b.ReportMetric(samples[len(samples)/2], "p50-ms")
			b.ReportMetric(samples[(95*len(samples)+99)/100-1], "p95-ms")
		})
	}
}

// ask restores the pre-tiering behaviour: every automatic registration is proposed.
func (e *env) ask() { e.engine.Svc.ProposalMode = core.ProposalsAsk }

func (e *env) write(rel string) string {
	e.t.Helper()
	path := filepath.Join(e.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(rel), 0o600); err != nil {
		e.t.Fatal(err)
	}
	return path
}

// event runs one hook event and returns the additionalContext it produced.
func (e *env) event(payload map[string]any) string {
	e.t.Helper()
	return e.eventWithRunner(payload, nil)
}

func (e *env) eventWithRunner(payload map[string]any, runner *Runner) string {
	e.t.Helper()
	payload["cwd"] = e.root
	if _, ok := payload["session_id"]; !ok {
		payload["session_id"] = "s1"
	}
	in, _ := json.Marshal(payload)
	var out bytes.Buffer
	Claude(e.engine, bytes.NewReader(in), &out, runner)
	// Under heavy load the hook may miss its one-second lock budget and queue
	// the registration; the runner replays it later. Replay here so tests see
	// the same eventual ledger state on a busy machine as on an idle one.
	if e.engine.Svc.Store.HasDeferredObservations() {
		if err := e.engine.ReplayObservations(); err != nil {
			e.t.Fatalf("replay queued observations: %v", err)
		}
	}
	if out.Len() == 0 {
		return ""
	}
	var o claudeOutput
	if err := json.Unmarshal(out.Bytes(), &o); err != nil {
		e.t.Fatalf("hook printed something that is not hook JSON: %q", out.String())
	}
	if o.HookSpecificOutput.HookEventName != payload["hook_event_name"] {
		e.t.Fatalf("hookEventName %q for %v", o.HookSpecificOutput.HookEventName, payload["hook_event_name"])
	}
	return o.HookSpecificOutput.AdditionalContext
}

// finishEvent follows cmdutil.Runner's handoff to `obit hook finish`: the
// session difference uses the original service after the bounded hook returns.
func (e *env) finishEvent(name, session string) {
	e.t.Helper()
	called := false
	runner := &Runner{Finish: func(root, gotSession string, final bool) {
		if root != e.root || gotSession != session || final != (name == "SessionEnd") {
			e.t.Fatalf("wrong finish job: %q %q final=%v", root, gotSession, final)
		}
		called = true
	}}
	if ctx := e.eventWithRunner(map[string]any{"hook_event_name": name, "session_id": session}, runner); ctx != "" {
		e.t.Fatalf("%s must stay silent, got %q", name, ctx)
	}
	if !called {
		e.t.Fatal("ending event did not dispatch the session difference")
	}
	if _, err := e.engine.Finish(e.root, session); err != nil {
		e.t.Fatalf("background finish: %v", err)
	}
}

func (e *env) wrote(path string) string {
	return e.event(map[string]any{"hook_event_name": "PostToolUse", "tool_name": "Write",
		"tool_input": map[string]any{"file_path": path, "content": "x"}})
}

func (e *env) state() core.State {
	e.t.Helper()
	st, err := e.engine.Svc.Store.Read()
	if err != nil {
		e.t.Fatal(err)
	}
	return st
}

func wsl(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	vol := filepath.VolumeName(path)
	return "/mnt/" + strings.ToLower(vol[:1]) + filepath.ToSlash(path[len(vol):])
}

func expectedGuardExecutable(t *testing.T, shell guardrm.Shell) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		t.Fatal(err)
	}
	if shell == guardrm.PowerShell {
		return "& '" + strings.ReplaceAll(self, "'", "''") + "'"
	}
	return "'" + strings.ReplaceAll(filepath.ToSlash(self), "'", `'\''`) + "'"
}

// noEquivalent is guardSuggestion's result for a denial that names a
// literal deletion but no exact equivalent command.
const noEquivalent = "<no exact equivalent>"

// guardSuggestion returns "" when the command passes, otherwise the denial's
// equivalent command or noEquivalent.
func guardSuggestion(t *testing.T, agent, cwd, tool string, input map[string]any) string {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": cwd, "tool_name": tool, "tool_input": input})
	e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
	var out bytes.Buffer
	Run(agent, e, bytes.NewReader(data), &out, nil)
	if _, err := os.Stat(filepath.Join(e.Svc.Store.Home, "store.json")); !os.IsNotExist(err) {
		t.Fatalf("pre opened ledger: %v", err)
	}
	if out.Len() == 0 {
		return ""
	}
	var response claudeOutput
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatal(out.String())
	}
	if strings.Contains(response.HookSpecificOutput.PermissionDecisionReason, "no exact obit equivalent applies") {
		return noEquivalent
	}
	parts := strings.Split(response.HookSpecificOutput.PermissionDecisionReason, "`")
	if len(parts) < 3 {
		t.Fatal(out.String())
	}
	return parts[1]
}

func testAgentShellDeclaration(t *testing.T, agent, declarationAgent string, tools []string) {
	t.Helper()
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			for _, tc := range []struct{ name, shell, command, want string }{
				{"unset", "", "rm -rf 'build dir'", ""},
				{"bash", "bash", "rm -rf 'build dir'", "rm -rf 'build dir'"},
				{"powershell", "powershell", `Remove-Item -LiteralPath '.\build dir' -Recurse`, `remove-item -LiteralPath '.\build dir' -Recurse`},
				{"powershell-bash-command", "powershell", "rm -rf build", noEquivalent},
				{"bash-powershell-command", "bash", "Remove-Item -LiteralPath build", noEquivalent},
				{"ambiguous-path", "bash", `rm .\build`, noEquivalent},
				{"string", "bash", "echo 'rm -rf build'", ""},
			} {
				t.Run(tc.name, func(t *testing.T) {
					dir := t.TempDir()
					t.Setenv("OBIT_CONFIG_DIR", dir)
					declaration := ""
					if tc.shell != "" {
						declaration = "[agent_shells]\n" + declarationAgent + " = '" + tc.shell + "'\n"
					}
					if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(declaration), 0600); err != nil {
						t.Fatal(err)
					}
					got := guardSuggestion(t, agent, testenv.NativeFixture(`C:\repo`), tool, map[string]any{"command": tc.command, "shell": "bash", "local": true})
					want := tc.want
					if want != "" && want != noEquivalent {
						want = expectedGuardExecutable(t, guardrm.Shell(tc.shell)) + " " + want
					}
					if got != want {
						t.Fatalf("declared agent shell: got %q, want %q", got, want)
					}
				})
			}
		})
	}
}

type replaceOnCaptureFS struct {
	core.FS
	path    string
	replace func()
}

func (f *replaceOnCaptureFS) Identify(path string) (core.Identity, error) {
	id, err := f.FS.Identify(path)
	if err == nil && path == f.path && f.replace != nil {
		replace := f.replace
		f.replace = nil
		replace()
	}
	return id, err
}
