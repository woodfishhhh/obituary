// Put the two builds into one static folder:
//   <out>/        the landing page (landing/dist)
//   <out>/docs/   the docs (docs/.vitepress/dist)
// <out> is dist/ for GitHub Pages and dist-cn/ for the mainland China mirror
// (VITE_SITE=cn); obituary.woodfish.site resolves to one or the other by
// where the visitor is. Both serve /agent-install.md, so the sentence for an
// agent is the same everywhere: the Pages copy installs from GitHub, the
// mirror copy from cn.obituary.woodfish.site, which also serves install.ps1.
import { cp, readFile, rm, stat, writeFile } from "node:fs/promises";

const mirror = process.env.VITE_SITE === "cn";
const root = new URL("../", import.meta.url);
const repo = new URL("../", root);
const out = new URL(mirror ? "dist-cn/" : "dist/", root);
const parts = [
  [new URL("landing/dist/", root), out],
  [new URL("docs/.vitepress/dist/", root), new URL("docs/", out)],
];

for (const [from] of parts) {
  await stat(from).catch(() => {
    throw new Error(`missing build output ${from.pathname}; run pnpm build`);
  });
}
await rm(out, { recursive: true, force: true });
for (const [from, to] of parts) await cp(from, to, { recursive: true });

// The mirror's copy of the agent install guide installs only through the
// mirror: Scoop (and winget, once listed) download from GitHub, so their lines go.
function mirrorGuide(text, host) {
  const swaps = [
    ["https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh", `${host}install.sh`],
    ["bash install.sh --version VERSION", "bash install.sh --version VERSION --mirror cn"],
    ["https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1", `${host}install.ps1`],
    [".\\install.ps1 -Version 1.0.0", ".\\install.ps1 -Version 1.0.0 -Mirror cn"],
    ["，按这个顺序选第一个可用的：", "，用国内镜像上的安装脚本："],
    [", using the first option that is available:", ", with the install script from the mainland China mirror:"],
    ["   - 没有 scoop：", "   - "],
    ["   - no scoop: ", "   - "],
  ];
  for (const [from, to] of swaps) {
    if (!text.includes(from)) throw new Error(`docs/agent-install.md changed; update the mirror copy in assemble.mjs (${from})`);
    text = text.replaceAll(from, to);
  }
  const lines = text.split("\n").filter((line) => !/^\s+- (有 scoop|有 winget|scoop:|winget:)/.test(line));
  const guide = lines.join("\n");
  if (/github\.com|githubusercontent/.test(guide)) throw new Error("the mirror's agent install guide still points at GitHub");
  return guide;
}

const guide = await readFile(new URL("docs/agent-install.md", repo), "utf8");
if (mirror) {
  await writeFile(new URL("agent-install.md", out), mirrorGuide(guide, "https://cn.obituary.woodfish.site/"));
  await cp(new URL("packaging/install.ps1", repo), new URL("install.ps1", out));
} else {
  await writeFile(new URL("agent-install.md", out), guide);
  // GitHub Pages serves this build at the custom domain.
  await writeFile(new URL("CNAME", out), "obituary.woodfish.site\n");
}
await cp(new URL("install.sh", repo), new URL("install.sh", out));
console.log(`assembled ${out.pathname}`);
