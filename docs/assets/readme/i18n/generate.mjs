// Regenerates the README diagrams: <lang>/{lifecycle,delete-guard}-{light,dark}.svg.
// layout.js measures every string with the browser's own text engine, so this
// needs the pinned Playwright and its Chromium. From this directory:
//   pnpm install --frozen-lockfile && pnpm browser   (once)
//   pnpm generate
// Then check the result with: pnpm check
import { createRequire } from "node:module";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
let chromium;
try {
  ({ chromium } = createRequire(path.join(here, "package.json"))("playwright"));
} catch {
  console.error("playwright is not installed: run `pnpm install --frozen-lockfile && pnpm browser` in " + here);
  process.exit(2);
}

const strings = JSON.parse(fs.readFileSync(path.join(here, "strings.json"), "utf8"));
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
await page.setContent("<!doctype html><meta charset=utf-8><body></body>");
await page.addScriptTag({ content: fs.readFileSync(path.join(here, "layout.js"), "utf8") });
const files = await page.evaluate((s) => window.buildAll(s), strings);
await browser.close();
for (const [name, svg] of Object.entries(files)) {
  const target = path.join(here, name);
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.writeFileSync(target, svg, "utf8");
}
console.log("generated", Object.keys(files).length, "SVG files");
