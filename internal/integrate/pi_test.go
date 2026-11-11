package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiInstallationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	testPluginLifecycle(t, "pi")
}
func TestPiPluginProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	runPluginNodeTests(t, "pi")
}

func TestPiWrittenPathMatchesHostNormalization(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, agent := range []string{"pi", "omp"} {
		for _, names := range [][2]string{{"@debug.log", "debug.log"}, {"debug\u00a0copy.log", "debug copy.log"}} {
			if agent == "omp" && names[0] == "@debug.log" {
				continue // OMP 21f1188c preserves a literal @name.
			}
			for _, tool := range []string{"write", "edit"} {
				t.Run(agent+"/"+tool+"/"+names[0], func(t *testing.T) {
					root, e := pluginEngine(t, true)
					neighbor, actual := filepath.Join(root, names[0]), filepath.Join(root, names[1])
					writeIntegrationFile(t, neighbor, "untouched")
					writeIntegrationFile(t, actual, "host wrote here")
					pluginRoundTrip(t, agent, root, e, map[string]any{"tool": tool, "path": names[0]})
					pluginSweepKeepsNeighbor(t, e, neighbor)
					if _, err := os.Stat(actual); !os.IsNotExist(err) {
						t.Fatalf("actual written target not observed: %v", err)
					}
				})
			}
		}
	}
}

func TestPiMCPConfigurationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, agent := range []string{"pi", "omp"} {
		for _, global := range []bool{true, false} {
			t.Run(agent+map[bool]string{true: "/global", false: "/project"}[global], func(t *testing.T) {
				s, dir := pluginScope(t, agent, global)
				a, _ := LookupAgent(agent)
				path := filepath.Join(dir, "mcp.json")
				writeIntegrationFile(t, path, "\xef\xbb\xbf"+`{"first":1,"mcpServers":{"user":{"command":"keep"}},"last":2}`)
				plan, err := a.PlanMCP(s, `C:\tools\obit.exe`, false)
				if !strings.Contains(read(t, path), `"last":2`) {
					t.Fatal("dry-run wrote")
				}
				applyIntegration(t, plan, err)
				got := read(t, path)
				if !strings.HasPrefix(got, "\xef\xbb\xbf") || strings.Index(got, "first") > strings.Index(got, "last") || !strings.Contains(got, "keep") {
					t.Fatalf("config preservation %s", got)
				}
				plan, err = a.PlanMCP(s, `C:\tools\obit.exe`, false)
				if err != nil || Pending(plan) {
					t.Fatalf("not idempotent: %v", err)
				}
				plan, err = a.PlanMCP(s, `C:\new\obit.exe`, false)
				applyIntegration(t, plan, err)
				if !strings.Contains(read(t, path), `new`) {
					t.Fatal("did not refresh")
				}
				plan, err = a.PlanMCP(s, `C:\new\obit.exe`, true)
				applyIntegration(t, plan, err)
				if strings.Contains(read(t, path), `obituary`) || !strings.Contains(read(t, path), "keep") {
					t.Fatal("uninstall entry")
				}
				plan, err = a.PlanMCP(s, `C:\new\obit.exe`, true)
				if err != nil || Pending(plan) || PreservedError(plan) != nil {
					t.Fatalf("repeat uninstall: %v %v", err, PreservedError(plan))
				}
			})
		}
	}
}

func TestPiOMPSharedOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	s, dir := pluginScope(t, "pi", true)
	cmd := Commands{Executable: `C:\tools\obit.exe`}
	for _, agent := range []string{"pi", "omp"} {
		a, _ := LookupAgent(agent)
		plan, err := a.Plan(s, cmd, false)
		applyIntegration(t, plan, err)
		plan, err = a.PlanMCP(s, cmd.Executable, false)
		applyIntegration(t, plan, err)
	}
	plan, err := PlanPi(s, cmd, true)
	applyIntegration(t, plan, err)
	plan, err = PlanPiMCP(s, cmd.Executable, true)
	applyIntegration(t, plan, err)
	if !strings.Contains(read(t, filepath.Join(dir, "mcp.json")), "obituary") {
		t.Fatal("Pi removed OMP MCP")
	}
	if _, err := os.Stat(filepath.Join(dir, "extensions", "obituary.ts")); err != nil {
		t.Fatal("Pi removed OMP extension", err)
	}
	if !strings.Contains(read(t, filepath.Join(dir, "AGENTS.md")), "obit:omp") {
		t.Fatal("Pi removed OMP rules")
	}
	plan, err = PlanOMP(s, Commands{Executable: `C:\new\obit.exe`}, false)
	applyIntegration(t, plan, err)
	plan, err = PlanOMP(s, cmd, true)
	applyIntegration(t, plan, err)
	plan, err = PlanOMPMCP(s, cmd.Executable, true)
	applyIntegration(t, plan, err)
	if _, err := os.Stat(filepath.Join(dir, "extensions", "obituary.ts")); !os.IsNotExist(err) {
		t.Fatal("last owner plugin remains")
	}
	if strings.Contains(read(t, filepath.Join(dir, "mcp.json")), "obituary") {
		t.Fatal("last owner MCP remains")
	}
}

func TestPiMCPRejectsMalformedAndChangedConfig(t *testing.T) {
	s, dir := pluginScope(t, "pi", false)
	path := filepath.Join(dir, "mcp.json")
	for _, bad := range []string{`{broken`, `{"mcpServers":[]}`, `{"mcpServers":{"obituary":{"command":"user.exe","args":["mcp"]}}}`} {
		writeIntegrationFile(t, path, bad)
		if _, err := PlanPiMCP(s, `C:\tools\obit.exe`, false); err == nil {
			t.Fatalf("accepted %s", bad)
		}
		if read(t, path) != bad {
			t.Fatal("changed bad config")
		}
	}
	writeIntegrationFile(t, path, "")
	plan, err := PlanPiMCP(s, `C:\tools\obit.exe`, false)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, path, `{"saved":true}`)
	if Apply(plan) == nil {
		t.Fatal("accepted stale MCP plan")
	}
	plan, err = PlanPiMCP(s, `C:\tools\obit.exe`, false)
	applyIntegration(t, plan, err)
}
