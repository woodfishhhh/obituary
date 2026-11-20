// Writes each page's first render into its built HTML, so the text is there before any script runs
// (search engines, readers with scripts off, a slow first paint). The browser hydrates the same markup.
//   vite build                         -> dist/index.html, dist/en/index.html
//   vite build --ssr src/entry-server.tsx -> dist-ssr/entry-server.js   (run before this script)
import { readFile, rm, writeFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

const root = new URL("../", import.meta.url);
const { render } = await import(pathToFileURL(new URL("dist-ssr/entry-server.js", root).pathname.replace(/^\/([A-Za-z]:)/, "$1")).href);
const empty = '<div id="root"></div>';

for (const [file, lang] of [["dist/index.html", "zh"], ["dist/en/index.html", "en"]]) {
  const path = new URL(file, root);
  const page = await readFile(path, "utf8");
  if (!page.includes(empty)) throw new Error(`${file}: no empty root to fill`);
  await writeFile(path, page.replace(empty, () => `<div id="root">${render(lang)}</div>`));
}
await rm(new URL("dist-ssr/", root), { recursive: true, force: true });
