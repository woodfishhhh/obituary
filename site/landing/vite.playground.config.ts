import { fileURLToPath, URL } from "node:url";
import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import squircleCorners from "../../cmd/obit-gui/frontend/scripts/squircle-corners.mjs";

// The logo-wall playground (playground/): its own page, built apart from the site. Everything (script, styles,
// images, fonts) is inlined or addressed relatively, and the script is a classic one rather than a module, so
// dist-playground/index.html also opens straight from the disk: browsers refuse module scripts and
// `crossorigin` resources on file:// pages.
const forFilePages = (): Plugin => ({
  name: "playground-file-pages",
  transformIndexHtml: {
    order: "post",
    handler: (html) => html.replace(/ type="module"/g, " defer").replace(/ crossorigin(="[^"]*")?/g, ""),
  },
});

const here = (p: string) => fileURLToPath(new URL(p, import.meta.url));

export default defineConfig({
  root: here("./playground"),
  base: "./",
  plugins: [react(), tailwindcss(), forFilePages()],
  css: { postcss: { plugins: [squircleCorners()] } },
  resolve: {
    alias: { "@": here("./src"), "@gui": here("../../cmd/obit-gui/frontend/src") },
    dedupe: ["react", "react-dom"],
  },
  build: {
    outDir: here("./dist-playground"),
    emptyOutDir: true,
    assetsInlineLimit: 100_000_000,
    cssCodeSplit: false,
    modulePreload: false,
    rollupOptions: { output: { format: "iife", entryFileNames: "playground.js", assetFileNames: "playground[extname]" } },
  },
  server: { port: 5193, strictPort: true, fs: { allow: [here("../..")] } },
});
