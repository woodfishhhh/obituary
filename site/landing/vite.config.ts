import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// GitHub Pages serves the project site under /obituary/; the docs live
// under /obituary/docs/ and are built separately (site/docs). The mainland
// China mirror (VITE_SITE=cn) serves from the root. SITE_BASE overrides both.
const base = process.env.SITE_BASE ?? (process.env.VITE_SITE === "cn" ? "/" : "/obituary/");

export default defineConfig({
  base,
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    rollupOptions: {
      input: {
        zh: fileURLToPath(new URL("./index.html", import.meta.url)),
        en: fileURLToPath(new URL("./en/index.html", import.meta.url)),
      },
    },
  },
  server: { port: 5192, strictPort: true },
});
