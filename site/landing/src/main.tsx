import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import { App } from "./App";

const lang = document.documentElement.lang.startsWith("zh") ? "zh" : "en";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App lang={lang} />
  </StrictMode>,
);
