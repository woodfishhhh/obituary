//go:build windows

package e2e_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

type agentLayout struct {
	files, absent, steps []string
	seeds                map[string]string
	sentinel             string
	exit                 int
}

// These discovery targets are pinned independently of the planners, following
// each host's source references in internal/hook/testdata/*-sources.json and
// internal/integrate/plugins. A new registry entry must supply its host contract.
func lifecycleLayout(t *testing.T, agent, root, repo string, global bool) agentLayout {
	t.Helper()
	l := agentLayout{seeds: map[string]string{}}
	user := filepath.Join(root, "user")
	dir := filepath.Join(repo, "."+agent)
	if global {
		dir = filepath.Join(root, agent)
	}
	add := func(base string, names ...string) {
		for _, name := range names {
			l.files = append(l.files, filepath.Join(base, name))
		}
	}
	jsonSeed := func(path string) {
		l.seeds[path] = "{\n  \"userSetting\": {\"text\": \"user-sentinel 中文 <>&\", \"enabled\": false, \"nested\": [1, 2]}\n}\n"
	}
	manual := func(steps ...string) { l.exit, l.steps = 1, append(l.steps, steps...) }
	skill := "skills/obituary/SKILL.md"
	switch agent {
	case "claude":
		add(dir, "settings.json", "OBIT.md", skill)
		jsonSeed(filepath.Join(dir, "settings.json"))
		if global {
			add(dir, "CLAUDE.md", ".claude.json")
			jsonSeed(filepath.Join(dir, ".claude.json"))
		} else {
			add(repo, "CLAUDE.md", ".mcp.json")
			jsonSeed(filepath.Join(repo, ".mcp.json"))
		}
	case "codex":
		add(dir, "hooks.json", "config.toml")
		jsonSeed(filepath.Join(dir, "hooks.json"))
		l.seeds[filepath.Join(dir, "config.toml")] = "# user-sentinel 中文\nmodel = 'user-sentinel'\n"
		if global {
			add(dir, "AGENTS.md")
			add(user, ".agents/skills/obituary/SKILL.md")
		} else {
			add(repo, "AGENTS.md", ".agents/skills/obituary/SKILL.md")
		}
	case "copilot":
		if !global {
			dir = filepath.Join(repo, ".github")
		}
		add(dir, "hooks/obituary.json", "copilot-instructions.md", skill)
		mcp := filepath.Join(repo, ".mcp.json")
		if global {
			mcp = filepath.Join(dir, "mcp-config.json")
		}
		l.files = append(l.files, mcp)
		jsonSeed(mcp)
		l.seeds[mcp] = strings.Replace(l.seeds[mcp], "{\n", "{\n  \"mcpServers\": {\"user-sentinel\": {\"command\": \"user-server\", \"args\": []}},\n", 1)
	case "cursor":
		if global {
			dir = filepath.Join(user, ".cursor")
			manual("Customize > Rules > User Rules")
			l.absent = append(l.absent, filepath.Join(dir, "rules"))
		} else {
			add(dir, "rules/obituary.mdc")
		}
		add(dir, "hooks.json", "mcp.json", skill)
		jsonSeed(filepath.Join(dir, "hooks.json"))
	case "gemini":
		if global {
			dir = filepath.Join(root, "gemini", ".gemini")
		}
		add(dir, "settings.json", skill)
		jsonSeed(filepath.Join(dir, "settings.json"))
		if global {
			add(dir, "GEMINI.md")
		} else {
			add(repo, "GEMINI.md")
		}
	case "droid":
		dir = filepath.Join(repo, ".factory")
		if global {
			dir = filepath.Join(root, "factory", ".factory")
		}
		add(dir, "hooks.json", "mcp.json", "AGENTS.md", skill)
		jsonSeed(filepath.Join(dir, "hooks.json"))
	case "antigravity":
		dir = filepath.Join(repo, ".agents")
		if global {
			dir = filepath.Join(user, ".gemini", "config")
		}
		add(dir, "plugins/obituary/hooks.json", "plugins/obituary/plugin.json", "mcp_config.json")
		jsonSeed(filepath.Join(dir, "mcp_config.json"))
	case "trae", "trae-cn":
		dir = filepath.Join(repo, ".trae")
		if global {
			dir = filepath.Join(user, "."+agent)
			manual("MCP", "mcpServers", "mcp")
			l.absent = append(l.absent, filepath.Join(dir, "mcp.json"))
		} else {
			add(dir, "mcp.json")
		}
		add(dir, "hooks.json")
		jsonSeed(filepath.Join(dir, "hooks.json"))
	case "vibe":
		add(dir, "hooks.toml", "config.toml")
		l.seeds[filepath.Join(dir, "hooks.toml")] = "# user-sentinel 中文\n"
	case "kimi":
		if global {
			add(dir, "config.toml", "mcp.json")
			add(user, ".kimi/skills/obituary/SKILL.md")
			l.seeds[filepath.Join(dir, "config.toml")] = "# user-sentinel 中文\n"
		} else {
			add(repo, "AGENTS.md")
			add(dir, skill)
			manual("--mcp-config-file", "mcpServers", "mcp")
			l.absent = append(l.absent, filepath.Join(dir, "config.toml"), filepath.Join(dir, "mcp.json"))
		}
	case "pi", "omp":
		if agent == "omp" && global {
			dir = filepath.Join(root, "pi") // PI_CODING_AGENT_DIR is shared by both hosts.
		}
		add(dir, "extensions/obituary.ts", "mcp.json")
		jsonSeed(filepath.Join(dir, "mcp.json"))
		l.steps = append(l.steps, "OBIT_"+strings.ToUpper(agent)+"_LOCAL=1", "SHELL=bash or powershell")
	case "opencode":
		add(dir, "plugins/obituary.ts")
		mcp := filepath.Join(repo, "opencode.json")
		if global {
			mcp = filepath.Join(dir, "opencode.json")
		}
		l.files = append(l.files, mcp)
		jsonSeed(mcp)
		l.steps = append(l.steps, "OBIT_OPENCODE_LOCAL=1", "SHELL=bash or powershell")
	case "openclaw":
		add(dir, "extensions/obituary/index.ts", "extensions/obituary/openclaw.plugin.json", "extensions/obituary/runtime.json")
		manual("plugins.entries.obituary.enabled=true", "plugins.allow", "openclaw plugins list", "mcp.servers.obituary", "OBIT_OPENCLAW_LOCAL=1", "OBIT_OPENCLAW_CWD")
		config := filepath.Join(root, "openclaw", "openclaw.json")
		l.seeds[config] = "// user-sentinel JSON5 gateway state\n{agents: {defaults: {name: 'user-sentinel'}}}\n"
	case "hermes":
		if global {
			add(dir, "plugins/obituary/__init__.py", "plugins/obituary/plugin.yaml", "config.yaml")
			l.seeds[filepath.Join(dir, "config.yaml")] = "# user-sentinel 中文\nuser_setting: user-sentinel\n"
		} else {
			add(repo, "AGENTS.md")
			manual("Hermes", "MCP")
			l.absent = append(l.absent, filepath.Join(dir, "plugins"), filepath.Join(dir, "config.yaml"))
		}
	case "cline":
		add(dir, "plugins/obituary.ts", "rules/obituary.md")
		if global {
			path := filepath.Join(root, "cline-data", "settings", "cline_mcp_settings.json")
			l.files = append(l.files, path)
			jsonSeed(path)
		} else {
			manual("Cline Settings > MCP Servers > Configure MCP Servers", "mcpServers", "mcp")
			l.absent = append(l.absent, filepath.Join(dir, "mcp.json"))
		}
		l.steps = append(l.steps, "OBIT_CLINE_LOCAL_SHELL")
	case "kilocode":
		dir = filepath.Join(repo, ".kilo")
		if global {
			dir = filepath.Join(root, "kilo")
		}
		add(dir, "plugin/obituary.ts", "kilo.json")
		jsonSeed(filepath.Join(dir, "kilo.json"))
		l.steps = append(l.steps, "OBIT_KILOCODE_LOCAL_SHELL")
	case "roo":
		if global {
			dir = filepath.Join(user, ".roo")
			manual("Edit Global MCP", "mcpServers", "mcp")
			l.absent = append(l.absent, filepath.Join(dir, "mcp.json"))
		} else {
			add(dir, "mcp.json")
			jsonSeed(filepath.Join(dir, "mcp.json"))
		}
		add(dir, "rules/obituary.md")
	case "windsurf":
		manual("Cascade > MCP Servers", "mcpServers", "mcp")
		if global {
			l.steps = append(l.steps, "Cascade > Customizations > Rules > Global", "obit:begin")
			l.absent = append(l.absent, filepath.Join(user, ".windsurf"))
		} else {
			add(repo, ".windsurfrules")
			l.seeds[filepath.Join(repo, ".windsurfrules")] = "# user-sentinel 中文\n"
		}
	case "codebuddy", "codebuddy-ide":
		dir = filepath.Join(repo, ".codebuddy")
		if global {
			dir = filepath.Join(root, "codebuddy")
			if agent == "codebuddy-ide" {
				dir = filepath.Join(user, ".codebuddy")
			}
		}
		add(dir, "settings.json", skill)
		jsonSeed(filepath.Join(dir, "settings.json"))
		if agent == "codebuddy-ide" {
			manual("Settings > MCP > configuration", "mcpServers.obituary", "mcp")
		} else if global {
			add(dir, ".mcp.json")
		} else {
			add(repo, ".mcp.json")
		}
	case "qoder", "qoder-cn", "lingma":
		if agent == "qoder-cn" && !global {
			dir = filepath.Join(repo, ".qoder")
		}
		if agent == "lingma" && global {
			dir = filepath.Join(user, ".lingma")
		}
		add(dir, "settings.json")
		jsonSeed(filepath.Join(dir, "settings.json"))
		if agent == "lingma" {
			manual("Personal Settings > MCP Servers", "mcpServers.obituary", "mcp")
			l.absent = append(l.absent, filepath.Join(dir, "mcp.json"))
		} else if global {
			// MCP and hooks share settings.json.
		} else {
			add(repo, ".mcp.json")
		}
	case "qoderwork":
		manual("QoderWork", "MCP/connector UI", "mcpServers", "mcp")
		l.absent = append(l.absent, filepath.Join(user, ".qoderwork"), filepath.Join(repo, ".qoderwork"))
	case "deepseek-harness":
		dir = filepath.Join(repo, ".dsh")
		if global {
			dir = filepath.Join(root, "deepseek")
		}
		add(dir, "obituary-hooks/hooks.json", "obituary-hooks/cordis.patch.yml", "obituary-mcp/cordis.patch.yml", skill)
		l.steps = append(l.steps, "dsh plugin --profile NAME add", "--patch", "OBIT_DEEPSEEK_LOCAL", "OBIT_DEEPSEEK_SHELL")
	case "minimax-code":
		if global {
			dir = filepath.Join(root, "minimax")
			add(dir, "plugins/obituary/hooks/hooks.json", "plugins/obituary-mcp/obituary.mcp.json", "plugins/obituary-skill/skills/obituary/SKILL.md")
			l.steps = append(l.steps, "mcode plugin marketplace upgrade", "mcode plugin enable", "mcode plugin list --json")
		} else {
			add(repo, "AGENTS.md", ".mcp.json")
			jsonSeed(filepath.Join(repo, ".mcp.json"))
			manual("Skill discovery is unverified", "MINIMAX_DATA_DIR")
			l.absent = append(l.absent, filepath.Join(repo, ".minimax", "plugins"))
		}
	case "workbuddy":
		if global {
			dir = filepath.Join(user, ".workbuddy")
		}
		add(dir, "mcp.json", "obituary-export/SETUP.md", "obituary-export/SKILL.md")
		jsonSeed(filepath.Join(dir, "mcp.json"))
		l.steps = append(l.steps, "插件 > MCP 服务器 > 配置 MCP", "技能 > 添加技能 > 上传技能", "cloud/remote")
		l.absent = append(l.absent, filepath.Join(dir, "hooks.json"), filepath.Join(dir, "skills"))
	case "zcode":
		config := filepath.Join(repo, "zcode.json")
		if global {
			dir = filepath.Join(user, ".zcode")
			config = filepath.Join(dir, "cli", "config.json")
		} else {
			l.steps = append(l.steps, "zcode hooks trust review", "zcode hooks trust grant", "--hook-digest", "zcode hooks trust status")
		}
		l.files = append(l.files, config)
		jsonSeed(config)
		add(dir, skill)
	default:
		t.Fatalf("new registered agent %q needs independently verified discovery paths", agent)
	}
	l.sentinel = filepath.Join(root, "user", "unrelated-user-settings.txt")
	l.seeds[l.sentinel] = "user-sentinel\x00\xff\r\n中文 unchanged\n"
	return l
}

func lifecycleSeed(t *testing.T, layout agentLayout) map[string]string {
	t.Helper()
	for path, data := range layout.seeds {
		// Seed a real unrelated callback alongside the settings/member sentinel.
		// This event is outside Obituary's registration inventory for every host.
		if json.Valid([]byte(data)) && (filepath.Base(path) == "settings.json" || filepath.Base(path) == "hooks.json") {
			data = strings.Replace(data, "{\n", "{\n  \"hooks\": {\"UserSentinel\": [{\"matcher\": \"user-sentinel\", \"hooks\": [{\"type\": \"command\", \"command\": \"echo user-sentinel\"}]}]},\n", 1)
		}
		layout.seeds[path] = data
		writeFixture(t, path, data)
	}
	return layout.seeds
}
