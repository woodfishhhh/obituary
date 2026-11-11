package init_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
)

func TestAgentCoverageMarkdownMatchesDocumentation(t *testing.T) {
	const regenerate = "obit init --show --format markdown > docs/agents.md"
	want, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "agents.md"))
	if err != nil {
		t.Fatalf("read generated agent coverage: %v; regenerate with: %s", err, regenerate)
	}
	// Static coverage also works outside a project and with no usable config.
	root := t.TempDir()
	t.Chdir(root)
	config := filepath.Join(root, "not-a-config-directory")
	if err := os.WriteFile(config, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OBIT_CONFIG_DIR", config)
	got, err := runCoverage(t)
	if err != nil {
		t.Fatal(err)
	}
	// Git may check Markdown out with CRLF on Windows; content must match.
	if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
		t.Fatalf("docs/agents.md is out of date; regenerate with: %s", regenerate)
	}
	for _, name := range initcmd.AgentNames() {
		if strings.Count(got, "| "+name+" |") != 1 {
			t.Errorf("coverage must contain exactly one row for %s", name)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("static coverage wrote files: %v %v", entries, err)
	}
}

func TestAgentCoverageMarkdownFiltersAndGroups(t *testing.T) {
	got, err := runCoverage(t, "--agent", "roo,opencode,codex,roo,kimi")
	if err != nil {
		t.Fatal(err)
	}
	previous := -1
	for _, text := range []string{"## 原生 hook", "| codex | 支持 | 支持<br>", "| kimi | 不支持<br>", "## 插件", "| opencode | 有限<br>", "## 只有规则和 MCP", "| roo | 不支持<br>"} {
		position := strings.Index(got, text)
		if position <= previous {
			t.Fatalf("missing or unordered %q: %s", text, got)
		}
		previous = position
	}
	for _, want := range []string{
		"Beta runtime context delivery unverified; no notices acknowledged",
		"Native JSONC mcp command array; ambiguous layers require manual selection",
		"managed Skill discovery has not been verified",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("capability detail missing: %q", want)
		}
	}
	if strings.Count(got, "| roo |") != 1 || strings.Contains(got, "| claude |") {
		t.Fatal("agent filter ignored or duplicated rows", got)
	}
	one, err := runCoverage(t, "--agent", "zcode")
	if err != nil || strings.Contains(one, "## 插件") || !strings.Contains(one, "&lt;project&gt;") || !strings.Contains(one, "a9fe401eede6e6e5e01be9048679c206fe6f02d9") {
		t.Fatalf("empty groups, literal text or target qualification: %s %v", one, err)
	}
}

func TestAgentCoverageMarkdownRejectsInvalidFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--format", "markdown"}, {"--show", "--format", "json"},
		{"--show", "--format", "markdown", "--context-menu"},
		{"--show", "--format", "markdown", "--agent", "unknown"},
		{"--show", "--format", "markdown", "--agent", ""},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out bytes.Buffer
			cmd := initcmd.NewCmd()
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil || out.Len() != 0 {
				t.Fatalf("invalid flags accepted or wrote output: %s %v", &out, err)
			}
		})
	}
}

func runCoverage(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := initcmd.NewCmdWithExecutable(func() (string, error) {
		t.Fatal("static coverage resolved the executable")
		return "", nil
	})
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--show", "--format", "markdown"}, args...))
	err := cmd.Execute()
	return out.String(), err
}
