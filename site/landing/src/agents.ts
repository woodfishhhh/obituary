// The agents named in the README's integration table, with their marks,
// used unaltered; sources and licences are in site/THIRD_PARTY_NOTICES.md.
import claudecode from "@lobehub/icons-static-svg/icons/claudecode-color.svg?url";
import codex from "@lobehub/icons-static-svg/icons/codex-color.svg?url";
import copilot from "@lobehub/icons-static-svg/icons/githubcopilot.svg?url";
import cursor from "@lobehub/icons-static-svg/icons/cursor.svg?url";
import geminicli from "@lobehub/icons-static-svg/icons/geminicli-color.svg?url";
import trae from "@lobehub/icons-static-svg/icons/trae-color.svg?url";
import kimi from "@lobehub/icons-static-svg/icons/kimi-color.svg?url";
import mistral from "@lobehub/icons-static-svg/icons/mistral-color.svg?url";
import antigravity from "@lobehub/icons-static-svg/icons/antigravity-color.svg?url";
import codebuddy from "@lobehub/icons-static-svg/icons/codebuddy-color.svg?url";
import qoder from "@lobehub/icons-static-svg/icons/qoder-color.svg?url";
import zhipu from "@lobehub/icons-static-svg/icons/zhipu-color.svg?url";
import opencode from "@lobehub/icons-static-svg/icons/opencode.svg?url";
import openclaw from "@lobehub/icons-static-svg/icons/openclaw-color.svg?url";
import hermes from "@lobehub/icons-static-svg/icons/hermesagent.svg?url";
import cline from "@lobehub/icons-static-svg/icons/cline.svg?url";
import kilocode from "@lobehub/icons-static-svg/icons/kilocode.svg?url";
import minimax from "@lobehub/icons-static-svg/icons/minimax-color.svg?url";
import deepseek from "@lobehub/icons-static-svg/icons/deepseek-color.svg?url";
import roocode from "@lobehub/icons-static-svg/icons/roocode.svg?url";
import windsurf from "@lobehub/icons-static-svg/icons/windsurf.svg?url";
import workbuddy from "@lobehub/icons-static-svg/icons/workbuddy-color.svg?url";
// Vendor marks for agents the set above does not cover; sources in
// site/THIRD_PARTY_NOTICES.md.
import factory from "@/assets/agents/factory.svg?url";
import lingma from "@/assets/agents/lingma.png";
import ohMyPi from "@/assets/agents/oh-my-pi.svg?url";
import pi from "@/assets/agents/pi.svg?url";

export type Kind = "hook" | "plugin" | "rules";

export interface Agent {
  name: string;
  // The value for obit init --agent.
  id: string;
  // How Obituary connects to it, as grouped in docs/agents.md.
  kind: Kind;
  logo?: string;
  // A light mark that needs a dark tile.
  dark?: boolean;
}

export const agents: Agent[] = [
  { name: "Claude Code", id: "claude", kind: "hook", logo: claudecode },
  { name: "Codex", id: "codex", kind: "hook", logo: codex },
  { name: "GitHub Copilot", id: "copilot", kind: "hook", logo: copilot },
  { name: "Cursor", id: "cursor", kind: "hook", logo: cursor },
  { name: "Gemini CLI", id: "gemini", kind: "hook", logo: geminicli },
  { name: "Factory Droid", id: "droid", kind: "hook", logo: factory },
  { name: "Trae", id: "trae", kind: "hook", logo: trae },
  { name: "Kimi", id: "kimi", kind: "hook", logo: kimi },
  { name: "Mistral Vibe", id: "vibe", kind: "hook", logo: mistral },
  { name: "Google Antigravity", id: "antigravity", kind: "hook", logo: antigravity },
  { name: "CodeBuddy", id: "codebuddy", kind: "hook", logo: codebuddy },
  { name: "Qoder", id: "qoder", kind: "hook", logo: qoder },
  { name: "Lingma", id: "lingma", kind: "hook", logo: lingma },
  { name: "ZCode", id: "zcode", kind: "hook", logo: zhipu },
  { name: "OpenCode", id: "opencode", kind: "plugin", logo: opencode },
  { name: "OpenClaw", id: "openclaw", kind: "plugin", logo: openclaw },
  { name: "Pi", id: "pi", kind: "plugin", logo: pi },
  { name: "Oh My Pi", id: "omp", kind: "plugin", logo: ohMyPi, dark: true },
  { name: "Hermes", id: "hermes", kind: "plugin", logo: hermes },
  { name: "Cline", id: "cline", kind: "plugin", logo: cline },
  { name: "Kilo Code", id: "kilocode", kind: "plugin", logo: kilocode },
  { name: "MiniMax Code", id: "minimax-code", kind: "plugin", logo: minimax },
  { name: "DeepSeek Harness", id: "deepseek-harness", kind: "plugin", logo: deepseek },
  { name: "Roo Code", id: "roo", kind: "rules", logo: roocode },
  { name: "Windsurf", id: "windsurf", kind: "rules", logo: windsurf },
  { name: "WorkBuddy", id: "workbuddy", kind: "rules", logo: workbuddy },
  { name: "QoderWork", id: "qoderwork", kind: "rules", logo: qoder },
];
