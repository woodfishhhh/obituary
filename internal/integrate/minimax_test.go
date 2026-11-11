package integrate

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/hook"
)

func minimaxScope(t *testing.T) (Scope, string, Commands) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("MINIMAX_DATA_DIR", root)
	return Scope{Global: true}, root, Commands{Executable: `C:\工具 space\obit.exe`, Native: `"C:\工具 space\obit.exe" hook minimax-code`}
}

func TestMiniMaxBundleLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, kind := range []string{"hooks", "mcp", "skill"} {
		t.Run(kind, func(t *testing.T) {
			s, root, cmd := minimaxScope(t)
			name := "obituary"
			plan := func(uninstall bool) ([]Change, error) { return PlanMiniMax(s, cmd, uninstall) }
			if kind == "mcp" {
				name = "obituary-mcp"
				plan = func(u bool) ([]Change, error) { return PlanMiniMaxMCP(s, cmd.Executable, u) }
			}
			if kind == "skill" {
				name = "obituary-skill"
				plan = func(u bool) ([]Change, error) { return PlanMiniMaxSkill(s, u) }
			}
			dir := filepath.Join(root, "plugins", name)
			manifestPath := filepath.Join(dir, ".minimax-plugin", "plugin.json")
			changes, err := plan(false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "plugins")); !os.IsNotExist(err) {
				t.Fatalf("dry run wrote: %v", err)
			}
			manifestPhase := -100
			for _, ch := range changes {
				if ch.Path == manifestPath {
					manifestPhase = ch.phase
				}
			}
			for _, ch := range changes {
				if ch.Path != manifestPath && ch.Path != ManagedAssetReceiptPath(manifestPath) && ch.phase >= manifestPhase {
					t.Fatalf("manifest not last: %+v", ch)
				}
			}
			applyIntegration(t, changes, nil)
			var manifest map[string]any
			if json.Unmarshal([]byte(read(t, manifestPath)), &manifest) != nil || manifest["name"] != name || manifest["schemaVersion"] != float64(1) {
				t.Fatalf("manifest: %v", manifest)
			}
			for _, field := range []string{"hooks", "mcpServers", "skills"} {
				for _, ref := range manifest[field].([]any) {
					if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(ref.(string)))); err != nil {
						t.Fatal(err)
					}
				}
			}
			changes, err = plan(false)
			if err != nil || Pending(changes) {
				t.Fatalf("repeat installation %v %+v", err, changes)
			}
			cmd.Executable = `C:\other space\obit.exe`
			changes, err = plan(false)
			applyIntegration(t, changes, err)
			if kind == "hooks" {
				raw := read(t, filepath.Join(dir, "hooks", "hooks.json"))
				if !strings.Contains(raw, `other space`) {
					t.Fatal("stale hook executable")
				}
				assertMiniMaxHooks(t, []byte(raw))
			}
			if kind == "mcp" {
				if !strings.Contains(read(t, filepath.Join(dir, "runtime.json")), `other space`) {
					t.Fatal("stale MCP executable")
				}
				var cfg struct {
					Servers map[string]struct {
						Command string
						Args    []string
					} `json:"mcpServers"`
				}
				if json.Unmarshal([]byte(read(t, filepath.Join(dir, "obituary.mcp.json"))), &cfg) != nil || cfg.Servers["obituary"].Command != "node" || !slices.Equal(cfg.Servers["obituary"].Args, []string{"./mcp.mjs"}) {
					t.Fatalf("native MCP invalid: %+v", cfg)
				}
			}
			writeIntegrationFile(t, filepath.Join(dir, "user.txt"), "user")
			changes, err = plan(true)
			if err != nil {
				t.Fatal(err)
			}
			for _, ch := range changes {
				if ch.Path == manifestPath && ch.phase != -30 {
					t.Fatal("manifest removal order")
				}
			}
			applyIntegration(t, changes, nil)
			if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
				t.Fatalf("manifest remains %v", err)
			}
			if read(t, filepath.Join(dir, "user.txt")) != "user" {
				t.Fatal("lost user file")
			}
			changes, err = plan(true)
			if err != nil || Pending(changes) || PreservedError(changes) != nil {
				t.Fatalf("repeat uninstall %v %v", err, PreservedError(changes))
			}
		})
	}
}

func assertMiniMaxHooks(t *testing.T, raw []byte) {
	t.Helper()
	var doc struct {
		Hooks map[string][]struct {
			Matcher string
			Hooks   []map[string]any
		}
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Hooks) != 4 {
		t.Fatalf("unexpected events: %+v", doc)
	}
	for _, event := range []string{"SessionStart", "PostToolUse", "Stop", "SessionEnd"} {
		groups := doc.Hooks[event]
		if len(groups) != 1 || len(groups[0].Hooks) != 1 {
			t.Fatalf("missing %s", event)
		}
		h := groups[0].Hooks[0]
		if len(h) != 3 || h["type"] != "command" || h["timeout"] != float64(5) || !OwnedHookCommand(h["commandWindows"].(string), minimaxAgent, nil) {
			t.Fatalf("native skipped handler: %+v", h)
		}
		if event == "PostToolUse" && groups[0].Matcher != "write|edit" {
			t.Fatal("compatible matcher installed in native bundle")
		}
	}
}

func TestMiniMaxProjectLifecycleAndSharedMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	s := Scope{Project: t.TempDir()}
	rules := filepath.Join(s.Project, "AGENTS.md")
	mcp := filepath.Join(s.Project, ".mcp.json")
	original := "\xef\xbb\xbf# User rules\r\n<!-- user comment -->\r\n"
	writeIntegrationFile(t, rules, original)
	cmd := Commands{Executable: `C:\tools\obit.exe`}
	changes, err := PlanMiniMax(s, cmd, false)
	if read(t, rules) != original {
		t.Fatal("dry run changed rules")
	}
	applyIntegration(t, changes, err)
	if !strings.HasPrefix(read(t, rules), original) {
		t.Fatal("rule prefix changed")
	}
	changes, err = PlanMiniMax(s, cmd, false)
	if err != nil || Pending(changes) {
		t.Fatal("rules not idempotent", err)
	}
	writeIntegrationFile(t, mcp, `{"first":1,"mcpServers":{"user":{"command":"user.exe"}},"last":2}`)
	changes, err = PlanMiniMaxMCP(s, cmd.Executable, false)
	applyIntegration(t, changes, err)
	got := read(t, mcp)
	if strings.Index(got, "first") > strings.Index(got, "last") || !strings.Contains(got, "user.exe") {
		t.Fatal("user keys/order changed")
	}
	changes, err = PlanMiniMaxMCP(s, cmd.Executable, false)
	if err != nil || Pending(changes) {
		t.Fatal("MCP not idempotent", err)
	}
	changes, err = planPluginMCP(s, mcp, "peer-agent", cmd.Executable, false)
	applyIntegration(t, changes, err)
	withEnv := strings.Replace(read(t, mcp), `"args": [`, `"env": {"USER_SETTING": "keep"}, "args": [`, 1)
	writeIntegrationFile(t, mcp, withEnv)
	changes, err = PlanMiniMaxMCP(s, `C:\new\obit.exe`, false)
	applyIntegration(t, changes, err)
	if !strings.Contains(read(t, mcp), `new`) || !strings.Contains(read(t, mcp), "USER_SETTING") {
		t.Fatal("MCP refresh lost executable or user env")
	}
	changes, err = PlanMiniMaxMCP(s, cmd.Executable, true)
	applyIntegration(t, changes, err)
	if !strings.Contains(read(t, mcp), "obituary") {
		t.Fatal("removed shared owner MCP")
	}
	changes, err = planPluginMCP(s, mcp, "peer-agent", cmd.Executable, true)
	applyIntegration(t, changes, err)
	if strings.Contains(read(t, mcp), "obituary") || !strings.Contains(read(t, mcp), "user.exe") {
		t.Fatal("uninstall changed user service")
	}
	changes, err = PlanMiniMax(s, cmd, true)
	applyIntegration(t, changes, err)
	if !strings.HasPrefix(read(t, rules), original) || strings.Contains(read(t, rules), "obit:minimax-code") {
		t.Fatal("rule uninstall lost user content")
	}
	if _, err := PlanMiniMaxSkill(s, false); err == nil {
		t.Fatal("unverified project skill supported")
	}
}

func TestMiniMaxRefusesUnownedAndChangedFiles(t *testing.T) {
	for _, bad := range []string{"", "\xef\xbb\xbf{}", `{broken`, `{"mcpServers":[]}`, `{// comment\n}`, `{"mcpServers":{"obituary":{"command":"user.exe","args":["mcp"]}}}`} {
		s := Scope{Project: t.TempDir()}
		path := filepath.Join(s.Project, ".mcp.json")
		writeIntegrationFile(t, path, bad)
		if _, err := PlanMiniMaxMCP(s, `C:\tools\obit.exe`, false); err == nil {
			t.Fatalf("invalid config accepted %q", bad)
		}
		if read(t, path) != bad {
			t.Fatal("invalid config modified")
		}
	}
	s, root, cmd := minimaxScope(t)
	changes, err := PlanMiniMax(s, cmd, false)
	applyIntegration(t, changes, err)
	path := filepath.Join(root, "plugins", "obituary", "hooks", "hooks.json")
	old := read(t, path)
	writeIntegrationFile(t, path, old+"\n")
	if _, err := PlanMiniMax(s, cmd, false); err == nil {
		t.Fatal("edited asset claimed")
	}
	changes, err = PlanMiniMax(s, cmd, true)
	if err != nil || PreservedError(changes) == nil {
		t.Fatalf("edited uninstall: %v", err)
	}
	if err := Apply(changes); err == nil {
		t.Fatal("edited asset preservation must be reported")
	}
	if read(t, path) != old+"\n" {
		t.Fatal("edited asset removed")
	}
}

func TestMiniMaxUnownedBundleFilesStayUntouched(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, content := range []string{"", "\xef\xbb\xbf{}", "{broken", "{\"user\":true}"} {
		s, root, cmd := minimaxScope(t)
		path := filepath.Join(root, "plugins", "obituary", ".minimax-plugin", "plugin.json")
		writeIntegrationFile(t, path, content)
		if _, err := PlanMiniMax(s, cmd, false); err == nil {
			t.Fatalf("unowned manifest accepted: %q", content)
		}
		changes, err := PlanMiniMax(s, cmd, true)
		if err != nil || Apply(changes) == nil || read(t, path) != content {
			t.Fatalf("unowned manifest not preserved: %v", err)
		}
	}
}

func TestMiniMaxMCPRefusesUnreceiptedSharedEntry(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, ".mcp.json")
	original := `{"mcpServers":{"obituary":{"command":"C:/tools/obit.exe","args":["mcp"]}}}`
	writeIntegrationFile(t, path, original)
	if _, err := PlanMiniMaxMCP(s, `C:\tools\obit.exe`, false); err == nil {
		t.Fatal("adopted another host's unreceipted obituary entry")
	}
	if read(t, path) != original {
		t.Fatal("unreceipted entry changed")
	}
}

func TestMiniMaxEditedManifestRetainsPayload(t *testing.T) {
	s, root, cmd := minimaxScope(t)
	changes, err := PlanMiniMax(s, cmd, false)
	applyIntegration(t, changes, err)
	dir := filepath.Join(root, "plugins", "obituary")
	manifest := filepath.Join(dir, ".minimax-plugin", "plugin.json")
	writeIntegrationFile(t, manifest, read(t, manifest)+"\n")
	changes, err = PlanMiniMax(s, cmd, true)
	if err != nil {
		t.Fatal(err)
	}
	if Apply(changes) == nil {
		t.Fatal("edited manifest must report a preservation conflict")
	}
	if _, err := os.Stat(filepath.Join(dir, "hooks", "hooks.json")); err != nil {
		t.Fatalf("active edited manifest lost its payload: %v", err)
	}
}

func TestMiniMaxPlansRejectRacesAndLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, target := range []string{"manifest", "mcp", "rules"} {
		t.Run(target, func(t *testing.T) {
			s, root, cmd := minimaxScope(t)
			path := filepath.Join(root, "plugins", "obituary", ".minimax-plugin", "plugin.json")
			changes, err := PlanMiniMax(s, cmd, false)
			if target != "manifest" {
				s = Scope{Project: t.TempDir()}
				path = filepath.Join(s.Project, ".mcp.json")
				changes, err = PlanMiniMaxMCP(s, cmd.Executable, false)
				if target == "rules" {
					path = filepath.Join(s.Project, "AGENTS.md")
					changes, err = PlanMiniMax(s, cmd, false)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			writeIntegrationFile(t, path, "user changed")
			if Apply(changes) == nil {
				t.Fatal("stale original accepted")
			}
			if read(t, path) != "user changed" {
				t.Fatal("overwrote concurrent edit")
			}
		})
	}
	for _, global := range []bool{false, true} {
		s, root, cmd := minimaxScope(t)
		outside := t.TempDir()
		if global {
			makeIntegrationJunction(t, filepath.Join(root, "plugins"), outside)
		} else {
			s = Scope{Project: t.TempDir()}
			makeIntegrationJunction(t, filepath.Join(s.Project, "AGENTS.md"), outside)
		}
		if _, err := PlanMiniMax(s, cmd, false); err == nil {
			t.Fatalf("junction accepted global=%v", global)
		}
		entries, err := os.ReadDir(outside)
		if err != nil || len(entries) != 0 {
			t.Fatalf("outside modified %v %v", entries, err)
		}
	}
}

func TestMiniMaxDeleteGuardInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	s, root, commands := minimaxScope(t)
	path := filepath.Join(root, "plugins", "obituary", "hooks", "hooks.json")
	plan, err := PlanMiniMax(s, commands, false)
	applyIntegration(t, plan, err)
	if strings.Contains(read(t, path), "PreToolUse") {
		t.Fatal("guard installed without opt-in")
	}
	s.GuardRM = true
	plan, err = PlanMiniMax(s, commands, false)
	applyIntegration(t, plan, err)
	if !strings.Contains(read(t, path), "PreToolUse") {
		t.Fatal("guard hook missing")
	}
	s.GuardRM = false
	plan, err = PlanMiniMax(s, commands, false)
	applyIntegration(t, plan, err)
	if !strings.Contains(read(t, path), "PreToolUse") {
		t.Fatal("refresh removed opted-in guard")
	}
	plan, err = PlanMiniMax(s, commands, true)
	applyIntegration(t, plan, err)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("uninstall left guard", err)
	}
}

func TestMiniMaxCapabilitiesAndWindowsCommand(t *testing.T) {
	s, _, cmd := minimaxScope(t)
	s.GuardRM = true
	if _, err := PlanMiniMax(s, cmd, false); err != nil {
		t.Fatal("declared-shell guard installation rejected", err)
	}
	s.GuardRM = false
	for _, bad := range []string{`relative\obit.exe`, `C:\%HOME%\obit.exe`, `C:\bad!\obit.exe`, `C:\a&b\obit.exe`} {
		cmd.Executable = bad
		if _, err := PlanMiniMax(s, cmd, false); err == nil {
			t.Fatalf("unsafe cmd accepted %s", bad)
		}
	}
	a, ok := LookupAgent(minimaxAgent)
	if !ok || a.Capabilities.DeleteGuard.Level != hook.Limited || a.PlanMCP == nil || a.PlanSkill == nil ||
		!strings.Contains(a.Capabilities.DeleteGuard.Detail, "agent_shells.minimax-code") {
		t.Fatalf("capabilities %+v", a)
	}
	if _, ok := LookupAgent("mmx"); ok {
		t.Fatal("mmx is not the coding host")
	}
}

func TestMiniMaxMCPRelay(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable")
	}
	dir := t.TempDir()
	wrapper, _ := minimaxFiles.ReadFile("plugins/minimax/mcp.mjs")
	writeIntegrationFile(t, filepath.Join(dir, "mcp.mjs"), string(wrapper))
	writeIntegrationFile(t, filepath.Join(dir, "echo.mjs"), `process.stdin.pipe(process.stdout);`)
	runtime, _ := json.Marshal(map[string]any{"command": node, "args": []string{filepath.Join(dir, "echo.mjs")}})
	writeIntegrationFile(t, filepath.Join(dir, "runtime.json"), string(runtime))
	cmd := exec.Command(node, filepath.Join(dir, "mcp.mjs"))
	cmd.Stdin = strings.NewReader("{\"jsonrpc\":\"2.0\"}\n")
	out, err := cmd.CombinedOutput()
	if err != nil || !bytes.Equal(out, []byte("{\"jsonrpc\":\"2.0\"}\n")) {
		t.Fatalf("relay %s %v", out, err)
	}
}
