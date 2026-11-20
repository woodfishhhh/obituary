import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// Both builds serve from the root of obituary.woodfish.site (GitHub Pages
// with a custom domain, and the mainland China mirror); the docs live under
// /docs/ and are built separately (site/docs). SITE_BASE overrides it.
const base = process.env.SITE_BASE ?? "/";

// The window shown on the page is the desktop app's own front end (cmd/obit-gui/frontend)
// running on its mock data. Its stylesheet assumes it owns the page, so the page-level rules
// are dropped here and its font files are read from where they live.
const guiRoot = fileURLToPath(new URL("../../cmd/obit-gui/frontend", import.meta.url));
const guiStyles = () => ({
  name: "obit-gui-styles",
  enforce: "pre" as const,
  transform(code: string, id: string) {
    const file = id.replaceAll("\\", "/").split("?")[0];
    const root = guiRoot.replaceAll("\\", "/");
    if (file.startsWith(`${root}/src/`) && /\.tsx$/.test(file)) return code.replaceAll('src="icon.png"', `src="${base}icon-128.png"`);
    if (file !== `${root}/src/styles.css`) return null;
    return code
      .replace(/^\* \{ box-sizing: border-box; \}\r?\n/m, "")
      .replace(/^html, body, #root \{[^\r\n]*\}\r?\n/m, "")
      .replaceAll('url("/fonts/', 'url("../public/fonts/');
  },
});

export default defineConfig(({ isSsrBuild }) => ({
  base,
  plugins: [guiStyles(), react(), tailwindcss()],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)), "@gui": `${guiRoot}/src` },
    // The front end sits outside this package: its imports resolve to the copies installed here.
    dedupe: ["react", "react-dom", "lucide-react", "@radix-ui/react-dialog"],
  },
  build: {
    outDir: isSsrBuild ? "dist-ssr" : "dist",
    emptyOutDir: true,
    // The server build (scripts/prerender.mjs) starts from its own entry, not the two pages.
    rollupOptions: isSsrBuild ? {} : {
      input: {
        zh: fileURLToPath(new URL("./index.html", import.meta.url)),
        en: fileURLToPath(new URL("./en/index.html", import.meta.url)),
      },
    },
  },
  server: { port: 5192, strictPort: true, fs: { allow: [fileURLToPath(new URL("../..", import.meta.url))] } },
}));
