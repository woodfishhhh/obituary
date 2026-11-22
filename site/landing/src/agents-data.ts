// The agents Obituary connects to, in the order the page shows them: the name, and the value of
// `obit init --agent`. Plain data and one function, with nothing that needs a bundler, so the build
// (site/scripts/assemble.mjs) can read it and check that every link below lands on a real heading.
export interface AgentData {
  name: string;
  id: string;
  // Connects for the user rather than for one project (`obit init -g`).
  global?: boolean;
  // Extra arguments of the connect command.
  flags?: string;
}

export const agentData: AgentData[] = [
  { name: "Claude Code", id: "claude", flags: "--guard-rm" },
  { name: "Codex", id: "codex", flags: "--guard-rm" },
  { name: "GitHub Copilot", id: "copilot" },
  { name: "Cursor", id: "cursor" },
  { name: "Gemini CLI", id: "gemini" },
  { name: "Factory Droid", id: "droid" },
  { name: "Trae", id: "trae" },
  { name: "Kimi", id: "kimi", global: true },
  { name: "Mistral Vibe", id: "vibe" },
  { name: "Google Antigravity", id: "antigravity" },
  { name: "CodeBuddy", id: "codebuddy" },
  { name: "Qoder", id: "qoder" },
  { name: "Lingma", id: "lingma" },
  { name: "ZCode", id: "zcode" },
  { name: "OpenCode", id: "opencode" },
  { name: "OpenClaw", id: "openclaw" },
  { name: "Pi", id: "pi" },
  { name: "Oh My Pi", id: "omp" },
  { name: "Hermes", id: "hermes", global: true },
  { name: "Cline", id: "cline", global: true },
  { name: "Kilo Code", id: "kilocode" },
  { name: "MiniMax Code", id: "minimax-code" },
  { name: "DeepSeek Harness", id: "deepseek-harness" },
  { name: "Roo Code", id: "roo" },
  { name: "Windsurf", id: "windsurf" },
  { name: "WorkBuddy", id: "workbuddy" },
  { name: "QoderWork", id: "qoderwork", global: true, flags: "--skill" },
];

// siteLink adds the .html suffix to this site-relative documentation route.
export function agentDocLink(id: string, lang: string): string {
  return `docs/${lang === "zh" ? "" : lang + "/"}agents/${id}`;
}

// `obit init --agent ` with the scope of this agent, up to where its id goes; the guides' show and uninstall commands start the same way.
export function initPrefix(a: AgentData): string {
  return `obit init${a.global ? " -g" : ""} --agent `;
}

// The command that connects an agent: what its guide shows under "Connect Obituary". `id` is the guide's variant (trae-cn, ...) when it has one.
export function initCommand(a: AgentData, id = a.id): string {
  return `${initPrefix(a)}${id} --mcp${a.flags ? " " + a.flags : ""}`;
}
