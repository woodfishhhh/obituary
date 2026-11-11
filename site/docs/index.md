# 快速开始

讣告给文件和文件夹设寿命。寿命一到，东西下葬进墓地；保留期内（默认 7 天）随时可以复活，过了保留期才真正删除。它为 AI agent 设计：agent 生成的调试脚本、日志、截图、测试产物，从生成那一刻起就带着寿命，用不着它「记得清理」，也不会被 `rm -rf` 误删。

## 1. 安装

目前只支持 Windows 10 / 11（x64 和 ARM64）。

**最省事：让 agent 帮你装。** 把这句话发给你正在用的 agent（Claude Code、Codex、Cursor……都行），它会装好讣告、接到自己身上，要你拍板的地方会停下来问你：

```text
读 https://obituary.woodfish.site/agent-install.md ，照里面的步骤帮我安装讣告并接入你自己。
```

**自己装**：下面三种方式任选其一，装好后执行 `obit doctor` 体检。

::: code-group

```powershell [Scoop]
scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket
scoop install obituary
```

```powershell [winget]
# 审核中，通过后可用
winget install woodfishhhh.Obituary
```

```powershell [安装脚本]
# 先下载、看一眼，再运行（装到 %LOCALAPPDATA%\Programs\obituary，加进用户 PATH，建开始菜单快捷方式）
irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1
.\install.ps1 -Version 1.0.0          # -WhatIf 只预览，-NoPath 不改 PATH
```

:::

- 三种方式下载的都是 GitHub Release 里的同一个压缩包，安装前都会核对 SHA-256，对不上就不解压。
- 1.0.0 还没有代码签名。用浏览器手动下载压缩包时，Windows 可能弹出「Windows 已保护你的电脑」，点「更多信息 → 仍要运行」即可；Scoop、winget 和安装脚本一般不会遇到。
- 小窗口 `obit-gui.exe` 需要 WebView2 运行时（Windows 11 自带，Windows 10 多数已经装好），目前只有 x64 版；ARM64 版只有命令行和后台托盘。
装好后先体检一遍：

```powershell
obit doctor              # 只读体检：PATH、obitd、运行器、Claude Code / Codex 集成、数据目录、账本、配置（支持 --json）
```

## 2. 接到 agent 身上

```powershell
obit init -g                 # Claude Code，所有项目；同时装好 Skill、右键菜单和快捷方式
obit init -g --agent codex   # Codex
```

装好后重启 agent。其他 agent、MCP 服务和 Agent Skill 见[接入 agent](./agents)。

## 3. 常用命令

`<编号>` 换成 `obit add`、`obit bury` 回执里或 `obit list`、`obit graveyard` 列出的编号。

```powershell
obit add debug.log 2h        # 2 小时后收走（别名 mark）
obit list                    # 谁还剩多久
obit extend <编号> 1d        # 续命 1 天
obit bury old-build          # 立刻下葬
obit graveyard               # 墓地里有什么、还剩几天被火化
obit resurrect <编号>        # 复活：放回原处，绝不覆盖
obit resurrect --last        # 撤销上一次删除
```

完整的命令见[命令参考](./commands)。

## 接下来

| 想知道 | 看这里 |
|---|---|
| 登记、下葬、复活、火化怎么运作，墓地放在哪里 | [讣告怎么运作](./how-it-works) |
| 接到 Claude Code、Codex 和其他 agent 上 | [接入 agent](./agents) |
| agent 的产物怎么自动带上寿命，哪些要你点头 | [自动登记与提议](./auto-register) |
| 把 agent 的 `rm -rf` 变成可以复活的下葬 | [删除护栏](./delete-guard) |
| agent 又用到已经下葬的文件时会发生什么 | [墓地提醒](./grave-notices) |
| 给下载、桌面这类文件夹设寿命 | [文件夹策略](./folder-policies) |
| 不用命令行也能用 | [小窗口、托盘和右键菜单](./desktop) |
| 会话里起的 Docker 容器 | [Docker 容器](./docker) |
| 每条命令的用法 | [命令参考](./commands) |
| 撤掉讣告 | [卸载](./uninstall) |
| 其他问题 | [常见问题](./faq) |
