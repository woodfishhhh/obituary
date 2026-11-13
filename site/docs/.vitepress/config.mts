import { defineConfig, type DefaultTheme } from "vitepress";

// Both builds serve the landing page at the root of obituary.woodfish.site
// (GitHub Pages abroad, the mainland China mirror built with VITE_SITE=cn at
// home); the docs sit under it at docs/. SITE_BASE and SITE_URL override them.
const mirror = process.env.VITE_SITE === "cn";
const siteBase = process.env.SITE_BASE ?? "/";
const base = `${siteBase}docs/`;
const siteUrl = process.env.SITE_URL ?? "https://obituary.woodfish.site";
// Programs fetch from this host, which always reaches the mirror server.
const mirrorHost = "https://cn.obituary.woodfish.site/";

// On the mirror, install lines point at files the mirror serves, so a reader
// who cannot reach GitHub can still follow them.
function toMirror(src: string): string {
  return src
    .replaceAll("https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh", `${mirrorHost}install.sh`)
    .replace(/bash install\.sh --version ([\w.-]+)(?![\w.-]| --mirror)/g, "bash install.sh --version $1 --mirror cn")
    .replaceAll("https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1", `${mirrorHost}install.ps1`)
    .replace(/\.\\install\.ps1 -Version ([\w.-]+)(?![\w.-]| -Mirror)/g, ".\\install.ps1 -Version $1 -Mirror cn");
}
const home = { zh: `${siteUrl}${siteBase}`, en: `${siteUrl}${siteBase}en/` };
const repo = "https://github.com/woodfishhhh/obituary";

function sidebar(prefix: string, t: Record<string, string>): DefaultTheme.SidebarItem[] {
  return [
    {
      text: t.start,
      items: [
        { text: t.quick, link: `${prefix}/` },
        { text: t.how, link: `${prefix}/how-it-works` },
      ],
    },
    {
      text: t.agentsGroup,
      items: [
        { text: t.agents, link: `${prefix}/agents` },
        { text: t.auto, link: `${prefix}/auto-register` },
        { text: t.guard, link: `${prefix}/delete-guard` },
        { text: t.notices, link: `${prefix}/grave-notices` },
      ],
    },
    {
      text: t.more,
      items: [
        { text: t.folders, link: `${prefix}/folder-policies` },
        { text: t.docker, link: `${prefix}/docker` },
        { text: t.desktop, link: `${prefix}/desktop` },
      ],
    },
    {
      text: t.reference,
      items: [
        { text: t.commands, link: `${prefix}/commands` },
        { text: t.uninstall, link: `${prefix}/uninstall` },
        { text: t.faq, link: `${prefix}/faq` },
        { text: t.signing, link: `${prefix}/code-signing` },
        { text: t.privacy, link: `${prefix}/privacy` },
      ],
    },
  ];
}

const zhSidebar = sidebar("", {
  start: "开始", quick: "快速开始", how: "讣告怎么运作",
  agentsGroup: "和 agent 一起用", agents: "接入 agent", auto: "自动登记与提议", guard: "删除护栏", notices: "墓地提醒",
  more: "更多", folders: "文件夹策略", docker: "Docker 容器", desktop: "小窗口、托盘和右键菜单",
  reference: "参考", commands: "命令参考", uninstall: "卸载", faq: "常见问题", signing: "代码签名政策", privacy: "隐私政策",
});

const enSidebar = sidebar("/en", {
  start: "Start", quick: "Quick start", how: "How it works",
  agentsGroup: "With your agents", agents: "Connect agents", auto: "Automatic lifespans", guard: "Delete guard", notices: "Grave notices",
  more: "More", folders: "Folder policies", docker: "Docker containers", desktop: "Window, tray and context menu",
  reference: "Reference", commands: "Commands", uninstall: "Uninstall", faq: "FAQ", signing: "Code signing policy", privacy: "Privacy policy",
});

export default defineConfig({
  base,
  markdown: {
    config(md) {
      if (!mirror) return;
      md.core.ruler.before("normalize", "obit-mirror-links", (state) => {
        state.src = toMirror(state.src);
      });
    },
  },
  cleanUrls: false,
  lastUpdated: false,
  sitemap: { hostname: `${siteUrl}${base}` },
  head: [
    ["link", { rel: "icon", type: "image/png", sizes: "32x32", href: `${base}icon-32.png` }],
    ["link", { rel: "icon", type: "image/png", sizes: "512x512", href: `${base}icon-512.png` }],
    ["link", { rel: "apple-touch-icon", sizes: "180x180", href: `${base}icon-180.png` }],
    ["meta", { name: "theme-color", content: "#ffffff" }],
  ],
  locales: {
    root: {
      label: "简体中文",
      lang: "zh-CN",
      title: "讣告 Obituary 文档",
      titleTemplate: ":title · 讣告",
      description: "讣告 · AI 的回收站。讣告 Obituary 的文档：给文件和文件夹设寿命，到期下葬进墓地，保留期内随时复活。",
      themeConfig: {
        siteTitle: "讣告",
        logoLink: home.zh,
        nav: [
          { text: "首页", link: home.zh, target: "_self", noIcon: true },
          { text: "快速开始", link: "/", activeMatch: "^/$" },
          { text: "文档", link: "/how-it-works", activeMatch: "^/(?!en/).+" },
          { text: "下载", link: `${repo}/releases` },
        ],
        sidebar: { "/en/": enSidebar, "/": zhSidebar },
        outline: { label: "本页内容", level: [2, 3] },
        docFooter: { prev: "上一篇", next: "下一篇" },
        darkModeSwitchLabel: "外观",
        lightModeSwitchTitle: "切换到浅色",
        darkModeSwitchTitle: "切换到深色",
        sidebarMenuLabel: "目录",
        returnToTopLabel: "回到顶部",
        langMenuLabel: "语言",
        notFound: { title: "这一页不在这里", quote: "它可能被改名或挪走了。", linkLabel: "回到文档首页", linkText: "回到文档首页" },
        footer: { message: "以 MIT 许可证开源。", copyright: "讣告 Obituary" },
        editLink: { pattern: `${repo}/edit/main/site/docs/:path`, text: "在 GitHub 上修改这一页" },
      },
    },
    en: {
      label: "English",
      lang: "en",
      link: "/en/",
      title: "Obituary docs",
      titleTemplate: ":title · Obituary",
      description: "Obituary — the trash bin for AI agents. Obituary's documentation: give files and folders a lifespan; when it runs out they are buried in a graveyard and can be resurrected during retention.",
      themeConfig: {
        siteTitle: "Obituary",
        logoLink: home.en,
        nav: [
          { text: "Home", link: home.en, target: "_self", noIcon: true },
          { text: "Quick start", link: "/en/", activeMatch: "^/en/$" },
          { text: "Docs", link: "/en/how-it-works", activeMatch: "^/en/.+" },
          { text: "Download", link: `${repo}/releases` },
        ],
        sidebar: { "/en/": enSidebar, "/": zhSidebar },
        outline: { label: "On this page", level: [2, 3] },
        footer: { message: "Released under the MIT license.", copyright: "Obituary" },
        editLink: { pattern: `${repo}/edit/main/site/docs/:path`, text: "Edit this page on GitHub" },
      },
    },
  },
  themeConfig: {
    logo: { src: "/icon-128.png", alt: "" },
    socialLinks: [{ icon: "github", link: repo }],
    search: {
      provider: "local",
      options: {
        locales: {
          root: {
            translations: {
              button: { buttonText: "搜索", buttonAriaLabel: "搜索文档" },
              modal: {
                displayDetails: "显示详细列表",
                resetButtonTitle: "清除",
                backButtonTitle: "关闭搜索",
                noResultsText: "没有找到",
                footer: { selectText: "选择", navigateText: "切换", closeText: "关闭" },
              },
            },
          },
        },
      },
    },
  },
});
