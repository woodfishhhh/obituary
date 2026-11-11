// Every sentence on the landing page, in both languages. Claims come from the
// repository's README.md / README.en.md only; keep them in step. The Chinese
// strings here also decide which glyphs scripts/subset-fonts.mjs keeps.

export type Lang = "zh" | "en";

export const repo = "https://github.com/woodfishhhh/obituary";
export const qqGroup = "1135603207";

// Commands are product text and stay identical in both languages; the install
// lines mirror the README.
export const installCode = {
  scoop: ["scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket", "scoop install obituary"],
  winget: ["winget install woodfishhhh.Obituary"],
  script: [
    "irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1",
    ".\\install.ps1 -Version 1.0.0",
  ],
} as const;
export type Method = keyof typeof installCode;

// The sentence a user sends to their agent; it matches README.md and
// README.en.md word for word.
export const agentInstall = {
  zh: "读 https://obituary.woodfish.site/agent-install.md ，照里面的步骤帮我安装讣告并接入你自己。",
  en: "Read https://obituary.woodfish.site/agent-install.md and follow it to install Obituary and connect it to yourself.",
} as const;

// One real run with the delete guard on: the reason the hook gave Claude Code
// when it denied `rm -rf build` (without its JSON envelope), and the commands
// that followed, output as printed.
export const guardRun = [
  {
    cmd: "rm -rf build",
    out: "Obituary's delete guard is on: `'C:/work/obit.exe' rm -rf build` provides restorable deletion; `obit resurrect` restores graveyard entries.",
    deny: true,
  },
  { cmd: "obit rm -rf build", out: "" },
  { cmd: "obit graveyard --here", out: "ID  BURIED            PURGE  SIZE  PATH\n#2  2026-11-11 00:36  in 7d  23 B  C:\\work\\build" },
  { cmd: "obit resurrect --last", out: "#2 resurrected  C:\\work\\build" },
] as const;

// A real run in Git Bash: with OUT empty, the shell turns "/" into Git's
// install folder, and obit rm refuses it.
export const emptyVarRun = {
  cmd: 'OUT=; obit rm -rf "$OUT"/',
  out: "rm: it is dangerous to operate recursively on 'C:/Program Files/Git/' (same as '/')",
};

// The grave notice an agent receives, in the hook's own wording; the path,
// number and dates are sample values.
export const graveNotice =
  'Obituary: "debug.log" was buried as #5 (buried 2026-11-11T09:20:00+08:00, restorable until 2026-11-18T09:20:00+08:00). `obit resurrect 5` restores it.';


export type LookId = "calendar" | "palette";
export type PageId = "upcoming" | "graveyard" | "decisions";

const zh = {
  lang: "zh" as Lang,
  docs: "docs/",
  docsHref: (page: string) => `docs/${page}`,
  switchLang: { label: "English", href: "en/" },
  nav: { why: "为什么", life: "一生", window: "小窗口", install: "安装", docs: "文档" },
  hero: {
    before: "给 AI 写下的文件一个",
    sparkle: "寿命",
    after: "",
    line: "写下的垃圾到期自己走，删掉的东西七天内都能找回。",
    primary: "安装到 Windows",
    secondary: "读文档",
    terms: "Windows 10 和 11，MIT 许可",
    altFront: "讣告小窗口的「一行」外观：在墓地里找文件，回车复活。示例数据。",
    altBack: "讣告小窗口的「日历」外观：每一列是一天，文件排在它火化的那天。示例数据。",
  },
  flow: {
    head: "agent 删掉的，先进墓地",
    lead: "打开删除护栏后，rm -rf、git clean、Remove-Item 换成等价的 obit 命令。",
    values: [
      { title: "默认就有", line: "agent 的调试脚本、日志、截图按规则带上寿命，到期进墓地，七天内能复活。" },
      { title: "想多管一点", line: "写 .obitrules 自定规则，给下载文件夹设策略，会话遗留物列成提议等你批准。" },
    ],
    agents: ["Claude Code", "Codex", "其他 agent"],
    obit: "讣告",
    obitNote: "拦下删除，给出等价命令",
    grave: "墓地",
    graveNote: "默认保留 7 天",
    back: "复活",
    backNote: "obit resurrect",
    end: "火化",
    endNote: "保留期过后",
  },
  story: {
    head: "讣告是被 agent 逼出来的",
    cards: {
      mess: {
        title: "agent 产垃圾，比人清理快",
        line: "规则认得的一生成就带上寿命；认不得的，等你点头才登记。",
        tracked: "Git 跟踪，不碰",
        rule: "按规则登记",
        proposal: "会话结束后提议",
      },
      notice: {
        title: "拿不准有没有用？先下葬",
        line: "七天里 agent 要是又用到它，讣告当场告诉它编号和复活命令。",
        call: "agent 读取",
      },
      guard: {
        title: "怕 agent 一句 rm -rf",
        line: "删除护栏拦下这一次删除，给出等价命令；东西进墓地，一条命令放回原处。",
        steps: ["agent 想删掉 build", "护栏拦下，rm 没有执行", "改用等价命令", "进了墓地，7 天后才火化", "原样放回"],
      },
      shell: {
        title: "怕命令在路上变了样",
        line: "OUT 为空时 Git Bash 把 / 改写成自己的安装目录；obit rm 按真实路径核对，拒绝了。",
      },
    },
  },
  life: {
    head: "一个文件的一生",
    lead: "日历外观里，每个文件排在它下葬或火化的那天。",
    steps: [
      { name: "登记", line: "手动给它寿命，或按规则自动登记；到期前随时续命。", code: "obit add debug.log 2h" },
      { name: "下葬", line: "挪进同一块盘上的墓地，原样保留；复活绝不覆盖。", code: "obit resurrect 3" },
      { name: "火化", line: "保留期（默认 7 天）过后才真正删除。", code: "obit purge" },
    ],
    alt: "讣告小窗口的「日历」外观，即将火化页：每一列是一天，墓地里的每样东西排在它火化的那天。示例数据。",
  },
  window: {
    head: "不会用命令行，也能复活",
    lead: "开始菜单、桌面、托盘都能打开；右键文件就能设寿命。",
    looks: { calendar: "日历", palette: "一行" } as Record<LookId, string>,
    pages: { upcoming: "即将下葬", graveyard: "即将火化", decisions: "待你决定" } as Record<PageId, string>,
    lookLabel: "外观",
    pageLabel: "页面",
    notes: {
      calendar: "每一列是一天，文件排在它下葬或火化的那天；拖到后面某天就延长到那天。",
      palette: "一行输入，键盘直达：搜文件，回车复活或延长。",
    } as Record<LookId, string>,
    pageNotes: {
      upcoming: "还在原处、带着寿命的东西；到期前随时续命，或者饶它一命。",
      graveyard: "已经下葬、还能复活的东西；复活放回原处，绝不覆盖，火化才是真删。",
      decisions: "第一次按规则登记、会话里新起的 Docker 容器，都只列成提议；MCP 里也没有替你批准的工具。",
    } as Record<PageId, string>,
    alt: (look: string, page: string) => `讣告小窗口的「${look}」外观，${page}页，示例数据`,
  },
  table: {
    head: "agent 做什么，讣告接着做什么",
    agentCol: "agent 做的事",
    obitCol: "讣告做的事",
    rows: [
      { agent: "rm -rf build", obit: "打开删除护栏时拦下，给出 obit rm -rf build；东西进墓地，能复活。" },
      { agent: "写了一个 debug.log", obit: "项目同意过之后，按规则带上寿命，到期下葬。" },
      { agent: "obit mktemp 1d", obit: "一天后整个临时目录自己下葬。" },
      { agent: "又用到一个已经下葬的文件", obit: "当场告诉它编号，obit resurrect 放回原处。" },
    ],
    promisesHead: "三个承诺",
    promises: [
      { title: "不碰 Git 跟踪的文件", line: "自动登记只登记 Git 工作区里没被跟踪的东西；目录里有一个被跟踪的文件，整个目录都不碰。" },
      { title: "七天内都能复活", line: "下葬的东西默认保留 7 天，期间 obit resurrect 原样放回（配置成回收站模式时除外）。" },
      { title: "不经你同意不登记", line: "在一个新项目里，讣告第一次想自动登记时只生成提议，你批准之后才开始。" },
    ],
  },
  agents: {
    head: "接到你用的 agent 上",
    lead: "obit init -g --agent 后面写上下面的灰色名字，一次接一家或几家。",
    note: "各家能做到的程度不一样：写文件后的登记、会话开头的提示、删除护栏、MCP、Skill。",
    link: "逐项看各家支持到哪一步",
    href: "agents",
    kinds: {
      hook: { title: "原生 hook", line: "讣告注册到 agent 自己的 hook 里。" },
      plugin: { title: "插件", line: "讣告装一个小插件，插件调用 obit hook。" },
      rules: { title: "规则文件和 MCP", line: "没有可用的 hook，只装规则文件和 MCP 服务。" },
    },
  },
  community: {
    head: "一起把讣告做好",
    lead: "欢迎提 issue 和 PR。最欢迎的是带着修复来的 issue：发现问题，顺手修好，你的头像就会出现在这里。",
    next: "下一个是你",
    howTo: "怎么参与",
    stars: "Star 走势",
    starHistoryAlt: "讣告仓库的 Star 数随时间变化的曲线",
    qqHead: "QQ 用户群",
    qqLine: "用着有问题、有想法，扫码进群聊。",
    qqAlt: "讣告用户群的 QQ 二维码，群号 1135603207",
  },
  install: {
    head: "安装",
    lead: "支持 Windows 10 / 11（x64 和 ARM64）。",
    mirrorFact: "镜像上的压缩包和 GitHub Release 里的是同一份，安装脚本同样先核对 SHA-256。",
    payoff: "装好后重启 agent。它生成的日志、截图按规则带上寿命（每个项目第一次先问你）；打开删除护栏的话，它的 rm -rf 会换成能复活的 obit rm -rf。",
    updateHead: "更新和卸载",
    update: "更新：Scoop 用 scoop update obituary，winget 用 winget upgrade woodfishhhh.Obituary，安装脚本换上新版本号再运行一次。obit update 会告诉你当前版本和该用的升级命令。",
    uninstall: "卸载：obit uninstall，先 --dry-run 看看会撤掉什么；墓地默认保留。",
    network: "讣告唯一的联网是每天最多一次检查新版本，只读公开的发布信息（GitHub 或国内镜像），可以在设置里关掉。",
    mirrorScript: "安装脚本和安装包都从国内镜像下载",
    downloadHead: "直接下载",
    downloads: [
      { label: "x64", file: "obituary_1.0.0_windows_amd64.zip" },
      { label: "ARM64", file: "obituary_1.0.0_windows_arm64.zip" },
      { label: "checksums.txt", file: "checksums.txt" },
    ],
    routeLabel: "安装方式",
    agentHead: "让 AI 装",
    agentLine: "把这句话发给你正在用的 agent（Claude Code、Codex、Cursor……都行），要你拍板的地方它会停下来问你。",
    agentLabel: "发给 agent 的一句话",
    handHead: "自己装",
    handLine: "下面三种方式任选其一，装好后执行 obit doctor 体检。",
    mirrorHandLine: "用安装脚本，或者直接下载压缩包；装好后执行 obit doctor 体检。",
    platformLabel: "系统",
    soonHead: "正在开发，即将推出",
    soonLine: (os: string) => `留下邮箱，${os} 版发布时我们发邮件告诉你。`,
    emailLabel: "邮箱",
    submit: "发布时提醒我",
    privacy: "只用来通知 macOS / Linux 版发布，每封邮件都能退订。",
    closed: "邮件提醒暂未开放，先在 GitHub 上关注发布。",
    sending: "正在提交……",
    sent: "收到了。发布时会发邮件给你。",
    failed: "没提交成功，可以稍后再试，或者先在 GitHub 上关注发布。",
    invalid: "这个邮箱地址看起来不对。",
    busy: "太频繁了，稍后再试。",
    watch: "或者不用留邮箱：在 GitHub 上点 Watch → Custom → Releases。",
    watchLink: "打开 GitHub 仓库",
    methods: { scoop: "Scoop", winget: "winget（审核中）", script: "安装脚本" } as Record<Method, string>,
    scriptNote: "先下载、看一眼，再运行；-WhatIf 只预览。",
    fact: "三种方式下载的都是 GitHub Release 里的同一个压缩包，安装前都核对 SHA-256。",
    then: "再接到 agent 身上",
    init: [
      { label: "Claude Code", code: "obit init -g" },
      { label: "Codex", code: "obit init -g --agent codex" },
    ],
    copy: "复制",
    copied: "已复制",
    copyFailed: "复制失败",
  },
  footer: {
    line: "给文件写好讣告。",
    links: [
      { label: "快速开始", href: "docs/" },
      { label: "命令参考", href: "docs/commands" },
      { label: "常见问题", href: "docs/faq" },
      { label: "GitHub", href: repo },
      { label: "发布记录", href: `${repo}/releases` },
      { label: "代码签名政策", href: "docs/code-signing" },
      { label: "隐私政策", href: "docs/privacy" },
    ],
    legal: "MIT 许可。截图里的文件都是示例数据。",
  },
};

export type Copy = typeof zh;

const en: Copy = {
  lang: "en",
  docs: "docs/en/",
  docsHref: (page: string) => `docs/en/${page}`,
  switchLang: { label: "中文", href: "" },
  nav: { why: "Why", life: "Lifecycle", window: "The window", install: "Install", docs: "Docs" },
  hero: {
    before: "Give AI files a ",
    sparkle: "lifespan",
    after: "",
    line: "Leftovers leave when they expire; deleted files come back for seven days.",
    primary: "Install on Windows",
    secondary: "Read the docs",
    terms: "Windows 10 and 11, MIT licensed",
    altFront: "Obituary's small window in the One line look: find a file in the graveyard and press Enter to resurrect it. Sample data.",
    altBack: "Obituary's small window in the Calendar look: a column per day, each file on the day it is purged. Sample data.",
  },
  flow: {
    head: "What an agent deletes goes to the graveyard first",
    lead: "With the delete guard on, rm -rf, git clean and Remove-Item become the equivalent obit command.",
    values: [
      { title: "Out of the box", line: "Your agent's debug scripts, logs and screenshots get a lifespan by rule, then go to the graveyard, restorable for seven days." },
      { title: "When you want more", line: "Write .obitrules, give your Downloads folder a policy, and review what sessions leave behind as proposals." },
    ],
    agents: ["Claude Code", "Codex", "Other agents"],
    obit: "Obituary",
    obitNote: "denies the delete, offers the equivalent",
    grave: "Graveyard",
    graveNote: "kept 7 days by default",
    back: "Resurrect",
    backNote: "obit resurrect",
    end: "Purge",
    endNote: "after retention",
  },
  story: {
    head: "Agents pushed us into writing it",
    cards: {
      mess: {
        title: "Agents make mess faster than people clean up",
        line: "Recognised output gets a lifespan; the rest waits for your approval.",
        tracked: "tracked by Git, left alone",
        rule: "registered by rule",
        proposal: "proposed at session end",
      },
      notice: {
        title: "Not sure it's still needed? Bury it",
        line: "Needed again within seven days? The agent is told the grave number.",
        call: "the agent reads",
      },
      guard: {
        title: "Afraid of one rm -rf",
        line: "The delete guard stops that one delete and offers the equivalent; one command puts it back.",
        steps: ["the agent deletes build", "the guard denies it; rm never runs", "the equivalent command", "in the graveyard, purged after 7 days", "back as it was"],
      },
      shell: {
        title: "Afraid the command changes on the way",
        line: "With OUT empty, Git Bash turned / into its install folder; obit rm checked the real path and refused.",
      },
    },
  },
  life: {
    head: "The life of a file",
    lead: "In the Calendar look, every file sits on the day it is buried or purged.",
    steps: [
      { name: "Register", line: "Give it a lifespan by hand, or automatically by rule; extend it any time.", code: "obit add debug.log 2h" },
      { name: "Bury", line: "It moves to a graveyard on the same drive and comes back without overwriting.", code: "obit resurrect 3" },
      { name: "Purge", line: "Only after retention (7 days by default) is it really deleted.", code: "obit purge" },
    ],
    alt: "Obituary's small window in the Calendar look, Graveyard page: a column per day, each grave on the day it is purged. Sample data.",
  },
  window: {
    head: "Resurrect without the command line",
    lead: "Open it from Start, the desktop or the tray; right-click a file to give it a lifespan.",
    looks: { calendar: "Calendar", palette: "One line" },
    pages: { upcoming: "Expiring", graveyard: "Graveyard", decisions: "For you" },
    lookLabel: "Look",
    pageLabel: "Page",
    notes: {
      calendar: "A column per day; each file sits on the day it is buried or cremated. Drag it to a later day to extend it.",
      palette: "One line of input, all from the keyboard: find a file, press Enter to resurrect or extend.",
    },
    pageNotes: {
      upcoming: "Things still in place with a lifespan; extend or spare them any time before they expire.",
      graveyard: "Buried and still restorable; resurrect puts things back without overwriting, and only purging really deletes.",
      decisions: "A project's first registration by rule and the containers a session started are only proposed; no MCP tool approves for you.",
    },
    alt: (look: string, page: string) => `Obituary's small window in the ${look} look, ${page} page, with sample data`,
  },
  table: {
    head: "What the agent does, and what Obituary does next",
    agentCol: "The agent",
    obitCol: "Obituary",
    rows: [
      { agent: "rm -rf build", obit: "With the delete guard on, denied with obit rm -rf build offered instead; it goes to the graveyard and can come back." },
      { agent: "writes a debug.log", obit: "Once the project has agreed, it gets a lifespan by rule and is buried when it expires." },
      { agent: "obit mktemp 1d", obit: "A day later the whole scratch folder buries itself." },
      { agent: "reads a file that was already buried", obit: "Is told the grave number on the spot; obit resurrect puts it back." },
    ],
    promisesHead: "Three promises",
    promises: [
      { title: "Git-tracked files are left alone", line: "Automatic registration only takes what Git does not track; a folder holding one tracked file is skipped whole." },
      { title: "Restorable for seven days", line: "Buried things are kept 7 days by default and obit resurrect puts them back as they were (unless you chose recycle mode)." },
      { title: "Nothing without your yes", line: "In a new project, the first automatic registration is only a proposal; it starts once you approve it." },
    ],
  },
  agents: {
    head: "Connect the agents you use",
    lead: "obit init -g --agent followed by the grey names below connects one agent or several.",
    note: "Agents differ in what they support: registration after writes, the session-start notice, the delete guard, MCP and the Skill.",
    link: "See what each one supports",
    href: "agents",
    kinds: {
      hook: { title: "Native hooks", line: "Obituary registers in the agent's own hooks." },
      plugin: { title: "Plugin", line: "Obituary installs a small plugin that calls obit hook." },
      rules: { title: "Rules and MCP", line: "No usable hook; only a rules file and the MCP server." },
    },
  },
  community: {
    head: "Make Obituary better with us",
    lead: "Issues and pull requests are welcome. Best of all is an issue that brings its fix: find a problem, fix it, and your avatar shows up here.",
    next: "You could be next",
    howTo: "How to contribute",
    stars: "Stars over time",
    starHistoryAlt: "Stars of the Obituary repository over time",
    qqHead: "QQ group",
    qqLine: "Chinese-speaking users chat here: scan the code or search the group number in QQ.",
    qqAlt: "QR code of the Obituary QQ group, number 1135603207",
  },
  install: {
    head: "Install",
    lead: "Windows 10 / 11 (x64 and ARM64).",
    mirrorFact: "The mirror serves the same archives as the GitHub release, and the install script checks their SHA-256 the same way.",
    payoff: "Then restart the agent. Its logs and screenshots get a lifespan by rule (each project asks you first); with the delete guard on, its rm -rf becomes a restorable obit rm -rf.",
    updateHead: "Update and uninstall",
    update: "Update: scoop update obituary, winget upgrade woodfishhhh.Obituary, or the install script again with the new version number. obit update shows your version and the command to use.",
    uninstall: "Uninstall: obit uninstall, with --dry-run first to see what it removes; the graveyard stays by default.",
    network: "Obituary's only network use is a check for a new version, at most once a day. It reads only public release information (GitHub or the mirror in mainland China), and you can turn it off in Settings.",
    mirrorScript: "The install script and the archives come from the mainland China mirror",
    downloadHead: "Direct download",
    downloads: [
      { label: "x64", file: "obituary_1.0.0_windows_amd64.zip" },
      { label: "ARM64", file: "obituary_1.0.0_windows_arm64.zip" },
      { label: "checksums.txt", file: "checksums.txt" },
    ],
    routeLabel: "How to install",
    agentHead: "Let your AI do it",
    agentLine: "Send this sentence to the agent you use (Claude Code, Codex, Cursor…). It stops wherever a decision is yours.",
    agentLabel: "The sentence for your agent",
    handHead: "By hand",
    handLine: "Pick one of the three, then run obit doctor to check the installation.",
    mirrorHandLine: "Use the install script or download the archive, then run obit doctor to check the installation.",
    platformLabel: "Platform",
    soonHead: "In development, coming soon",
    soonLine: (os: string) => `Leave your email and we'll tell you when the ${os} version ships.`,
    emailLabel: "Email",
    submit: "Notify me",
    privacy: "Only used to announce the macOS / Linux release; every email has an unsubscribe link.",
    closed: "Email notices aren't open yet; follow the releases on GitHub for now.",
    sending: "Sending…",
    sent: "Got it. We'll email you when it ships.",
    failed: "That didn't go through. Try again later, or follow the releases on GitHub.",
    invalid: "That email address doesn't look right.",
    busy: "Too many tries; try again in a while.",
    watch: "Or skip the email: on GitHub, choose Watch → Custom → Releases.",
    watchLink: "Open the GitHub repository",
    methods: { scoop: "Scoop", winget: "winget (in review)", script: "Install script" },
    scriptNote: "Download it, read it, then run it; -WhatIf previews.",
    fact: "All three download the same archive from the GitHub release and check its SHA-256 before unpacking.",
    then: "Then connect your agents",
    init: [
      { label: "Claude Code", code: "obit init -g" },
      { label: "Codex", code: "obit init -g --agent codex" },
    ],
    copy: "Copy",
    copied: "Copied",
    copyFailed: "Copy failed",
  },
  footer: {
    line: "Every file has a deadline.",
    links: [
      { label: "Quick start", href: "docs/en/" },
      { label: "Commands", href: "docs/en/commands" },
      { label: "FAQ", href: "docs/en/faq" },
      { label: "GitHub", href: repo },
      { label: "Releases", href: `${repo}/releases` },
      { label: "Code signing policy", href: "docs/en/code-signing" },
      { label: "Privacy policy", href: "docs/en/privacy" },
    ],
    legal: "MIT licensed. The files in the screenshots are sample data.",
  },
};

export const copy: Record<Lang, Copy> = { zh, en };
