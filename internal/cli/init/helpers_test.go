package init_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	doctorcmd "github.com/woodfishhhh/obituary/internal/cli/doctor"
)

func doctorChecks(t *testing.T, executable cmdutil.Executable) (map[string]doctorcmd.DoctorCheck, error) {
	t.Helper()
	out, stderr, err := clitest.Execute(t, doctorcmd.NewCmdWithExecutable(executable, "claude", "codex"), "--json")
	if stderr != "" {
		t.Fatalf("doctor JSON stderr: %s", stderr)
	}
	var checks []doctorcmd.DoctorCheck
	if jerr := json.Unmarshal([]byte(out), &checks); jerr != nil {
		t.Fatalf("doctor JSON: %v: %s", jerr, out)
	}
	got := map[string]doctorcmd.DoctorCheck{}
	for _, c := range checks {
		got[c.Name] = c
	}
	for _, name := range []string{"path", "obitd", "runner", "claude", "claude-guard-rm", "codex", "codex-guard-rm", "mcp", "skill", "context-menu", "data", "store", "trust", "hook-log", "graveyard-recovery", "config", "codex-windows-shell"} {
		if got[name].Name == "" {
			t.Fatalf("missing %s check: %+v", name, checks)
		}
	}
	if len(got) != len(checks) {
		t.Fatalf("duplicate checks: %+v", checks)
	}
	return got, err
}

func agentFixture(t *testing.T) (string, string, cmdutil.Executable) {
	t.Helper()
	repo, _ := clitest.CommandRepo(t)
	root, executable := clitest.DoctorFixture(t)
	for name, sub := range map[string]string{
		"VIBE_HOME": "vibe", "KIMI_SHARE_DIR": "kimi",
		"COPILOT_HOME": "copilot", "GEMINI_CLI_HOME": "gemini-home", "FACTORY_HOME_OVERRIDE": "factory-home",
		"HERMES_HOME": "hermes", "CLINE_DIR": "cline", "CLINE_DATA_DIR": "cline-data", "KILO_CONFIG_DIR": "kilo",
		"OPENCODE_CONFIG_DIR": "opencode", "OPENCLAW_STATE_DIR": "openclaw", "OPENCLAW_HOME": "openclaw-home",
		"PI_CODING_AGENT_DIR": "pi", "XDG_CONFIG_HOME": "xdg-config", "CODEBUDDY_CONFIG_DIR": "codebuddy",
	} {
		t.Setenv(name, filepath.Join(root, sub))
	}
	return root, repo, executable
}
