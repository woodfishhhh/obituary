import type { Lang } from "../locales";
import type { Strings } from "./types";

export * from "./base";
export type { Strings };
export type { Lang };

// Every sentence on the landing page, per language. Claims come from the repository's README.md
// (the English source) only; keep them in step. The CJK strings also decide which glyphs
// scripts/subset-fonts.mjs keeps (Simplified Chinese only; the others use the system's fonts).
export type Copy = Strings & { lang: Lang; docs: string };

// Each language is a module of its own, so a page loads its own words and no one else's; the built
// page names that module in a modulepreload link (scripts/prerender.mjs), so it arrives with the script.
export const loaders: Record<Lang, () => Promise<Strings>> = {
  zh: () => import("./zh").then((m) => m.zh),
  "zh-TW": () => import("./zh-tw").then((m) => m.zhTW),
  en: () => import("./en").then((m) => m.en),
  ja: () => import("./ja").then((m) => m.ja),
  ko: () => import("./ko").then((m) => m.ko),
  es: () => import("./es").then((m) => m.es),
  fr: () => import("./fr").then((m) => m.fr),
  de: () => import("./de").then((m) => m.de),
  "pt-BR": () => import("./pt-br").then((m) => m.ptBR),
  ru: () => import("./ru").then((m) => m.ru),
};

// The docs are in Chinese and English; the other languages open the English ones.
export const withLang = (lang: Lang, strings: Strings): Copy => ({ ...strings, lang, docs: lang === "zh" ? "docs/" : "docs/en/" });
export const loadCopy = async (lang: Lang): Promise<Copy> => withLang(lang, await loaders[lang]());
