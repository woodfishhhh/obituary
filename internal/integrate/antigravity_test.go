package integrate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAntigravityBundleLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{true: "global", false: "project"}[global], func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("OBIT_USER_HOME", home)
			s := Scope{Global: global, Project: t.TempDir()}
			root, err := antigravityRoot(s)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, "plugins", "obituary")
			user := filepath.Join(dir, "user.txt")
			writeIntegrationFile(t, user, "user asset")
			commands := Commands{Native: `"C:/space & tools/obit.exe" hook antigravity`}
			plan, err := PlanAntigravity(s, commands, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(dir, "plugin.json")); !os.IsNotExist(err) {
				t.Fatal("dry-run wrote manifest")
			}
			var writes []string
			afterIntegrationWrite := func(path string) error { writes = append(writes, path); return nil }

			if err := apply(plan, afterIntegrationWrite); err != nil {
				t.Fatal(err)
			}
			manifest := filepath.Join(dir, "plugin.json")
			if writes[len(writes)-2] != manifest || writes[len(writes)-1] != ManagedAssetReceiptPath(manifest) {
				t.Fatal("activation was not last", writes)
			}
			hooksPath := filepath.Join(dir, "hooks.json")
			var groups map[string]map[string]json.RawMessage
			if json.Unmarshal([]byte(read(t, hooksPath)), &groups) != nil || len(groups) != 1 {
				t.Fatal("invalid bundle hook map")
			}
			for _, event := range []string{"SessionStart", "Stop"} {
				var handlers []map[string]any
				if json.Unmarshal(groups["obituary"][event], &handlers) != nil || len(handlers) != 1 || handlers[0]["command"] != commands.Native+" --event "+event || handlers[0]["type"] != "command" || handlers[0]["hooks"] != nil {
					t.Fatalf("lifecycle hook must be a flat native handler: %s", groups["obituary"][event])
				}
			}
			if len(groups["obituary"]) != 3 {
				t.Fatal("unsupported native event registered")
			}
			if !strings.Contains(read(t, filepath.Join(dir, "rules", "AGENTS.md")), ownedMarker) {
				t.Fatal("missing rule ownership marker")
			}
			plan, err = PlanAntigravity(s, commands, false)
			if err != nil || Pending(plan) {
				t.Fatalf("idempotence: %v", err)
			}
			commands.Native = `"D:/new location/obit.exe" hook antigravity`
			plan, err = PlanAntigravity(s, commands, false)
			applyIntegration(t, plan, err)
			if !strings.Contains(read(t, hooksPath), "new location") || strings.Contains(read(t, hooksPath), "space & tools") {
				t.Fatal("executable not refreshed")
			}
			plan, err = PlanAntigravity(s, Commands{}, true)
			applyIntegration(t, plan, err)
			for _, name := range []string{"plugin.json", "hooks.json", "rules/AGENTS.md"} {
				path := filepath.Join(dir, name)
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("owned asset remains", path)
				}
				if _, err := os.Stat(ManagedAssetReceiptPath(path)); !os.IsNotExist(err) {
					t.Fatal("receipt remains", path)
				}
			}
			if read(t, user) != "user asset" {
				t.Fatal("user asset changed")
			}
			plan, err = PlanAntigravity(s, Commands{}, true)
			if err != nil || Pending(plan) {
				t.Fatalf("repeat uninstall: %v", err)
			}
		})
	}
}

func TestAntigravityPreservesEditedAssetsAndConcurrentChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, name := range []string{"plugin.json", "hooks.json", "rules/AGENTS.md", "hooks.json.obituary-asset"} {
		t.Run(name, func(t *testing.T) {
			s := Scope{Project: t.TempDir()}
			commands := Commands{Native: "C:/tools/obit.exe hook antigravity"}
			plan, err := PlanAntigravity(s, commands, false)
			applyIntegration(t, plan, err)
			dir := filepath.Join(s.Project, ".agents", "plugins", "obituary")
			path := filepath.Join(dir, name)
			writeIntegrationFile(t, path, "user-modified")
			if _, err := PlanAntigravity(s, commands, false); err == nil {
				t.Fatal("edited asset claimed")
			}
			plan, err = PlanAntigravity(s, Commands{}, true)
			if err != nil || PreservedError(plan) == nil || Apply(plan) == nil || read(t, path) != "user-modified" {
				t.Fatalf("edited file not explicitly preserved: %v", err)
			}
			if name == "plugin.json" {
				if _, err := os.Stat(filepath.Join(dir, "hooks.json")); err != nil {
					t.Fatal("active edited manifest lost its implementation", err)
				}
			}
		})
	}
	s := Scope{Project: t.TempDir()}
	plan, err := PlanAntigravity(s, Commands{Native: "C:/tools/obit.exe hook antigravity"}, false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Project, ".agents", "plugins", "obituary", "hooks.json")
	writeIntegrationFile(t, path, "concurrent")
	if Apply(plan) == nil || read(t, path) != "concurrent" {
		t.Fatal("concurrent edit overwritten")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatal("partial write before preflight", err)
	}
}

func TestAntigravityMCPAndBoundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, global := range []bool{false, true} {
		s := Scope{Project: t.TempDir(), Global: global}
		t.Setenv("OBIT_USER_HOME", t.TempDir())
		root, _ := antigravityRoot(s)
		path := filepath.Join(root, "mcp_config.json")
		writeIntegrationFile(t, path, "\xef\xbb\xbf"+`{"user":true,"mcpServers":{"other":{"command":"keep"}}}`)
		plan, err := PlanAntigravityMCP(s, "C:/space dir/obit.exe", false)
		applyIntegration(t, plan, err)
		plan, err = PlanAntigravityMCP(s, "D:/new/obit.exe", false)
		applyIntegration(t, plan, err)
		plan, err = PlanAntigravityMCP(s, "D:/new/obit.exe", false)
		if err != nil || Pending(plan) {
			t.Fatal("MCP not idempotent", err)
		}
		plan, err = PlanAntigravityMCP(s, "", true)
		applyIntegration(t, plan, err)
		if !strings.Contains(read(t, path), `"other"`) || !bytes.HasPrefix([]byte(read(t, path)), []byte("\xef\xbb\xbf")) || strings.Contains(read(t, path), "obituary") {
			t.Fatal("MCP user state changed")
		}
	}
	s := Scope{Project: t.TempDir()}
	makeIntegrationJunction(t, filepath.Join(s.Project, ".agents"), t.TempDir())
	if _, err := PlanAntigravity(s, Commands{Native: "C:/tools/obit.exe hook antigravity"}, false); err == nil {
		t.Fatal("external plugin root accepted")
	}
}

func TestAntigravitySkillTargets(t *testing.T) {
	checkAgentSkillTargets(t, "antigravity")
}

func TestAntigravitySkillSharedOwners(t *testing.T) {
	checkSharedSkillOwners(t, Scope{Project: t.TempDir()}, "codex", "antigravity")
}
