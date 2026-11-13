package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONCCommentsAndTrailingCommas(t *testing.T) {
	t.Parallel()
	input := `// document
{
  "theme": "dark", // preference
  "mcp": { // servers
    "other": {"command": ["user"],},
  },
  "large": 9007199254740993,
}
`
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "opencode.jsonc")
	writeIntegrationFile(t, path, input)
	spec := MCPConfig{Format: ConfigJSONC, ServersPath: []string{"mcp"}, Codec: CommandArrayMCP{}}
	for _, exe := range []string{testenv.NativeFixture("C:/old/obit.exe"), testenv.NativeFixture("C:/new/obit.exe")} {
		ch, err := PlanMCPConfig(s, path, spec, exe, false)
		applyIntegration(t, []Change{ch}, err)
		got := read(t, path)
		for _, preserved := range []string{"// document", "// preference", "// servers", `"other": {"command": ["user"],}`, "9007199254740993"} {
			if !strings.Contains(got, preserved) {
				t.Fatalf("lost %s: %s", preserved, got)
			}
		}
		ch, err = PlanMCPConfig(s, path, spec, exe, false)
		if err != nil || ch.Action != "unchanged" {
			t.Fatalf("repeat changed JSONC: %+v %v", ch, err)
		}
	}
	ch, err := PlanMCPConfig(s, path, spec, "", true)
	applyIntegration(t, []Change{ch}, err)
	got, err := jsoncJSON([]byte(read(t, path)))
	want, _ := jsoncJSON([]byte(input))
	if err != nil || !sameConfigJSON(got, want) {
		t.Fatalf("uninstall semantics: %s %v", got, err)
	}
}

func TestJSONCHookCommentsWithinUserEntries(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "hooks.jsonc")
	writeIntegrationFile(t, path, read(t, filepath.Join("testdata", "hooks-comments.jsonc"))+"// end")
	spec := HookConfig{Agent: "cursor", Shape: FlatHooks, Format: ConfigJSONC, Events: []HookRegistration{{Event: "pre", Handler: json.RawMessage(testenv.NativeFixture(`{"command":"C:/tools/obit.exe hook cursor"}`))}}}
	ch, err := PlanHookConfig(s, path, spec, true)
	applyIntegration(t, []Change{ch}, err)
	for _, text := range []string{"/* user block */", "/* env */", "// end", "echo hello"} {
		if !strings.Contains(read(t, path), text) {
			t.Fatal("comment lost", text, read(t, path))
		}
	}
}

func TestJSONCRejectsAmbiguousKeysAndJSON5(t *testing.T) {
	t.Parallel()
	for _, input := range []string{`{"a":1,"a":2}`, `{"mcp":{"obituary":{},"obituary":{}}}`, `{unquoted:'value'}`, `{"a":1} trailing`} {
		if _, err := jsoncJSON([]byte(input)); err == nil {
			t.Fatal("accepted ambiguous/malformed document", input)
		}
	}
}
