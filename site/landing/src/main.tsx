import { StrictMode } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";
import "./index.css";
import { App } from "./App";

const lang = document.documentElement.lang.startsWith("zh") ? "zh" : "en";
const root = document.getElementById("root")!;
const page = (
  <StrictMode>
    <App lang={lang} />
  </StrictMode>
);

// The built pages arrive with their text already in the HTML (scripts/prerender.mjs); in dev the root is empty.
if (root.hasChildNodes()) hydrateRoot(root, page);
else createRoot(root).render(page);
