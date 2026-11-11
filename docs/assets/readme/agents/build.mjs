// Builds the README's agent wall: agents-light.svg and agents-dark.svg.
// Logos only; each one stands for an agent Obituary connects to (agents
// that share a mark, such as Trae and Trae CN, appear once).
// Marks: @lobehub/icons-static-svg (MIT) and the vendors' own marks kept in
// site/landing/src/assets/agents; see docs/assets/THIRD_PARTY_NOTICES.md.
// Run from the repository root after `pnpm install` in site/:
//   node docs/assets/readme/agents/build.mjs
import { readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, "../../../..");
const require = createRequire(join(root, "site/landing/package.json"));
const lobe = join(dirname(require.resolve("@lobehub/icons-static-svg/package.json")), "icons");
const vendor = join(root, "site/landing/src/assets/agents");

// [name, mark]; a mark is a lobe-icons name or vendor:<file>.
const agents = [
  ["Claude Code", "claudecode-color"],
  ["Codex", "codex-color"],
  ["Cursor", "cursor"],
  ["GitHub Copilot", "githubcopilot"],
  ["Gemini CLI", "geminicli-color"],
  ["Windsurf", "windsurf"],
  ["Trae", "trae-color"],
  ["Qoder", "qoder-color"],
  ["CodeBuddy", "codebuddy-color"],
  ["Kimi", "kimi-color"],
  ["DeepSeek Harness", "deepseek-color"],
  ["WorkBuddy", "workbuddy-color"],
  ["OpenCode", "opencode"],
  ["Cline", "cline"],
  ["Roo Code", "roocode"],
  ["Kilo Code", "kilocode"],
  ["OpenClaw", "openclaw-color"],
  ["Hermes", "hermesagent"],
  ["MiniMax Code", "minimax-color"],
  ["Lingma", "vendor:lingma.png"],
  ["Google Antigravity", "antigravity-color"],
  ["Mistral Vibe", "mistral-color"],
  ["Factory Droid", "vendor:factory.svg"],
  ["ZCode", "zhipu-color"],
  ["Pi", "vendor:pi.svg"],
  ["Oh My Pi", "vendor:oh-my-pi.svg"],
];
// Marks drawn in a light colour that need a dark tile on the light page.
const darkTile = new Set(["Oh My Pi"]);

const themes = {
  light: { bg: "#F5F8FD", tile: "#FFFFFF", line: "#DDE6F3", mono: "#0F1B33", ink: "#0F1B33" },
  dark: { bg: "#0B1120", tile: "#121B2E", line: "#24324D", mono: "#E8EEF8", ink: "#0F1B33" },
};

const prefixIds = (src, p) =>
  src.replace(/id="([^"]+)"/g, `id="${p}-$1"`).replace(/url\(#([^)]+)\)/g, `url(#${p}-$1)`).replace(/href="#([^"]+)"/g, `href="#${p}-$1"`);

// One mark as a nested <svg> (or an <image> for a raster mark), ids prefixed
// so marks cannot collide.
function mark(file, p, x, y, size, color) {
  if (file.startsWith("vendor:") && file.endsWith(".png")) {
    const data = readFileSync(join(vendor, file.slice(7))).toString("base64");
    return `<image x="${x}" y="${y}" width="${size}" height="${size}" href="data:image/png;base64,${data}"/>`;
  }
  let src = readFileSync(file.startsWith("vendor:") ? join(vendor, file.slice(7)) : join(lobe, `${file}.svg`), "utf8");
  src = src.replace(/<\?xml[^>]*>\s*/, "").replace(/<!--[\s\S]*?-->/g, "");
  // Unwrap the outer <svg> (the Factory mark nests one svg inside another).
  const open = /^<svg[^>]*>/.exec(src.trim())[0];
  let inner = src.trim().slice(open.length).replace(/<\/svg>\s*$/, "").replace(/<title>.*?<\/title>/, "");
  let viewBox = /viewBox="([^"]+)"/.exec(open)?.[1];
  if (!viewBox) {
    const w = /width="([\d.]+)"/.exec(open)[1], h = /height="([\d.]+)"/.exec(open)[1];
    viewBox = `0 0 ${w} ${h}`;
  }
  const fill = /\sfill="([^"]+)"/.exec(open)?.[1];
  inner = prefixIds(inner, p);
  return `<svg x="${x}" y="${y}" width="${size}" height="${size}" viewBox="${viewBox}"${fill ? ` fill="${fill}"` : ""} color="${color}" preserveAspectRatio="xMidYMid meet">${inner}</svg>`;
}

function wall(theme) {
  const t = themes[theme];
  const cols = 9, s = 112, gap = 14, mx = 32, my = 32, logo = 56;
  const rows = Math.ceil(agents.length / cols);
  const width = 1200, height = my * 2 + rows * s + (rows - 1) * gap;
  const tiles = agents.map(([name, file], i) => {
    if (theme === "dark" && file === "codex-color") file = "codex"; // the colour mark carries a white plate
    const row = Math.floor(i / cols), inRow = Math.min(cols, agents.length - row * cols);
    const left = (width - (inRow * s + (inRow - 1) * gap)) / 2;
    const x = left + (i % cols) * (s + gap), y = my + row * (s + gap);
    const tile = darkTile.has(name) && theme === "light" ? t.ink : t.tile;
    return `  <g><title>${name}</title>
    <rect x="${x}" y="${y}" width="${s}" height="${s}" rx="22" fill="${tile}" stroke="${t.line}"/>
    ${mark(file, `m${i}`, x + (s - logo) / 2, y + (s - logo) / 2, logo, t.mono)}
  </g>`;
  }).join("\n");
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}" role="img" aria-labelledby="t d">
  <title id="t">Agents Obituary connects to</title>
  <desc id="d">${agents.map(([name]) => name).join(", ")}</desc>
  <rect width="${width}" height="${height}" rx="28" fill="${t.bg}"/>
${tiles}
</svg>
`;
}

for (const theme of Object.keys(themes)) writeFileSync(join(here, `agents-${theme}.svg`), wall(theme));
console.log(`written agents-light.svg, agents-dark.svg (${agents.length} marks)`);
