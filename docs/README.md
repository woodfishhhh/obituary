<img src="assets/brand/obituary-mark-512.png" width="72" height="72" align="right" alt="讣告 Obituary">

# 讣告 Obituary 文档

[首页](../README.md) · **文档** · [架构](architecture.md)

讣告的文档索引：每份文档讲什么、写给谁，以及从哪里读起。

## 文档一览

| 文档 | 讲什么 | 写给谁 |
|---|---|---|
| [README](../README.md) | 用法：常用命令、接入 Claude Code 和 Codex、删除护栏、MCP 服务与 Agent Skill、自动登记、安全设计、后台运行、构建 | 用讣告的人 |
| [architecture.md](architecture.md) | 架构：东西怎么流动，每个决定在哪里做出，哪些约定不能破。包括威胁模型、关卡、各条流程、`core.FS` 契约、数据布局、运行器和测试 | 要改代码的人和 agent |
| [agent-install.md](agent-install.md) | 一段可以直接发给 agent 的话：让它装好讣告、接到自己身上，要你拍板的地方停下来问你（中英文） | 想让 agent 替自己装的人 |
| [agents.md](agents.md) | agent 覆盖表：30 家 agent 按接入方式分组，写文件后登记、会话提示、删除护栏、MCP、Skill 各支持到哪一步、为什么。由 `obit init --show --format markdown` 生成，不要手改 | 想知道自己用的 agent 能接到什么程度的人 |
| [AGENTS.md](../AGENTS.md) | 代码约定：布局、规则、命令、已知坑 | 在这个仓库里写代码的 agent |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | 怎么提 issue 和 PR，提交前跑什么 | 想参与的人 |
| [SECURITY.md](../SECURITY.md) | 可能误删文件的问题怎么私下报告 | 发现安全问题的人 |

## 从哪里读起

- **第一次用**：README 的[快速上手](../README.md#快速上手)，再按需要看[接入 Claude Code](../README.md#接入-claude-code) 或[接入 Codex](../README.md#接入-codex)。
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

[`assets/readme/`](assets/readme/) 里是 README 的两张图解，浅色、深色各一份，同样用 `<picture>` 切换：

| 文件 | 内容 |
|---|---|
| `lifecycle-light.svg`、`lifecycle-dark.svg` | 生命周期：登记 → 到期 → 下葬进墓地 → 复活或火化 |
| `delete-guard-light.svg`、`delete-guard-dark.svg` | 删除护栏：拦下删除命令 → 等价的 `obit` 命令 → 复活 |

图里的命令和说明都取自 README 正文，正文里的这些内容改了，图要跟着改：SVG 是手写的结构，直接改文件里对应的 `<text>`，浅色、深色两份都要改。配色取自品牌标志：主色 `#126CFF`，「复活」上的四角星沿用标志的火花渐变。

README 开头的演示动图在 [`assets/demo/`](assets/demo/)：`demo-light.gif`、`demo-dark.gif` 是中文版，`demo-en-*.gif` 是英文版。它们由 [`assets/readme/demo/build.py`](assets/readme/demo/build.py) 生成：先写出 SVG 源文件和动作描述（`*.motion.json`），再逐帧渲染成 GIF，每个约 7 秒、0.6 MB。终端里的文字照抄 `obit` 的真实输出，路径换成了演示目录。要改文案，就改 `build.py` 再重新生成四个 GIF，`build.py` 开头写了命令。
