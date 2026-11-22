// Screenshots of every wall in the logo-wall playground (dist-playground/), from the built file:// page in a
// headless browser: desktop light and dark, and a phone, into dist-playground/shots/. Run after
// `pnpm build:playground`: `node scripts/playground-shots.mjs [style-id ...]`.
import { execSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "dist-playground");
const out = path.join(root, "shots");
mkdirSync(out, { recursive: true });
const globalRoot = execSync("npm root -g", { encoding: "utf8" }).trim();
const { chromium } = createRequire(path.join(globalRoot, "x.js"))("playwright");

const all = [["stream", "01-stream"], ["dock", "02-dock"], ["spotlight", "03-spotlight"], ["orbit", "04-orbit"], ["bento", "05-bento"], ["canvas", "06-canvas"], ["flip", "07-flip"], ["prompt", "08-prompt"]];
const only = process.argv.slice(2);
const styles = all.filter(([id]) => only.length === 0 || only.includes(id));
const shots = [
  { name: "light", theme: "light", viewport: { width: 1440, height: 900 }, scale: 1 },
  { name: "dark", theme: "dark", viewport: { width: 1440, height: 900 }, scale: 1 },
  { name: "mobile", theme: "light", viewport: { width: 390, height: 844 }, scale: 2 },
];

const browser = await chromium.launch();
for (const [id, base] of styles) {
  for (const s of shots) {
    const page = await browser.newPage({ viewport: s.viewport, deviceScaleFactor: s.scale, hasTouch: s.name === "mobile", isMobile: s.name === "mobile" });
    page.on("pageerror", (e) => console.error(`${id}/${s.name}:`, e.message));
    await page.goto(`${pathToFileURL(path.join(root, "index.html")).href}#s=${id}&theme=${s.theme}&lang=zh&bare&pick=claude`);
    await page.waitForSelector(".pg-stage section");
    await page.waitForTimeout(1500);
    // Desktop: park the pointer on the wall, so the hover effects (the Dock's swell, the spotlight) are in the picture.
    const hover = { dock: "[role=toolbar]", spotlight: ".pg-spot-card >> nth=9", flip: ".pg-flip >> nth=11", bento: ".pg-bento-tile >> nth=4", stream: ".pg-chip >> nth=6", orbit: ".pg-orb >> nth=3" }[id];
    if (s.name !== "mobile" && hover) {
      const box = await page.locator(hover).first().boundingBox();
      if (box) await page.mouse.move(box.x + box.width * (id === "dock" ? 0.4 : 0.5), box.y + box.height * 0.5);
      await page.waitForTimeout(800);
    }
    await page.locator(".pg-stage").screenshot({ path: path.join(out, `${base}-${s.name}.png`) });
    await page.close();
  }
  console.log("shot", id);
}
await browser.close();
