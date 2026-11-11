package graveyard

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	doctorcmd "github.com/woodfishhhh/obituary/internal/cli/doctor"
	"github.com/woodfishhhh/obituary/internal/core"
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
	for _, name := range []string{"path", "obitd", "runner", "claude", "claude-guard-rm", "codex", "codex-guard-rm", "mcp", "skill", "data", "store", "trust", "hook-log", "graveyard-recovery", "config", "codex-windows-shell"} {
		if got[name].Name == "" {
			t.Fatalf("missing %s check: %+v", name, checks)
		}
	}
	if len(got) != len(checks) {
		t.Fatalf("duplicate checks: %+v", checks)
	}
	return got, err
}

type failBurialCommitFS struct {
	core.FS
	blocked string
}

func (f *failBurialCommitFS) MovePinned(path string, want core.Identity, to string) error {
	if err := f.FS.MovePinned(path, want, to); err != nil {
		return err
	}
	if f.blocked != "" {
		blocked := f.blocked
		f.blocked = ""
		return os.Mkdir(blocked, 0o700) // only the following store commit fails
	}
	return nil
}
