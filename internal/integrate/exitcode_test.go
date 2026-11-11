//go:build windows

package integrate

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/hook"
	"golang.org/x/sys/windows"
)

func exitCodePlan(t *testing.T, s Scope, bridge ExitCodeBridge, uninstall bool) ([]Change, error) {
	t.Helper()
	h, err := bridge.Handler()
	if err != nil {
		return nil, err
	}
	ch, err := PlanHookConfig(s, filepath.Join(s.Project, "settings.json"), HookConfig{Agent: bridge.Agent, Shape: GroupedHooks, OwnsHandler: bridge.Owns,
		Events: []HookRegistration{{Event: "PreToolUse", Matcher: "Bash", Handler: h}}}, uninstall)
	if err != nil {
		return nil, err
	}
	return bridge.Plan(s, []Change{ch}, uninstall)
}

func TestExitCodeBridgeManagedLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	s := Scope{Project: t.TempDir()}
	b := ExitCodeBridge{Agent: "qoderwork", Executable: "C:/old/obit.exe", ScriptPath: filepath.Join(s.Project, "plugins", "bridge.ps1")}
	settings := filepath.Join(s.Project, "settings.json")
	original := `{"theme":"keep","hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"user-hook"}]}]}}`
	writeIntegrationFile(t, settings, original)
	for _, exe := range []string{"C:/old/obit.exe", "C:/new folder/obit.exe"} {
		b.Executable = exe
		plan, err := exitCodePlan(t, s, b, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(b.ScriptPath); os.IsNotExist(err) && exe != "C:/old/obit.exe" {
			t.Fatal("script lost")
		}
		applyIntegration(t, plan, nil)
		if !strings.Contains(read(t, settings), "user-hook") {
			t.Fatal("foreign hook lost")
		}
		plan, err = exitCodePlan(t, s, b, false)
		if err != nil || Pending(plan) {
			t.Fatalf("repeat: %+v %v", plan, err)
		}
	}
	plan, err := exitCodePlan(t, s, b, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range plan {
		if ch.Path == settings {
			if ch.phase >= -20 {
				t.Fatal("unregister must precede asset removal")
			}
		}
	}
	applyIntegration(t, plan, nil)
	if _, err := os.Stat(b.ScriptPath); !os.IsNotExist(err) {
		t.Fatal("owned script remains")
	}
	if !sameConfigJSON([]byte(read(t, settings)), []byte(original)) {
		t.Fatal("uninstall changed user config")
	}
	if _, err := os.Stat(filepath.Dir(b.ScriptPath)); err != nil {
		t.Fatal("removed plugin directory")
	}
}

func TestExitCodeBridgePreservesEditsAndChecksRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	s := Scope{Project: t.TempDir()}
	b := ExitCodeBridge{Agent: "qoderwork", Executable: "C:/tools/obit.exe", ScriptPath: filepath.Join(s.Project, "bridge.ps1")}
	plan, err := exitCodePlan(t, s, b, false)
	applyIntegration(t, plan, err)
	settings := filepath.Join(s.Project, "settings.json")
	plan, err = exitCodePlan(t, s, b, true)
	if err != nil {
		t.Fatal(err)
	}
	before := read(t, b.ScriptPath)
	plan = editAfterPreflight(t, plan, settings, `{"user-edit":true}`)
	plan[0].phase = -50
	if err := Apply(plan); err == nil || read(t, b.ScriptPath) != before {
		t.Fatal("failed unregistration removed script")
	}
	plan, err = exitCodePlan(t, s, b, false)
	applyIntegration(t, plan, err)
	writeIntegrationFile(t, b.ScriptPath, before+"# user edit\n")
	plan, err = exitCodePlan(t, s, b, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err == nil {
		t.Fatal("edited script not reported")
	}
	if read(t, b.ScriptPath) != before+"# user edit\n" || strings.Contains(read(t, settings), "bridge.ps1") {
		t.Fatal("edit lost or registration not removed")
	}
	if _, err := b.Plan(s, []Change{{Action: "preserve", Path: settings, Detail: "cannot unregister"}}, true); err == nil {
		t.Fatal("unverified unregistration accepted")
	}
}

func TestExitCodeBridgeOwnership(t *testing.T) {
	t.Parallel()
	b := ExitCodeBridge{Agent: "qoderwork", Executable: "C:/tools/obit.exe", ScriptPath: "C:/plugins/bridge.ps1"}
	h, err := b.Handler()
	if err != nil || !b.Owns(h) {
		t.Fatalf("generated handler: %s %v", h, err)
	}
	var entry map[string]any
	_ = json.Unmarshal(h, &entry)
	for _, key := range []string{"args", "async", "commandWindows", "shell", "Command"} {
		copy := map[string]any{}
		for k, v := range entry {
			copy[k] = v
		}
		copy[key] = true
		if b.Owns(mustJSON(copy)) {
			t.Fatal("claimed extra execution field", key)
		}
	}
	for _, suffix := range []string{" --help", " && echo user", "; echo user"} {
		copy := map[string]any{"type": "command", "command": entry["command"].(string) + suffix}
		if b.Owns(mustJSON(copy)) {
			t.Fatal("claimed compound launcher")
		}
	}
	for _, bad := range []ExitCodeBridge{
		{Agent: "qoderwork", Executable: "obit.exe", ScriptPath: b.ScriptPath},
		{Agent: "qoderwork & echo bad", Executable: b.Executable, ScriptPath: b.ScriptPath},
		{Agent: "qoderwork", Executable: b.Executable, ScriptPath: "relative.ps1"},
		{Agent: "qoderwork", Executable: b.Executable, ScriptPath: "C:/with $expansion/bridge.ps1"},
	} {
		if _, err := bad.Handler(); err == nil {
			t.Fatal("accepted unsafe bridge", bad)
		}
	}
}

func TestExitCodeBridgeKeepsScriptReferencedByEditedHandler(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	b := ExitCodeBridge{Agent: "qoderwork", Executable: "C:/obit.exe", ScriptPath: filepath.Join(s.Project, "bridge.ps1")}
	plan, err := exitCodePlan(t, s, b, false)
	applyIntegration(t, plan, err)
	command, err := b.command()
	if err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(s.Project, "settings.json")
	modified := mustJSON(map[string]any{"hooks": map[string]any{"PreToolUse": []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command + " && echo user"}}}}}})
	writeIntegrationFile(t, settings, string(modified))
	if _, err := exitCodePlan(t, s, b, true); err == nil {
		t.Fatal("deleted script referenced by an edited handler")
	}
	if read(t, settings) != string(modified) {
		t.Fatal("changed user handler")
	}
	if _, err := os.Stat(b.ScriptPath); err != nil {
		t.Fatal("removed referenced script")
	}
}

func TestExitCodeBridgeNativeProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root := t.TempDir()
	fixture := filepath.Join(root, "fixture.go")
	writeIntegrationFile(t, fixture, `package main
import("encoding/json";"io";"os";"time")
func main(){
 if len(os.Args)!=5 || os.Args[1]!="hook" || os.Args[2]!="qoderwork" || os.Args[3]!="--event" || os.Args[4]!="PreToolUse" { os.Exit(9) }
 data,_:=io.ReadAll(os.Stdin)
 var in struct{ Output,Error string; Exit int; Hang bool }
 if json.Unmarshal(data,&in)!=nil { os.Exit(8) }
 if in.Hang { time.Sleep(30*time.Second) }
 io.WriteString(os.Stdout,in.Output);io.WriteString(os.Stderr,in.Error);os.Exit(in.Exit)
}`)
	b := ExitCodeBridge{Agent: "qoderwork", Executable: filepath.Join(root, "bin 中文 '$literal'", "obit.exe"), ScriptPath: filepath.Join(root, "plugins 中文 'quoted'", "bridge.ps1")}
	if err := os.MkdirAll(filepath.Dir(b.Executable), 0o700); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", b.Executable, fixture)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %s %v", out, err)
	}
	s := Scope{Project: root}
	plan, err := exitCodePlan(t, s, b, false)
	applyIntegration(t, plan, err)
	system, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatal(err)
	}
	powershell := filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe")
	deny, _ := hook.EncodeExitCodeResponse(hook.Event{Name: "PreToolUse"}, hook.Result{Deny: true, Reason: "建议 `obit rm '中文'`\n保留原命令"})
	for _, tc := range []struct {
		name, output, stderr string
		exit                 int
		hang                 bool
		want                 int
	}{
		{name: "deny", output: string(deny), want: 2},
		{name: "neutral", output: "{}\n"},
		{name: "empty"},
		{name: "malformed", output: "{"},
		{name: "invalid-single-quote-escape", output: `{"version":1,"event":"PreToolUse","deny":true,"reason":"bad\'escape"}`},
		{name: "invalid-hex-escape", output: `{"version":1,"event":"PreToolUse","deny":true,"reason":"bad\x41"}`},
		{name: "invalid-unicode-escape", output: `{"version":1,"event":"PreToolUse","deny":true,"reason":"bad\u01xz"}`},
		{name: "nonzero", output: string(deny), exit: 1},
		{name: "stderr", output: string(deny), stderr: "error"},
		{name: "timeout", hang: true},
		{name: "wrong-version", output: strings.Replace(string(deny), `"version":1`, `"version":2`, 1)},
		{name: "non-pre", output: strings.Replace(string(deny), "PreToolUse", "Stop", 1)},
		{name: "string-deny", output: strings.Replace(string(deny), `"deny":true`, `"deny":"true"`, 1)},
		{name: "extra-field", output: strings.Replace(string(deny), `"deny":true`, `"deny":true,"allow":true`, 1)},
		{name: "duplicate-deny", output: strings.Replace(string(deny), `"deny":true`, `"deny":false,"deny":true`, 1)},
		{name: "trailing", output: string(deny) + "garbage"},
		{name: "long-reason", output: `{"version":1,"event":"PreToolUse","deny":true,"reason":"` + strings.Repeat("x", 2001) + `"}`},
		{name: "oversized", output: strings.Repeat("x", 16385)},
		{name: "missing-exe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "missing-exe" {
				b.Executable = filepath.Join(root, "missing", "obit.exe")
				plan, err := exitCodePlan(t, s, b, false)
				applyIntegration(t, plan, err)
			}
			payload := mustJSON(map[string]any{"Output": tc.output, "Error": tc.stderr, "Exit": tc.exit, "Hang": tc.hang})
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, powershell, "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", b.ScriptPath)
			cmd.Stdin = bytes.NewReader(payload)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			code := 0
			if err != nil {
				if cmd.ProcessState == nil {
					t.Fatal(err)
				}
				code = cmd.ProcessState.ExitCode()
			}
			if code != tc.want || stdout.Len() != 0 {
				t.Fatalf("exit=%d want=%d stdout=%q stderr=%q err=%v", code, tc.want, stdout.String(), stderr.String(), err)
			}
			if tc.want == 2 && !strings.Contains(stderr.String(), "建议 `obit rm '中文'`") {
				t.Fatal("reason not delivered intact", stderr.String())
			}
			if tc.want == 0 && stderr.Len() != 0 {
				t.Fatal("transport failure leaked denial", stderr.String())
			}
		})
	}
}
