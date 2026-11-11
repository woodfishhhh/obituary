package integrate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/hook"
)

type workBuddyFixture struct {
	scope Scope
	root  string
	agent Agent
}

func newWorkBuddyFixture(t *testing.T, global bool) workBuddyFixture {
	t.Helper()
	home := filepath.Join(t.TempDir(), "用户 home")
	project := filepath.Join(t.TempDir(), "办公 project")
	for _, dir := range []string{home, project} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("OBIT_USER_HOME", home)
	root := filepath.Join(project, ".workbuddy")
	if global {
		root = filepath.Join(home, ".workbuddy")
	}
	agent, ok := LookupAgent("workbuddy")
	if !ok {
		t.Fatal("missing WorkBuddy installer")
	}
	return workBuddyFixture{Scope{Global: global, Project: project}, root, agent}
}

func workBuddyPlan(t *testing.T, f workBuddyFixture, executable string, uninstall bool) []Change {
	t.Helper()
	plan, err := f.agent.Plan(f.scope, Commands{Executable: executable}, uninstall)
	if err != nil {
		t.Fatal(err)
	}
	mcp, err := f.agent.PlanMCP(f.scope, executable, uninstall)
	if err != nil {
		t.Fatal(err)
	}
	skill, err := f.agent.PlanSkill(f.scope, uninstall)
	if err != nil {
		t.Fatal(err)
	}
	return append(append(plan, mcp...), skill...)
}

func TestWorkBuddyInstallationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, global := range []bool{false, true} {
		for _, bom := range []string{"", "\xef\xbb\xbf"} {
			name := "project"
			if global {
				name = "user"
			}
			if bom != "" {
				name += "-BOM"
			}
			t.Run(name, func(t *testing.T) {
				f := newWorkBuddyFixture(t, global)
				path := filepath.Join(f.root, "mcp.json")
				export := filepath.Join(f.root, "obituary-export")
				original := bom + `{"first":"<keep>&","mcpServers":{"other":{"command":"user-tool","args":[]}},"last":7}`
				writeIntegrationFile(t, path, original)
				var untouched []string
				for _, root := range []string{filepath.Dir(f.root), f.scope.Project} {
					for _, leaf := range []string{".workbuddy-ai", ".codebuddy"} {
						sentinel := filepath.Join(root, leaf, "mcp.json")
						writeIntegrationFile(t, sentinel, "user-owned "+leaf)
						untouched = append(untouched, sentinel)
					}
				}
				executable := `C:\工具 first\obit.exe`
				plan := workBuddyPlan(t, f, executable, false)
				if !Pending(plan) || read(t, path) != original {
					t.Fatal("planning changed configuration or produced no changes")
				}
				for _, p := range []string{export, path + ".bak"} {
					if _, err := os.Stat(p); !os.IsNotExist(err) {
						t.Fatalf("dry run created %s: %v", p, err)
					}
				}
				if err := Apply(plan); err != nil {
					t.Fatal(err)
				}
				installed := read(t, path)
				if strings.HasPrefix(installed, "\xef\xbb\xbf") != (bom != "") || !strings.Contains(installed, "<keep>&") || strings.Index(installed, `"first"`) > strings.Index(installed, `"mcpServers"`) || strings.Index(installed, `"last"`) < strings.Index(installed, `"mcpServers"`) {
					t.Fatal("BOM, literal text or user key order changed", installed)
				}
				if read(t, path+".bak") != original || read(t, filepath.Join(export, "SKILL.md")) != string(skillContent) {
					t.Fatal("backup or existing skill bytes changed")
				}
				if !strings.Contains(read(t, filepath.Join(export, "SETUP.md")), "没有插件 manifest") || !json.Valid([]byte(read(t, filepath.Join(export, "sources.json")))) {
					t.Fatal("export is missing its limits or sources")
				}
				// User fields on the owned entry must survive a later executable refresh.
				installed = strings.Replace(installed, `"obituary": {`, `"obituary": {"env":{"USER_SETTING":"keep"},"disabled":true,`, 1)
				writeIntegrationFile(t, path, installed)
				plan = workBuddyPlan(t, f, executable, false)
				if Pending(plan) {
					t.Fatal("repeat installation is not idempotent", plan)
				}
				if err := Apply(plan); err != nil || read(t, path) != installed {
					t.Fatal("idempotent apply changed bytes", err)
				}
				executable = `C:\工具 next\obit.exe`
				plan = workBuddyPlan(t, f, executable, false)
				if !Pending(plan) {
					t.Fatal("executable change was not detected")
				}
				if err := Apply(plan); err != nil {
					t.Fatal(err)
				}
				var config struct {
					Servers map[string]struct {
						Command  string            `json:"command"`
						Args     []string          `json:"args"`
						Env      map[string]string `json:"env"`
						Disabled bool              `json:"disabled"`
					} `json:"mcpServers"`
				}
				if err := json.Unmarshal(bytes.TrimPrefix([]byte(read(t, path)), []byte("\xef\xbb\xbf")), &config); err != nil {
					t.Fatal(err)
				}
				entry := config.Servers["obituary"]
				if entry.Command != executable || len(entry.Args) != 1 || entry.Args[0] != "mcp" || entry.Env["USER_SETTING"] != "keep" || !entry.Disabled || config.Servers["other"].Command != "user-tool" {
					t.Fatal("MCP refresh lost user data or argv", config)
				}
				writeIntegrationFile(t, filepath.Join(export, "user-note.txt"), "keep")
				f.scope.GuardRM = true // Unsupported guard must still permit removal.
				if err := Apply(workBuddyPlan(t, f, "", true)); err != nil {
					t.Fatal(err)
				}
				remaining := read(t, path)
				if strings.Contains(remaining, `"obituary"`) || !strings.Contains(remaining, `"other"`) || strings.HasPrefix(remaining, "\xef\xbb\xbf") != (bom != "") || read(t, path+".bak") != original {
					t.Fatal("uninstall damaged user MCP settings or first backup", remaining)
				}
				for _, name := range []string{"SKILL.md", "SETUP.md", "sources.json"} {
					for _, p := range []string{filepath.Join(export, name), ManagedAssetReceiptPath(filepath.Join(export, name))} {
						if _, err := os.Stat(p); !os.IsNotExist(err) {
							t.Fatalf("owned export remains: %s %v", p, err)
						}
					}
				}
				if read(t, filepath.Join(export, "user-note.txt")) != "keep" || Pending(workBuddyPlan(t, f, "", true)) {
					t.Fatal("uninstall changed user export files or is not idempotent")
				}
				for _, p := range untouched {
					if read(t, p) != "user-owned "+filepath.Base(filepath.Dir(p)) {
						t.Fatal("another edition or CodeBuddy configuration changed", p)
					}
				}
			})
		}
	}
}

func TestWorkBuddyCapabilitiesAndExportScope(t *testing.T) {
	f := newWorkBuddyFixture(t, false)
	caps := f.agent.Capabilities
	for _, support := range []hook.Support{caps.DeleteGuard, caps.WriteObservation, caps.SessionStart, caps.Stop, caps.SessionEnd, caps.Context} {
		if support.Level != hook.Unsupported || support.Detail == "" {
			t.Fatal("unverified callback advertised", support)
		}
	}
	if caps.MCP.Level != hook.Supported || caps.Skill.Level != hook.Limited || !strings.Contains(caps.MCP.Detail, "international") || !strings.Contains(caps.MCP.Detail, "cloud") {
		t.Fatal("edition or capability limit missing", caps)
	}
	if _, ok := hook.Lookup("workbuddy"); ok {
		t.Fatal("consumer hook adapter registered without a verified protocol")
	}
	f.scope.GuardRM = true
	if changes, err := f.agent.Plan(f.scope, Commands{}, false); err == nil || len(changes) != 0 || !strings.Contains(err.Error(), "配置 MCP") {
		t.Fatal("unsupported guard needs explicit manual guidance", changes, err)
	}
	f.scope.GuardRM = false
	for _, change := range workBuddyPlan(t, f, `C:\tools\obit.exe`, false) {
		rel, err := filepath.Rel(f.root, change.Path)
		if err != nil || strings.HasPrefix(rel, "..") || strings.Contains(rel, "plugin.json") || strings.Contains(rel, "hooks") || strings.Contains(rel, "settings") || change.Detail == "" {
			t.Fatal("unverified activation or out-of-scope write", change)
		}
	}
	for _, scope := range []Scope{{}, {Project: "relative"}} {
		if _, err := PlanWorkBuddy(scope, Commands{}, false); err == nil {
			t.Fatal("relative project accepted")
		}
		if _, err := PlanWorkBuddyMCP(scope, `C:\tools\obit.exe`, false); err == nil {
			t.Fatal("relative MCP project accepted")
		}
		if _, err := PlanWorkBuddySkill(scope, false); err == nil {
			t.Fatal("relative skill project accepted")
		}
	}
	for _, exe := range []string{"obit.exe", "relative/obit.exe", `C:\tools\other.exe`} {
		if _, err := PlanWorkBuddyMCP(f.scope, exe, false); err == nil {
			t.Fatal("unfixed or unowned executable accepted", exe)
		}
	}
}

func TestWorkBuddyMCPRejectsInvalidConfiguration(t *testing.T) {
	for _, content := range []string{"", " \r\n", "\xef\xbb\xbf", "{broken", "null", "[]", `{"mcpServers":[]}`, `{"mcpServers":{"obituary":{"command":"user.exe","args":["mcp"]}}}`, "{\n// user comment\n\"mcpServers\":{}\n}"} {
		t.Run(content, func(t *testing.T) {
			f := newWorkBuddyFixture(t, false)
			path := filepath.Join(f.root, "mcp.json")
			writeIntegrationFile(t, path, content)
			if _, err := PlanWorkBuddyMCP(f.scope, `C:\tools\obit.exe`, false); err == nil {
				t.Fatal("invalid config accepted")
			}
			changes, err := PlanWorkBuddyMCP(f.scope, "", true)
			if err == nil && PreservedError(changes) == nil {
				t.Fatal("invalid or unowned config uninstall reported success", changes)
			}
			if read(t, path) != content {
				t.Fatal("invalid config or its comments changed")
			}
		})
	}
}

func TestWorkBuddyMCPFreshInstall(t *testing.T) {
	f := newWorkBuddyFixture(t, false)
	changes, err := PlanWorkBuddyMCP(f.scope, `C:\tools\obit.exe`, false)
	applyIntegration(t, changes, err)
	data := read(t, filepath.Join(f.root, "mcp.json"))
	if strings.HasPrefix(data, "\xef\xbb\xbf") || !strings.Contains(data, `"obituary"`) || strings.Contains(data, `"type"`) {
		t.Fatal("unexpected fresh MCP schema", data)
	}
}

func TestWorkBuddyPreservesEditedAndUnownedExports(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, filename := range []string{"SETUP.md", "sources.json", "SKILL.md"} {
		for _, owned := range []bool{false, true} {
			t.Run(filename+map[bool]string{false: "-unowned", true: "-edited"}[owned], func(t *testing.T) {
				f := newWorkBuddyFixture(t, false)
				path := filepath.Join(f.root, "obituary-export", filename)
				if owned {
					if err := Apply(workBuddyPlan(t, f, `C:\tools\obit.exe`, false)); err != nil {
						t.Fatal(err)
					}
				}
				writeIntegrationFile(t, path, "user content")
				planner := func(uninstall bool) ([]Change, error) { return PlanWorkBuddy(f.scope, Commands{}, uninstall) }
				if filename == "SKILL.md" {
					planner = func(uninstall bool) ([]Change, error) { return PlanWorkBuddySkill(f.scope, uninstall) }
				}
				if _, err := planner(false); err == nil {
					t.Fatal("user export overwritten")
				}
				changes, err := planner(true)
				if err != nil || PreservedError(changes) == nil {
					t.Fatal("user export should be explicitly preserved", err)
				}
				if err := Apply(changes); err == nil || read(t, path) != "user content" {
					t.Fatal("user export not retained", err)
				}
			})
		}
	}
}

func TestWorkBuddyRefusesChangedPlan(t *testing.T) {
	for _, name := range []string{"mcp.json", filepath.Join("obituary-export", "SKILL.md")} {
		t.Run(name, func(t *testing.T) {
			f := newWorkBuddyFixture(t, false)
			plan := workBuddyPlan(t, f, `C:\tools\obit.exe`, false)
			path := filepath.Join(f.root, name)
			writeIntegrationFile(t, path, "concurrent user edit")
			if err := Apply(plan); err == nil {
				t.Fatal("stale plan applied")
			}
			if read(t, path) != "concurrent user edit" {
				t.Fatal("concurrent edit replaced")
			}
			if _, err := os.Stat(filepath.Join(f.root, "obituary-export", "SETUP.md")); !os.IsNotExist(err) {
				t.Fatal("preflight wrote another file", err)
			}
		})
	}
}

func TestWorkBuddyRefusesJunctions(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, global := range []bool{false, true} {
		for _, link := range []string{"root", "export", "late-root"} {
			t.Run(map[bool]string{false: "project-", true: "user-"}[global]+link, func(t *testing.T) {
				f := newWorkBuddyFixture(t, global)
				outside := t.TempDir()
				writeIntegrationFile(t, filepath.Join(outside, "mcp.json"), `{"keep":true}`)
				var plan []Change
				if link == "late-root" {
					plan = workBuddyPlan(t, f, `C:\tools\obit.exe`, false)
				}
				path := f.root
				if link == "export" {
					path = filepath.Join(f.root, "obituary-export")
					if err := os.MkdirAll(f.root, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				makeIntegrationJunction(t, path, outside)
				if link == "late-root" {
					if err := Apply(plan); err == nil {
						t.Fatal("late junction accepted")
					}
				} else {
					for _, uninstall := range []bool{false, true} {
						if _, err := PlanWorkBuddy(f.scope, Commands{}, uninstall); err == nil {
							t.Fatal("export junction accepted")
						}
						if _, err := PlanWorkBuddySkill(f.scope, uninstall); err == nil {
							t.Fatal("skill junction accepted")
						}
						if link == "root" {
							if _, err := PlanWorkBuddyMCP(f.scope, `C:\tools\obit.exe`, uninstall); err == nil {
								t.Fatal("MCP junction accepted")
							}
						}
					}
				}
				entries, err := os.ReadDir(outside)
				if err != nil || len(entries) != 1 || read(t, filepath.Join(outside, "mcp.json")) != `{"keep":true}` {
					t.Fatal("outside directory changed", err, entries)
				}
			})
		}
	}
}
