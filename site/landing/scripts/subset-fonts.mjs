// The landing page sets its Chinese heads in Noto Sans SC. The full face is
// several megabytes, so build only the glyphs the page's Chinese copy uses.
// Output is generated (git-ignored) and rebuilt on every dev and build run.
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import subsetFont from "subset-font";

const require = createRequire(import.meta.url);
const copyFile = new URL("../src/copy.ts", import.meta.url);
const outDir = new URL("../src/assets/fonts/", import.meta.url);
const weights = [600];

const copy = await readFile(copyFile, "utf8");
// CJK ideographs, CJK punctuation and full-width forms; Latin stays in the
// Latin face.
const glyphs = new Set(copy.match(/[　-〿㐀-鿿＀-￯]/gu) ?? []);
const text = [...glyphs].sort().join("");

await mkdir(outDir, { recursive: true });
for (const weight of weights) {
  const source = require.resolve(`@fontsource/noto-sans-sc/files/noto-sans-sc-chinese-simplified-${weight}-normal.woff2`);
  const font = await subsetFont(await readFile(source), text, { targetFormat: "woff2" });
  await writeFile(new URL(`display-sc-${weight}.woff2`, outDir), font);
  console.log(`display-sc-${weight}.woff2: ${glyphs.size} glyphs, ${(font.length / 1024).toFixed(1)} KiB`);
}
