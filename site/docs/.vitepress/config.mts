import { defineConfig } from "vitepress";

import { locales } from '../../landing/src/locales';
import { docLocales, searchLocales } from './navigation.mts';
import { pageSuffix } from './routes';

// Both builds serve the landing page at the root of obituary.woodfish.site
// (GitHub Pages abroad, the mainland China mirror built with VITE_SITE=cn at
// home); the docs sit under it at docs/. SITE_BASE and SITE_URL override them.
const mirror = process.env.VITE_SITE === "cn";
const siteBase = process.env.SITE_BASE ?? "/";
const base = `${siteBase}docs/`;
const siteUrl = process.env.SITE_URL ?? "https://obituary.woodfish.site";
// Programs fetch from this host, which always reaches the mirror server.
const mirrorHost = "https://cn.obituary.woodfish.site/";

// On the mirror, install lines point at files the mirror serves, so a reader
// who cannot reach GitHub can still follow them.
function toMirror(src: string): string {
  return src
    .replaceAll("https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh", `${mirrorHost}install.sh`)
    .replace(/bash install\.sh(?= *(#|$))/gm, "bash install.sh --mirror cn")
    .replaceAll("https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1", `${mirrorHost}install.ps1`)
    .replace(/\.\\install\.ps1(?= *(#|$))/gm, ".\\install.ps1 -Mirror cn");
}
const repo = "https://github.com/woodfishhhh/obituary";

export default defineConfig({
  base,
  markdown: {
    config(md) {
      if (!mirror) return;
      md.core.ruler.before("normalize", "obit-mirror-links", (state) => {
        state.src = toMirror(state.src);
      });
    },
  },
  cleanUrls: false,
  lastUpdated: false,
  sitemap: { hostname: `${siteUrl}${base}` },
  head: [
    ["link", { rel: "icon", type: "image/png", sizes: "32x32", href: `${base}icon-32.png` }],
    ["link", { rel: "icon", type: "image/png", sizes: "512x512", href: `${base}icon-512.png` }],
    ["link", { rel: "apple-touch-icon", sizes: "180x180", href: `${base}icon-180.png` }],
    ["meta", { name: "theme-color", content: "#ffffff" }],
  ],
  locales: docLocales(`${siteUrl}${siteBase}`, repo),
  transformHead({ pageData }) {
    if (pageData.isNotFound) return [];
    const suffix = pageSuffix(pageData.relativePath);
    return locales.map(l => ['link', {rel:'alternate', hreflang:l.html, href:`${siteUrl}${base}${l.path}${suffix}`}]);
  },
  themeConfig: {
    logo: { src: "/icon-128.png", alt: "" },
    socialLinks: [{ icon: "github", link: repo }],
    search: {provider:'local', options:{locales:searchLocales}},
  },
});
