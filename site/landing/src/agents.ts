// The marks of the agents in agents-data.ts, used unaltered; sources and licences are in site/THIRD_PARTY_NOTICES.md.
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

import { agentData } from "./agents-data";
export { agentDocLink } from "./agents-data";

export interface Agent {
  name: string;
  // The value for obit init --agent.
  id: string;
  logo?: string;
  // A light mark that needs a dark tile.
  dark?: boolean;
  // A black mark, which has to turn white on a dark ground: "ink" is black alone, "duo" has colour beside the black.
  tone?: "ink" | "duo";
}

const marks: Record<string, { logo: string; dark?: boolean; tone?: "ink" | "duo" }> = {
  claude: { logo: claudecode },
  codex: { logo: codex },
  copilot: { logo: copilot, tone: "ink" },
  cursor: { logo: cursor, tone: "ink" },
  gemini: { logo: geminicli },
  droid: { logo: factory },
  trae: { logo: trae },
  kimi: { logo: kimi, tone: "duo" },
  vibe: { logo: mistral },
  antigravity: { logo: antigravity },
  codebuddy: { logo: codebuddy },
  qoder: { logo: qoder, tone: "duo" },
  lingma: { logo: lingma },
  zcode: { logo: zhipu },
  opencode: { logo: opencode, tone: "ink" },
  openclaw: { logo: openclaw },
  pi: { logo: pi },
  omp: { logo: ohMyPi, dark: true },
  hermes: { logo: hermes, tone: "ink" },
  cline: { logo: cline, tone: "ink" },
  kilocode: { logo: kilocode, tone: "ink" },
  "minimax-code": { logo: minimax },
  "deepseek-harness": { logo: deepseek },
  roo: { logo: roocode, tone: "ink" },
  windsurf: { logo: windsurf, tone: "ink" },
  workbuddy: { logo: workbuddy },
  qoderwork: { logo: qoder, tone: "duo" },
};

export const agents: Agent[] = agentData.map((a) => ({ ...a, ...marks[a.id] }));
