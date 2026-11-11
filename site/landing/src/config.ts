// Build-time settings for the landing page.

// One address for everyone: obituary.woodfish.site resolves to the mainland
// China mirror inside China and to GitHub Pages elsewhere. Both builds come
// from the same source; `VITE_SITE=cn` selects the mirror (`pnpm build:cn`).
// cn.obituary.woodfish.site always reaches the mirror server and carries what
// programs fetch: release archives, the install script, the waitlist API.
export const isMirror = import.meta.env.VITE_SITE === "cn";

export const urls = {
  site: "https://obituary.woodfish.site/",
  machine: "https://cn.obituary.woodfish.site/",
  repo: "https://github.com/woodfishhhh/obituary",
};

// The version the download links and install lines name.
export const version = "1.0.0";

// The mirror serves the release archives and the install script, so the
// mirror build makes no request to GitHub. Each build serves its own
// agent-install.md at the site root, so the sentence for an agent is the
// same everywhere.
export const mirrorFiles = {
  download: (file: string) => `${urls.machine}download/v${version}/${file}`,
  installScript: `${urls.machine}install.ps1`,
};
export const agentInstallUrl = `${urls.site}agent-install.md`;

// The ICP filing number shown in the mirror build's footer; empty hides it.
export const icp = "赣ICP备2026008680号-1";

// The macOS / Linux launch-notice lists on our own server. The form POSTs
// JSON { email, name, list_uuids } to `endpoint`; the server accepts requests
// from the site origin only, so `pnpm dev` answers with a local
// stand-in (see sections/Install.tsx). An empty endpoint or list hides the
// form and leaves the GitHub release notification route. Emails are never
// stored or logged in this repository.
export const waitlist: { endpoint: string; lists: { macos: string; linux: string } } = {
  endpoint: "https://cn.obituary.woodfish.site/api/public/subscription",
  lists: {
    macos: "29503a49-33f5-4a61-bca4-cafca03bb699",
    linux: "f896dbb8-c27f-4b27-a297-2052986685b4",
  },
};
