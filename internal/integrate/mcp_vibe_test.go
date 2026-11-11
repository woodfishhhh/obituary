package integrate

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVibeMCPLifecycle(t *testing.T) {
	for _, global := range []bool{false, true} {
		s := Scope{Project: t.TempDir(), Global: global}
		t.Setenv("VIBE_HOME", t.TempDir())
		root, _ := vibeRoot(s)
		path := filepath.Join(root, "config.toml")
		input := "\xef\xbb\xbf# model settings\nmodel='keep'\n[[mcp_servers]]\nname='user'\ncommand='echo user' # user comment\n"
		writeIntegrationFile(t, path, input)
		plan, err := PlanVibeMCP(s, "C:/space dir/obit.exe", false)
		if err != nil || read(t, path) != input {
			t.Fatal("dry-run", err)
		}
		applyIntegration(t, plan, nil)
		got := strings.Replace(read(t, path), "# obit:vibe-mcp:end", "env = { USER = 'keep' } # env comment\n# obit:vibe-mcp:end", 1)
		writeIntegrationFile(t, path, got)
		plan, err = PlanVibeMCP(s, "D:/new location/obit.exe", false)
		applyIntegration(t, plan, err)
		got = read(t, path)
		if !strings.HasPrefix(got, input) || !strings.Contains(got, "# env comment") || !strings.Contains(got, "USER = 'keep'") || !strings.Contains(got, "D:/new location/obit.exe") {
			t.Fatal("refresh lost user fields", got)
		}
		plan, err = PlanVibeMCP(s, "D:/new location/obit.exe", false)
		if err != nil || Pending(plan) {
			t.Fatal("not idempotent", err)
		}
		plan, err = PlanVibeMCP(s, "", true)
		applyIntegration(t, plan, err)
		if strings.TrimSpace(read(t, path)) != strings.TrimSpace(input) {
			t.Fatal("MCP user state changed")
		}
		plan, err = PlanVibeMCP(s, "", true)
		if err != nil || Pending(plan) {
			t.Fatal("repeat uninstall", err)
		}
	}
}

func TestVibeMCPRejectsUnownedAndChangedConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, input := range []string{"broken=[", "mcp_servers={}", "[[mcp_servers]]\nname='obituary'\ncommand='user'\n", "# obit:vibe-mcp:begin\n[[mcp_servers]]\nname='obituary'\ncommand='evil'\nargs=['mcp']\n# obit:vibe-mcp:end\n"} {
		s := Scope{Project: t.TempDir()}
		path := filepath.Join(s.Project, ".vibe", "config.toml")
		writeIntegrationFile(t, path, input)
		for _, uninstall := range []bool{false, true} {
			if _, err := PlanVibeMCP(s, "C:/tools/obit.exe", uninstall); err == nil {
				t.Fatal("invalid MCP accepted", input)
			}
		}
		if read(t, path) != input {
			t.Fatal("modified invalid input")
		}
	}
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, ".vibe", "config.toml")
	writeIntegrationFile(t, path, "")
	plan, err := PlanVibeMCP(s, "C:/tools/obit.exe", false)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, path, "# concurrent edit\n")
	if Apply(plan) == nil || read(t, path) != "# concurrent edit\n" {
		t.Fatal("concurrent edit lost")
	}
}
