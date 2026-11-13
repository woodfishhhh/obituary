# 使用手册

[English](usage.en.md) · [首页](../README.md)

讣告各项功能的详细用法。安装见[安装文档](install.md)，命令速查见 README 的[快速上手](../README.md#快速上手)。

## 给人用：托盘、小窗口、右键菜单

```powershell
obit init                                   # 装 Claude Code 的 hook，并让托盘随登录启动
obit init --context-menu --yes              # 兼容写法，右键菜单默认安装（经典菜单，Win11 在「显示更多选项」里；--dry-run 列出每一项菜单名和命令）
obit watch C:\Users\你\Downloads 2w          # 下载文件夹：放进来的每样东西两周后进墓地（生成提议，obit approve 或在「待你决定」里批准后才生效；文件到期、并且安静 10 分钟没被改动后自动下葬，子文件夹到期另出提议）
obit watch --list                           # 有哪些文件夹策略，各自的豁免和安静期
obit watch --remove C:\Users\你\Downloads    # 移除策略，它登记过的寿命一起取消
```

- 托盘菜单：最近下葬的（点一条就复活）、24 小时内要到期的、暂停 / 继续、打开讣告（小窗口）、退出（定时任务仍在兜底）。
- 小窗口 `obit-gui.exe` 和 `obitd.exe` 放在同一个目录，托盘才能打开它。每个开关先给你看要改什么，确认后才写入；信任 `.obitrules` 时只信任你在窗口里看到的那一版内容。
- 资源管理器里的日期靠文件夹里隐藏的 `desktop.ini`，默认只在下载、桌面和文件夹策略管的目录里显示；`config.toml` 的 `[explorer]` 可以关掉（`enabled = false`）或扩大到其他位置（`everywhere = true`）。资源管理器有自己的缓存，延长寿命后同一个窗口最长约一分钟还显示旧日期。

## 接入 Claude Code

```powershell
obit init -g             # 所有项目（~/.claude）；obit init 只装当前项目（.claude/）
obit init -g --show --agent claude  # 装了什么（不加 --agent 先列每家一行的摘要，再给完整明细）
obit init -g --uninstall # 只删讣告加的东西
```

`obit init` 默认安装所选 agent 的 Agent Skill、右键菜单、开始菜单和桌面快捷方式；可用 `--no-skill` / `--no-context-menu` / `--no-shortcuts` 跳过，右键菜单和快捷方式以后也能在设置里关闭。托盘图标和小窗口的任务栏图标也可在设置里切换，隐藏托盘后后台清理仍会运行。

`obit init` 在 `settings.json` 里注册 `obit hook claude`（SessionStart、每次调用工具前、写文件后、Stop、SessionEnd），写一份给 agent 看的 `OBIT.md`，并在 `CLAUDE.md` 里加一行引用：用户级（`-g`）是 `~/.claude/CLAUDE.md` 里的 `@OBIT.md`，项目级是项目根 `CLAUDE.md` 里的 `@.claude/OBIT.md`。

- <details>
  <summary><code>obit init</code> 怎么改你的配置</summary>

  - 你原来的设置和键顺序保持不变。重复执行不会重复添加；`obit.exe` 换了位置，再执行一次就会刷新路径。
  - 每次改之前都先备份：第一次是 `.bak`，之后是 `.bak.1`、`.bak.2` …，旧备份永远不会被覆盖。从算出改动到写入之间，如果你保存过这个文件，讣告就拒绝写入。
  - 只有带讣告标记的 `OBIT.md` 才会被替换或删除，同名的其他文件不会动；你自己写的、恰好包含 `obit hook claude` 的复合命令，也不会被当成讣告的。只有用绝对路径指向 `obit.exe` 的 hook 才算讣告的（`obit init` 总是写绝对路径）；用相对路径或裸命令 `obit` 写的，讣告不改也不删。

  </details>

- hook 出任何错都静默放行（错误写进 `hook.log`）。设置 `OBIT_DISABLED=1` 可以临时关掉；设置 `OBIT_HOOK_AUDIT=1` 会记下每次 hook 收到了什么、做了什么，用 `obit hook audit` 查看。

```powershell
obit doctor              # 只读体检：PATH、obitd、运行器、Claude Code / Codex 集成、数据目录、账本、配置（支持 --json）
```

装好后：

- **第一次征得同意**：在一个项目里，讣告第一次想按规则登记东西时，不会直接动手，而是生成一份提议。agent 会把它转告你（真机测试里 Claude 的原话是 "Obituary has a pending proposal… I haven't acted on it."）。你同意后执行 `obit approve <id>`，此后这个项目里的东西就自动登记；`obit reject <id>` 则表示以后别再问这个项目。
- **超大的东西**（超过 1 GB 或 200 个文件）照样先提议。
- **会话差分**：会话开始时记一份快照（存在数据目录，不往你的仓库里放东西），结束时对比。规则覆盖的东西照常登记；规则覆盖不到的，只有 Git 忽略、又能重新生成的东西（依赖目录、构建输出、缓存，如 `node_modules`、`dist`、`.turbo`）才列成提议，批准后 3 天到期下葬、可以复活。没被 Git 忽略的新文件是你还没提交的代码，`.env` 这类被忽略的本地配置也不能重新生成，这两类都不会出现在提议里，所以 agent 可以放心建议你批准。下次会话开始时，agent 会看到「上次留下了哪些东西」。你复活或饶过的东西，在再次被写入之前不会被会话差分重新登记。
- **下葬前再问一次 Git**：按规则或提议登记的东西，到期时如果已经被提交，就不下葬。
- **提议会自己过期**：展示提议时，已经不存在的文件不再列出；后台运行器每次都按项目分批问一遍 git，已经被提交或删掉的条目从提议里去掉，全部失效的提议自动撤回（记进审计日志，不算你拒绝）。项目文件夹或 Git 仓库暂时访问不到时（改名、盘离线），提议原样保留。
- **墓地提醒**：agent 的工具调用用到一个已经下葬、还能复活的路径（或下葬目录里的东西）时，讣告告诉它编号、什么时候下葬的、还能复活到什么时候和 `obit resurrect <编号>`。只提醒，不拦也不改它的操作；同一个会话里每座坟只提一次。Claude Code 和 Codex 在调用工具前当场提醒，其他家按 `obit init --show` 里写的时机。以前装过的，重新执行一次 `obit init` 才有。
- **开工清单（可选）**：为了省 agent 的 token，会话开始时默认只转告需要你拍板的提议。在桌面程序「设置」里打开「开工清单」，或在 `config.toml` 里设 `[hooks] session_summary = true`，agent 开工时还会看到这个项目 24 小时内要到期的条目和最近下葬、还能复活的东西（`obit mktemp` 的临时目录不算）。
- **到期预告（开工清单打开时）**：会话开始时，agent 会看到这个项目 24 小时内要到期的条目。

```powershell
obit proposals           # 等你点头的提议
obit approve 3           # 同意：登记里面列的东西
obit reject 3            # 拒绝
```

## 接入 Codex

```powershell
obit init -g --agent codex   # ~/.codex/hooks.json + AGENTS.md 里的说明块；obit init --agent codex 只装当前项目
```

- 同一份 `hooks.json` 同时支持 Windows 和 WSL：`command` 写 `/mnt/d/…/obit.exe hook codex`（WSL 里通过 interop 调用 Windows 版），`commandWindows` 写 Windows 路径。WSL 传来的 `/mnt/<盘>/…` 路径会自动换成 `<盘>:\…`；WSL 自己文件系统里的路径（`/home/…`）暂时静默跳过。
- Codex 改文件用 `apply_patch`，讣告从补丁里解析出新增、修改、移动后的文件（删除的不算）。
- **Codex 只执行你信任过的 hook**：装完后打开 Codex，运行 `/hooks`，信任讣告的 hook。项目级的 hook 还需要这个项目本身是受信任的。
- 项目级安装时，如果 `.codex`、`.claude` 或备份文件是指向项目外的链接，就拒绝写入。

## 接入其他 agent

```powershell
obit init -g --show                         # 列出全部可接入的 agent、各自能做到哪一步、装没装
obit init -g --agent gemini,cursor          # 装一家或几家；不带 -g 只装当前项目
obit init -g --agent gemini --mcp --skill   # 同时注册 MCP 服务、安装 Skill（看各家是否支持）
obit init -g --agent gemini --uninstall     # 只删讣告加的东西
```

不带 `--agent` 时只装 Claude Code（Codex 照上一节单独装），不会替你给别的 agent 装东西。

| 方式 | agent |
|---|---|
| 原生 hook | GitHub Copilot（VS Code 和 Copilot CLI）、Cursor、Gemini CLI、Factory Droid、Trae / Trae CN、Kimi、Mistral Vibe、Google Antigravity；国内的腾讯 CodeBuddy（CLI 和 IDE）、Qoder / Qoder CN / 灵码、智谱 ZCode |
| 插件（讣告装一个小插件，插件调用 `obit hook`） | OpenCode、OpenClaw、Pi、Oh My Pi、Hermes、Cline、Kilo Code、MiniMax Code、DeepSeek Harness |
| 只有规则文件和 MCP | Roo Code、Windsurf、WorkBuddy、QoderWork |

- 各家能做到的程度不一样：写文件后的登记、会话开头的提示、删除护栏、MCP、Skill。逐项的「支持 / 有限 / 不支持」和原因见 [agent 覆盖表](agents.md)，它由 `obit init --show --format markdown` 从代码里的能力声明生成，和代码对不上时测试会失败。删除护栏只在宿主能证明命令在本机、用哪种 shell 执行时才开。
- 公开资料不足以安全地自动写入的地方（例如 OpenClaw 的 JSON5 配置、Windsurf 和 Cursor 的全局规则、部分 MCP 位置），`obit init` 打印完整的手动步骤，结果标成「未完成，需要手动步骤」，不会假装装好了，也不会往猜测的目录里写东西。
- 插件类需要你确认它在本机执行（例如设 `OBIT_OPENCODE_LOCAL=1`、`OBIT_CLINE_LOCAL_SHELL=bash`），安装输出里会写清楚要设什么；远端或云端执行的操作不会登记本机同名的文件。

## Docker 容器

```powershell
obit docker add web-1 3d       # 按名字或 ID 登记一个容器，三天后到期
obit docker list               # 登记了哪些容器（--json 可用）
obit resurrect <编号>          # 到期被停下的容器重新启动
```

- 讣告只管容器。卷里是数据，删卷就是永久删除，所以卷、镜像和网络一律不碰。
- **到期 = 停下。** `docker stop` 之后容器和它的可写层都还在，`obit resurrect <编号>` 就是 `docker start`，复活后不再跟踪它的寿命。保留期（`[graveyard] retention_days`，默认 7 天）过后才 `docker rm`，不带 `-v`、不带 `-f`；那时容器要是在运行（被你或别人手动启动过），就当作复活，不删。`obit purge` 和 `--all` 对容器同样要等保留期。回收站模式对容器不适用，照样是先停下、保留期后删除。
- **按完整 ID 认容器。** 名字只用于显示；每次停、启、删之前都重新核对完整 ID 和创建时间，对不上就不动。讣告还记住容器所在的 Docker 上下文和 daemon，之后只对那一个 daemon 操作；你切换了 `docker context` 也不会动到别处的同名容器。容器被你在讣告之外删掉了，讣告会把它记成「外部移除」并清掉记录。设了 `--rm`（AutoRemove）的容器停下就会被 Docker 自己删掉，讣告拒绝登记它。
- **会话里新起的容器只提议。** agent 会话结束后，讣告在后台列出这个会话期间新建的容器：只列 compose 的工作目录或绑定挂载在项目里的；归不到项目的可能是你在别处起的，不列。你 `obit approve` 才登记。hook 里从不调用 docker；没装 Docker 或 Docker 没启动时静默跳过，`obit doctor` 会说明。

## 删除护栏：agent 删掉的东西都能复活

```powershell
obit init -g --guard-rm                 # Claude Code；Codex 加 --agent codex。默认不开
```

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/readme/delete-guard-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="assets/readme/delete-guard-light.svg">
    <img alt="删除护栏：agent 发出 rm -rf build，讣告拦下这一次执行（只拦不改），告诉 agent 等价命令 obit rm -rf build；agent 改用它，东西进墓地，obit resurrect &lt;id&gt; 一条命令放回原处" src="assets/readme/delete-guard-light.svg" width="100%">
  </picture>
</p>

开启后，agent 执行 `rm -rf build`、`rmdir`、`git clean -fdx`、`Remove-Item -Recurse` 这类删除命令时，讣告会拦下这一次执行，并告诉 agent 一条等价命令，比如 `obit rm -rf build`。agent 改用它之后，东西进墓地而不是消失，`obit resurrect <id>` 一条命令放回原处。

- **只拦不改。** 讣告不改写 agent 的命令：Claude Code 会按改写后的命令检查权限，改写会让你设的 `deny` 规则失效。拦下再让 agent 自己发出等价命令，你的所有权限规则照常生效。
- **出现字面的删除就拦。** 命令里任何一个命令位置上出现 `rm`、`rmdir`、`git clean`、`Remove-Item`（含 `del`、`rd`、`ri` 这些别名），不论在 `&&`、`||`、`;`、管道、`$(…)`、子 shell、条件还是循环里，都拦下这一次执行，例如 `rm -rf build && npm run build`、`rm -rf build 2>/dev/null`、`git clean -fdx && npm ci`。拦下本身什么都不执行，所以多拦不会删错东西。字符串、注释、`echo` 的参数里的 `rm` 不算；`rm --help`、`git clean -n`、`Remove-Item -WhatIf` 只是预览，也不算；`sudo rm`、`xargs rm`、`find -delete` 里的删除是别的程序在做，照常放行（设了 `OBIT_HOOK_AUDIT=1` 会记进审计日志）。
- **不会退回成真删。** `obit rm` / `obit remove-item` / `obit git-clean` 的每个路径都走下葬；被 Guard 拒绝（盘符根、系统目录、用户目录本身等）时照 `rm` 的格式报错、返回非零。
- 被 Git 跟踪的文件也照样下葬，这样未提交的修改也找得回来（这是 agent 主动删除，和「自动登记绝不碰被跟踪文件」不冲突）。
- `obit rm -rf` 一个含 `.git` 的目录会把整个仓库下葬：真正的 `rm -rf` 会把它永久删掉，下葬至少还能复活；拒绝反而可能让 agent 改用 `/bin/rm`。系统目录、用户文件夹这些不能强制的保护照样生效。
- **等价命令刻意收窄**：等价命令绝不能比原命令选中更多东西，所以讣告不做跨 shell 的翻译，只在下表的写法里给。给不出时照样拦下，理由里写明为什么给不出，并说明 `obit bury <路径>`（字面路径）和 `obit git-clean` 可以做到可复活的删除，agent 据此改写命令。

  <details>
  <summary>各场景下拦下和放行的写法</summary>

  | 场景 | 拦下并给等价命令 | 拦下，附原因和通用说明 | 放行 |
  |---|---|---|---|
  | Claude Code（Bash 工具） | `rm`、`rmdir`、`git clean`，前面只能有字面的 `cd x`（用 `&&`、`;` 或换行连接）；`rm -f *.log`（当前目录里没有以 `-` 开头的文件时）、`rm -rf ../dist`、`rm -- "$target"`（变量在 `--` 之后） | 其他字面删除：前后还有别的语句（`npm …`、`trap`、`hash` 等）、`--` 之前的变量（可能展开成选项）、以 `/` 开头的 POSIX 绝对路径、重定向、`\|\|`、管道、`$(…)`、交互式、带路径的 `/usr/bin/rm` | 不是删除的命令 |
  | Codex，WSL 的 `/mnt/<盘>` 下 | 单独一条 `rm` / `rmdir`，参数都是不含 `..`、`~`、通配、引号的相对路径（建议用 interop 的 `obit.exe`） | 其他字面删除，含 `git clean` 和指向 WSL 自己文件系统的路径 | 工作目录在 WSL 自己的文件系统里 |
  | Codex，明确是 Windows PowerShell | 单独一条 `Remove-Item` 或别名，只带一个字面路径（可加 `-Recurse`、`-Force`） | 其他字面删除：多条语句、管道、`-ErrorAction` 等其他参数、provider 路径、`~`、通配、反引号 | 弯引号字符串里的文字 |
  | Codex 在 Windows 上但看不出是哪种 shell、Linux 上的 pwsh、`git rm` | — | — | 全部放行 |

  另外几条给等价命令的规则：删除命令必须是整条命令的**最后一段**，前面只能用 `&&`、`;` 或换行连接（`obit rm` 和 `rm` 的成功与失败不完全一样，后面若还有依赖退出码的命令，结果可能分叉）；文件名以点或空格结尾的不给（Windows 会把它们归一化成另一个名字）；bash 命令里有回车、NUL 字节、反引号，或者反斜杠续行和 `#` 注释同时出现、续行不止一处的，也不给。这些情况下命令里的字面删除照样拦下。建议命令里的 obit 与已安装的 hook 用同一个路径。

  </details>

- 你自己也可以直接用：`obit rm`、`obit rmdir`、`obit git-clean` 接受 `rm`、`rmdir`、`git clean` 常用的非交互参数（`-i`、`rmdir -p` 这类不支持的参数会报错，不会悄悄忽略）；`obit remove-item` 只接受一个字面路径（不展开通配，也不遵守 `$WhatIfPreference`），不是完整的 `Remove-Item` 替代；要用通配，在 bash 里用 `obit rm *.log`。已知限制见 [docs/architecture.md](architecture.md#威胁模型)。
- 临时关掉：`OBIT_GUARD_RM=0`。`obit stats` 会显示拦下了多少次删除、埋了多少东西。

## MCP 服务与 Agent Skill

```powershell
obit init -g --mcp --skill                 # Claude Code；Codex 加 --agent codex。Skill 默认安装，MCP 按需启用
obit init -g --show                        # 也会列出 mcp / skill 是否装了、是不是最新
```

- **`obit mcp`** 是一个 stdio MCP 服务，把讣告的操作做成 agent 可以直接调用的工具：`obit_mark`、`obit_mktemp`、`obit_bury`、`obit_resurrect`、`obit_extend`、`obit_spare`，以及查询用的 `obit_list`、`obit_graveyard`、`obit_proposals`、`obit_stats`（`obit_graveyard` 会顺手补记核实过的中断复活，其余只读）。
- **你的决定不交给 agent。** 批准或拒绝提议、信任 `.obitrules`、火化墓地、改配置，MCP 里都没有对应的工具，工具清单由测试固定。agent 只能把提议的内容转述给你。
- 路径必须是绝对路径：MCP 服务的工作目录不一定是 agent 的，不猜相对路径。编号写成 `5` 或 `#5`，找不到这个编号就报错，不会退回去当成文件名。一次最多 100 项；备注、会话这类文字最长 4 KiB（命令行同样限制）；`obit_list`、`obit_graveyard`、`obit_proposals` 每页最多 200 条，按编号用 `limit` / `after` 翻页；回执带 `version`，翻页时传回它，列表在两页之间变了会标出 `changed: true`，这时从头再读一遍才是完整的，回执里过长的文字会截断并标明，保证回执始终在协议的大小上限以内。
- <details>
  <summary>注册位置</summary>

  - Claude Code 用户级交给官方命令（`claude mcp add --scope user --transport stdio obituary -- <obit.exe> mcp` / `claude mcp remove --scope user obituary`，`--dry-run` 会打印完整命令），需要 Claude Code 2.1.318 或更新（`claude update`）：更早的版本写这个文件时不拿锁。`~/.claude.json` 是 Claude Code 运行时不断整体重写的状态文件，讣告只读它、不直接改。项目级直接写 `.mcp.json`。
  - Codex 写 `config.toml` 的 `[mcp_servers.obituary]`，用 `# obit:mcp:begin` / `# obit:mcp:end` 两行注释包起来。卸载删掉整个标记块，包括你在块里加的选项（如 `enabled`、`env`）。块里的命令或参数被改成别的东西、或者块外也定义了 `obituary`，讣告就不认这块，安装和卸载都不动它。

  </details>

- **Agent Skill** 是一份 `SKILL.md`，告诉 agent 什么时候给产物设寿命、怎么可复活地删除、哪些事要交给你决定。Claude Code 装到 `~/.claude/skills/obituary/`，Codex 装到 `~/.agents/skills/obituary/`（项目级对应 `.claude/skills/`、`.agents/skills/`）。旁边的 `.obituary-skill` 记着装进去时的哈希：你改过这份文件，讣告就不再覆盖或删除它。
- 普通的 `obit init -g` 会安装或刷新 Skill（`--no-skill` 跳过）；Codex 的 MCP 块会在已安装时刷新（比如 `obit.exe` 换了位置），没装的 MCP 不会新装。Claude Code 用户级的 MCP 注册例外：官方命令只能先删再加，不是原子的，所以普通刷新只报告「过期」并给出手动命令，要替换就显式加 `--mcp`。扩展的配置文件坏了也不会挡住 hooks 的安装，只给出警告。
- 安装器只执行 PATH 里绝对目录下的 `claude.exe`（npm 安装的会顺着 `claude.cmd` 找到它背后的 exe），落在当前目录或当前仓库里的一律拒绝，免得在别人的仓库里运行 `obit init` 时执行了仓库里放的程序。判断按文件夹在磁盘上的身份，不按路径写法，所以换个名字（junction、`\\localhost\C$`、短名、硬链接）也绕不过去。手动命令里的参数含双引号时，需要在 PowerShell 7.3 或更新版本里粘贴。
- `config.toml` 里设了 `mode = "recycle"` 时，下葬进的是系统回收站，MCP 工具的说明会如实写明，`obit resurrect` 也救不回来。
- 装完重启 agent。`obit doctor` 会检查两项是否装好、指向的 `obit.exe` 是否是当前这个（没装算正常，它们是可选的）；它只说明配置写对了，不代表 agent 已经连上。

## 给 agent 产物自动登记寿命

```powershell
obit rules                       # 当前项目生效的规则（内置规则 + 已信任的 .obitrules）
obit rules check debug.log out/  # 预览：会登记什么、按哪条规则（不做任何改动）
obit rules apply debug.log       # 按规则登记
$dir = obit mktemp 1d            # 会自动下葬的临时目录：项目里放在 .obit/tmp/（不进 Git），否则放在系统临时目录；bash 里写 dir=$(obit mktemp 1d)
```

内置规则覆盖 agent 常见的垃圾：`.playwright-mcp/`、`test-results/`、`__pycache__/`、`*.log` 等。项目根目录的 `.obitrules` 用 `.gitignore` 的写法，每条后面跟寿命：

```
output/**        7d      # output 下的每一项单独计算寿命
!output/keep/**          # 永远不碰（否定规则优先，连上级目录都不会被整体登记）
test-results/    off     # 关掉某条内置规则
```

和 `.gitignore` 不同的地方：否定规则优先，不是最后一条为准；Windows 上匹配不分大小写（非 ASCII 字母也一样，不看 Git 的 `core.ignorecase`）；写错的模式（比如没闭合的 `[`）会报错并指出文件和行号，这个项目的自动登记先停下，不会把它悄悄当成不匹配。

**自动登记的硬规则：** 只登记 Git 工作区里、没有被 Git 跟踪的东西；目录里只要有一个被跟踪的文件，整个目录都不碰；Git 回答不了时一律跳过；已经有寿命的不会被改短。

**项目规则要先信任。** 仓库自带的 `.obitrules` 在你审过并执行 `obit trust` 之前不生效（防止别人的仓库用规则收走你的私人文件）。信任时记下文件的 SHA-256，内容一改就要重新审。非交互环境必须加 `--yes`；CI 里可以用 `OBIT_TRUST_PROJECT_RULES=1`。

时长单位：`s` `m` `h` `d` `w`，可以组合（`1d12h`）。查询和结果类命令（`list`、`graveyard`、`proposals`、`stats`、`mark`、`bury`、`resurrect` 等）支持 `--json`，方便脚本和 AI agent 调用；`obit rm` 这类删除包装器照原命令的格式输出，不支持 `--json`。

`resurrect` 和 `purge` 里的纯数字（`5`、`#5`）一律当编号，找不到就报错，不会退回去当成文件名；文件名本身是数字时写成 `./5` 或绝对路径。按路径引用时，短名写法（如 `DIRECT~1`）会先展开成长名再查。

## 安全设计

- **先下葬，再火化。** 到期的东西搬进墓地，默认保留 7 天（`config.toml` 里 `[graveyard]` 的 `retention_days` 可改）后才真正删除，期间随时 `obit resurrect`。墓地在同一块盘上（数据目录所在的盘用数据目录里的墓地，其他盘用该盘根目录下隐藏的 `.obituary`），所以下葬只是改名，又快又不会只搬了一半。也可以配置成直接进系统回收站（`[graveyard]` 的 `mode = "recycle"`），这时要从回收站恢复，`obit resurrect` 救不回来。
- **火化是唯一的永久删除。** 每次下葬都新建一个坟墓目录，并记下它的文件 ID。火化和复活之前都要核对：这个坟墓目录还是当初那个，里面只有一项，而且就是当初埋进去的对象。所以账本被改坏了、墓地被换成了链接，也删不到墓地以外的东西。
- **复活只回原处。** 原处已经有同名的东西时，不覆盖它，改名放在旁边（`名字 (resurrected).扩展名`），两份都保留，回执里写着实际放回的路径。原路径就是原位置：上级目录被删了，复活会重建；被换成另一个普通文件夹也照常放回。要拦的是「被引到别处」：上级目录变成了 junction / 符号链接、原本的 junction 被改指、盘符换了卷，都拒绝复活（墓地原样保留）。最后一步固定住目标文件夹再移动，检查之后有人改名或换链接也影响不了落点。按路径复活时，路径完全一致的优先；只差英文字母大小写的，结果必须唯一，否则要求用编号。
- **认文件，不认文件名。** 每条记录都存着卷序列号和 128 位文件 ID：文件改名或移动了照样找得到；原路径上换成了别的文件，就不碰它。删除前再核对一次身份。有多个硬链接的文件一律拒绝。
- **受保护路径。** 盘符根目录、`C:\Windows`、Program Files、ProgramData，以及用户目录、桌面、文档、下载等文件夹本身和它们的上级目录都拒绝（被重定向到别处的也算）。这些文件夹里面的东西可以设。比较之前先把 8.3 短名、父目录里的 junction 都还原成真实路径；链接本身一律拒绝。
- **Git 仓库。** `obit add`、`obit bury` 一个任何一层含 `.git` 的文件夹都要加 `--force`；到期时会再查一次，查不完也拒绝。例外是删除护栏给的 `obit rm -rf`：它照真正的 `rm -rf` 的语义整个下葬（见[删除护栏](#删除护栏agent-删掉的东西都能复活)）。
- **不确定就不删。** 只有确定文件已经不在了才放弃这条记录。拒绝访问、卷被拔掉这类情况一律稍后重试。
- **全程留痕。** 每次添加、续期、取消、删除都写进审计日志（`obit log`）。

## 后台运行

默认是托盘常驻：`obitd.exe` 随登录启动，到期精确到秒，托盘图标是它唯一的界面。另外保留一个 Windows 任务计划每 5 分钟（以及每次登录时）运行一次 `obitd --once` 作兜底：托盘退出或没启动时，到期的东西照样会被收走、过了保留期的照样火化。它在任务计划程序里设为隐藏，用电池时照样运行，错过的时间点（关机、睡眠）会补跑。agent 的 hook 在会话开始和结束时也会顺手处理一轮；`obit reap` 立即处理。`list`、`graveyard`、`stats`、`log` 这类查询只读，不会顺手下葬。

```powershell
obit init                       # 托盘随登录启动（HKCU\...\Run），--uninstall 撤掉
obit daemon enable              # 托盘随登录启动，并装兜底的定时任务（--dry-run 只列出要装什么）
obit daemon status              # 现在是哪种模式、下一次到期（支持 --json）
obit daemon disable             # 撤掉定时任务和登录启动，并停掉正在运行的常驻进程（--dry-run 只预览）
```

登记（`add`、`approve`、`mktemp`、`rules apply`）或下葬之后，如果没有运行器在服务你的数据目录，讣告会先临时启动一个常驻进程，并提示你执行 `enable`。定时任务服务的是另一个数据目录（比如你用 `OBIT_HOME` 换过位置）时也算没有；`obit daemon status` 会指出这一点。

数据放在 `%LOCALAPPDATA%\obituary`：`store.json`（账本）、`audit.jsonl`（审计日志）、`obitd.log`（后台进程日志）、`graveyard\`（这块盘的墓地），完整清单见 [docs/architecture.md](architecture.md#数据布局)。配置文件是 `%APPDATA%\obituary\config.toml`，`obit config` 可以查看，`obit config --create` 生成带注释的默认配置：

```toml
[graveyard]
mode = "graveyard"      # 或 "recycle"：直接进系统回收站
retention_days = 7      # 墓地保留天数
```

写错的键会让配置无效，而不是被悄悄忽略：报错和 `obit doctor` 都会指出文件、行号和键名，并给出最接近的正确写法（例如 `retention` → `retention_days`）。否则一个拼写错误就会让默认值继续生效，坟墓比你以为的早火化。

可以用环境变量 `OBIT_HOME`（数据目录）、`OBIT_CONFIG_DIR`（配置目录）、`OBIT_USER_HOME`（用户主目录）换位置；`obit init` 找 Claude Code 和 Codex 的配置时，也认它们自己的 `CLAUDE_CONFIG_DIR` 和 `CODEX_HOME`。测试就是靠这几个变量隔离的。

## 构建

需要 Go 1.26+。目前只支持 Windows，macOS 和 Linux 在计划中。

```bash
go build -o bin/obit.exe ./cmd/obit
go build -ldflags=-H=windowsgui -o bin/obitd.exe ./cmd/obitd   # 必须加这个参数，否则登录时会弹出黑框
CGO_ENABLED=0 go build -tags desktop,production -ldflags "-H windowsgui" -o bin/obit-gui.exe ./cmd/obit-gui
go test -short ./...                                           # 快的一档；全量在 CI 上跑
```

三个 exe 要放在同一个目录。小窗口的界面在 `cmd/obit-gui/frontend`（React，pnpm），构建结果 `dist/` 已经提交，只改 Go 代码时不需要 Node；改界面时 `pnpm install`、`pnpm dev` 打开预览，`pnpm build` 更新 `dist/`。真实回收站测试默认跳过，设置 `OBIT_TEST_RECYCLE=1` 才会跑，跑的时候测试文件会进你的回收站。

