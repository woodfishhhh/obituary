package init

import (
	"errors"
	"fmt"
	"html"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

func validateShowFormat(cmd *cobra.Command, format string, show bool) error {
	if format != "text" && format != "markdown" {
		return fmt.Errorf("unknown format %q (supported: text, markdown)", format)
	}
	if cmd.Flags().Changed("format") && !show {
		return errors.New("--format requires --show")
	}
	return nil
}

// Build Markdown from the command's existing metadata to keep this static
// report consistent with the CLI without running installation plans.
func coverageMarkdown(names []string) string {
	var out strings.Builder
	out.WriteString("<!-- 由 obit init --show --format markdown 生成，不要手改。 -->\n\n# Agent 覆盖表\n")
	agents := integrate.Agents()
	for _, method := range []integrate.IntegrationMethod{integrate.NativeHooks, integrate.Plugin, integrate.RulesAndMCP} {
		var rows []string
		for _, agent := range agents {
			if agent.Method == method && slices.Contains(names, agent.Name) {
				rows = append(rows, coverageRow(agent))
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&out, "\n## %s\n\n", method)
		out.WriteString("| agent | 写文件后登记 | 会话提示 | 删除护栏 | MCP | Skill |\n|---|---|---|---|---|---|\n")
		out.WriteString(strings.Join(rows, "\n") + "\n")
	}
	return out.String()
}

func coverageRow(agent integrate.Agent) string {
	c := agent.Capabilities
	fields := []string{markdownCell(agent.Name)}
	// Context describes delivery to the agent; SessionStart alone only promises
	// a lifecycle callback and can take a snapshot without displaying a notice.
	for _, support := range []hook.Support{c.WriteObservation, c.Context, c.DeleteGuard, c.MCP, c.Skill} {
		fields = append(fields, coverageSupport(support, c.Target))
	}
	return "| " + strings.Join(fields, " | ") + " |"
}

func coverageSupport(support hook.Support, target hook.Target) string {
	level := "不支持"
	switch support.Level {
	case hook.Supported:
		level = "支持"
	case hook.Limited:
		level = "有限"
	}
	parts := []string{level}
	if support.Detail != "" {
		parts = append(parts, markdownCell(support.Detail))
	}
	if support.Target != (hook.Target{}) {
		target = support.Target
	}
	if text := targetText(target); text != "" {
		parts = append(parts, markdownCell(text))
	}
	return strings.Join(parts, "<br>")
}

func markdownCell(text string) string {
	return strings.NewReplacer("\\", "\\\\", "|", "\\|", "\r\n", "<br>", "\r", "<br>", "\n", "<br>").Replace(html.EscapeString(text))
}
