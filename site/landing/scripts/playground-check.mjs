// Checks the built playground for stray scrollbars: the page's own horizontal overflow, and any element that shows
// a scrollbar of its own, in every wall, both schemes, four widths (one at 125% zoom), with every dial at its
// minimum, maximum and default, after a mouse click and after keyboard focus. Classic scrollbars are switched on
// (Playwright hides them by default) because those are the ones that take room on Windows.
// Run after `pnpm build:playground`: `node scripts/playground-check.mjs`; it exits 1 on any problem.
import { execSync } from "node:child_process";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "dist-playground");
const { chromium } = createRequire(path.join(execSync("npm root -g", { encoding: "utf8" }).trim(), "x.js"))("playwright");
// Classic scrollbars, which take room (Windows Chrome / Edge); Playwright hides them by default.
const browser = await chromium.launch({ ignoreDefaultArgs: ["--hide-scrollbars"] });
const styles = ["stream", "dock", "spotlight", "orbit", "bento", "canvas", "flip", "prompt"];
const problems = [];
let checks = 0;

// The page's own horizontal overflow, and every element showing a scrollbar of its own (the JSON box in the panel is meant to).
const scan = (page) => page.evaluate(() => {
  const d = document.documentElement, out = [];
  if (d.scrollWidth > d.clientWidth) out.push(`document ${d.scrollWidth}>${d.clientWidth}`);
  for (const e of document.querySelectorAll("body *")) {
    if (e.closest(".pg-panel pre")) continue;
    const cs = getComputedStyle(e);
    if (cs.scrollbarWidth === "none") continue;
    const x = (cs.overflowX === "auto" || cs.overflowX === "scroll") && e.offsetHeight - e.clientHeight - parseFloat(cs.borderTopWidth) - parseFloat(cs.borderBottomWidth) > 1 && e.scrollWidth > e.clientWidth;
    const y = (cs.overflowY === "auto" || cs.overflowY === "scroll") && e.offsetWidth - e.clientWidth - parseFloat(cs.borderLeftWidth) - parseFloat(cs.borderRightWidth) > 1 && e.scrollHeight > e.clientHeight;
    if (x || y) out.push(`${e.tagName}.${String(e.className).slice(0, 40)} ${x ? "x" : ""}${y ? "y" : ""}`);
  }
  return out;
});
const note = async (page, tag) => { checks++; const r = await scan(page); if (r.length) problems.push(`${tag}: ${r.join("; ")}`); };

for (const [w, zoom] of [[1280, 1], [1536, 1.25], [1920, 1], [390, 1]]) {
  const ctx = await browser.newContext({ viewport: { width: Math.round(w / zoom), height: Math.round(900 / zoom) }, deviceScaleFactor: zoom });
  const page = await ctx.newPage();
  for (const s of styles) for (const dark of [false, true]) {
    const tag = `${s} ${w}px@${zoom} ${dark ? "dark" : "light"}`;
    await page.goto(`${pathToFileURL(path.join(root, "index.html")).href}#s=${s}&theme=${dark ? "dark" : "light"}&lang=en&pick=deepseek-harness`);
    await page.reload();
    await page.waitForSelector(".pg-stage");
    await page.waitForTimeout(300);
    await note(page, tag);
    // Every dial at its minimum and at its maximum.
    const ids = await page.$$eval(".pg-panel input[type=range]", (els) => els.map((e) => [e.id, e.min, e.max, e.value]));
    for (const [id, min, max, value] of ids) {
      for (const v of [min, max, value]) { await page.locator(`#${id}`).fill(v); await page.waitForTimeout(40); await note(page, `${tag} ${id}=${v}`); }
    }
    // A mouse click on an item, then keyboard focus on the next one.
    const item = page.locator(".pg-item, .agent-chip").first();
    await item.click({ force: true }).catch(() => {});
    await note(page, `${tag} after click`);
    await page.keyboard.press("Tab");
    await page.keyboard.press("Tab");
    await note(page, `${tag} after Tab`);
  }
  await ctx.close();
}
console.log("checks:", checks, "problems:", problems.length);
console.log(problems.slice(0, 30).join("\n"));
await browser.close();
if (problems.length) process.exit(1);
