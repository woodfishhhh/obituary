package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestYAMLNodeEditsPreserveCommentsOrderAndSemantics(t *testing.T) {
	t.Parallel()
	input := `# user's configuration
z: 9007199254740993
plugins:
  enabled:
    - user-plugin # keep plugin
mcp_servers:
  other: # other server
    command: user
    args: []
theme: "dark" # keep theme
`
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "config.yaml")
	writeIntegrationFile(t, path, input)
	spec := MCPConfig{Format: ConfigYAML, ServersPath: []string{"mcp_servers"}, Codec: CommandArgsMCP{}}
	for _, exe := range []string{testenv.NativeFixture("C:/old/obit.exe"), testenv.NativeFixture("C:/new/obit.exe")} {
		ch, err := PlanMCPConfig(s, path, spec, exe, false)
		applyIntegration(t, []Change{ch}, err)
		ch, err = PlanMCPConfig(s, path, spec, exe, false)
		if err != nil || ch.Action != "unchanged" {
			t.Fatalf("not idempotent: %+v %v", ch, err)
		}
		for _, text := range []string{"# user's configuration", "# keep plugin", "# other server", "# keep theme", "9007199254740993"} {
			if !strings.Contains(read(t, path), text) {
				t.Fatal("lost comment/scalar", text)
			}
		}
		if strings.Index(read(t, path), "z:") > strings.Index(read(t, path), "plugins:") {
			t.Fatal("reordered user YAML")
		}
	}
	ch, err := PlanMCPConfig(s, path, spec, "", true)
	applyIntegration(t, []Change{ch}, err)
	got, err := yamlJSON([]byte(read(t, path)))
	want, _ := yamlJSON([]byte(input))
	if err != nil || !sameConfigJSON(got, want) {
		t.Fatalf("uninstall changed YAML: %s %v", got, err)
	}
}

func TestYAMLSequenceEditsAndInvalidDocuments(t *testing.T) {
	t.Parallel()
	input := []byte("plugins:\n  enabled:\n    - obituary # managed comment\n    - user # user comment\n")
	out, err := editConfig(input, ConfigYAML, []ConfigEdit{{Path: []string{"plugins", "enabled"}, Value: json.RawMessage(`["user"]`)}})
	if err != nil || !strings.Contains(string(out), "# user comment") || !strings.Contains(string(out), "# managed comment") {
		t.Fatalf("sequence comments: %s %v", out, err)
	}
	for _, input := range []string{"a: 1\na: 2\n", "a: 1\n---\na: 2\n", "- list\n", "a: [broken", "base: &b {args: []}\nmcp_servers: {obituary: *b}\n"} {
		_, err := editConfig([]byte(input), ConfigYAML, []ConfigEdit{{Path: []string{"mcp_servers", "obituary"}, Value: json.RawMessage(`{"command":"obit","args":["mcp"]}`)}})
		if err == nil {
			t.Fatal("ambiguous/invalid YAML edited", input)
		}
	}
}
