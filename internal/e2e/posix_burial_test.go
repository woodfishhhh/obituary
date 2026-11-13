//go:build linux || darwin

package e2e_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	doctorcmd "github.com/woodfishhhh/obituary/internal/cli/doctor"
	graveyardcmd "github.com/woodfishhhh/obituary/internal/cli/graveyard"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestPOSIXQuarantineDiagnostics(t *testing.T) {
	clitest.IsolateRunner(t)
	root, store := clitest.CommandDir(t)
	path := filepath.Join(root, "payload")
	clitest.WriteFixture(t, path, "keep")
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := svc.Bury([]string{path}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury: %+v %v", rows, err)
	}
	detail := "identity mismatch; never purge; content retained at " + rows[0].Burial.Grave + "; inspect both locations and manually move the retained object to an unused name"
	if err := store.UpdateGraveyard(func(st *core.State) error { st.Burials[0].IdentityMismatch = detail; return nil }); err != nil {
		t.Fatal(err)
	}
	out, _, err := clitest.Execute(t, graveyardcmd.NewCmd(), []string{}...)
	if err != nil || !strings.Contains(out, "never (identity mismatch)") || !strings.Contains(out, detail) {
		t.Fatalf("quarantine hidden from graveyard: %q %v", out, err)
	}
	self := cmdutil.Executable(func() (string, error) { return filepath.Join(root, "missing-obit"), nil })
	out, _, _ = clitest.Execute(t, doctorcmd.NewCmdWithExecutable(self, "claude"), "--json")
	var checks []doctorcmd.DoctorCheck
	if err := json.Unmarshal([]byte(out), &checks); err != nil {
		t.Fatal(out, err)
	}
	seen := map[string]bool{}
	for _, check := range checks {
		switch check.Name {
		case "graveyard-recovery":
			seen[check.Name] = check.Status == "WARN" && strings.Contains(check.Detail, detail)
		case "automatic-burial":
			seen[check.Name] = check.Status == "INFO" && strings.Contains(check.Detail, "no-replace rename") && !strings.Contains(check.Detail, "resurrection are unsupported")
		}
	}
	if !seen["graveyard-recovery"] || !seen["automatic-burial"] {
		t.Fatalf("POSIX doctor hid burial or quarantine: %s", out)
	}
}
