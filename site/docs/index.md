# 快速开始

讣告给文件和文件夹设寿命。寿命一到，东西下葬进墓地；保留期内（默认 7 天）随时可以复活，过了保留期才真正删除。它为 AI agent 设计：agent 生成的调试脚本、日志、截图、测试产物，从生成那一刻起就带着寿命，用不着它「记得清理」，也不会被 `rm -rf` 误删。

## 1. 安装

支持 Windows 10 / 11、macOS 和 Linux（x64 和 ARM64）：

| | Windows | macOS | Linux |
|---|---|---|---|
| 怎么装 | Scoop、安装脚本 `install.ps1`（winget 审核中） | Homebrew、安装脚本 `install.sh` | 安装脚本 `install.sh` |
| 每 5 分钟检查到期 | 托盘常驻进程，任务计划程序兜底 | launchd 用户代理 | `systemd --user` 定时器；没有 systemd 时用用户 crontab |
| 命令行、墓地与复活、自动登记（可选）、删除护栏、agent 集成（hook、Skill、MCP）、Docker 容器、文件夹策略 | 有 | 有 | 有 |
| 托盘、小窗口、右键菜单、资源管理器里的下葬日期、开始菜单和桌面快捷方式 | 有 | 用命令行代替：`obit list`、`obit graveyard`、`obit resurrect` | 同 macOS |
| 回收站模式 | 有 | 只用墓地 | 只用墓地 |

**最省事：让 agent 帮你装。** 把这句话发给你正在用的 agent（Claude Code、Codex、Cursor……都行），它会装好讣告、接到自己身上，要你拍板的地方会停下来问你：

```text
读 https://obituary.woodfish.site/agent-install.md ，照里面的步骤帮我安装讣告并接入你自己。
```

**自己装**：按操作系统选择安装方式，装好后执行 `obit doctor` 体检。

### macOS / Linux

macOS 用 Homebrew 一行装好：

```bash
brew install --cask woodfishhhh/tap/obituary
```

也可以下载安装脚本，阅读后运行（macOS 和 Linux 通用）：

```bash
curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh
bash install.sh
# 自定义目录：加 --install-dir "$HOME/bin"；只预览：加 --dry-run
```

脚本默认安装最新正式版（指定版本用 `--version <版本号>`），压缩包分 4 段同时下载（有的线路限制单个连接的速度），不行就自动改用一个连接；识别系统和架构，核对同一来源的 SHA-256 后，把 `obit` 和 `obitd` 装到 `~/.local/bin`。按输出提示把目录加入 PATH，运行 `obit --version`。国内镜像使用 `--mirror cn`，对应脚本地址是 `https://cn.obituary.woodfish.site/install.sh`。

macOS 首次使用先运行 `obit --version` 和 `obitd --help`。若系统要求确认开发者，在「系统设置 → 隐私与安全性」为已核对来源的两个程序分别点「仍要打开」，再重试命令。

WSL 中请把项目和 agent 配置放在 Linux 文件系统（例如 `~/项目`）；`/mnt/c` 等 Windows 挂载盘可能不支持安装所需的安全改名。这种情况会报错并保留原配置，也可以直接使用 Windows 版讣告。

### Windows

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
.\install.ps1          # 默认装最新正式版；-WhatIf 只预览，-NoPath 不改 PATH
```

:::

- 三种方式下载的都是 GitHub Release 里的同一个压缩包，安装前都会核对 SHA-256，对不上就不解压。
- 代码签名：Free code signing provided by [SignPath.io](https://about.signpath.io/), certificate by [SignPath Foundation](https://signpath.org/)。详见[代码签名政策](code-signing)和[隐私政策](privacy)。
- 讣告还没有代码签名。用浏览器手动下载压缩包时，Windows 可能弹出「Windows 已保护你的电脑」，点「更多信息 → 仍要运行」即可；Scoop、winget 和安装脚本一般不会遇到。
- 小窗口 `obit-gui.exe` 需要 WebView2 运行时（Windows 11 自带，Windows 10 多数已经装好），目前只有 x64 版；ARM64 版只有命令行和后台托盘。
装好后先体检一遍：

```powershell
obit doctor              # 只读体检：PATH、obitd、运行器、Claude Code / Codex 集成、数据目录、账本、配置（支持 --json）
```

## 2. 接到 agent 身上

```powershell
obit init -g                 # Claude Code，所有项目；同时装好 Skill（Windows 还会装桌面集成）
obit init -g --agent codex   # Codex
```

装好后重启 agent。其他 agent、MCP 服务和 Agent Skill 见[接入 agent](./agents)。

macOS / Linux 的 agent 配置沿用各自目录：Claude Code 是 `~/.claude`，Codex 是 `~/.codex`，共享 Skill 是 `~/.agents/skills`；也支持 `CLAUDE_CONFIG_DIR` 和 `CODEX_HOME` 指定的目录。

启用每 5 分钟检查到期文件的用户级运行器：

```bash
obit daemon enable --dry-run
obit daemon enable
obit doctor
```

macOS 使用登录用户的 launchd，Linux 使用 `systemd --user`。没有 systemd 的 Linux 可在用户 crontab 中每 5 分钟运行一次 `obitd --once`，填写 `obitd`、`--home` 数据目录和 `--config-dir` 配置目录的绝对路径。agent hook 也会按需启动一次后台检查。

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
| 什么进讣告：agent 登记、右键放进来，以及可选的自动登记 | [什么进讣告](./auto-register) |
| 把 agent 的 `rm -rf` 变成可以复活的下葬 | [删除护栏](./delete-guard) |
| agent 又用到已经下葬的文件时会发生什么 | [墓地提醒](./grave-notices) |
| 给下载、桌面这类文件夹设寿命 | [文件夹策略](./folder-policies) |
| 不用命令行也能用 | [小窗口、托盘和右键菜单](./desktop) |
| 会话里起的 Docker 容器 | [Docker 容器](./docker) |
| 每条命令的用法 | [命令参考](./commands) |
| 撤掉讣告 | [卸载](./uninstall) |
| 其他问题 | [常见问题](./faq) |
