import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import { copy } from "@/copy";
import { locales, siteOrigin, type Lang } from "@/locales";
import { App } from "./App";

export { locales, siteOrigin };
// The page as HTML, for scripts/prerender.mjs; the browser then hydrates it (main.tsx).
export const render = (lang: Lang) => renderToString(<StrictMode><App lang={lang} /></StrictMode>);
// What goes in the page's <head> and <noscript> besides the body.
export const meta = (lang: Lang) => ({ ...copy[lang].meta, docs: copy[lang].docs, noscriptSuffix: "GitHub" });
