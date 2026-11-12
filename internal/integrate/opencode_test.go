package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeInstallationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	testPluginLifecycle(t, "opencode")
}
func TestOpenCodePluginProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	runPluginNodeTests(t, "opencode")
}

func TestOpenCodeMCPLifecycle(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{true: "global", false: "project"}[global], func(t *testing.T) {
			s, dir := pluginScope(t, "opencode", global)
			path := filepath.Join(dir, "opencode.jsonc")
			original := "\xef\xbb\xbf{\n // user comment\n \"first\":1,\n \"mcp\":{\"user\":{\"type\":\"remote\",\"url\":\"https://example.test\"}},\n \"last\":2,\n}\n"
			writeIntegrationFile(t, path, original)
			plan, err := PlanOpenCodeMCP(s, testenv.NativeFixture(`C:\Program Files\obit.exe`), false)
			if read(t, path) != original {
				t.Fatal("dry-run wrote")
			}
			applyIntegration(t, plan, err)
			got := read(t, path)
			if !strings.Contains(got, "// user comment") || !strings.HasPrefix(got, "\xef\xbb\xbf") || strings.Index(got, "first") > strings.Index(got, "last") {
				t.Fatalf("lost formatting: %s", got)
			}
			plan, err = PlanOpenCodeMCP(s, testenv.NativeFixture(`C:\Program Files\obit.exe`), false)
			if err != nil || Pending(plan) {
				t.Fatalf("not idempotent %v", err)
			}
			plan, err = PlanOpenCodeMCP(s, testenv.NativeFixture(`C:\new\obit.exe`), false)
			applyIntegration(t, plan, err)
			if !strings.Contains(read(t, path), "new") {
				t.Fatal("path stale")
			}
			plan, err = PlanOpenCodeMCP(s, testenv.NativeFixture(`C:\new\obit.exe`), true)
			applyIntegration(t, plan, err)
			if strings.Contains(read(t, path), "obituary") || !strings.Contains(read(t, path), "example.test") || !strings.Contains(read(t, path), "// user comment") {
				t.Fatal("uninstall lost user data")
			}
		})
	}
}

func TestOpenCodeMCPInvalidAndAmbiguous(t *testing.T) {
	s, dir := pluginScope(t, "opencode", false)
	path := filepath.Join(dir, "opencode.jsonc")
	writeIntegrationFile(t, path, `{bad`)
	if _, err := PlanOpenCodeMCP(s, testenv.NativeFixture(`C:\tools\obit.exe`), false); err == nil {
		t.Fatal("accepted bad JSONC")
	}
	writeIntegrationFile(t, path, "")
	plan, err := PlanOpenCodeMCP(s, testenv.NativeFixture(`C:\tools\obit.exe`), false)
	applyIntegration(t, plan, err)
	writeIntegrationFile(t, filepath.Join(s.Project, "opencode.json"), `{}`)
	if _, err := PlanOpenCodeMCP(s, testenv.NativeFixture(`C:\tools\obit.exe`), false); err == nil {
		t.Fatal("accepted ambiguous layers")
	}
}

func TestOpenCodeSkillTargets(t *testing.T) {
	checkAgentSkillTargets(t, "opencode")
}

func TestOpenCodeSkillSharedOwners(t *testing.T) {
	checkSharedSkillOwners(t, Scope{Project: t.TempDir()}, "codex", "opencode")
}
