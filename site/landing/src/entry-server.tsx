import { StrictMode } from "react";
import { prerenderToNodeStream } from "react-dom/static";
import { loadCopy } from "@/copy";
import { locales, siteOrigin, type Lang } from "@/locales";
import { App } from "./App";

export { locales, siteOrigin };
// The page as HTML, for scripts/prerender.mjs; the browser then hydrates it (main.tsx). Unlike
// renderToString this waits for the lazily loaded sections, so their text is in the HTML too.
export async function render(lang: Lang): Promise<string> {
  // A static page needs completed sections inline so native fragment navigation can see their layout.
  const { prelude } = await prerenderToNodeStream(<StrictMode><App c={await loadCopy(lang)} /></StrictMode>, { progressiveChunkSize: Infinity });
  const chunks: Buffer[] = [];
  for await (const chunk of prelude) chunks.push(Buffer.from(chunk));
  return Buffer.concat(chunks).toString("utf8");
}
// What goes in the page's <head> and <noscript> besides the body.
export const meta = async (lang: Lang) => { const c = await loadCopy(lang); return { ...c.meta, docs: c.docs }; };
