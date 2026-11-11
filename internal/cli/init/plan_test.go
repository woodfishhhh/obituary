package init_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/config"
)

func TestInitDeclaredAgentGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range []string{"codebuddy", "codebuddy-ide", "minimax-code"} {
		t.Run(agent, func(t *testing.T) {
			_, _, executable := agentFixture(t)
			t.Setenv("CODEBUDDY_CODE_SHELL", "")
			t.Setenv("CODEBUDDY_CODE_SHELL_PREFIX", "")
			t.Setenv("CODEBUDDY_SANDBOX_IMAGE", "")
			path, err := config.Path()
			if err != nil {
				t.Fatal(err)
			}
			out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--guard-rm", "--yes")
			if err != nil || !strings.Contains(out, "agent_shells") {
				t.Fatalf("guard installation: %s %v", out, err)
			}
			if agent != "minimax-code" {
				for _, want := range []string{path, "[agent_shells]", agent, "\"bash\"", "\"powershell\"", "pass through"} {
					if !strings.Contains(out, want) {
						t.Fatalf("missing instruction %q: %s", want, out)
					}
				}
			}
			original := "# declared by owner\n[agent_shells]\n" + agent + " = 'bash'\n"
			clitest.WriteFixture(t, path, original)
			out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--guard-rm", "--show")
			if err != nil || !strings.Contains(out, "agent_shells."+agent+" = \"bash\"") {
				t.Fatalf("show declaration: %s %v", out, err)
			}
			out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--uninstall", "--yes")
			if agent == "codebuddy-ide" {
				if !errors.Is(err, cmdutil.ErrReported) || !strings.Contains(out, "CodeBuddy IDE/plugin MCP") || !strings.Contains(out, "mcpServers.obituary") {
					t.Fatalf("missing existing IDE MCP unregistration guidance: %s %v", out, err)
				}
			} else if err != nil {
				t.Fatal(out, err)
			}
			if data, readErr := os.ReadFile(path); readErr != nil || string(data) != original {
				t.Fatalf("uninstall modified declaration: %s %s %v %v", data, out, err, readErr)
			}
		})
	}
}

func TestInitUninstallKeepsOtherAgentMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, repo, executable := agentFixture(t)
	if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", "qoder", "--mcp", "--yes"); err != nil {
		t.Fatal(out, err)
	}
	if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", "copilot", "--uninstall", "--yes"); err != nil {
		t.Fatal(out, err)
	}
	got, err := os.ReadFile(filepath.Join(repo, ".mcp.json"))
	if err != nil || !strings.Contains(string(got), `"obituary"`) {
		t.Fatalf("Copilot uninstall deleted Qoder MCP: err=%v content=%s", err, got)
	}
}

func TestInitUninstallKeepsOtherAgentRules(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, repo, executable := agentFixture(t)
	if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", "codex", "--yes"); err != nil {
		t.Fatal(out, err)
	}
	out, _, uninstallErr := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", "hermes", "--uninstall", "--yes")
	got, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if err != nil || !strings.Contains(string(got), "obit:begin") {
		t.Fatalf("Hermes uninstall deleted Codex AGENTS: read=%v content=%s uninstall=%v output=%s", err, got, uninstallErr, out)
	}
}

func TestInitUninstallKeepsMalformedRuleExample(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, repo, executable := agentFixture(t)
	path := filepath.Join(repo, "AGENTS.md")
	original := "User policy\n<!-- obit:pi:begin --> is only an example\nKEEP MY POLICY\n<!-- obit:pi:end -->\n"
	clitest.WriteFixture(t, path, original)
	out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", "pi", "--uninstall", "--yes")
	got, readErr := os.ReadFile(path)
	if err == nil || readErr != nil || string(got) != original {
		t.Fatalf("CLI failed to reject malformed markers: err=%v read=%v content=%q output=%s", err, readErr, got, out)
	}
}

func TestInitUninstallHooksDespiteUnownedMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, repo, executable := agentFixture(t)
	if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", "gemini", "--yes"); err != nil {
		t.Fatal(out, err)
	}
	path := filepath.Join(repo, ".gemini", "settings.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(original), "{", `{"mcpServers":{"obituary":{"command":"user-server","args":[]}},`, 1)
	clitest.WriteFixture(t, path, text)
	out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", "gemini", "--uninstall", "--yes")
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(got), "hook gemini") {
		t.Fatalf("unrelated owned hooks could not uninstall: err=%v output=%s content=%s", err, out, got)
	}
	if !strings.Contains(string(got), "user-server") {
		t.Fatal("unowned server lost")
	}
	if err == nil {
		t.Fatal("unowned MCP conflict was not reported")
	}
}

func TestInitSharedOwnersCanInstallTogether(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agents := range []string{"codebuddy,codebuddy-ide", "qoder,qoder-cn"} {
		t.Run(agents, func(t *testing.T) {
			_, _, executable := agentFixture(t)
			if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agents, "--yes"); err != nil {
				t.Fatalf("CLI rejects supported shared owners: %v %s", err, out)
			}
		})
	}
}

func TestInitMultipleAgentsComposeRules(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, repo, executable := agentFixture(t)
	path := filepath.Join(repo, "AGENTS.md")
	user := "User instructions stay here.\r\n"
	clitest.WriteFixture(t, path, user)
	args := []string{"--agent", "pi,omp", "--agent", "opencode", "--agent", "pi", "--mcp", "--yes"}
	before := clitest.TreeSnapshot(t, repo)
	if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), append(args, "--dry-run")...); err != nil {
		t.Fatal(out, err)
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, repo)) {
		t.Fatal("multi-agent dry-run wrote files")
	}
	for i := 0; i < 2; i++ {
		if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), args...); err != nil {
			t.Fatal(out, err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(data), user) {
		t.Fatalf("user text lost: %s %v", data, err)
	}
	for _, name := range []string{"pi", "omp", "opencode"} {
		if strings.Count(string(data), "<!-- obit:"+name+":begin -->") != 1 {
			t.Fatalf("lost or duplicate %s rules: %s", name, data)
		}
	}
	if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", "pi", "--uninstall", "--yes"); err != nil {
		t.Fatal(out, err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "obit:pi:begin") || !strings.Contains(string(data), "obit:omp:begin") {
		t.Fatalf("single uninstall affected other agent: %s", data)
	}
	if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", "omp,opencode", "--uninstall", "--yes"); err != nil {
		t.Fatal(out, err)
	}
	data, _ = os.ReadFile(path)
	if !strings.HasPrefix(string(data), user) || strings.Contains(string(data), "obit:") {
		t.Fatalf("uninstall lost user text or left rules: %s", data)
	}
}

func TestInitDefaultStillOnlyClaude(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root, repo, executable := agentFixture(t)
	beforeRoot, beforeRepo := clitest.TreeSnapshot(t, root), clitest.TreeSnapshot(t, repo)
	defaultOut, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	claudeOut, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", "claude", "--dry-run")
	if err != nil || defaultOut != claudeOut {
		t.Fatalf("default behavior changed: %s / %s %v", defaultOut, claudeOut, err)
	}
	if !reflect.DeepEqual(beforeRoot, clitest.TreeSnapshot(t, root)) || !reflect.DeepEqual(beforeRepo, clitest.TreeSnapshot(t, repo)) {
		t.Fatal("dry-run changed defaults")
	}
	if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--yes"); err != nil || !strings.Contains(out, "restart Claude Code") {
		t.Fatal(out, err)
	}
	entries, err := os.ReadDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != ".git" && entry.Name() != ".claude" && entry.Name() != "CLAUDE.md" &&
			entry.Name() != "CLAUDE.md.obituary-reference" && entry.Name() != "CLAUDE.md.obituary-reference.obituary-asset" &&
			entry.Name() != "CLAUDE.md.obituary-reference.lock" && entry.Name() != "CLAUDE.md.obituary-reference.obituary-asset.bak" {
			t.Fatalf("default created extra agent target: %s", entry.Name())
		}
	}
}

func TestInitUnsupportedFlagsAndConflictsAreReadOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root, repo, executable := agentFixture(t)
	for _, args := range [][]string{
		{"--agent", "unknown"}, {"--agent="},
		{"--agent", "claude,roo", "--guard-rm"},
		{"--agent", "pi", "--skill"},
		{"--agent", "trae,trae-cn"},
	} {
		beforeRoot, beforeRepo := clitest.TreeSnapshot(t, root), clitest.TreeSnapshot(t, repo)
		out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), append(args, "--yes")...)
		if err == nil || strings.Contains(out, "installed; restart") {
			t.Fatalf("unsupported request succeeded: %v %s", args, out)
		}
		if !reflect.DeepEqual(beforeRoot, clitest.TreeSnapshot(t, root)) || !reflect.DeepEqual(beforeRepo, clitest.TreeSnapshot(t, repo)) {
			t.Fatalf("invalid request partly applied: %v", args)
		}
	}
}

func TestInitManualStepsKeepAutomaticWorkAndHostState(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root, repo, executable := agentFixture(t)
	config := filepath.Join(root, "openclaw", "openclaw.json")
	original := "// Gateway state\n{plugins: {allow: ['user']}}\n"
	clitest.WriteFixture(t, config, original)
	out, stderr, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", "openclaw,gemini", "--mcp", "--yes")
	if !errors.Is(err, cmdutil.ErrReported) || stderr != "" {
		t.Fatalf("manual setup was treated as a crash: %s %s %v", out, stderr, err)
	}
	for _, step := range []string{"plugins.entries.obituary.enabled=true", "plugins.allow", "mcp.servers.obituary", "OBIT_OPENCLAW_CWD", "未完成，需要手动步骤"} {
		if !strings.Contains(out, step) {
			t.Fatalf("missing complete step %q: %s", step, out)
		}
	}
	data, _ := os.ReadFile(config)
	if string(data) != original {
		t.Fatal("manual Gateway state overwritten")
	}
	for _, path := range []string{filepath.Join(repo, ".openclaw", "extensions", "obituary", "index.ts"), filepath.Join(repo, ".gemini", "settings.json")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("automatic work missing: %s %v", path, err)
		}
	}
	checks, err := doctorChecks(t, executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"openclaw", "gemini", "windsurf", "cursor"} {
		if check, exists := checks[agent]; exists {
			t.Fatalf("agent without global installation was diagnosed: %+v", check)
		}
	}
	out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--show", "--agent", "openclaw")
	if err != nil || !strings.Contains(out, "需要手动步骤") || !strings.Contains(out, "Next:") {
		t.Fatalf("project manual status: %s %v", out, err)
	}
}
