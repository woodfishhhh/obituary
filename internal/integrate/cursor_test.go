package integrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCursorInstallerLifecycle(t *testing.T) { testNativeJSONInstallerLifecycle(t, "cursor") }
func TestCursorInstallerBoundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testNativeJSONInstallerBoundaries(t, "cursor")
}
func TestCursorInstallerExtensions(t *testing.T) { testNativeJSONExtensions(t, "cursor") }

func TestCursorInstallsOnlyEffectiveCallbacks(t *testing.T) {
	f := newNativeJSONInstallFixture(t, "cursor", false)
	plan, err := PlanCursor(f.scope, nativeJSONCommands("cursor", "tools"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	var config struct {
		Hooks map[string][]map[string]any `json:"hooks"`
	}
	json.Unmarshal([]byte(read(t, f.hookPath)), &config)
	if len(config.Hooks) != 3 {
		t.Fatal("unsupported callback installed", config.Hooks)
	}
	for _, event := range []string{"sessionStart", "stop", "sessionEnd"} {
		entries := config.Hooks[event]
		if len(entries) != 1 {
			t.Fatal("missing lifecycle hook", event)
		}
		command, _ := entries[0]["command"].(string)
		if !strings.HasPrefix(command, "& ") || entries[0]["failClosed"] != false || entries[0]["timeout"] != float64(5) {
			t.Fatal("incorrect Windows hook", entries)
		}
	}
	rule := read(t, f.rulesPath)
	if !strings.HasPrefix(rule, "---\n") || !strings.Contains(rule, "alwaysApply: true") || !strings.Contains(rule, Awareness) {
		t.Fatal("invalid MDC rule", rule)
	}
	f.scope.GuardRM = true
	if _, err := PlanCursor(f.scope, nativeJSONCommands("cursor", "tools"), false); err == nil {
		t.Fatal("unsupported guard claimed installed")
	}
}

func TestCursorEditedRuleIsPreservedButHooksAreRemoved(t *testing.T) {
	f := newNativeJSONInstallFixture(t, "cursor", false)
	commands := nativeJSONCommands("cursor", "tools")
	plan, err := PlanCursor(f.scope, commands, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, f.rulesPath, "user-edited rule")
	if _, err := PlanCursor(f.scope, commands, false); err == nil {
		t.Fatal("edited rule overwritten")
	}
	plan, err = PlanCursor(f.scope, commands, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err == nil {
		t.Fatal("edited rule reported removed")
	}
	if read(t, f.rulesPath) != "user-edited rule" || strings.Contains(read(t, f.hookPath), "hook cursor") {
		t.Fatal("incorrect partial uninstall")
	}
}

func TestNativeVersionedHooksRejectOtherVersions(t *testing.T) {
	for _, name := range []string{"copilot", "cursor"} {
		t.Run(name, func(t *testing.T) {
			f := newNativeJSONInstallFixture(t, name, false)
			old := `{"version":2,"hooks":{}}`
			writeIntegrationFile(t, f.hookPath, old)
			if _, err := f.agent.Plan(f.scope, nativeJSONCommands(name, "tools"), false); err == nil {
				t.Fatal("unsupported version edited")
			}
			if read(t, f.hookPath) != old {
				t.Fatal("version changed")
			}
		})
	}
}

func TestCursorGlobalRulesRequireExplicitManualStep(t *testing.T) {
	f := newNativeJSONInstallFixture(t, "cursor", true)
	plan, err := PlanCursor(f.scope, nativeJSONCommands("cursor", "tools"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := PreservedError(plan); err == nil || !strings.Contains(err.Error(), "Customize > Rules > User Rules") {
		t.Fatal("manual setup not surfaced", err)
	}
	if _, err := os.Stat(filepath.Join(f.dir, "rules")); !os.IsNotExist(err) {
		t.Fatal("invented global filesystem rules", err)
	}
}
