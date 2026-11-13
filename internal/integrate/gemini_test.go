package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGeminiInstallerLifecycle(t *testing.T) { testNativeJSONInstallerLifecycle(t, "gemini") }
func TestGeminiInstallerBoundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testNativeJSONInstallerBoundaries(t, "gemini")
}
func TestGeminiInstallerExtensions(t *testing.T) { testNativeJSONExtensions(t, "gemini") }

func TestGeminiPreservesCommentedSettingsAcrossHookAndMCPEdits(t *testing.T) {
	f := newNativeJSONInstallFixture(t, "gemini", false)
	writeIntegrationFile(t, f.hookPath, f.userConfig)
	plan, err := PlanGemini(f.scope, nativeJSONCommands("gemini", "tools"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	plan, err = PlanGeminiMCP(f.scope, testenv.NativeFixture("C:/tools/obit.exe"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	installed := read(t, f.hookPath)
	for _, text := range []string{"// keep this comment", "user-tool", "hook gemini", "mcpServers"} {
		if !strings.Contains(installed, text) {
			t.Fatal("lost setting", text)
		}
	}
	data, err := configJSON([]byte(installed), ConfigJSONC)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Hooks map[string][]struct {
			Matcher string           `json:"matcher"`
			Hooks   []map[string]any `json:"hooks"`
		} `json:"hooks"`
	}
	json.Unmarshal(data, &config)
	for _, event := range []string{"SessionStart", "AfterTool", "AfterAgent", "SessionEnd"} {
		found := false
		for _, group := range config.Hooks[event] {
			for _, handler := range group.Hooks {
				command, _ := handler["command"].(string)
				if strings.Contains(command, "hook gemini") {
					found = true
					if !strings.HasPrefix(command, "& ") || handler["timeout"] != float64(5000) {
						t.Fatal("wrong Windows hook/timeout", handler)
					}
					if event == "AfterTool" && group.Matcher != "write_file|replace" {
						t.Fatal("wrong write matcher")
					}
				}
			}
		}
		if !found {
			t.Fatal("missing event", event)
		}
	}
	if config.Hooks["BeforeTool"] != nil {
		t.Fatal("unknown shell guard installed")
	}
	plan, err = PlanGemini(f.scope, nativeJSONCommands("gemini", "tools"), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, f.hookPath), "mcpServers") {
		t.Fatal("hook removal deleted MCP")
	}
	plan, err = PlanGeminiMCP(f.scope, testenv.NativeFixture("C:/tools/obit.exe"), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, f.hookPath), "// keep this comment") {
		t.Fatal("extension removal deleted comment")
	}
}

func TestGeminiMCPPlanDoesNotOverwriteNewHookConfiguration(t *testing.T) {
	f := newNativeJSONInstallFixture(t, "gemini", false)
	writeIntegrationFile(t, f.hookPath, f.userConfig)
	stale, err := PlanGeminiMCP(f.scope, testenv.NativeFixture("C:/tools/obit.exe"), false)
	if err != nil {
		t.Fatal(err)
	}
	hooks, err := PlanGemini(f.scope, nativeJSONCommands("gemini", "tools"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(hooks); err != nil {
		t.Fatal(err)
	}
	before := read(t, f.hookPath)
	if err := Apply(stale); err == nil {
		t.Fatal("independent stale MCP plan overwrote hooks")
	}
	if read(t, f.hookPath) != before {
		t.Fatal("stale edit changed new hook config")
	}
}

func TestNativeMCPConfigBoundaries(t *testing.T) {
	for _, name := range []string{"copilot", "cursor", "gemini", "droid"} {
		t.Run(name, func(t *testing.T) {
			for _, invalid := range []string{"", "\xef\xbb\xbf", "{", "[]"} {
				f := newNativeJSONInstallFixture(t, name, false)
				writeIntegrationFile(t, f.mcpPath, invalid)
				if _, err := f.agent.PlanMCP(f.scope, testenv.NativeFixture("C:/tools/obit.exe"), false); err == nil {
					t.Fatal("invalid MCP accepted", invalid)
				}
				if read(t, f.mcpPath) != invalid {
					t.Fatal("invalid MCP changed")
				}
			}
			f := newNativeJSONInstallFixture(t, name, false)
			writeIntegrationFile(t, f.mcpPath, `{"mcpServers":{}}`)
			plan, err := f.agent.PlanMCP(f.scope, testenv.NativeFixture("C:/tools/obit.exe"), false)
			if err != nil {
				t.Fatal(err)
			}
			writeIntegrationFile(t, f.mcpPath, `{"userEdit":true}`)
			if err := Apply(plan); err == nil {
				t.Fatal("stale MCP applied")
			}
			if read(t, f.mcpPath) != `{"userEdit":true}` {
				t.Fatal("MCP race lost edit")
			}
		})
	}
}
