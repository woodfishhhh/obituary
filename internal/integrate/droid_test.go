package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDroidInstallerLifecycle(t *testing.T) { testNativeJSONInstallerLifecycle(t, "droid") }
func TestDroidInstallerBoundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testNativeJSONInstallerBoundaries(t, "droid")
}
func TestDroidInstallerExtensions(t *testing.T) { testNativeJSONExtensions(t, "droid") }

func TestDroidPreservesExistingLegacyHookLocations(t *testing.T) {
	for _, target := range []string{"legacy", "settings"} {
		t.Run(target, func(t *testing.T) {
			f := newNativeJSONInstallFixture(t, "droid", false)
			path := filepath.Join(f.dir, "hooks", "hooks.json")
			original := `{"Notification":[{"hooks":[{"type":"command","command":"user-tool"}]}]}`
			if target == "settings" {
				path = filepath.Join(f.dir, "settings.json")
				original = `{"theme":"dark","hooks":{"Notification":[{"hooks":[{"type":"command","command":"user-tool"}]}]}}`
			}
			writeIntegrationFile(t, path, original)
			commands := nativeJSONCommands("droid", "tools")
			plan, err := PlanDroid(f.scope, commands, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(plan); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(f.hookPath); !os.IsNotExist(err) {
				t.Fatal("new canonical file shadows old hooks", err)
			}
			installed := read(t, path)
			if !strings.Contains(installed, "user-tool") || !strings.Contains(installed, "hook droid") || strings.Contains(installed, "& ") {
				t.Fatal("incorrect native cmd hook", installed)
			}
			plan, err = PlanDroid(f.scope, commands, false)
			if err != nil || Pending(plan) {
				t.Fatal("legacy install not idempotent", err)
			}
			plan, err = PlanDroid(f.scope, commands, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(plan); err != nil {
				t.Fatal(err)
			}
			removed := read(t, path)
			if strings.Contains(removed, "hook droid") || !strings.Contains(removed, "user-tool") {
				t.Fatal("legacy uninstall changed user", removed)
			}
		})
	}
}

func TestDroidMixedLoaderConfigurationNeedsManualResolution(t *testing.T) {
	f := newNativeJSONInstallFixture(t, "droid", false)
	settings := filepath.Join(f.dir, "settings.json")
	root := testenv.NativeFixture(`{"SessionStart":[{"hooks":[{"command":"C:/tools/obit.exe hook droid"},{"command":"root-user"}]}]}`)
	nested := testenv.NativeFixture(`{"hooks":{"Stop":[{"hooks":[{"command":"C:/tools/obit.exe hook droid"},{"command":"settings-user"}]}]}}`)
	writeIntegrationFile(t, f.hookPath, root)
	writeIntegrationFile(t, settings, nested)
	commands := nativeJSONCommands("droid", "tools")
	if _, err := PlanDroid(f.scope, commands, false); err == nil || !strings.Contains(err.Error(), "consolidate") {
		t.Fatal("ambiguous hooks claimed installed", err)
	}
	if read(t, f.hookPath) != root || read(t, settings) != nested {
		t.Fatal("manual-required inputs changed")
	}
	plan, err := PlanDroid(f.scope, commands, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{f.hookPath, settings} {
		if strings.Contains(read(t, path), "hook droid") || !strings.Contains(read(t, path), "-user") {
			t.Fatal("uninstall left owned entry or lost user")
		}
	}
}

func TestDroidSelectionIsRecheckedBeforeApply(t *testing.T) {
	f := newNativeJSONInstallFixture(t, "droid", false)
	legacy := filepath.Join(f.dir, "hooks", "hooks.json")
	writeIntegrationFile(t, legacy, `{"Stop":[]}`)
	plan, err := PlanDroid(f.scope, nativeJSONCommands("droid", "tools"), false)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, f.hookPath, `{"userCreated":true}`)
	if err := Apply(plan); err == nil {
		t.Fatal("new canonical config silently shadowed installation")
	}
	if read(t, legacy) != `{"Stop":[]}` {
		t.Fatal("legacy changed despite preflight rejection")
	}
}

func TestNativeUnsupportedGuardOptionsDoNotInstall(t *testing.T) {
	for _, name := range []string{"cursor", "gemini", "droid"} {
		t.Run(name, func(t *testing.T) {
			f := newNativeJSONInstallFixture(t, name, false)
			f.scope.GuardRM = true
			if _, err := f.agent.Plan(f.scope, nativeJSONCommands(name, "tools"), false); err == nil || !strings.Contains(err.Error(), "unsupported") {
				t.Fatal("unsupported guard not reported", err)
			}
			if _, err := os.Stat(f.hookPath); !os.IsNotExist(err) {
				t.Fatal("unsupported guard wrote config")
			}
		})
	}
}
