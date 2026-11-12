package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeSharedMCPKeepsHooksAndReceiptInOneBatch(t *testing.T) {
	for agent, env := range map[string]string{"qoder": "QODER_CONFIG_DIR", "qoder-cn": "QODERCN_CONFIG_DIR"} {
		t.Run(agent, func(t *testing.T) {
			t.Setenv(env, t.TempDir())
			s := Scope{Global: true}
			a, _ := LookupAgent(agent)
			for _, uninstall := range []bool{false, true} {
				plan, err := a.Plan(s, ownershipCommands(agent), uninstall)
				if err != nil {
					t.Fatal(err)
				}
				mcp, err := a.PlanMCP(s, testenv.NativeFixture("C:/tools/obit.exe"), uninstall)
				if err != nil {
					t.Fatal(err)
				}
				for i := range mcp {
					mcp[i].Extension = "mcp"
				}
				plan, err = ComposeAgentChanges(append(plan, mcp...))
				if err != nil {
					t.Fatal(err)
				}
				for _, group := range []string{"", "mcp"} {
					var batch []Change
					for _, ch := range plan {
						if ch.Extension == group {
							batch = append(batch, ch)
						}
					}
					if err := Apply(batch); err != nil {
						t.Fatalf("composed shared resource split across batches: %v", err)
					}
				}
			}
		})
	}
}

func TestComposeSharedOwnersInstallTogether(t *testing.T) {
	t.Parallel()
	for _, agents := range [][2]string{{"codebuddy", "codebuddy-ide"}, {"qoder", "qoder-cn"}} {
		t.Run(strings.Join(agents[:], "+"), func(t *testing.T) {
			t.Parallel()
			s := Scope{Project: t.TempDir()}
			planTogether := func(uninstall bool) []Change {
				t.Helper()
				var plans []Change
				for _, agent := range agents {
					p, err := PlanAgent(agent, s, ownershipCommands(agent), uninstall)
					if err != nil {
						t.Fatal(err)
					}
					plans = append(plans, p...)
				}
				joined, err := ComposeAgentChanges(plans)
				if err != nil {
					t.Fatalf("compatible shared-owner agents cannot install together: %v", err)
				}
				return joined
			}
			first := planTogether(false)
			afterIntegrationWrite := func(path string) error { return errors.New("interrupted after first receipt") }
			err := apply(first, afterIntegrationWrite)
			if err == nil {
				t.Fatal("expected installation interruption")
			}
			applyIntegration(t, planTogether(false), nil)
			path := filepath.Join(s.Project, "."+agents[0], "settings.json")
			installed := read(t, path)
			for i, agent := range agents {
				plan, err := PlanAgent(agent, s, ownershipCommands(agent), true)
				applyIntegration(t, plan, err)
				if i == 0 && read(t, path) != installed {
					t.Fatal("first owner uninstall removed shared hooks")
				}
			}
			if strings.Contains(read(t, path), "hook "+agents[0]) {
				t.Fatal("last owner left shared hooks")
			}
			applyIntegration(t, planTogether(false), nil)
			applyIntegration(t, planTogether(true), nil)
			if strings.Contains(read(t, path), "hook "+agents[0]) {
				t.Fatal("combined uninstall left shared hooks")
			}
		})
	}
}
