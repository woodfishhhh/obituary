package integrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPBareCommandCaseLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, command := range []string{"OBIT.EXE", "Obit"} {
		for _, agent := range []string{"claude", "codex"} {
			t.Run(agent+command, func(t *testing.T) {
				scope := Scope{Project: t.TempDir()}
				plan, err := PlanMCP(scope, agent, command, false)
				if err != nil {
					t.Fatalf("bare command casing rejected: %v", err)
				}
				applyIntegration(t, plan, nil)
				status, err := InspectExtension(scope, agent, "mcp", command)
				if err != nil || !status.Installed || !status.Current {
					t.Fatalf("bare command casing not diagnosed: %+v %v", status, err)
				}
				plan, err = PlanMCP(scope, agent, command, true)
				applyIntegration(t, plan, err)
				status, err = InspectExtension(scope, agent, "mcp", command)
				if err != nil || status.Installed {
					t.Fatalf("bare command casing not removed: %+v %v", status, err)
				}
			})
		}
	}
	for _, command := range []string{filepath.Join("relative", "OBIT.EXE"), `C:OBIT.EXE`, `.\Obit`} {
		if _, err := PlanMCP(Scope{Project: t.TempDir()}, "claude", command, false); err == nil {
			t.Fatalf("relative command accepted: %s", command)
		}
	}
}

func TestMCPJSONPreservesUserServers(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, entry := range []string{
		`{"command":"lint && obit","args":["mcp"]}`,
		`{"command":"obit","args":["mcp","extra"]}`,
		`{"command":"../obit.exe","args":["mcp"]}`,
		`{"type":"http","url":"https://example.test","command":"obit","args":["mcp"]}`,
	} {
		scope := Scope{Project: t.TempDir()}
		path := filepath.Join(scope.Project, ".mcp.json")
		original := `{"mcpServers":{"obituary":` + entry + `,"user":{"command":"keep"}},"secret":"preserve"}`
		writeIntegrationFile(t, path, original)
		if _, err := PlanMCP(scope, "claude", "obit", false); err == nil {
			t.Fatal("claimed user entry", entry)
		}
		plan, err := PlanMCP(scope, "claude", "obit", true)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(plan); err == nil {
			t.Fatal("unowned MCP conflict not reported")
		}
		if read(t, path) != original {
			t.Fatal("uninstall changed user entry", entry)
		}
	}
}

func TestMCPInstallLifecycle(t *testing.T) {
	for _, agent := range []string{"claude", "codex"} {
		for _, global := range []bool{false, true} {
			if agent == "claude" && global {
				continue
			} // covered with the official CLI fixture
			t.Run(agent+map[bool]string{true: "-global", false: "-project"}[global], func(t *testing.T) {
				root := t.TempDir()
				t.Setenv("OBIT_USER_HOME", root)
				t.Setenv("CLAUDE_CONFIG_DIR", "")
				t.Setenv("CODEX_HOME", filepath.Join(root, ".codex"))
				scope := Scope{Global: global, Project: root}
				plan, err := PlanMCP(scope, agent, "C:/old/obit.exe", false)
				if err != nil || !Pending(plan) {
					t.Fatalf("plan: %v %v", plan, err)
				}
				path := plan[0].Path
				original := "\xef\xbb\xbf" + "# user formatting\r\nmodel = 'test'\r\n\r\n[mcp_servers.other]\r\ncommand = 'other' # retain this\r\n"
				if agent == "claude" {
					original = "\xef\xbb\xbf" + `{"theme":"dark","mcpServers":{"other":{"command":"user && helper","args":["x"]}},"projects":{"private":{}}}`
				}
				writeIntegrationFile(t, path, original)
				plan, err = PlanMCP(scope, agent, "C:/old/obit.exe", false)
				if err != nil {
					t.Fatal(err)
				}
				if read(t, path) != original {
					t.Fatal("planning wrote configuration")
				}
				applyIntegration(t, plan, err)
				if read(t, path+".bak") != original || !strings.HasPrefix(read(t, path), "\xef\xbb\xbf") {
					t.Fatal("original backup or BOM lost")
				}
				plan, err = PlanMCP(scope, agent, "C:/old/obit.exe", false)
				if err != nil || Pending(plan) {
					t.Fatalf("not idempotent: %+v %v", plan, err)
				}
				plan, err = PlanMCP(scope, agent, "C:/new path/obit.exe", false)
				applyIntegration(t, plan, err)
				if !strings.Contains(read(t, path), "C:/new path/obit.exe") {
					t.Fatal("binary path not refreshed")
				}
				if _, err := os.Stat(path + ".bak.1"); err != nil {
					t.Fatal("numbered backup missing", err)
				}
				plan, err = PlanMCP(scope, agent, "C:/new path/obit.exe", true)
				applyIntegration(t, plan, err)
				got := read(t, path)
				if agent == "codex" && got != original {
					t.Fatalf("TOML user bytes changed: %q != %q", got, original)
				}
				if agent == "claude" && !sameJSON(bytes.TrimPrefix([]byte(got), []byte("\xef\xbb\xbf")), bytes.TrimPrefix([]byte(original), []byte("\xef\xbb\xbf"))) {
					t.Fatal("JSON user entries changed", got)
				}
			})
		}
	}
}

func TestMCPRejectsConcurrentEditAndLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			scope := Scope{Project: t.TempDir()}
			plan, err := PlanMCP(scope, agent, "obit", false)
			if err != nil {
				t.Fatal(err)
			}
			path := plan[0].Path
			writeIntegrationFile(t, path, "user saved this")
			if err := Apply(plan); err == nil || read(t, path) != "user saved this" {
				t.Fatal("concurrent edit was not preserved")
			}
			// A project file planted as a link must be rejected before planning.
			other := filepath.Join(t.TempDir(), "config.toml")
			content := "# valid external TOML\n"
			if agent == "claude" {
				content = "{}\n"
			}
			writeIntegrationFile(t, other, content)
			linked := Scope{Project: t.TempDir()}
			plan, err = PlanMCP(linked, agent, "obit", false)
			if err != nil {
				t.Fatal(err)
			}
			if agent == "codex" {
				// The linked parent contains a readable, valid config file. Without
				// checkInside, planning succeeds; ReadFile cannot mask this guard.
				makeIntegrationJunction(t, filepath.Dir(plan[0].Path), filepath.Dir(other))
			} else if err := os.Symlink(other, plan[0].Path); err != nil {
				t.Skipf("file symlink unavailable: %v", err)
			}
			if _, err := PlanMCP(linked, agent, "obit", false); err == nil {
				t.Fatal("project link accepted")
			}
			if err := Apply(plan); err == nil || read(t, other) != content {
				t.Fatal("link planted after planning accepted")
			}
		})
	}
}
