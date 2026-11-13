package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPConfigRootPathsAndEnv(t *testing.T) {
	t.Parallel()
	for _, root := range [][]string{{"mcpServers"}, {"servers"}, {"mcp"}, {"mcp", "servers"}, {"mcp_servers"}} {
		for _, codec := range []MCPEntryCodec{CommandArgsMCP{Type: "stdio"}, CommandArrayMCP{}} {
			t.Run(strings.Join(root, ".")+"/"+strings.TrimPrefix(strings.TrimSuffix(string(mustJSON(codec)), "}"), "{"), func(t *testing.T) {
				t.Parallel()
				s := Scope{Project: t.TempDir()}
				path := filepath.Join(s.Project, "mcp.json")
				spec := MCPConfig{ServersPath: root, Codec: codec}
				writeIntegrationFile(t, path, "\xef\xbb\xbf"+`{"first":"<&>","last":9007199254740993}`)
				ch, err := PlanMCPConfig(s, path, spec, testenv.NativeFixture("C:/old/obit.exe"), false)
				applyIntegration(t, []Change{ch}, err)
				data, _ := configJSON([]byte(read(t, path)), ConfigJSON)
				entryPath := append(append([]string{}, root...), "obituary")
				raw, _ := configMember(data, entryPath)
				obj, _ := parseObject(raw)
				_ = obj.set("env", json.RawMessage(`{"TOKEN":"keep&secret"}`))
				_ = obj.set("disabled", true)
				raw, _ = obj.MarshalJSON()
				ch, err = PlanConfigEdits(s, path, ConfigJSON, []ConfigEdit{{Path: entryPath, Value: raw}})
				applyIntegration(t, []Change{ch}, err)
				ch, err = PlanMCPConfig(s, path, spec, testenv.NativeFixture("C:/new/obit.exe"), false)
				applyIntegration(t, []Change{ch}, err)
				if !strings.Contains(read(t, path), "keep&secret") || !strings.Contains(read(t, path), `"disabled": true`) || !strings.Contains(read(t, path), testenv.NativeFixture("C:/new/obit.exe")) {
					t.Fatal("refresh lost fields", read(t, path))
				}
				ch, err = PlanMCPConfig(s, path, spec, testenv.NativeFixture("C:/new/obit.exe"), false)
				if err != nil || ch.Action != "unchanged" {
					t.Fatalf("not idempotent: %+v %v", ch, err)
				}
				ch, err = PlanMCPConfig(s, path, spec, "", true)
				applyIntegration(t, []Change{ch}, err)
				if strings.Contains(read(t, path), `"obituary"`) || !strings.Contains(read(t, path), "9007199254740993") {
					t.Fatal("uninstall modified siblings", read(t, path))
				}
			})
		}
	}
}

func TestMCPEntryCodecsStrictOwnership(t *testing.T) {
	t.Parallel()
	codec := CommandArgsMCP{}
	for _, raw := range []string{
		`{"command":"obit","args":["mcp"]}`, testenv.NativeFixture(`{"command":"C:/bin/obit.exe","args":["mcp"],"env":{"A":"B"}}`),
	} {
		if !codec.Owned([]byte(raw)) {
			t.Fatal("owned entry rejected", raw)
		}
	}
	for _, raw := range []string{
		`{"command":"./obit","args":["mcp"]}`, `{"command":"obit","args":["mcp","--help"]}`, `{"command":"obit","args":["mcp"],"type":"http"}`,
		`{"Command":"obit","Args":["mcp"]}`, `{"command":"obit","args":["mcp"],"type":null}`, `{"command":"obit","args":["mcp"],"transport":"sse"}`, `{"command":"echo obit","args":["mcp"]}`,
	} {
		if codec.Owned([]byte(raw)) {
			t.Fatal("foreign entry claimed", raw)
		}
	}
	for _, raw := range []string{`{"type":"remote","command":["obit","mcp"]}`, `{"type":"local","command":["obit","mcp","--help"]}`, `{"type":"local","command":["./obit","mcp"]}`} {
		if (CommandArrayMCP{}).Owned([]byte(raw)) {
			t.Fatal("foreign array entry claimed", raw)
		}
	}
}

func TestMCPConfigUnownedConflictAndJSON5Manual(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "mcp.json")
	input := `{"mcpServers":{"obituary":{"command":"user","args":["mcp"]}}}`
	writeIntegrationFile(t, path, input)
	spec := MCPConfig{ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{}}
	if _, err := PlanMCPConfig(s, path, spec, "obit", false); err == nil {
		t.Fatal("replaced user registration")
	}
	ch, err := PlanMCPConfig(s, path, spec, "", true)
	if err != nil || ch.Action != "preserve" || Apply([]Change{ch}) == nil || read(t, path) != input {
		t.Fatalf("unowned uninstall: %+v %v", ch, err)
	}
	spec.Format, spec.ServersPath = ConfigJSON5, []string{"mcp", "servers"}
	if _, err := PlanMCPConfig(s, path, spec, testenv.NativeFixture("C:/bin/obit.exe"), false); err == nil || !strings.Contains(err.Error(), `["mcp","servers","obituary"]`) || !strings.Contains(err.Error(), testenv.NativeFixture("C:/bin/obit.exe")) {
		t.Fatalf("missing exact manual instruction: %v", err)
	}
}
