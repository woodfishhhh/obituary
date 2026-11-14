// Build-time settings for the landing page.

// One address for everyone: obituary.woodfish.site resolves to the mainland
// China mirror inside China and to GitHub Pages elsewhere. Both builds come
// from the same source; `VITE_SITE=cn` selects the mirror (`pnpm build:cn`).
// cn.obituary.woodfish.site always reaches the mirror server and carries what
// programs fetch: release archives and the install scripts.
export const isMirror = import.meta.env.VITE_SITE === "cn";

export const urls = {
  site: "https://obituary.woodfish.site/",
  machine: "https://cn.obituary.woodfish.site/",
  repo: "https://github.com/woodfishhhh/obituary",
};

// The version the download links and install lines name.
export const version = "1.3.0";

// The mirror serves the release archives and the install script, so the
// mirror build makes no request to GitHub. Each build serves its own
// agent-install.md at the site root, so the sentence for an agent is the
// same everywhere.
export const mirrorFiles = {
  download: (file: string) => `${urls.machine}download/v${version}/${file}`,
  installScript: `${urls.machine}install.ps1`,
  installShell: `${urls.machine}install.sh`,
};
export const agentInstallUrl = `${urls.site}agent-install.md`;

// The ICP filing number shown in the mirror build's footer; empty hides it.
export const icp = "赣ICP备2026008680号-1";
