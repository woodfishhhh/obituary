import { agentData } from '../../../landing/src/agents-data';

// Defaults for the scope used by each guide, from internal/integrate and internal/userdirs.
type Setup = { paths: string[] | Record<string, string>; global?: boolean; flags?: string; variant?: string; manual?: boolean };
const setup: Record<string, Setup> = {
  claude: { paths: ['.claude/settings.json', 'CLAUDE.md', '.mcp.json'], flags: '--guard-rm' },
  codex: { paths: ['.codex/hooks.json', 'AGENTS.md', '.codex/config.toml'], flags: '--guard-rm' },
  copilot: { paths: ['.github/hooks/obituary.json', '.github/copilot-instructions.md', '.mcp.json'] },
  cursor: { paths: ['.cursor/hooks.json', '.cursor/rules/obituary.mdc', '.cursor/mcp.json'] },
  gemini: { paths: ['.gemini/settings.json', 'GEMINI.md'] },
  droid: { paths: ['.factory/hooks.json', '.factory/AGENTS.md', '.factory/mcp.json'] },
  trae: { paths: ['.trae/hooks.json', '.trae/mcp.json'], variant: 'trae-cn' },
  kimi: { paths: ['~/.kimi/config.toml', '~/.kimi/mcp.json'], global: true },
  vibe: { paths: ['.vibe/hooks.toml', '.vibe/config.toml', 'AGENTS.md'] },
  antigravity: { paths: ['.agents/plugins/obituary/', '.agents/mcp_config.json'] },
  codebuddy: { paths: ['.codebuddy/settings.json', 'CODEBUDDY.md', '.mcp.json'], variant: 'codebuddy-ide' },
  qoder: { paths: ['.qoder/settings.json', '.qoder/rules/obituary.md', '.mcp.json'], variant: 'qoder-cn' },
  lingma: { paths: ['.lingma/settings.json', '.lingma/rules/obituary.md'] },
  zcode: { paths: ['zcode.json', 'AGENTS.md'] },
  opencode: { paths: ['.opencode/plugins/obituary.ts', 'AGENTS.md', 'opencode.json'] },
  openclaw: { paths: ['.openclaw/extensions/obituary/', 'AGENTS.md', '~/.openclaw/openclaw.json'] },
  pi: { paths: ['.pi/extensions/obituary.ts', '.pi/mcp.json', 'AGENTS.md'] },
  omp: { paths: ['.omp/extensions/obituary.ts', '.omp/mcp.json', 'AGENTS.md'] },
  hermes: { paths: { Windows: '%LOCALAPPDATA%/hermes/config.yaml', 'macOS / Linux': '~/.hermes/config.yaml' }, global: true },
  cline: { paths: ['~/.cline/plugins/obituary.ts', '~/.cline/data/settings/cline_mcp_settings.json'], global: true },
  kilocode: { paths: ['.kilo/plugin/obituary.ts', '.kilo/kilo.json', '.kilocode/rules/obituary.md'] },
  'minimax-code': { paths: ['AGENTS.md', '.mcp.json'] },
  'deepseek-harness': { paths: ['.dsh/obituary-hooks/', '.dsh/obituary-mcp/cordis.patch.yml', 'AGENTS.md'] },
  roo: { paths: ['.roo/rules/obituary.md', '.roo/mcp.json'], manual: true },
  windsurf: { paths: ['.windsurfrules'], manual: true },
  workbuddy: { paths: ['.workbuddy/obituary-export/SETUP.md', '.workbuddy/mcp.json'], manual: true },
  qoderwork: { paths: ['~/.qoderwork/skills/obituary/SKILL.md'], global: true, flags: '--skill', manual: true },
};

export const guides = agentData.map(a => ({ ...a, ...setup[a.id] })).sort((a, b) => a.name.localeCompare(b.name, 'en'));
