import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import type { Lang } from "@/copy";
import { App } from "./App";

// The page as HTML, for scripts/prerender.mjs; the browser then hydrates it (main.tsx).
export const render = (lang: Lang) => renderToString(<StrictMode><App lang={lang} /></StrictMode>);
