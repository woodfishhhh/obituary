package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeMCPPreservationIsMemberScoped(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "config.json")
	writeIntegrationFile(t, path, `{"mcpServers":{"obituary":{"command":"user-server","args":[]}},"hooks":{"Stop":[]}}`)
	preserved, err := PlanMCPConfig(s, path, MCPConfig{ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{}}, "", true)
	if err != nil {
		t.Fatal(err)
	}
	hooks, err := PlanConfigEdits(s, path, ConfigJSON, []ConfigEdit{{Path: []string{"hooks"}, Remove: true}})
	if err != nil {
		t.Fatal(err)
	}
	for _, order := range [][]Change{{hooks, preserved}, {preserved, hooks}} {
		combined, err := ComposeAgentChanges(order)
		if err != nil {
			t.Fatal(err)
		}
		if PreservedError(combined) == nil {
			t.Fatal("scoped conflict receipt lost")
		}
	}
	for _, member := range [][]string{{"mcpServers"}, {"mcpServers", "obituary", "command"}} {
		overlap, err := PlanConfigEdits(s, path, ConfigJSON, []ConfigEdit{{Path: member, Remove: true}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ComposeAgentChanges([]Change{overlap, preserved}); err == nil {
			t.Fatal("preserved MCP member was editable")
		}
	}
}

func TestComposeAgentChanges(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, ext, original string
		format              ConfigFormat
	}{
		{"json", "json", "\xef\xbb\xbf{\"z\":1,\"a\":2}", ConfigJSON},
		{"jsonc", "jsonc", "\xef\xbb\xbf{\n// user comment\n\"z\":1,\"a\":2,\n}", ConfigJSONC},
		{"yaml", "yaml", "\xef\xbb\xbf# user comment\nz: 1\na: 2\n", ConfigYAML},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, "config."+tc.ext)
			writeIntegrationFile(t, path, tc.original)
			left, err := PlanConfigEdits(s, path, tc.format, []ConfigEdit{{Path: []string{"plugins", "enabled"}, Value: []byte(`["obituary"]`)}})
			if err != nil {
				t.Fatal(err)
			}
			right, err := PlanConfigEdits(s, path, tc.format, []ConfigEdit{{Path: []string{"mcp", "obituary"}, Value: []byte(`{"command":"obit.exe"}`)}})
			if err != nil {
				t.Fatal(err)
			}
			right.Extension = "mcp"
			right.Implicit = true
			plan, err := ComposeAgentChanges([]Change{left, right})
			if err != nil {
				t.Fatal(err)
			}
			if len(plan) != 1 || plan[0].Extension != "" || plan[0].Implicit {
				t.Fatalf("metadata: %+v", plan)
			}
			if read(t, path) != tc.original {
				t.Fatal("composition wrote file")
			}
			if err := Apply(plan); err != nil {
				t.Fatal(err)
			}
			got := read(t, path)
			if !strings.HasPrefix(got, "\xef\xbb\xbf") {
				t.Fatal("BOM lost")
			}
			if tc.format != ConfigJSON && !strings.Contains(got, "user comment") {
				t.Fatal("comment lost")
			}
			data, _ := configJSON([]byte(got), tc.format)
			for _, key := range []string{"plugins", "mcp"} {
				raw, err := configMember(data, []string{key})
				if err != nil || raw == nil {
					t.Fatalf("missing %s: %s %v", key, data, err)
				}
			}
			left, err = PlanConfigEdits(s, path, tc.format, []ConfigEdit{{Path: []string{"plugins", "enabled"}, Value: []byte(`["obituary"]`)}})
			if err != nil {
				t.Fatal(err)
			}
			right, err = PlanConfigEdits(s, path, tc.format, []ConfigEdit{{Path: []string{"mcp", "obituary"}, Value: []byte(`{"command":"obit.exe"}`)}})
			if err != nil {
				t.Fatal(err)
			}
			repeat, err := ComposeAgentChanges([]Change{left, right})
			if err != nil || Pending(repeat) {
				t.Fatalf("repeat: %+v %v", repeat, err)
			}
			if err := Apply(repeat); err != nil {
				t.Fatal(err)
			}
			if got != read(t, path) {
				t.Fatal("repeat changed bytes")
			}
		})
	}
}

func TestComposeAgentChangesConflicts(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "config.json")
	writeIntegrationFile(t, path, `{"items":[1],"object":{"child":1}}`)
	plan := func(edit ConfigEdit) Change {
		ch, err := PlanConfigEdits(s, path, ConfigJSON, []ConfigEdit{edit})
		if err != nil {
			t.Fatal(err)
		}
		return ch
	}
	for _, pair := range [][2]ConfigEdit{
		{{Path: []string{"items"}, Value: []byte(`[1,2]`)}, {Path: []string{"items"}, Value: []byte(`[1,3]`)}},
		{{Path: []string{"object"}, Remove: true}, {Path: []string{"object", "child"}, Value: []byte(`2`)}},
	} {
		if _, err := ComposeAgentChanges([]Change{plan(pair[0]), plan(pair[1])}); err == nil {
			t.Fatalf("accepted overlap: %+v", pair)
		}
	}
	identical := plan(ConfigEdit{Path: []string{"object", "child"}, Value: []byte(`2`)})
	combined, err := ComposeAgentChanges([]Change{identical, identical})
	if err != nil {
		t.Fatal(err)
	}
	applyIntegration(t, combined, nil)
	if !strings.Contains(read(t, path), `"child":2`) {
		t.Fatal("identical edit did not apply")
	}
	left := plan(ConfigEdit{Path: []string{"left"}, Value: []byte(`1`)})
	right := plan(ConfigEdit{Path: []string{"right"}, Value: []byte(`2`)})
	right.orig = []byte(`{}`)
	if _, err := ComposeAgentChanges([]Change{left, right}); err == nil {
		t.Fatal("different originals accepted")
	}
	right = left
	right.next = &Change{}
	if _, err := ComposeAgentChanges([]Change{left, right}); err == nil {
		t.Fatal("receipt transaction combined")
	}
}

func TestComposeTOMLBlocks(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "config.toml")
	original := "\xef\xbb\xbf# user comment\nmodel = 'keep'\n"
	writeIntegrationFile(t, path, original)
	spec := func(id, table string) TOMLBlock {
		return TOMLBlock{ID: id, Content: []byte("[" + table + "]\ncommand = 'obit.exe'\n"), Owns: func(map[string]any) bool { return true }, Refresh: func(data []byte) ([]byte, error) {
			return RefreshTOMLString(data, []string{table}, "command", testenv.NativeFixture("C:/new/obit.exe"))
		}}
	}
	hooks, mcp := spec("hooks", "hooks"), spec("mcp", "mcp")
	for _, remove := range []bool{false, false, true, true} {
		a, err := PlanTOMLBlock(s, path, hooks, remove)
		if err != nil {
			t.Fatal(err)
		}
		b, err := PlanTOMLBlock(s, path, mcp, remove)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := ComposeAgentChanges([]Change{a, b})
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(plan); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(read(t, path), original) {
			t.Fatal("TOML user bytes changed")
		}
	}
}

func TestComposePreservesNewSiblingKeysAndRejectsLostComments(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "config.jsonc")
	writeIntegrationFile(t, path, "{\"a\": {/* user's value */ \"x\":1},\"b\":0}")
	left, err := PlanConfigEdits(s, path, ConfigJSONC, []ConfigEdit{{Path: []string{"shared", "left"}, Value: []byte(`1`)}})
	if err != nil {
		t.Fatal(err)
	}
	right, err := PlanConfigEdits(s, path, ConfigJSONC, []ConfigEdit{{Path: []string{"shared", "right"}, Value: []byte(`2`)}})
	if err != nil {
		t.Fatal(err)
	}
	joined, err := ComposeAgentChanges([]Change{left, right})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(joined[0].data, []byte("user's value")) {
		t.Fatal("comment lost")
	}
	left.data = bytes.ReplaceAll(left.data, []byte(`"left":1`), []byte(`"left":1 /* new comment */`))
	if _, err := ComposeAgentChanges([]Change{left, right}); err == nil {
		t.Fatal("lost newly planned comment accepted")
	}
}

func TestComposeTOMLRejectsScopeAndKeyConflicts(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "config.toml")
	original := []byte("# keep\nuser = 'value'\n")
	change := func(text string) Change {
		return Change{Path: path, Action: "update", existed: true, orig: original, data: []byte(text), project: s.Project}
	}
	left := change(string(original) + "[a]\nx = 1\n")
	right := change(string(original) + "b = 2\n")
	if _, err := ComposeAgentChanges([]Change{left, right}); err == nil {
		t.Fatal("appended root key acquired table scope")
	}
	right = change(string(original) + "[a]\nx = 2\n")
	if _, err := ComposeAgentChanges([]Change{left, right}); err == nil {
		t.Fatal("overlapping TOML keys accepted")
	}
	left = change("# keep\nuser = 'changed'\nleft = 1\n")
	right = change("# changed comment\nuser = 'value'\nright = 1\n")
	if _, err := ComposeAgentChanges([]Change{left, right}); err == nil {
		t.Fatal("overlapping text/comment rewrite accepted")
	}
}

func TestComposeEmptyFilesAndMixedGroups(t *testing.T) {
	t.Parallel()
	for _, ext := range []string{"json", "jsonc", "yaml", "toml"} {
		t.Run(ext, func(t *testing.T) {
			t.Parallel()
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, "config."+ext)
			original := []byte("\xef\xbb\xbf")
			var a, b Change
			if ext == "toml" {
				a = Change{Path: path, Action: "update", existed: true, orig: original, data: append(bytes.Clone(original), []byte("[a]\nx = 1\n")...), project: s.Project}
				b = Change{Path: path, Action: "update", existed: true, orig: original, data: append(bytes.Clone(original), []byte("[b]\nx = 2\n")...), project: s.Project}
			} else {
				writeIntegrationFile(t, path, string(original))
				format := ConfigJSONC
				if ext == "yaml" {
					format = ConfigYAML
				}
				var err error
				a, err = PlanConfigEdits(s, path, format, []ConfigEdit{{Path: []string{"a"}, Value: []byte(`1`)}})
				if err != nil {
					t.Fatal(err)
				}
				b, err = PlanConfigEdits(s, path, format, []ConfigEdit{{Path: []string{"b"}, Value: []byte(`2`)}})
				if err != nil {
					t.Fatal(err)
				}
			}
			asset := Change{Path: filepath.Join(s.Project, "plugin.ts"), Action: "create", data: []byte("plugin"), next: &Change{Action: "update"}}
			joined, err := ComposeAgentChanges([]Change{a, asset, b})
			if err != nil {
				t.Fatal(err)
			}
			if len(joined) != 2 || joined[1].next != asset.next || !bytes.HasPrefix(joined[0].data, original) {
				t.Fatalf("combined plan: %+v", joined)
			}
		})
	}
}

func TestComposePreflightRemainsAtomic(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "config.yaml")
	writeIntegrationFile(t, path, "user: keep\n")
	a, err := PlanConfigEdits(s, path, ConfigYAML, []ConfigEdit{{Path: []string{"a"}, Value: []byte(`1`)}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := PlanConfigEdits(s, path, ConfigYAML, []ConfigEdit{{Path: []string{"b"}, Value: []byte(`2`)}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := ComposeAgentChanges([]Change{a, b})
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, path, "user: concurrently edited\n")
	if err := Apply(plan); err == nil {
		t.Fatal("concurrent save accepted")
	}
	if read(t, path) != "user: concurrently edited\n" {
		t.Fatal("concurrent save lost")
	}
}
