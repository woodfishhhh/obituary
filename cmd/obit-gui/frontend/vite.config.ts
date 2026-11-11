import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";

// The built app runs under a CSP: scripts only from the app itself. Inline
// styles stay allowed (React style props, Radix's scroll lock); paths and
// log lines are untrusted text that React escapes. Dev skips the meta tag
// because Vite injects styles inline while serving.
const csp: Plugin = {
  name: "obit-csp",
  apply: "build",
  transformIndexHtml: (html) => html.replace("<title>", `<meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'">
<title>`),
};

// The production build is the app only (embedded by cmd/obit-gui through
// go:embed all:frontend/dist). playground.html exists for `pnpm dev` and is
// never built into dist.
export default defineConfig({
  plugins: [react(), csp],
  base: "./",
  build: {
    outDir: "dist",
    emptyOutDir: true,
    modulePreload: { polyfill: false }, // no inline script under the CSP
    rollupOptions: { input: "index.html" },
  },
  server: { port: 5178, strictPort: true },
});
