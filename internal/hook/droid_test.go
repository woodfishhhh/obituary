package hook

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/core"
)

func droidFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var in map[string]any
	if err := json.Unmarshal([]byte(testenv.NativeFixture(string(raw))), &in); err != nil {
		t.Fatal(err)
	}
	return in
}

func TestDroidDocumentedPayloads(t *testing.T) {
	t.Parallel()
	a, ok := Lookup("droid")
	if !ok {
		t.Fatal("adapter missing")
	}
	for _, tc := range []struct{ file, event string }{{"droid-pre", "PreToolUse"}, {"droid-start", "SessionStart"}} {
		t.Run(tc.file, func(t *testing.T) {
			raw, _ := json.Marshal(droidFixture(t, tc.file))
			for _, prefix := range []string{"", "\xef\xbb\xbf"} {
				ev, err := Decode(a.Adapter, append([]byte(prefix), raw...), Invocation{Agent: "droid"})
				if err != nil || ev.Name != tc.event || ev.Session != "droid:droid-1" {
					t.Fatalf("%+v %v", ev, err)
				}
			}
		})
	}
}

func TestDroidUnverifiedToolsRemainNeutral(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"{", "[]", `{"tool_input":7}`, `{"session_id":false}`} {
		if _, err := Decode(droidAdapter{}, []byte(raw), Invocation{Agent: "droid"}); err == nil {
			t.Fatal("malformed accepted", raw)
		}
	}
	for _, tool := range []string{"Execute", "Bash", "remote_exec", "code"} {
		in := droidFixture(t, "droid-pre")
		in["tool_name"] = tool
		delete(in, "session_id")
		in["tool_input"] = map[string]any{"command": "rm a.log", "shell": "bash"}
		raw, _ := json.Marshal(in)
		ev, err := Decode(droidAdapter{}, raw, Invocation{Agent: "droid"})
		if err != nil || ev.SkipReason == "" || ev.Shell != "" {
			t.Fatalf("unverified shell accepted: %+v %v", ev, err)
		}
	}
	for _, host := range []string{"unknown", "PostToolUse", "PostToolUseFailure", "SubagentStop"} {
		in := droidFixture(t, "droid-write-unverified")
		in["hook_event_name"] = host
		raw, _ := json.Marshal(in)
		if ev, err := Decode(droidAdapter{}, raw, Invocation{Agent: "droid"}); !errors.Is(err, ErrSkip) || len(ev.Paths) != 0 {
			t.Fatalf("unsafe write: %+v %v", ev, err)
		}
	}
	raw, _ := json.Marshal(droidFixture(t, "droid-start"))
	if _, err := Decode(droidAdapter{}, raw, Invocation{Agent: "droid", Event: "SessionEnd"}); err == nil {
		t.Fatal("event conflict accepted")
	}
}

func TestDroidUnverifiedWriteDoesNotPropose(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	path := e.write("debug.log")
	in := droidFixture(t, "droid-write-unverified")
	in["cwd"] = e.root
	in["tool_input"] = map[string]any{"file_path": path}
	raw, _ := json.Marshal(in)
	var out bytes.Buffer
	Run("droid", e.engine, bytes.NewReader(raw), &out, nil)
	if len(e.state().Items) != 0 || len(e.state().Proposals) != 0 || out.Len() != 0 {
		t.Fatal("unverified write changed ledger")
	}
}

func TestDroidLifecycleAndReplies(t *testing.T) {
	t.Parallel()
	a := droidAdapter{}
	id := strings.Repeat("会", 5000)
	for _, host := range []string{"SessionStart", "Stop", "SessionEnd"} {
		in := droidFixture(t, "droid-start")
		in["hook_event_name"] = host
		in["session_id"] = id
		raw, _ := json.Marshal(in)
		ev, err := Decode(a, raw, Invocation{Agent: "droid"})
		if err != nil || ev.Session != core.NormalizeSession("droid:"+id) {
			t.Fatalf("%+v %v", ev, err)
		}
		data, _ := a.Encode(ev, Result{Deny: true, Reason: "no", Context: strings.Repeat("中", 2000)})
		if host != "SessionStart" {
			if len(data) != 0 {
				t.Fatalf("ending continued: %s", data)
			}
		} else {
			var out claudeOutput
			json.Unmarshal(data, &out)
			text := out.HookSpecificOutput.AdditionalContext
			if len(text) > maxContext || text == "" || !utf8.ValidString(text) {
				t.Fatal("bad context budget")
			}
		}
		delete(in, "session_id")
		raw, _ = json.Marshal(in)
		if _, err := Decode(a, raw, Invocation{Agent: "droid"}); !errors.Is(err, ErrSkip) {
			t.Fatal("missing session accepted")
		}
		in["session_id"] = "s"
		in["cwd"] = testenv.ForeignFixture("/remote/repo")
		raw, _ = json.Marshal(in)
		if _, err := Decode(a, raw, Invocation{Agent: "droid"}); !errors.Is(err, ErrSkip) {
			t.Fatal("remote filesystem accepted")
		}
	}
	if len(a.Fallback(Event{})) != 0 {
		t.Fatal("invalid fallback")
	}
	data, _ := a.Encode(Event{Name: "PreToolUse", HostEvent: "PreToolUse"}, Result{Deny: true, Reason: "obit rm a.log", Context: "ignored"})
	var out claudeOutput
	json.Unmarshal(data, &out)
	if out.HookSpecificOutput.PermissionDecision != "deny" || out.HookSpecificOutput.PermissionDecisionReason != "obit rm a.log" || out.HookSpecificOutput.AdditionalContext != "" {
		t.Fatalf("wrong denial: %s", data)
	}
}
