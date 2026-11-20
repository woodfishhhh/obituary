import { StrictMode } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";
import "./index.css";
import { App } from "./App";
import { langFromHtml, locales, type Lang } from "./locales";

// The built pages carry their language in <html lang>; in dev there is one page, and ?lang= picks the language.
const wanted = import.meta.env.DEV ? new URLSearchParams(location.search).get("lang") : null;
const lang: Lang = locales.find((l) => l.id === wanted)?.id ?? langFromHtml(document.documentElement.lang);
if (import.meta.env.DEV) document.documentElement.lang = locales.find((l) => l.id === lang)!.html;
const root = document.getElementById("root")!;
const page = (
  <StrictMode>
    <App lang={lang} />
  </StrictMode>
);

// The built pages arrive with their text already in the HTML (scripts/prerender.mjs); in dev the root is empty.
if (root.hasChildNodes()) hydrateRoot(root, page);
else createRoot(root).render(page);
