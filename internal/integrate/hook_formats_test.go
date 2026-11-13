package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPOSIXOwnedHookExec(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native POSIX executable names")
	}
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/opt/obituary/obit", true}, {"/tmp/bin with spaces/obit", true},
		{"obit", false}, {"./obit", false}, {"/tmp/other", false}, {"/tmp/OBIT", false},
	} {
		if got := OwnedHookExec(tc.path, []string{"hook", "claude"}, "claude", nil); got != tc.want {
			t.Fatalf("ownership %q = %t", tc.path, got)
		}
	}
}

func TestOwnedHookCommand(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		testenv.NativeFixture(`C:/tools/obit.exe hook cursor`), testenv.NativeFixture(`"C:/Program Files/obit.exe" hook cursor --event preToolUse`),
		testenv.NativeFixture(`& 'C:\tools\obit.exe' hook cursor --event=preToolUse`), `/mnt/c/tools/obit.exe hook cursor`,
	} {
		if !OwnedHookCommand(command, "cursor", []string{"preToolUse"}) {
			t.Errorf("owned command rejected: %s", command)
		}
	}
	for _, command := range []string{
		`obit hook cursor`, `obit.exe hook cursor`, `/mnt/not-a-drive/obit.exe hook cursor`,
		`obit hook other`, `other hook cursor`, `./obit hook cursor`, `tools/obit.exe hook cursor`,
		testenv.NativeFixture(`echo x && C:/tools/obit.exe hook cursor`), testenv.NativeFixture(`C:/tools/obit.exe hook cursor; echo x`), testenv.NativeFixture(`C:/tools/obit.exe hook cursor > log`),
		testenv.NativeFixture(`C:/tools/obit.exe hook cursor --event unknown`), testenv.NativeFixture(`C:/tools/obit.exe hook cursor --event preToolUse --yes`),
		testenv.NativeFixture(`C:/tools/obit.exe hook cursor --event=`), testenv.NativeFixture(`C:/tools/obit.exe hook cursor --help`), `"$HOME/obit.exe" hook cursor`,
		testenv.NativeFixture("C:/tools/obit.exe hook cursor\n"), testenv.NativeFixture(`C:/tools/obit.exe hook cursor --event "$(echo preToolUse)"`),
	} {
		if runtime.GOOS != "windows" && command == `/mnt/not-a-drive/obit.exe hook cursor` {
			if !OwnedHookCommand(command, "cursor", []string{"preToolUse"}) {
				t.Fatal("native absolute executable was rejected")
			}
			continue
		}
		if OwnedHookCommand(command, "cursor", []string{"preToolUse"}) {
			t.Errorf("foreign command claimed: %q", command)
		}
	}
}

func TestHookConfigExecOwnership(t *testing.T) {
	t.Parallel()
	spec := HookConfig{Agent: "zcode", EventArgs: []string{"PreToolUse"}}
	for _, raw := range []string{
		testenv.NativeFixture(`{"type":"process","command":"C:/Program Files/obit.exe","args":["hook","zcode"]}`),
		testenv.NativeFixture(`{"type":"command","command":"C:/tools/obit.exe","args":["hook","zcode","--event","PreToolUse"]}`),
	} {
		if !spec.owns(json.RawMessage(raw)) {
			t.Errorf("exec handler not recognized: %s", raw)
		}
	}
	for _, raw := range []string{
		`{"command":"obit hook zcode","args":["--help"]}`,
		`{"type":"process","command":"obit.exe","args":["hook","zcode"]}`,
		testenv.NativeFixture(`{"command":"C:/tools/obit.exe","args":["hook","zcode","--help"]}`),
		testenv.NativeFixture(`{"command":"C:/tools/obit.exe","args":["hook","zcode"],"commandWindows":"user-command"}`),
		testenv.NativeFixture(`{"command":"C:/tools/obit.exe","args":["hook","zcode"],"shell":true}`),
		testenv.NativeFixture(`{"command":"C:/tools/obit.exe","args":["hook","zcode"],"ARGS":["--help"]}`),
	} {
		if spec.owns(json.RawMessage(raw)) {
			t.Errorf("foreign exec handler claimed: %s", raw)
		}
	}
}

func TestOwnedHookExec(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		command string
		args    []string
		want    bool
	}{
		{testenv.NativeFixture(`C:\tools\obit.exe`), []string{"hook", "zcode"}, true},
		{testenv.NativeFixture(`C:/中文 ' $ folder/obit.exe`), []string{"hook", "zcode", "--event=PreToolUse"}, true},
		{`\\server\share\obit.exe`, []string{"hook", "zcode"}, true},
		{`obit.exe`, []string{"hook", "zcode"}, false},
		{`./obit.exe`, []string{"hook", "zcode"}, false},
		{`C:obit.exe`, []string{"hook", "zcode"}, false},
		{testenv.NativeFixture(`C:/tools/obit.exe hook zcode`), nil, false},
		{testenv.NativeFixture(`"C:/tools/obit.exe"`), []string{"hook", "zcode"}, false},
		{testenv.NativeFixture(`C:/tools/obit`), []string{"hook", "zcode"}, false},
		{testenv.NativeFixture(`C:/tools/obit.exe`), []string{"hook", "claude"}, false},
		{testenv.NativeFixture(`C:/tools/obit.exe`), []string{"hook", "zcode", "--event", "Other"}, false},
		{testenv.NativeFixture(`C:/tools/obit.exe`), []string{"hook", "zcode", "--event", "PreToolUse", "--event", "PreToolUse"}, false},
		{testenv.NativeFixture(`C:/tools/obit.exe`), []string{"hook", "zcode", "&&", "echo"}, false},
		{testenv.NativeFixture(`C:/tools/obit.exe`), []string{"hook", "zcode", ""}, false},
	} {
		if runtime.GOOS != "windows" {
			if strings.HasPrefix(tc.command, `\\`) {
				tc.want = false
			}
			if tc.command == "/tools/obit" {
				tc.want = true
			}
		}
		if got := OwnedHookExec(tc.command, tc.args, "zcode", []string{"PreToolUse"}); got != tc.want {
			t.Errorf("%q %q: owned=%v", tc.command, tc.args, got)
		}
	}
}

func TestNestedHookConfigLifecycle(t *testing.T) {
	t.Parallel()
	for _, format := range []ConfigFormat{ConfigJSON, ConfigJSONC} {
		for _, global := range []bool{false, true} {
			t.Run(string(format)+fmt.Sprint(global), func(t *testing.T) {
				t.Parallel()
				s := Scope{Global: global, Project: t.TempDir()}
				path := filepath.Join(s.Project, "config.json")
				comment := ""
				if format == ConfigJSONC {
					comment = "// keep comment\n"
				}
				original := "\xef\xbb\xbf{" + comment + `"theme":"深色","hooks":{"enabled":false,"audit":true,"events":{"Stop":[{"hooks":[{"command":"user-command"}]}]}},"last":9007199254740993}`
				writeIntegrationFile(t, path, original)
				spec := HookConfig{Agent: "zcode", Shape: GroupedHooks, Format: format, ContainerPath: []string{"hooks", "events"}, EnablePath: []string{"hooks", "enabled"}}
				for _, binary := range []string{testenv.NativeFixture("C:/old/obit.exe"), testenv.NativeFixture("C:/new folder/obit.exe")} {
					handler := mustJSON(map[string]any{"type": "process", "command": binary, "args": []string{"hook", "zcode"}, "timeoutMs": 5000})
					spec.Events = []HookRegistration{{Event: "SessionStart", Handler: handler}, {Event: "Stop", Handler: handler}}
					ch, err := PlanHookConfig(s, path, spec, false)
					if err != nil {
						t.Fatal(err)
					}
					before := read(t, path)
					if !strings.Contains(before, "user-command") || binary == testenv.NativeFixture("C:/old/obit.exe") && before != original {
						t.Fatal("dry run wrote config")
					}
					applyIntegration(t, []Change{ch}, nil)
					got := read(t, path)
					if !strings.HasPrefix(got, "\xef\xbb\xbf") || !strings.Contains(got, comment) || !strings.Contains(got, "user-command") || !strings.Contains(got, "9007199254740993") || strings.Count(got, binary) != 2 {
						t.Fatal(got)
					}
					if strings.Index(got, `"theme"`) > strings.Index(got, `"hooks"`) || strings.Index(got, `"hooks"`) > strings.Index(got, `"last"`) {
						t.Fatal("key order changed")
					}
					raw, _ := configJSON([]byte(got), format)
					enabled, _ := configMember(raw, []string{"hooks", "enabled"})
					if string(enabled) != "true" {
						t.Fatalf("hooks not enabled: %s", got)
					}
					again, err := PlanHookConfig(s, path, spec, false)
					if err != nil || again.Action != "unchanged" {
						t.Fatalf("repeat: %+v %v", again, err)
					}
				}
				ch, err := PlanHookConfig(s, path, spec, true)
				applyIntegration(t, []Change{ch}, err)
				got := read(t, path)
				if strings.Contains(got, "obit.exe") || !strings.Contains(got, "user-command") || !strings.Contains(got, `"audit"`) {
					t.Fatal(got)
				}
				raw, _ := configJSON([]byte(got), format)
				enabled, _ := configMember(raw, []string{"hooks", "enabled"})
				if string(enabled) != "true" {
					t.Fatal("uninstall reset user's switch")
				}
				backup := path + ".bak"
				if !s.Global {
					backup = testBackupPath(t, path)
				}
				if read(t, backup) != original {
					t.Fatal("backup changed")
				}
			})
		}
	}
}

func TestNestedHookEnableRequiresExplicitHook(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "config.json")
	original := `{"hooks":{"enabled":false,"userFlag":"keep"}}`
	writeIntegrationFile(t, path, original)
	spec := HookConfig{Agent: "zcode", Shape: GroupedHooks, ContainerPath: []string{"hooks", "events"}, EnablePath: []string{"hooks", "enabled"}, Events: []HookRegistration{{Event: "Stop", Handler: json.RawMessage(testenv.NativeFixture(`{"type":"process","command":"C:/obit.exe","args":["hook","zcode"]}`)), Optional: true}}}
	for _, uninstall := range []bool{false, true} {
		ch, err := PlanHookConfig(s, path, spec, uninstall)
		if err != nil || ch.Action != "unchanged" || read(t, path) != original {
			t.Fatalf("implicit enabled hooks: %+v %v", ch, err)
		}
	}
	spec.Events[0].Optional = false
	for _, broken := range []string{"", "\xef\xbb\xbf \r\n", `{"hooks":{"events":null}}`, `{"hooks":{"events":[]}}`, `{"hooks":{"events":{"Stop":{}}}}`} {
		writeIntegrationFile(t, path, broken)
		if _, err := PlanHookConfig(s, path, spec, false); err == nil {
			t.Fatal("accepted malformed nested hooks")
		}
		if read(t, path) != broken {
			t.Fatal("changed invalid config")
		}
	}
	writeIntegrationFile(t, path, original)
	ch, err := PlanHookConfig(s, path, spec, false)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, path, `{"concurrent":true}`)
	if err := Apply([]Change{ch}); err == nil {
		t.Fatal("overwrote concurrent edit")
	}
}

func TestHookConfigShapesLifecycle(t *testing.T) {
	t.Parallel()
	for _, shape := range []HookShape{GroupedHooks, FlatHooks, RootHooks} {
		for _, bom := range []string{"", "\xef\xbb\xbf"} {
			t.Run(string(shape)+"/bom="+string(rune(len(bom)+'0')), func(t *testing.T) {
				t.Parallel()
				s := Scope{Project: t.TempDir()}
				path := filepath.Join(s.Project, "hooks.json")
				foreign := json.RawMessage(`{"type":"command","command":"echo user && obit hook gemini"}`)
				entry := foreign
				if shape != FlatHooks {
					entry, _ = marshal(map[string]any{"matcher": "Bash", "hooks": []json.RawMessage{foreign}})
				}
				original := `{"theme":"深色<&>","hooks":{"BeforeTool":[` + string(entry) + `]},"large":9007199254740993}`
				if shape == RootHooks {
					original = `{"theme":"深色<&>","BeforeTool":[` + string(entry) + `],"large":9007199254740993}`
				}
				writeIntegrationFile(t, path, bom+original)
				spec := HookConfig{Agent: "gemini", Shape: shape, EventArgs: []string{"BeforeTool"}, Events: []HookRegistration{{Event: "BeforeTool", Matcher: "Bash"}}}
				for _, binary := range []string{testenv.NativeFixture("C:/old/obit.exe"), testenv.NativeFixture("C:/new/obit.exe")} {
					spec.Events[0].Handler = mustJSON(map[string]any{"type": "command", "command": binary + " hook gemini --event BeforeTool", "timeout": 5000})
					ch, err := PlanHookConfig(s, path, spec, false)
					applyIntegration(t, []Change{ch}, err)
					got := read(t, path)
					if !strings.HasPrefix(got, bom) || !strings.Contains(got, binary) || !strings.Contains(got, "echo user && obit hook gemini") || !strings.Contains(got, "9007199254740993") || strings.Contains(got, `\u003c`) {
						t.Fatalf("user content or BOM changed: %s", got)
					}
					if strings.Index(got, `"theme"`) > strings.Index(got, `"BeforeTool"`) || strings.Index(got, `"large"`) < strings.Index(got, `"BeforeTool"`) {
						t.Fatal("key order changed")
					}
					ch, err = PlanHookConfig(s, path, spec, false)
					if err != nil || ch.Action != "unchanged" {
						t.Fatalf("repeat install: %+v %v", ch, err)
					}
				}
				ch, err := PlanHookConfig(s, path, spec, true)
				applyIntegration(t, []Change{ch}, err)
				if !sameConfigJSON([]byte(strings.TrimPrefix(read(t, path), bom)), []byte(original)) || read(t, testBackupPath(t, path)) != bom+original {
					t.Fatal("uninstall/backup changed user configuration")
				}
			})
		}
	}
}

func TestHookConfigRefreshOptionalDeduplicateAndMetadata(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "hooks.json")
	writeIntegrationFile(t, path, testenv.NativeFixture(`{"hooks":{"Pre":[{"matcher":"Old","label":"user label","hooks":[{"command":"C:/tools/obit.exe hook trae"}]},{"matcher":"Shell","hooks":[{"command":"C:/tools/obit.exe hook trae"},{"command":"C:/tools/obit.exe hook trae"},{"type":"prompt","command":"C:/tools/obit.exe hook trae"}]}]}}`))
	spec := HookConfig{Agent: "trae", Shape: GroupedHooks, Events: []HookRegistration{{Event: "Pre", Matcher: "Shell", Handler: json.RawMessage(testenv.NativeFixture(`{"command":"C:/new/obit.exe hook trae"}`)), Optional: true}, {Event: "Start", Handler: json.RawMessage(testenv.NativeFixture(`{"command":"C:/tools/obit.exe hook trae"}`)), Optional: true}}}
	ch, err := PlanHookConfig(s, path, spec, false)
	applyIntegration(t, []Change{ch}, err)
	got := read(t, path)
	if strings.Count(got, testenv.NativeFixture("C:/new/obit.exe")) != 1 || !strings.Contains(got, "user label") || !strings.Contains(got, `"prompt"`) || strings.Contains(got, `"Start"`) {
		t.Fatal(got)
	}
	ch, err = PlanHookConfig(s, path, spec, true)
	applyIntegration(t, []Change{ch}, err)
	got = read(t, path)
	if strings.Contains(got, testenv.NativeFixture("C:/new")) || !strings.Contains(got, "user label") || !strings.Contains(got, `"prompt"`) {
		t.Fatal(got)
	}
}

func TestHookConfigRejectsMalformedConcurrentAndOutside(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "hooks.json")
	spec := HookConfig{Agent: "cursor", Shape: FlatHooks, Events: []HookRegistration{{Event: "pre", Handler: json.RawMessage(testenv.NativeFixture(`{"command":"C:/tools/obit.exe hook cursor"}`))}}}
	for _, input := range []string{`{`, `{"hooks":null}`, `{"hooks":{"pre":null}}`, `{"hooks":{"pre":{}}}`} {
		writeIntegrationFile(t, path, input)
		if _, err := PlanHookConfig(s, path, spec, false); err == nil {
			t.Fatal("accepted malformed config", input)
		}
		if read(t, path) != input {
			t.Fatal("planner wrote file")
		}
	}
	writeIntegrationFile(t, path, `{}`)
	ch, err := PlanHookConfig(s, path, spec, false)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, path, `{"edit":true}`)
	if err := Apply([]Change{ch}); err == nil {
		t.Fatal("accepted concurrent edit")
	}
	outside := t.TempDir()
	makeIntegrationJunction(t, filepath.Join(s.Project, "linked"), outside)
	if _, err := PlanHookConfig(s, filepath.Join(s.Project, "linked", "hooks.json"), spec, false); err == nil {
		t.Fatal("accepted outside link")
	}
}

func TestHookConfigDefaultsAndDuplicateKeys(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "hooks.json")
	writeIntegrationFile(t, path, `{"theme":"old","hooks":{},"theme":"last"}`)
	spec := HookConfig{Agent: "cursor", Shape: FlatHooks, Defaults: json.RawMessage(`{"version":1}`), Events: []HookRegistration{{Event: "pre", Handler: json.RawMessage(testenv.NativeFixture(`{"command":"C:/tools/obit.exe hook cursor"}`))}}}
	ch, err := PlanHookConfig(s, path, spec, false)
	applyIntegration(t, []Change{ch}, err)
	got := read(t, path)
	if strings.Count(got, `"theme"`) != 1 || !strings.Contains(got, `"theme": "last"`) || strings.Index(got, `"theme"`) > strings.Index(got, `"hooks"`) || !strings.Contains(got, `"version": 1`) {
		t.Fatal("duplicate-key/default semantics changed", got)
	}
	spec.Defaults = json.RawMessage(`{"hooks":{}}`)
	if _, err := PlanHookConfig(s, path, spec, false); err == nil {
		t.Fatal("defaults overwrote hook edits")
	}
}

func TestHookConfigCommandVariantsAndOwnedOnly(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "hooks.json")
	spec := HookConfig{Agent: "copilot", Shape: FlatHooks, CommandKeys: []string{"powershell", "bash"}, Events: []HookRegistration{{Event: "pre", Handler: json.RawMessage(testenv.NativeFixture(`{"type":"command","powershell":"C:/tools/obit.exe hook copilot","bash":"C:/tools/obit.exe hook copilot"}`))}}}
	writeIntegrationFile(t, path, testenv.NativeFixture(`{"hooks":{"pre":[{"powershell":"C:/tools/obit.exe hook copilot","bash":"user command"}]}}`))
	ch, err := PlanHookConfig(s, path, spec, true)
	if err != nil || ch.Action != "unchanged" {
		t.Fatalf("mixed-owned handler removed: %+v %v", ch, err)
	}
	writeIntegrationFile(t, path, `{}`)
	ch, err = PlanHookConfig(s, path, spec, false)
	applyIntegration(t, []Change{ch}, err)
	ch, err = PlanHookConfig(s, path, spec, true)
	applyIntegration(t, []Change{ch}, err)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("owned-only JSON remains", err)
	}
}
