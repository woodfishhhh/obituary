// Writes each language's page from the one built template, with its text already in the HTML, so the
// words are there before any script runs (search engines, readers with scripts off, a slow first
// paint). The browser hydrates the same markup. Also writes sitemap.xml and robots.txt.
//   vite build                            -> dist/index.html (the template; becomes the Chinese page)
//   vite build --ssr src/entry-server.tsx -> dist-ssr/entry-server.js   (run before this script)
import { mkdir, readdir, readFile, rm, writeFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

const root = new URL("../", import.meta.url);
const { render, meta, locales, siteOrigin } = await import(pathToFileURL(new URL("dist-ssr/entry-server.js", root).pathname.replace(/^\/([A-Za-z]:)/, "$1")).href);
const template = await readFile(new URL("dist/index.html", root), "utf8");
const empty = '<div id="root"></div>';
for (const marker of [empty, "<!--page-head-->", "<!--page-noscript-->", '<html lang="en">']) {
  if (!template.includes(marker)) throw new Error(`dist/index.html: missing ${marker}`);
}

// Each page's own language is a module of its own; naming it up front lets the browser fetch it with the script.
const manifest = JSON.parse(await readFile(new URL("dist/.vite/manifest.json", root), "utf8"));
const languageModule = (l) => {
  const entry = manifest[`src/copy/${l.id === "pt-BR" ? "pt-br" : l.id === "zh-TW" ? "zh-tw" : l.id}.ts`];
  if (!entry) throw new Error(`no module for ${l.id} in the build manifest`);
  return `<link rel="modulepreload" crossorigin href="/${entry.file}" />`;
};

// The first screen's fonts are fetched with the HTML. Without that the text shows in a fallback face first and
// the page shifts when the real one arrives. Simplified Chinese adds its display face, Russian its Cyrillic one.
const assets = await readdir(new URL("dist/assets/", root));
const font = (prefix) => assets.find((f) => f.startsWith(prefix) && f.endsWith(".woff2"));
const preloadFonts = (l) =>
  [font("geist-latin-wght-normal-"), l.id === "zh" ? font("display-sc-600-") : null, l.id === "ru" ? font("geist-cyrillic-wght-normal-") : null]
    .filter(Boolean)
    .map((f) => `<link rel="preload" as="font" type="font/woff2" crossorigin href="/assets/${f}" />`);

const esc = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
const url = (l) => `${siteOrigin}/${l.path}`;
// hreflang says which page is for which reader; x-default is the one for everybody else.
const alternates = [
  ...locales.map((l) => `<link rel="alternate" hreflang="${l.html}" href="${url(l)}" />`),
  `<link rel="alternate" hreflang="x-default" href="${url(locales.find((l) => l.id === "en"))}" />`,
].join("\n    ");

for (const l of locales) {
  const m = await meta(l.id);
  const head = [
    `<title>${esc(m.title)}</title>`,
    `<meta name="description" content="${esc(m.description)}" />`,
    languageModule(l),
    ...preloadFonts(l),
    `<link rel="canonical" href="${url(l)}" />`,
    alternates,
    `<meta property="og:type" content="website" />`,
    `<meta property="og:site_name" content="Obituary" />`,
    `<meta property="og:locale" content="${l.html.replace("-", "_")}" />`,
    `<meta property="og:title" content="${esc(m.title)}" />`,
    `<meta property="og:description" content="${esc(m.ogDescription)}" />`,
    `<meta property="og:url" content="${url(l)}" />`,
  ].join("\n    ");
  const noscript = `<noscript>\n      <p>${esc(m.noscript)} <a href="/${m.docs}">${esc(m.noscriptDocs)}</a> · <a href="https://github.com/woodfishhhh/obituary">GitHub</a></p>\n    </noscript>`;
  const html = await render(l.id);
  const page = template
    .replace('<html lang="en">', `<html lang="${l.html}">`)
    .replace("<!--page-head-->", () => head)
    .replace("<!--page-noscript-->", () => noscript)
    .replace(empty, () => `<div id="root">${html}</div>`);
  const out = new URL(`dist/${l.path}index.html`, root);
  await mkdir(new URL("./", out), { recursive: true });
  await writeFile(out, page);
}

const sitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">
${locales.map((l) => `  <url>\n    <loc>${url(l)}</loc>\n${locales.map((a) => `    <xhtml:link rel="alternate" hreflang="${a.html}" href="${url(a)}" />`).join("\n")}\n    <xhtml:link rel="alternate" hreflang="x-default" href="${url(locales.find((a) => a.id === "en"))}" />\n  </url>`).join("\n")}
</urlset>
`;
await writeFile(new URL("dist/sitemap.xml", root), sitemap);
// The docs build writes its own sitemap under /docs/.
await writeFile(new URL("dist/robots.txt", root), `User-agent: *\nAllow: /\n\nSitemap: ${siteOrigin}/sitemap.xml\nSitemap: ${siteOrigin}/docs/sitemap.xml\nSitemap: ${siteOrigin}/blog/sitemap.xml\n`);
await rm(new URL("dist-ssr/", root), { recursive: true, force: true });
await rm(new URL("dist/.vite/", root), { recursive: true, force: true });
