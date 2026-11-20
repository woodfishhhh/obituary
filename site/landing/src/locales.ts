// The languages of the site. Simplified Chinese keeps the root address and English keeps /en/;
// each other language has its own folder. The order is the README's language bar. Each name is
// written in its own language, and Traditional Chinese carries the flag of China like Simplified.
export const locales = [
  { id: "en", path: "en/", html: "en", name: "English", flag: "us" },
  { id: "zh", path: "", html: "zh-CN", name: "简体中文", flag: "cn" },
  { id: "zh-TW", path: "zh-TW/", html: "zh-TW", name: "繁體中文", flag: "cn" },
  { id: "ja", path: "ja/", html: "ja", name: "日本語", flag: "jp" },
  { id: "ko", path: "ko/", html: "ko", name: "한국어", flag: "kr" },
  { id: "es", path: "es/", html: "es", name: "Español", flag: "es" },
  { id: "fr", path: "fr/", html: "fr", name: "Français", flag: "fr" },
  { id: "de", path: "de/", html: "de", name: "Deutsch", flag: "de" },
  { id: "pt-BR", path: "pt-BR/", html: "pt-BR", name: "Português (Brasil)", flag: "br" },
  { id: "ru", path: "ru/", html: "ru", name: "Русский", flag: "ru" },
] as const;

export type Lang = (typeof locales)[number]["id"];
export type Locale = (typeof locales)[number];

export const localeOf = (id: Lang): Locale => locales.find((l) => l.id === id)!;
export const siteOrigin = "https://obituary.woodfish.site";
// The page's language is the one in <html lang>; the built pages carry it, and dev takes ?lang=.
export const langFromHtml = (html: string): Lang => locales.find((l) => l.html === html)?.id ?? "en";

// The page a browser's language list points at, or undefined when none of the site's languages fits.
export function matchBrowser(tags: readonly string[]): Lang | undefined {
  for (const raw of tags) {
    const t = raw.toLowerCase();
    if (/^zh(-(tw|hk|mo|hant))/.test(t)) return "zh-TW";
    if (t.startsWith("zh")) return "zh";
    if (t.startsWith("pt")) return "pt-BR";
    const hit = locales.find((l) => l.id !== "zh" && l.id !== "zh-TW" && l.id !== "pt-BR" && t.split("-")[0] === l.id);
    if (hit) return hit.id;
  }
  return undefined;
}
