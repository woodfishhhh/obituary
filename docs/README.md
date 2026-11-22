<img src="assets/brand/obituary-mark-512.png" width="72" height="72" align="right" alt="讣告 Obituary">

# 讣告 Obituary 文档

[首页](../README.md) · **文档** · [架构](architecture.md)

讣告的文档索引：每份文档讲什么、写给谁，以及从哪里读起。

## 文档一览

| 文档 | 讲什么 | 写给谁 |
|---|---|---|
| [README](../README.md) | 讣告是什么、为什么有它、交给 agent 安装、常用命令 | 第一次来的人 |
| [usage.md](usage.md)（[English](usage.en.md)） | 使用手册：托盘和小窗口、接入 Claude Code、Codex 和其他 agent、Docker、删除护栏、MCP 与 Skill、自动登记、安全设计、后台运行、构建 | 用讣告的人 |
| [architecture.md](architecture.md) | 架构：东西怎么流动，每个决定在哪里做出，哪些约定不能破。包括威胁模型、关卡、各条流程、`core.FS` 契约、数据布局、运行器和测试 | 要改代码的人和 agent |
| [install.md](install.md)（[English](install.en.md)） | 自己动手安装：Scoop、winget、安装脚本和国内镜像，接到 agent 上，体检、更新和卸载 | 想自己装的人 |
| [agent-install.md](agent-install.md) | 一段可以直接发给 agent 的话：让它装好讣告、接到自己身上，要你拍板的地方停下来问你（中英文） | 想让 agent 替自己装的人 |
| [agents.md](agents.md) | agent 覆盖表：30 家 agent 按接入方式分组，写文件后登记、会话提示、删除护栏、MCP、Skill 各支持到哪一步、为什么。由 `obit init --show --format markdown` 生成，不要手改 | 想知道自己用的 agent 能接到什么程度的人 |
| [AGENTS.md](../AGENTS.md) | 代码约定：布局、规则、命令、已知坑 | 在这个仓库里写代码的 agent |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | 怎么提 issue 和 PR，提交前跑什么 | 想参与的人 |
| [发版和镜像](../site/docs/dev/release.md) | 发版后核对下载、镜像索引和 Scoop / Homebrew CLI 实装收据 | 发布维护者 |
| [SECURITY.md](../SECURITY.md) | 可能误删文件的问题怎么私下报告 | 发现安全问题的人 |

## 从哪里读起

- **第一次用**：README 的[快速上手](../README.md#快速上手)，再按需要看使用手册的[接入 Claude Code](usage.md#接入-claude-code) 或[接入 Codex](usage.md#接入-codex)。
- **要改代码**：先读 [AGENTS.md](../AGENTS.md)；改动跨模块时先读 [architecture.md](architecture.md)，尤其是[关卡](architecture.md#关卡)：每类决定只在一处做出，新入口必须调用这些关卡。

## 品牌物料

[`assets/brand/`](assets/brand/) 里只放文档用到的标志，都是透明底 PNG：

| 文件 | 内容 | 用在 |
|---|---|---|
| `obituary-horizontal-light.png` | 双语横版标志，浅色页面用 | README 页头（浅色主题） |
| `obituary-horizontal-dark.png` | 双语横版标志，深色页面用 | README 页头（深色主题） |
| `obituary-mark-512.png` | 512 px 方形标志，墓碑占满画面，从横版标志里裁出 | 各文档页头、程序和托盘图标、小窗口里的标志、头像 |
| `obituary-icon-128.png` | 128 px 图标，留白较多 | 插件清单里要求的图标 |

README 用 `<picture>` 按访问者的主题在浅色版和深色版之间切换。用 PNG 而不用 SVG，是因为 PNG 保留了草叶、水彩渐变和微光的细节，全彩 SVG 是重建的路径，单个约 1.5 MB。完整的品牌包（SVG、16–1024 px 图标、favicon、单色版）没有放进仓库。

## README 图解

[`assets/readme/i18n/`](assets/readme/i18n/) 里是 README 和使用说明里的图解，每种语言两张，浅色、深色各一份，同样用 `<picture>` 切换：

| 文件 | 内容 |
|---|---|
| `assets/readme/i18n/<语言>/lifecycle-light.svg`、`lifecycle-dark.svg` | 10 种语言的生命周期：登记 → 到期 → 下葬进墓地 → 复活或永久删除 |
| `assets/readme/i18n/<语言>/delete-guard-light.svg`、`delete-guard-dark.svg` | 10 种语言的删除护栏：拦下删除命令 → 等价的 `obit` 命令 → 复活 |
| `assets/readme/i18n/strings.json` | 图片文案表，语言代码与 README 文件一致 |
| `assets/readme/i18n/layout.js`、`generate.mjs`、`package.json`、`pnpm-lock.yaml` | 排版引擎、生成脚本，以及固定版本的 Playwright |

图里的命令和说明都取自 README 正文。修改英文源 README 或图片文案后，在 `docs/assets/readme/i18n/` 里运行 `pnpm generate`（第一次先运行 `pnpm install --frozen-lockfile && pnpm browser`），重新生成 10 种语言的浅色、深色变体。生成脚本在无头浏览器里用真实字体量出每段文字的宽度（同时按 Verdana 这类更宽的回退字体留出余量），再排版：每步一张宽卡片，箭头上的说明放在箭头旁的空隙里，长文案按词或按标点折行，不会拆开单词、不会让标点出现在行首、不会留下孤字；不使用横向压缩，说明文字在 880 px 宽度下不小于 14 px。生成后运行 `pnpm check`（即 `scripts/check-readme-diagrams.mjs`）：它在浏览器里取每段文字和每个卡片、命令框、箭头的真实边界框，检查文字是否互相压住、是否越出卡片或命令框、间距和内边距是否够、是否碰到箭头、对比度、孤字和行首标点，以及折行后的文字是否仍与 `strings.json` 一致，同时再用 Verdana 回退字体检查一遍；`pnpm test` 会把图故意弄坏，确认检查脚本仍能抓到；再运行 `python scripts/check-readme-i18n.py` 检查 README 的链接、锚点和图片路径。README、`docs/assets/readme/` 或检查脚本改动时，CI（`readme-diagrams.yml`）会安装 Chromium 把这些检查全部跑一遍。

README 开头的演示动图在 [`assets/demo/`](assets/demo/)：`demo-light.gif`、`demo-dark.gif` 是中文版，`demo-en-*.gif` 是英文版。它们由 [`assets/readme/demo/build.py`](assets/readme/demo/build.py) 生成：先写出 SVG 源文件和动作描述（`*.motion.json`），再逐帧渲染成 GIF，每个约 7 秒、0.6 MB。终端里的文字照抄 `obit` 的真实输出，路径换成了演示目录。要改文案，就改 `build.py` 再重新生成四个 GIF，`build.py` 开头写了命令。
