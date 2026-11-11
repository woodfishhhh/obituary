// Builds the README's agent wall: agents-light.svg and agents-dark.svg.
// Marks come from @lobehub/icons-static-svg (MIT; see docs/assets/THIRD_PARTY_NOTICES.md),
// used unaltered. Run from the repository root after `pnpm install` in site/:
//   node docs/assets/readme/agents/build.mjs
import { readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const require = createRequire(join(here, "../../../../site/landing/package.json"));
const icons = join(dirname(require.resolve("@lobehub/icons-static-svg/package.json")), "icons");

// The best-known agents, in the order people look for them; `id` is the
// value for `obit init --agent`.
const agents = [
  ["Claude Code", "claude", "claudecode-color"],
  ["Codex", "codex", "codex-color"],
  ["Cursor", "cursor", "cursor"],
  ["GitHub Copilot", "copilot", "githubcopilot"],
  ["Gemini CLI", "gemini", "geminicli-color"],
  ["Windsurf", "windsurf", "windsurf"],
  ["Trae", "trae", "trae-color"],
  ["Qoder", "qoder", "qoder-color"],
  ["CodeBuddy", "codebuddy", "codebuddy-color"],
  ["Kimi", "kimi", "kimi-color"],
  ["DeepSeek Harness", "deepseek-harness", "deepseek-color"],
  ["WorkBuddy", "workbuddy", "workbuddy-color"],
  ["OpenCode", "opencode", "opencode"],
  ["Cline", "cline", "cline"],
  ["Roo Code", "roo", "roocode"],
  ["Kilo Code", "kilocode", "kilocode"],
  ["OpenClaw", "openclaw", "openclaw-color"],
  ["Hermes", "hermes", "hermesagent"],
];

const themes = {
  light: { bg: "#F5F8FD", tile: "#FFFFFF", line: "#DDE6F3", ink: "#0F1B33", muted: "#5B6B85", mono: "#0F1B33" },
  dark: { bg: "#0B1120", tile: "#121B2E", line: "#24324D", ink: "#E8EEF8", muted: "#93A2BC", mono: "#E8EEF8" },
};

const SANS = "'Segoe UI', -apple-system, 'Helvetica Neue', Arial, sans-serif";
const MONO = "'Cascadia Mono', Consolas, 'JetBrains Mono', monospace";

// One icon as a nested <svg>, with its ids prefixed so icons cannot collide.
function icon(file, prefix, x, y, size, color) {
  let src = readFileSync(join(icons, `${file}.svg`), "utf8");
  const viewBox = /viewBox="([^"]+)"/.exec(src)[1];
  const rootFill = /^<svg[^>]*\sfill="([^"]+)"/.exec(src)?.[1];
  let inner = src.replace(/^<svg[^>]*>/, "").replace(/<\/svg>\s*$/, "").replace(/<title>.*?<\/title>/, "");
  inner = inner.replace(/id="([^"]+)"/g, `id="${prefix}-$1"`).replace(/url\(#([^)]+)\)/g, `url(#${prefix}-$1)`).replace(/href="#([^"]+)"/g, `href="#${prefix}-$1"`);
  const fill = rootFill ? ` fill="${rootFill}"` : "";
  return `<svg x="${x}" y="${y}" width="${size}" height="${size}" viewBox="${viewBox}"${fill} color="${color}" fill-rule="evenodd">${inner}</svg>`;
}

function wall(theme) {
  const t = themes[theme];
  const cols = 6, w = 176, h = 150, gap = 16, mx = 32, my = 32;
  const rows = Math.ceil(agents.length / cols);
  const width = 1200, height = my * 2 + rows * h + (rows - 1) * gap;
  const tiles = agents.map(([name, id, file], i) => {
    // The colour Codex mark carries a white plate; the plain mark reads better on dark.
    if (theme === "dark" && file === "codex-color") file = "codex";
    const x = mx + (i % cols) * (w + gap), y = my + Math.floor(i / cols) * (h + gap);
    return `  <g>
    <rect x="${x}" y="${y}" width="${w}" height="${h}" rx="18" fill="${t.tile}" stroke="${t.line}"/>
    ${icon(file, `i${i}`, x + (w - 52) / 2, y + 22, 52, t.mono)}
    <text x="${x + w / 2}" y="${y + 106}" text-anchor="middle" font-family="${SANS}" font-size="20" font-weight="600" fill="${t.ink}">${name}</text>
    <text x="${x + w / 2}" y="${y + 132}" text-anchor="middle" font-family="${MONO}" font-size="18" fill="${t.muted}">${id}</text>
  </g>`;
  }).join("\n");
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}" role="img" aria-labelledby="t d">
  <title id="t">Agents Obituary connects to</title>
  <desc id="d">${agents.map(([name, id]) => `${name} (${id})`).join(", ")}</desc>
  <rect width="${width}" height="${height}" rx="28" fill="${t.bg}"/>
${tiles}
</svg>
`;
}

for (const theme of Object.keys(themes)) writeFileSync(join(here, `agents-${theme}.svg`), wall(theme));
console.log("written agents-light.svg, agents-dark.svg");
