// Build the landing page and the docs, then assemble them.
//   node scripts/build.mjs        GitHub Pages build into dist/
//   node scripts/build.mjs cn     mainland China mirror build into dist-cn/
import { execSync } from "node:child_process";

const mirror = process.argv[2] === "cn";
const env = { ...process.env, ...(mirror ? { VITE_SITE: "cn" } : {}) };
const run = (cmd) => execSync(cmd, { stdio: "inherit", env, cwd: new URL("../", import.meta.url) });

run("node scripts/community.mjs");
run("pnpm --filter landing build");
run("pnpm --filter docs build");
run("node scripts/assemble.mjs");
