package integrate

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestKimiMCPLifecycleAndProjectManualSteps(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KIMI_SHARE_DIR", root)
	s := Scope{Global: true}
	path := filepath.Join(root, "mcp.json")
	input := "\xef\xbb\xbf" + `{"theme":"keep","mcpServers":{"other":{"command":"user"}}}`
	writeIntegrationFile(t, path, input)
	plan, err := PlanKimiMCP(s, "C:/space dir/obit.exe", false)
	if err != nil || read(t, path) != input {
		t.Fatal("dry-run", err)
	}
	applyIntegration(t, plan, nil)
	writeIntegrationFile(t, path, strings.Replace(read(t, path), `"args": [`, `"env": {"USER":"keep"}, "args": [`, 1))
	plan, err = PlanKimiMCP(s, "D:/new dir/obit.exe", false)
	applyIntegration(t, plan, err)
	if !strings.Contains(read(t, path), `"USER"`) || !strings.Contains(read(t, path), "D:/new dir/obit.exe") {
		t.Fatal("refresh lost user fields")
	}
	plan, err = PlanKimiMCP(s, "D:/new dir/obit.exe", false)
	if err != nil || Pending(plan) {
		t.Fatal("not idempotent", err)
	}
	plan, err = PlanKimiMCP(s, "", true)
	applyIntegration(t, plan, err)
	if strings.Contains(read(t, path), "obituary") || !strings.Contains(read(t, path), `"other"`) {
		t.Fatal("uninstall lost user fields")
	}
	for _, uninstall := range []bool{false, true} {
		if _, err := PlanKimiMCP(Scope{Project: t.TempDir()}, "C:/tools/obit.exe", uninstall); err == nil || !strings.Contains(err.Error(), "--mcp-config-file") {
			t.Fatal("missing manual project instructions", err)
		}
	}
	writeIntegrationFile(t, path, `{"mcpServers":{"obituary":{"command":"user","args":[]}}}`)
	if _, err := PlanKimiMCP(s, "C:/tools/obit.exe", false); err == nil {
		t.Fatal("claimed user MCP entry")
	}
	plan, err = PlanKimiMCP(s, "", true)
	if err != nil || Apply(plan) == nil || !strings.Contains(read(t, path), `"command":"user"`) {
		t.Fatal("unowned MCP not preserved", err)
	}
}
