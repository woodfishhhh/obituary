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
    // In its own cascade layer, below Tailwind's utilities: its bare `button { color: inherit; font: inherit }`
    // would otherwise beat every text colour and font utility on the page's own buttons.
    const sheet = code
      .replace(/^\* \{ box-sizing: border-box; \}\r?\n/m, "")
      .replace(/^html, body, #root \{[^\r\n]*\}\r?\n/m, "")
      .replaceAll('url("/fonts/', 'url("../public/fonts/')
      // Its text stays hidden until its faces arrive, rather than showing in another face and shifting when they do.
      .replaceAll("font-display: swap", "font-display: block");
    return `@layer gui {\n${sheet}\n}`;
  },
});

// The page's own faces never swap in late: with font-display "optional" a face that has not arrived in the
// first fraction of a second (the first screen's are preloaded, so it normally has) is left for the next
// visit, instead of replacing the fallback in the middle of reading and shifting the layout.
const fontsOptional = {
  postcssPlugin: "obit-fonts-optional",
  AtRule: {
    "font-face": (rule: { walkDecls: (name: string, cb: (d: { value: string }) => void) => void; toString: () => string }) => {
      if (/Geist/.test(rule.toString())) rule.walkDecls("font-display", (d) => { d.value = "optional"; });
    },
  },
};

export default defineConfig(({ isSsrBuild }) => ({
  base,
  plugins: [guiStyles(), react(), tailwindcss()],
  css: { postcss: { plugins: [fontsOptional] } },
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)), "@gui": `${guiRoot}/src` },
    // The front end sits outside this package: its imports resolve to the copies installed here.
    dedupe: ["react", "react-dom", "lucide-react", "@radix-ui/react-dialog"],
  },
  build: {
    outDir: isSsrBuild ? "dist-ssr" : "dist",
    emptyOutDir: true,
    // scripts/prerender.mjs reads it to name each page's language module in a modulepreload link.
    manifest: !isSsrBuild,
    // One template: scripts/prerender.mjs writes a page per language from it. The server build starts from its own entry.
    rollupOptions: isSsrBuild ? {} : { input: fileURLToPath(new URL("./index.html", import.meta.url)) },
  },
  server: { port: 5192, strictPort: true, fs: { allow: [fileURLToPath(new URL("../..", import.meta.url))] } },
}));
