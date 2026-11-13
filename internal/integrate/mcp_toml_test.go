package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func FuzzMCPTOML(f *testing.F) {
	f.Fuzz(func(t *testing.T, original []byte) {
		bom := []byte("\xef\xbb\xbf")
		data := bytes.TrimPrefix(original, bom)
		start, end, rangeErr := mcpBlockRange(data)
		if rangeErr == nil && (start < -1 || start == -1 && end != -1 || start >= 0 && (end <= start || end > len(data))) {
			t.Fatalf("invalid block range: %d:%d of %d", start, end, len(data))
		}
		for _, uninstall := range []bool{false, true} {
			input := bytes.Clone(original)
			ch, err := planMCPTOML(Change{Path: "config.toml", orig: input, existed: true, Action: "unchanged"}, testenv.NativeFixture("C:/fuzz/obit.exe"), uninstall)
			if !bytes.Equal(input, original) {
				t.Fatal("planning mutated the original bytes")
			}
			if err != nil {
				continue
			}
			if rangeErr != nil {
				t.Fatalf("invalid ownership markers accepted: %v", rangeErr)
			}
			out := ch.data
			if ch.Action == "unchanged" {
				out = original
			}
			var decoded map[string]any
			if err := toml.Unmarshal(bytes.TrimPrefix(out, bom), &decoded); err != nil {
				t.Fatalf("plan produced invalid TOML: %v", err)
			}
			prefix := []byte(nil)
			if bytes.HasPrefix(original, bom) {
				prefix = bom
			}
			if start >= 0 {
				prefix = append(bytes.Clone(prefix), data[:start]...)
				if !bytes.HasPrefix(out, prefix) || !bytes.HasSuffix(out, data[end:]) {
					t.Fatal("plan changed bytes outside the ownership block")
				}
				if uninstall && !bytes.Equal(out, append(prefix, data[end:]...)) {
					t.Fatal("uninstall removed bytes outside the ownership block")
				}
			} else if uninstall && !bytes.Equal(out, original) || !uninstall && !bytes.HasPrefix(out, original) {
				t.Fatal("unmarked TOML bytes changed")
			}
			if !uninstall {
				removed, err := planMCPTOML(Change{Path: "config.toml", orig: out, existed: true, Action: "unchanged"}, "obit", true)
				if err != nil {
					t.Fatalf("installed block cannot be removed: %v", err)
				}
				if start < 0 && !bytes.Equal(removed.data, original) {
					t.Fatalf("install/uninstall did not preserve original bytes: %q -> %q", original, removed.data)
				}
			}
		}
	})
}

func TestMCPTOMLPreservesFormattingAndServerOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, original := range []string{"", "\xef\xbb\xbf", "model='example' # no final newline", "# CRLF\r\n[mcp_servers.user]\r\ncommand='user'\r\n"} {
		scope := Scope{Project: t.TempDir()}
		path := filepath.Join(scope.Project, ".codex", "config.toml")
		writeIntegrationFile(t, path, original)
		plan, err := PlanMCP(scope, "codex", testenv.NativeFixture("C:/old/obit.exe"), false)
		applyIntegration(t, plan, err)
		// User options inside our table and unrelated tables after our block
		// remain byte-for-byte intact when only the executable path changes.
		extra := "enabled = false # deliberate user setting\n"
		tail := "\r\n[mcp_servers.after]\r\ncommand = 'after' # keep\r\n"
		withOptions := strings.Replace(read(t, path), mcpBlockEnd, extra+mcpBlockEnd, 1) + tail
		writeIntegrationFile(t, path, withOptions)
		plan, err = PlanMCP(scope, "codex", testenv.NativeFixture("C:/new/obit.exe"), false)
		applyIntegration(t, plan, err)
		if got := read(t, path); got != strings.Replace(withOptions, testenv.NativeFixture("C:/old/obit.exe"), testenv.NativeFixture("C:/new/obit.exe"), 1) {
			t.Fatal("refresh changed bytes outside command value", got)
		}
		plan, err = PlanMCP(scope, "codex", "obit", true)
		applyIntegration(t, plan, err)
		if got := read(t, path); got != original+tail {
			t.Fatalf("uninstall changed user bytes: %q != %q", got, original+tail)
		}
	}
}

func TestMCPTOMLParentForms(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, original := range []string{
		"model = 'test'\n[mcp_servers]\n",
		"[mcp_servers]\nother = { command = 'x', args = ['a'] }\n",
		"mcp_servers.other.command = 'x'\nmcp_servers.other.args = ['a']\n",
		"[mcp_servers.other]\ncommand = 'x'\n",
	} {
		t.Run(original, func(t *testing.T) {
			scope := Scope{Project: t.TempDir()}
			path := filepath.Join(scope.Project, ".codex", "config.toml")
			writeIntegrationFile(t, path, original)
			plan, err := PlanMCP(scope, "codex", testenv.NativeFixture("C:/tools/obit.exe"), false)
			if err != nil {
				t.Fatalf("valid parent form rejected: %v", err)
			}
			applyIntegration(t, plan, nil)
			var before, after map[string]any
			if err := toml.Unmarshal([]byte(original), &before); err != nil {
				t.Fatal(err)
			}
			if err := toml.Unmarshal([]byte(read(t, path)), &after); err != nil {
				t.Fatalf("installed TOML invalid: %v", err)
			}
			servers := after["mcp_servers"].(map[string]any)
			if servers["obituary"] == nil {
				t.Fatal("registration missing")
			}
			delete(servers, "obituary")
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("other settings changed: %v != %v", before, after)
			}
		})
	}
	for _, original := range []string{"mcp_servers = {}\n", "mcp_servers = {other = {command = 'x'}}\n"} {
		scope := Scope{Project: t.TempDir()}
		path := filepath.Join(scope.Project, ".codex", "config.toml")
		writeIntegrationFile(t, path, original)
		if _, err := PlanMCP(scope, "codex", "obit", false); err == nil || !strings.Contains(err.Error(), "inline") {
			t.Fatalf("inline parent needs clear refusal: %v", err)
		}
		if read(t, path) != original {
			t.Fatal("inline parent changed")
		}
	}
}

func TestMCPTOMLConvertedCRLF(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	scope := Scope{Project: t.TempDir()}
	path := filepath.Join(scope.Project, ".codex", "config.toml")
	original := "model = 'test'\n[mcp_servers.other]\ncommand = 'x'\n"
	writeIntegrationFile(t, path, original)
	plan, err := PlanMCP(scope, "codex", testenv.NativeFixture("C:/old/obit.exe"), false)
	applyIntegration(t, plan, err)
	writeIntegrationFile(t, path, strings.ReplaceAll(read(t, path), "\n", "\r\n"))
	for _, exe := range []string{testenv.NativeFixture("C:/old/obit.exe"), testenv.NativeFixture("C:/new/obit.exe")} {
		plan, err = PlanMCP(scope, "codex", exe, false)
		if err != nil {
			t.Fatalf("CRLF refresh failed: %v", err)
		}
		applyIntegration(t, plan, nil)
		st, err := InspectExtension(scope, "codex", "mcp", exe)
		if err != nil || !st.Current {
			t.Fatalf("CRLF inspect failed: %+v %v", st, err)
		}
	}
	plan, err = PlanMCP(scope, "codex", "obit", true)
	applyIntegration(t, plan, err)
	if got := read(t, path); got != strings.ReplaceAll(original, "\n", "\r\n") {
		t.Fatalf("CRLF uninstall left bytes: %q", got)
	}
}

func TestMCPTOMLRefusesChangedScopeAndBrokenMarkers(t *testing.T) {
	t.Parallel()
	scope := Scope{Project: t.TempDir()}
	path := filepath.Join(scope.Project, ".codex", "config.toml")
	plan, err := PlanMCP(scope, "codex", "obit", false)
	applyIntegration(t, plan, err)
	installed := read(t, path)
	for _, content := range []string{
		installed + "env = { USER_EDIT = 'outside block' }\n",
		strings.Replace(installed, mcpBlockEnd, "# missing end", 1),
		mcpBlockBegin + "\n" + installed,
		installed + mcpBlockEnd + "\n",
	} {
		writeIntegrationFile(t, path, content)
		for _, uninstall := range []bool{false, true} {
			if _, err := PlanMCP(scope, "codex", "obit", uninstall); err == nil {
				t.Fatal("ambiguous ownership accepted", content)
			}
		}
		if read(t, path) != content {
			t.Fatal("rejected plan wrote file")
		}
	}
}

func TestMCPTOMLOnlyRemovesOwnedBlock(t *testing.T) {
	t.Parallel()
	scope := Scope{Project: t.TempDir()}
	path := filepath.Join(scope.Project, ".codex", "config.toml")
	unowned := "# user server without ownership markers\n[mcp_servers.obituary]\ncommand = 'obit'\nargs = ['mcp']\n"
	writeIntegrationFile(t, path, unowned)
	plan, err := PlanMCP(scope, "codex", "obit", true)
	applyIntegration(t, plan, err)
	if got := read(t, path); got != unowned {
		t.Fatal("uninstall removed unowned TOML", got)
	}
	if _, err := PlanMCP(scope, "codex", "obit", false); err == nil {
		t.Fatal("install claimed an unmarked user server")
	}
	// A marker-looking line inside a multiline value is user data, not a block.
	quoted := "text = '''\n" + mcpBlockBegin + "\n" + mcpBlockEnd + "\n'''\n"
	writeIntegrationFile(t, path, quoted)
	plan, err = PlanMCP(scope, "codex", "obit", false)
	applyIntegration(t, plan, err)
	plan, err = PlanMCP(scope, "codex", "obit", true)
	applyIntegration(t, plan, err)
	if read(t, path) != quoted {
		t.Fatal("removed markers inside a string")
	}
	for _, body := range []string{
		"[mcp_servers.obituary]\ncommand = 'other'\nargs = ['mcp']\n",
		"[mcp_servers.obituary]\ncommand = 'obit'\nargs = ['mcp', 'extra']\n",
		"[mcp_servers.obituary]\ncommand = 'obit'\nargs = ['mcp']\n[mcp_servers.user]\ncommand = 'keep'\n",
	} {
		original := mcpBlockBegin + "\n" + body + mcpBlockEnd + "\n"
		writeIntegrationFile(t, path, original)
		plan, err = PlanMCP(scope, "codex", "obit", true)
		if err == nil {
			applyIntegration(t, plan, nil)
		}
		if read(t, path) != original {
			t.Fatal("removed user content inside edited block")
		}
	}
}
