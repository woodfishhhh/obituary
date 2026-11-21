// Run against a built landing preview: node --test scripts/anchors.test.mjs (SITE_URL overrides the origin).
// Uses the headless Chromium already pinned for the README diagram checks.
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";

const { chromium } = createRequire(new URL("../../../docs/assets/readme/i18n/package.json", import.meta.url))("playwright");
const origin = process.env.SITE_URL ?? "http://localhost:4173";

test("direct fragment visits clear the fixed navigation, with or without hydration", async (t) => {
  const browser = await chromium.launch();
  t.after(() => browser.close());
  for (const javaScriptEnabled of [false, true]) {
    const context = await browser.newContext({ javaScriptEnabled, viewport: { width: 390, height: 844 } });
    for (const id of ["why", "life", "window", "install"]) {
      const page = await context.newPage();
      await page.goto(origin + "/en/#" + id);
      const { top, nav, padding, behavior } = await page.evaluate(() => {
        const style = getComputedStyle(document.documentElement);
        return { top: document.querySelector(":target").getBoundingClientRect().top,
          nav: document.querySelector("nav").getBoundingClientRect().bottom,
          padding: parseFloat(style.scrollPaddingTop), behavior: style.scrollBehavior };
      });
      assert.ok(top >= nav && Math.abs(top - padding) <= 2, JSON.stringify({ id, javaScriptEnabled, top, nav, padding }));
      assert.equal(behavior, "auto", "a direct visit must not animate from the top");
      await page.close();
    }
    await context.close();
  }
});
