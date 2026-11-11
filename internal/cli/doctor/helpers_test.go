package doctor

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
)

// Any accidental registry mutation panics through the nil embedded interface.
type readOnlyMenuRegistry struct {
	contextmenu.Registry
	trees map[string]contextmenu.Tree
}

func (r readOnlyMenuRegistry) ReadTree(path string) (contextmenu.Tree, error) {
	return r.trees[path], nil
}

func doctorAgentFixture(t *testing.T) (string, cmdutil.Executable) {
	t.Helper()
	root, executable := clitest.DoctorFixture(t)
	for key, sub := range map[string]string{
		"GEMINI_CLI_HOME": "home", "OPENCODE_CONFIG_DIR": "opencode",
		"OPENCLAW_STATE_DIR": "openclaw", "OPENCLAW_HOME": "home",
	} {
		t.Setenv(key, filepath.Join(root, sub))
	}
	return root, executable
}

func doctorChecks(t *testing.T, executable cmdutil.Executable, agents ...string) (map[string]DoctorCheck, error) {
	t.Helper()
	if agents == nil {
		agents = []string{"claude", "codex"}
	}
	out, stderr, err := clitest.Execute(t, newCmdWithAgents(executable, agents), "--json")
	if stderr != "" {
		t.Fatalf("doctor JSON stderr: %s", stderr)
	}
	var checks []DoctorCheck
	if jerr := json.Unmarshal([]byte(out), &checks); jerr != nil {
		t.Fatalf("doctor JSON: %v: %s", jerr, out)
	}
	got := map[string]DoctorCheck{}
	for _, c := range checks {
		got[c.Name] = c
	}
	for _, name := range []string{"path", "obitd", "gui", "registry-transactions", "runner", "tray-startup", "claude", "claude-guard-rm", "codex", "codex-guard-rm", "mcp", "skill", "context-menu", "data", "store", "watch", "explorer", "trust", "hook-log", "graveyard-recovery", "slow-git", "config", "codex-windows-shell"} {
		if got[name].Name == "" {
			t.Fatalf("missing %s check: %+v", name, checks)
		}
	}
	if len(got) != len(checks) {
		t.Fatalf("duplicate checks: %+v", checks)
	}
	return got, err
}
