# 命令参考

`<编号>` 换成 `obit add`、`obit bury` 回执里或 `obit list`、`obit graveyard` 列出的编号。时长单位：`s` `m` `h` `d` `w`，可以组合（`1d12h`）。

## 寿命

```powershell
obit add debug.log 2h                       # 2 小时后收走（别名 mark）
obit add output\shots 1d12h                 # 文件夹也可以
obit add report.zip --at "2026-12-01 18:00" # 指定时刻
obit list                                   # 谁还剩多久
obit extend <编号> 1d                       # 续命 1 天
obit spare <编号>                           # 饶它一命：不再跟踪，文件保留（同 cancel）
obit reap                                   # 现在就把已到期的下葬（同 sweep，--dry-run 只预览）
```

## 墓地

```powershell
obit bury old-build                         # 立刻下葬
obit graveyard                              # 墓地里有什么、还剩几天被火化（--here 只看当前目录下的）
obit resurrect <编号>                       # 复活：放回原处；原处已有同名的，改名放回，绝不覆盖
obit resurrect --last                       # 撤销上一次删除：一条 obit bury / obit rm 埋下的几项一起回来
obit purge                                  # 火化保留期已过的（--all 全部，先确认；--dry-run 预览）
obit purge <编号>                           # 立即火化这一座（永久删除，先确认；非终端要加 --yes）
```

`resurrect` 和 `purge` 里的纯数字（`5`、`#5`）一律当编号，找不到就报错，不会退回去当成文件名；文件名本身是数字时写成 `./5` 或绝对路径。

## 删除包装

```powershell
obit rm -rf build            # 和 rm 一样的常用参数，每个路径都走下葬
obit rmdir empty-dir
obit git-clean -fdx
obit remove-item .\build     # 只接受一个字面路径
```

## 规则与提议

```powershell
obit rules                       # 当前项目生效的规则
obit rules check debug.log out/  # 预览：会登记什么、按哪条规则
obit rules apply debug.log       # 按规则登记
obit trust                       # 信任这个项目的 .obitrules（非交互环境加 --yes）
obit proposals                   # 等你点头的提议，和没问你就登记了的东西
obit approve 3                   # 同意
obit reject 3                    # 拒绝
$dir = obit mktemp 1d            # 会自动下葬的临时目录
```

## 文件夹策略与 Docker

```powershell
obit watch C:\Users\你\Downloads 2w
obit watch --list
obit watch --remove C:\Users\你\Downloads
obit docker add web-1 3d
obit docker list
```

运行 `obit mark --docker-volume cache 3d`（也可用 `add`）让 `cache` 三天后下葬；要立即下葬，运行 `obit bury --docker-volume -- cache`。[Docker 容器和卷](./docker)

## 接入与后台

```powershell
obit init                       # Claude Code 的 hook 和 Skill、右键菜单、开始菜单和桌面快捷方式，托盘随登录启动
obit init -g --agent codex      # Codex
obit init -g --guard-rm         # 删除护栏
obit init -g --mcp              # 加装 MCP 服务
obit init --no-context-menu --no-shortcuts --no-skill   # 不装右键菜单、快捷方式或 Skill
obit init -g --uninstall --no-context-menu   # 卸载时留下右键菜单，其余照常撤掉；--no-shortcuts、--no-skill 同理
obit init -g --show             # 装了什么
obit daemon enable              # 托盘随登录启动，并装兜底的定时任务
obit daemon status
obit daemon disable
```

桌面版启用监督后，托盘常驻进程在到期时处理，另有独立任务每小时兜底一次。未启用托盘监督的数据目录每 5 分钟检查一次。

## 查询与体检

```powershell
obit doctor              # 体检：一行一个方面，完整报告存成 HTML（--open 打开，--verbose 看全文，--json）
obit stats               # 悼词：埋了多少、腾出多少空间、复活过几次（同 eulogy）
obit log                 # 生平记录：做过的所有事
obit config              # 查看配置；--create 生成带注释的默认配置
obit config telemetry off # 关闭使用统计并清除本地安装 ID；不影响查更新
obit hook audit          # 设了 OBIT_HOOK_AUDIT=1 时，查看每次 hook 收到了什么、做了什么
obit uninstall           # 撤掉讣告装过的所有东西（先 --dry-run 看看）
```

查询和结果类命令（`list`、`graveyard`、`proposals`、`stats`、`mark`、`bury`、`resurrect` 等）支持 `--json`，方便脚本和 AI agent 调用；`obit rm` 这类删除包装器照原命令的格式输出，不支持 `--json`。

## 环境变量

| 变量 | 作用 |
|---|---|
| `OBIT_DISABLED=1` | 临时关掉 hook |
| `OBIT_TELEMETRY=0`、`DO_NOT_TRACK=1` | 禁止当前进程上报使用统计；CI 默认不上报 |
| `OBIT_GUARD_RM=0` | 临时关掉删除护栏 |
| `OBIT_HOOK_AUDIT=1` | 记下每次 hook 收到了什么、做了什么 |
| `OBIT_TRUST_PROJECT_RULES=1` | CI 里信任项目的 `.obitrules` |
| `OBIT_HOME`、`OBIT_CONFIG_DIR`、`OBIT_USER_HOME` | 换数据目录、配置目录、用户主目录 |



## 删除兼容参数

- git-clean 接受 Git clean 的 -d、-f、-n、-q、-x、-X、-e pattern、--exclude pattern 和 --；也接受 --dry-run、--force、--quiet。
- rm 接受常用的 -r、-R、-f、-d、-v、--recursive、--force、--dir、--verbose、--preserve-root、--，以及其后的路径。
- rmdir 只接受 -v 或 --verbose 及路径；不接受 rmdir -p 等其他参数。
- Windows 的 remove-item 只接受 -Recurse、-Force、-Path 或 -LiteralPath，并且只能有一个字面路径。需要通配符时在 Bash 中使用 obit rm。

<!-- generated-command-reference:start -->
## 参数参考

### `obit`

讣告（Obituary），AI agent 的回收站：给文件设定寿命，寿命用完就下葬进墓地，墓地里的东西可以复活

用法: `obit [command]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | obit 的帮助 |
| `--version` | `-v` | `false` | obit 的版本信息 |

### `obit add`

给路径设定寿命（对已跟踪的路径再次登记，会重置它的到期时间）

别名: `add`, `mark`
用法: `obit add <path>... <duration> | add <path>... --at <time> [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--at` | 无 | `""` | 在指定的本地时间到期："2006-01-02 15:04"、"2006-01-02" 或 "15:04" |
| `--docker-volume` | 无 | `false` | 把参数当作 Docker 卷名 |
| `--force` | 无 | `false` | 允许处理含有 .git 仓库的文件夹 |
| `--help` | `-h` | `false` | add 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |
| `--no-daemon` | 无 | `false` | 不启动后台进程 |
| `--note` | 无 | `""` | 自由文本备注，显示在列表中 |

示例：

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`

批准提议：登记提议里列出的内容

用法: `obit approve <proposal id>... [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | approve 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |

### `obit bury`

立刻把路径移入墓地（用 obit resurrect 恢复）

用法: `obit bury <path>... [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--docker-volume` | 无 | `false` | 把参数当作 Docker 卷名 |
| `--force` | 无 | `false` | 允许处理含有 .git 仓库的文件夹 |
| `--help` | `-h` | `false` | bury 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |
| `--note` | 无 | `""` | 随这次下葬一并保存的自由文本备注 |

### `obit cancel`

饶过路径或容器 ID：不再跟踪，对象原样保留

别名: `cancel`, `spare`
用法: `obit cancel <id|path>... [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | cancel 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |

### `obit completion`

为指定的 shell 生成 obit 的自动补全脚本。

用法: `obit completion [command]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | completion 的帮助 |

### `obit completion bash`

为 bash shell 生成自动补全脚本。

用法: `obit completion bash`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | bash 的帮助 |
| `--no-descriptions` | 无 | `false` | 关闭补全项的说明 |

### `obit completion fish`

为 fish shell 生成自动补全脚本。

用法: `obit completion fish [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | fish 的帮助 |
| `--no-descriptions` | 无 | `false` | 关闭补全项的说明 |

### `obit completion powershell`

为 powershell 生成自动补全脚本。

用法: `obit completion powershell [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | powershell 的帮助 |
| `--no-descriptions` | 无 | `false` | 关闭补全项的说明 |

### `obit completion zsh`

为 zsh shell 生成自动补全脚本。

用法: `obit completion zsh [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | zsh 的帮助 |
| `--no-descriptions` | 无 | `false` | 关闭补全项的说明 |

### `obit config`

显示配置文件和当前生效的设置（--create 写入默认配置）

用法: `obit config [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--create` | 无 | `false` | 按默认值写出配置文件 |
| `--dry-run` | 无 | `false` | 预演 --max-size 会改动什么及将火化哪些墓地，不修改任何文件 |
| `--help` | `-h` | `false` | config 的帮助 |
| `--language` | 无 | `""` | 设置语言：auto、en 或 zh-CN |
| `--max-size` | 无 | `""` | 设置墓地容量上限（如 5GB、500MB；0 或 unlimited 表示不限制） |
| `--yes` | 无 | `false` | 不提示确认即应用更低的 --max-size 并火化最早的墓地（agent 请先征得用户同意） |

### `obit config telemetry`

使用统计：随机生成的安装 ID、版本、操作系统、架构、界面语言和安装渠道，用来统计安装数量和各版本的使用情况。不发送文件、路径或命令，也不保存 IP 地址和 User-Agent。用 obit config telemetry off 或 OBIT_TELEMETRY=0 关闭。

用法: `obit config telemetry on|off [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | telemetry 的帮助 |

### `obit daemon`

控制后台进程（obitd）

用法: `obit daemon [command]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | daemon 的帮助 |

### `obit daemon disable`

移除定时任务和常驻进程

用法: `obit daemon disable [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--dry-run` | 无 | `false` | 只打印改动 |
| `--help` | `-h` | `false` | disable 的帮助 |

### `obit daemon enable`

启用用户定时任务（启用托盘监督后每小时一次，否则每 5 分钟一次）；在 Windows 上同时启用登录托盘

用法: `obit daemon enable [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--dry-run` | 无 | `false` | 只打印改动 |
| `--help` | `-h` | `false` | enable 的帮助 |
| `--resident` | 无 | `false` | 只启用托盘，不改动兜底的定时任务 |

### `obit daemon start`

立即启动后台进程

用法: `obit daemon start [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | start 的帮助 |

### `obit daemon status`

显示后台进程的状态和下一次到期时间

用法: `obit daemon status [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | status 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |

### `obit daemon stop`

停止后台进程

用法: `obit daemon stop [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | stop 的帮助 |

### `obit docker`

按不可变的完整 ID 和创建时间跟踪 Docker 容器。到期时停止容器并保留它的可写层，复活时启动它。过了 [graveyard] retention_days 之后，火化会移除已停止的容器，但不移除卷；回收站模式下同样如此。镜像、卷和网络都会保留。

用法: `obit docker [command]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | docker 的帮助 |

### `obit docker add`

登记一个 Docker 容器；到期时停止它

用法: `obit docker add <container name|ID> <lifespan> [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | add 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |

### `obit docker list`

列出已登记的 Docker 容器

用法: `obit docker list [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | list 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |

### `obit doctor`

检查安装、集成和数据，只会写下自己的报告

用法: `obit doctor [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | doctor 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |
| `--open` | 无 | `false` | 用默认浏览器打开完整的 HTML 报告 |
| `--verbose` | 无 | `false` | 在终端打印完整文字版：每项检查的详情和 agent 的能力详情 |

### `obit extend`

续命：把到期时间往后推（已经到期的从现在起算）

用法: `obit extend <id|path>... <duration> [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | extend 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |

示例：

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`

把 git clean 试运行选中的路径下葬

用法: `obit git-clean [options] [--] [pathspec]... [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | git-clean 的帮助 |

### `obit graveyard`

列出墓地里还能复活的东西

别名: `graveyard`, `graves`
用法: `obit graveyard [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | graveyard 的帮助 |
| `--here` | 无 | `false` | 只显示原路径在当前目录及其所有下级目录中的项目 |
| `--json` | 无 | `false` | 输出 JSON |

### `obit help`

显示本程序中任意命令的帮助。

用法: `obit help [command] [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help 的帮助 |

### `obit hook`

agent hook 的入口（由 agent 调用，不是给人用的）

用法: `obit hook [command]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | hook 的帮助 |

### `obit hook antigravity`

处理来自 stdin 的一个 antigravity hook 事件；出错时放行

用法: `obit hook antigravity [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | antigravity 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook audit`

显示 hook 和删除护栏的审计记录

用法: `obit hook audit [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | audit 的帮助 |
| `--lines` | `-n` | `40` | 每个来源显示的行数 |

### `obit hook claude`

处理来自 stdin 的一个 claude hook 事件；出错时放行

用法: `obit hook claude [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | claude 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook cline`

处理来自 stdin 的一个 cline hook 事件；出错时放行

用法: `obit hook cline [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | cline 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook codebuddy`

处理来自 stdin 的一个 codebuddy hook 事件；出错时放行

用法: `obit hook codebuddy [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | codebuddy 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook codebuddy-ide`

处理来自 stdin 的一个 codebuddy-ide hook 事件；出错时放行

用法: `obit hook codebuddy-ide [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | codebuddy-ide 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook codex`

处理来自 stdin 的一个 codex hook 事件；出错时放行

用法: `obit hook codex [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | codex 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook copilot`

处理来自 stdin 的一个 copilot hook 事件；出错时放行

用法: `obit hook copilot [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | copilot 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook cursor`

处理来自 stdin 的一个 cursor hook 事件；出错时放行

用法: `obit hook cursor [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | cursor 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook deepseek-harness`

处理来自 stdin 的一个 deepseek-harness hook 事件；出错时放行

用法: `obit hook deepseek-harness [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | deepseek-harness 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook droid`

处理来自 stdin 的一个 droid hook 事件；出错时放行

用法: `obit hook droid [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | droid 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook gemini`

处理来自 stdin 的一个 gemini hook 事件；出错时放行

用法: `obit hook gemini [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | gemini 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook hermes`

处理来自 stdin 的一个 hermes hook 事件；出错时放行

用法: `obit hook hermes [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | hermes 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook kilocode`

处理来自 stdin 的一个 kilocode hook 事件；出错时放行

用法: `obit hook kilocode [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | kilocode 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook kimi`

处理来自 stdin 的一个 kimi hook 事件；出错时放行

用法: `obit hook kimi [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | kimi 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook lingma`

处理来自 stdin 的一个 lingma hook 事件；出错时放行

用法: `obit hook lingma [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | lingma 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook minimax-code`

处理来自 stdin 的一个 minimax-code hook 事件；出错时放行

用法: `obit hook minimax-code [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | minimax-code 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook omp`

处理来自 stdin 的一个 omp hook 事件；出错时放行

用法: `obit hook omp [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | omp 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook openclaw`

处理来自 stdin 的一个 openclaw hook 事件；出错时放行

用法: `obit hook openclaw [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | openclaw 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook opencode`

处理来自 stdin 的一个 opencode hook 事件；出错时放行

用法: `obit hook opencode [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | opencode 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook pi`

处理来自 stdin 的一个 pi hook 事件；出错时放行

用法: `obit hook pi [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | pi 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook qoder`

处理来自 stdin 的一个 qoder hook 事件；出错时放行

用法: `obit hook qoder [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | qoder 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook qoder-cn`

处理来自 stdin 的一个 qoder-cn hook 事件；出错时放行

用法: `obit hook qoder-cn [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | qoder-cn 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook trae`

处理来自 stdin 的一个 trae hook 事件；出错时放行

用法: `obit hook trae [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | trae 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook trae-cn`

处理来自 stdin 的一个 trae-cn hook 事件；出错时放行

用法: `obit hook trae-cn [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | trae-cn 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook vibe`

处理来自 stdin 的一个 vibe hook 事件；出错时放行

用法: `obit hook vibe [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | vibe 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook zcode`

处理来自 stdin 的一个 zcode hook 事件；出错时放行

用法: `obit hook zcode [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--event` | 无 | `""` | payload 里没有事件名时使用的宿主事件名 |
| `--help` | `-h` | `false` | zcode 的帮助 |
| `--notice-only` | 无 | `false` | 只传递提示性上下文，不启用删除护栏 |

### `obit hook finish`

计算会话留下的文件（由 hook 在后台启动）

用法: `obit hook finish [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | finish 的帮助 |
| `--project` | 无 | `""` | 项目根目录 |
| `--session` | 无 | `""` | agent 会话 ID |

### `obit init`

为 agent 安装（或用 --uninstall 卸载）讣告的 hook 和说明

用法: `obit init [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--agent` | 无 | `[claude]` | 要接入的 agent（可重复指定，也可用逗号分隔；默认 claude） |
| `--context-menu` | 无 | `false` | 安装文件管理器右键菜单（默认启用） |
| `--dry-run` | 无 | `false` | 只打印改动 |
| `--format` | 无 | `"text"` | --show 的输出格式：text 或 markdown（agent 支持范围） |
| `--global` | `-g` | `false` | 对当前用户生效，而不是只对这个项目生效 |
| `--guard-rm` | 无 | `false` | 拒绝受支持的删除命令，并给出可以复活的 obit 替代命令 |
| `--help` | `-h` | `false` | init 的帮助 |
| `--mcp` | 无 | `false` | 注册讣告的 MCP stdio 服务 |
| `--no-context-menu` | 无 | `false` | 跳过文件管理器右键菜单 |
| `--no-shortcuts` | 无 | `false` | 不创建开始菜单和桌面快捷方式 |
| `--no-skill` | 无 | `false` | 不改动讣告的 Agent Skill |
| `--show` | 无 | `false` | 显示已安装的内容 |
| `--skill` | 无 | `false` | 安装讣告的 Agent Skill（默认启用） |
| `--uninstall` | 无 | `false` | 移除讣告添加的内容 |
| `--verbose` | 无 | `false` | 与 --show 一起用：列出每个 agent 的能力详情 |
| `--yes` | 无 | `false` | 不询问，直接执行 |

示例：

```text
obit init -g  # Claude Code，对所有项目生效（~/.claude）
obit init  # Claude Code，只对这个项目生效（.claude/）
obit init -g --agent codex  # Codex（~/.codex/hooks.json + AGENTS.md）
obit init -g --uninstall  # 只移除讣告添加的内容
obit init -g --show  # 已安装的内容
obit init --show --format markdown > docs/agents.md
```

### `obit list`

显示已跟踪的路径和 Docker 容器，最先到期的排在最前

别名: `list`, `ls`
用法: `obit list [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | list 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |

### `obit log`

显示讣告所做的一切的审计日志

用法: `obit log [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | log 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |
| `--lines` | `-n` | `20` | 显示的事件条数 |

### `obit mcp`

通过 MCP stdio 提供讣告的工具

用法: `obit mcp [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | mcp 的帮助 |

### `obit mktemp`

创建一个会自己下葬的临时目录，并打印它的路径

用法: `obit mktemp <lifespan> [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | mktemp 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |
| `--name` | 无 | `"tmp"` | 目录名的前缀 |
| `--session` | 无 | `""` | 目录所属的 agent 会话 |

示例：

```text
dir=$(obit mktemp 1d)
obit mktemp 2h --name screenshots
```

### `obit proposals`

等你批准的登记，以及没问你就已经做了的登记

用法: `obit proposals [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | proposals 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |

### `obit purge`

火化选中的或保留期已过的已下葬文件。--all 和明确指定的编号或路径会跳过保留期，所以会先询问；不在终端里运行时必须加 --yes。Docker 容器必须等 [graveyard] retention_days 过完才能火化，用 --all 或明确指定 ID 也一样；正在运行的容器视为已复活。移除 Docker 容器时保留卷，也从不强制移除。回收站模式下对容器的处理完全相同。

别名: `purge`, `cremate`
用法: `obit purge [id|original path]... [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--all` | 无 | `false` | 火化全部已下葬的内容，不只是过了保留期的 |
| `--dry-run` | 无 | `false` | 只显示会发生什么 |
| `--help` | `-h` | `false` | purge 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |
| `--yes` | 无 | `false` | 不询问，直接火化 --all 或所选的已下葬内容（agent：先问用户） |

### `obit reject`

拒绝提议

用法: `obit reject <proposal id>... [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | reject 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |

### `obit remove-item`

用有限的一部分 PowerShell 参数下葬文件系统项目。

用法: `obit remove-item [-Recurse] [-Force] [-Path|-LiteralPath] <path> [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | remove-item 的帮助 |

### `obit resurrect`

恢复文件，不覆盖已有的内容；按 ID 启动已下葬的 Docker 容器

别名: `resurrect`, `raise`
用法: `obit resurrect <id|original path>... | --last [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | resurrect 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |
| `--last` | 无 | `false` | 恢复当前会话或 Git 项目里最近一批删除的内容 |

### `obit rm`

立刻把路径移入墓地（用 obit resurrect 恢复）

用法: `obit rm [options] <path>... [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | rm 的帮助 |

### `obit rmdir`

立刻把路径移入墓地（用 obit resurrect 恢复）

用法: `obit rmdir [options] <path>... [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | rmdir 的帮助 |

### `obit rules`

显示这里生效的规则：内置规则，以及信任之后项目的 .obitrules

用法: `obit rules [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | rules 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |

### `obit rules apply`

按规则登记路径（被 Git 跟踪的文件永远不登记）

用法: `obit rules apply <path>... [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | apply 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |
| `--session` | 无 | `""` | 这些路径来自的 agent 会话 |

### `obit rules check`

显示规则会为这些路径登记什么（不做任何改动）

用法: `obit rules check <path>... [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | check 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |
| `--session` | 无 | `""` | 这些路径来自的 agent 会话 |

### `obit stats`

讣告做过的事：跟踪了多少、下葬了多少、腾出多少空间、复活了几次

别名: `stats`, `eulogy`
用法: `obit stats [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | stats 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |

### `obit sweep`

立刻收割：不等后台进程，把所有到期的都下葬

别名: `sweep`, `reap`
用法: `obit sweep [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--dry-run` | 无 | `false` | 只显示会发生什么 |
| `--help` | `-h` | `false` | sweep 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |

### `obit trust`

审阅并信任项目的 .obitrules（修改后需要重新审阅）

用法: `obit trust [rules file] [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | trust 的帮助 |
| `--yes` | 无 | `false` | 不询问，直接信任（只在审阅过规则之后使用） |

### `obit uninstall`

移除讣告的集成和后台服务；默认保留数据

用法: `obit uninstall [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--dry-run` | 无 | `false` | 只打印卸载计划，不改动文件和服务 |
| `--help` | `-h` | `false` | uninstall 的帮助 |
| `--purge-data` | 无 | `false` | 永久火化列出的墓，并移除讣告自己的数据；不认识的文件保留 |
| `--yes` | 无 | `false` | 确认计划（只有同时给了 --purge-data，才同时确认清除数据） |

### `obit untrust`

不再应用项目的 .obitrules

用法: `obit untrust [rules file] [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | untrust 的帮助 |

### `obit update`

检查或安装正式版更新

用法: `obit update [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--apply` | 无 | `false` | 下载、校验并安装已缓存的较新正式版（手动安装只替换讣告自己的程序） |
| `--check` | 无 | `false` | 立即检查 GitHub，自动检查被关闭时也会检查 |
| `--help` | `-h` | `false` | update 的帮助 |
| `--relaunch-gui` | 无 | `false` | 更新后重新打开小窗口（Windows） |
| `--wait-pid` | 无 | `0` | 等待该进程退出后再更新（Windows） |

### `obit watch`

给文件夹里的每个第一层子项各自设定寿命

用法: `obit watch <directory> <lifespan> | watch --list | watch --remove <directory> [flags]`

| 参数 | 简写 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--age` | 无 | `"first-seen"` | 计龄基准：first-seen 或 modified（需要批准） |
| `--clear-exclusion` | 无 | `0` | 明确清除这个豁免 ID（与 --policy 一起使用） |
| `--help` | `-h` | `false` | watch 的帮助 |
| `--json` | 无 | `false` | 输出 JSON |
| `--list` | 无 | `false` | 列出文件夹策略 |
| `--no-daemon` | 无 | `false` | 不启动后台进程 |
| `--policy` | 无 | `0` | 要清除豁免的策略 ID |
| `--remove` | 无 | `""` | 移除策略，并取消它登记的寿命 |
| `--remove-id` | 无 | `0` | 按 ID 移除文件夹策略，路径已不存在的也可以 |

<!-- generated-command-reference:end -->
