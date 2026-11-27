# 子系统维护

跨模块流程见[架构](../architecture.md)。

## 命令、账本与文件系统

- `internal/cli/<命令>/`：每个命令一个包，构造函数是 `NewCmd`；`internal/cli/cmdutil` 放共用的打开服务、输出与回执（`ReportGraves`、`ErrReported`）、拉起后台进程（`runner.go`）；`internal/cli/clitest` 是只给测试用的公共辅助（文件、Git、stdio、runner、墓地夹具、MCP 连接）。
- `cmd/obitd`：后台进程，必须用 `-ldflags=-H=windowsgui` 构建；`--home` 指定数据目录。
- `internal/core/*`（`lifecycle`、`ledger`、`guard`、`lifespan`、`fs`）：跟踪列表（带修订号 `Rev`）、到期清理（逐条认领、按修订号回写、同一个 store 同时只跑一个 sweep）、安全检查（Guard）、时长解析、审计日志。调用方直接导入对应子包，访问文件系统只走 `internal/core/fs.FS` 接口。
- `internal/winfs`：`core.FS` 的 Windows 实现：128 位文件 ID（`GetFileInformationByHandleEx` / `OpenFileById`）、硬链接检查、`Canonical`（真实路径）、逐级展开长名（`longpath_windows.go`）、按序数规则比较名字（`name_windows.go`）、复活用的父目录句柄、按句柄建目录和改名（`restore_windows.go`），以及通过 `IFileOperation` vtable 移进回收站。
- `internal/core/lifecycle` 的墓地按职责分文件：`bury.go` / `bury_move.go` / `burial_lifespans.go`（下葬：独占建坟墓、写清单、锁内复核后按句柄移动、随父目录下葬的子条目）、`resurrect.go`（复活，不覆盖）以及 `restore_intent.go` / `restore_parents.go` / `restore_creation.go` / `restore_cleanup.go`（复活意图、祖先核对、按句柄建目录与撤销、清理回执）、`abandon.go`（显式放弃核实不了的复活）、`purge.go`（火化）、`manifest.go` / `recovery.go`（清单与恢复）、`sweep.go` / `settle.go`（到期收割）、`register.go`（登记）、`lookup.go`（按编号或路径找条目和墓地记录）、`graveyard.go` / `graveyard_owner.go`（墓地根、归属标记与公共逻辑）。复活和火化之前都必须过 `verifyGrave`。
- `internal/core/ledger` 负责账本读写、审计与待补队列、锁忙时延后处理的写入事件。Windows 文件锁、按目录句柄替换、共享删除的读取在 `internal/core/internal/fileio`。见[账本与审计](../architecture.md#账本与审计)。
- `internal/listing` 和 `internal/texttable`：`obit list` 的渲染和按终端单元格对齐的表格。命令自己读账本和常驻进程代为打印（`internal/daemonipc` 的 list 请求，`app/resident_ipc.go`）走同一份代码，输出逐字节相同；`cmdutil.Table` / `cmdutil.Column` 转发到 texttable。改 `obit list` 的输出时两条路径都要测（`internal/app/resident_ipc_test.go`）。
- `internal/app`：组装服务和 Guard（含已知文件夹），控制 obitd：桌面版启用受监督常驻后，独立 `--once` 兜底为每小时；无托盘版本保持 5 分钟。Windows 定时任务用 XML 定义，因为 schtasks 的参数设不了电池、补跑、隐藏这些选项；桌面常驻由用户级监督入口负责，HKCU Run 只用于迁移前的旧入口；`RunOnce` 是给 hook 用的一次性后台处理。
- `internal/userdirs`：解析数据目录、配置目录、用户主目录、`~/.claude`（`ClaudeDir`）、`~/.codex`（`CodexDir`）的唯一入口，可以用 `OBIT_HOME` / `OBIT_CONFIG_DIR` / `OBIT_USER_HOME` / `CLAUDE_CONFIG_DIR` / `CODEX_HOME` 重定向。其他代码不准自己去找这些目录，`source_test.go` 会扫描源码强制这一点；确实需要（比如 Guard 要保护真实的用户文件夹）就加进豁免表并写明理由。
- `internal/pathutil`：卷根（`VolumeRoot`，扩展 UNC 带上服务器和共享）和引用路径的绝对化（`Abs`：`\\?\` 写法只在 Windows 的规范化不改变它时接受，并转成普通写法；拒绝备用数据流；保留结尾分隔符表示的「必须是目录」）。文件系统读出来的名字再当普通路径用之前，先过 `Unchanged`。路径代码不要直接用 `filepath.VolumeName` 求卷根，也不要对 `\\?\` 写法直接调 `filepath.Abs`。

底层实现优先核对标准库和已有依赖。Windows 目录句柄相对替换可用 Go 1.25 起的 `os.Root.Rename`。Bash 解析、gitignore、保序 JSON 和文件锁分别已有 `mvdan.cc/sh`、go-git `plumbing/format/gitignore`、`tidwall/sjson`、`gofrs/flock`。

## 项目、规则与自动登记

- `internal/rules`：`.gitignore` 写法加寿命的规则、内置规则、信任存储（先信任再加载）。否定规则优先：目录里有受保护的东西时，不把整个目录当成一个单元。
- `internal/project`：Git 工作区。`Find` 只向上找 `.git`，不启动 git 进程（hook 每次都要调用它，而且 git 出错时必须得到「重试」而不是「不在项目里」），遵守 `GIT_CEILING_DIRECTORIES`。是否被跟踪只问 `git`（每次调用最多 15 秒，超时算错误），问不出来就当作已被跟踪。`.obit/` 会写进 `info/exclude`。
- `internal/auto`：按规则自动登记（Apply）、`MkTemp`；`observe.go` 给 hook 用：Observe（还没问过的项目先提议，已同意就按规则登记，太大的改为提议）、Snapshot / Leftovers（会话差分，新路径交给规则，规则认不得的不登记也不提议）。`[proposals] mode` 默认 `off`，这些都不做；依赖、构建输出和缓存（`NeverAutomatic`）任何模式都不自动登记。
- 自动登记相关的改动，测试要跑在真的临时 Git 仓库里（参考 `internal/auto` 的 `newRepo`），不要假装 Git 的回答。

## 常驻、hook 和接入

- **常驻进程和托盘不定时轮询（2026-11-14 定）。** 只靠事件醒来：数据目录和配置目录的文件变化、下一个到期时间，唯一的定时是 5 分钟一次的全量对账。一串连续事件合并成一次处理（安静 300 毫秒，最多推迟 2 秒），自己的提交不唤醒自己。新增唤醒来源时，同时加一条「空闲零唤醒」测试（参考 `internal/app/resident_test.go` 的 `TestIdleResidentHasZeroWakeups`）。
- `internal/hook`：各 agent 的薄壳，只把 payload 换成通用的 `Event`，具体处理全在 `Handle`。`claude.go`、`codex.go`（解析 apply_patch 补丁，只认第 0 列的文件头，把 WSL 的 `/mnt/x` 路径换成 `X:`）。永远退出 0，错误写进 `hook.log`；返回给 agent 的文字不超过 2000 字符。慢活通过 `Runner` 交给后台进程（`internal/cli/cmdutil/runner.go`），hook 里不做。
- Claude Code 的 Stop hook 每个回合都会触发，只要返回 additionalContext 或 block，Claude 就会接着跑下去，所以 Stop / SessionEnd 默认静默。唯一的例外是收工整理（`[hooks] session_cleanup`）：只对能接着干的宿主（Claude Code、Codex 的 Stop，由适配器设 `CanContinue`）、每个会话最多一次（先独占写下 `sessions/*.cleanup` 再输出，宁可丢一次也不循环）、只列 agent 自己这次新建且没归类的文件，`stop_hook_active` 时不出声。SessionEnd 默认只有 1.5 秒，重活交给 `obitd --once`。给 agent 的文字写成陈述，不写成命令（官方文档的建议，祈使句可能触发提示注入防护）。
- hook 的真机验收：在临时 Git 仓库里执行 `obit init --yes`（只写项目里的 `.claude/`，不碰全局），设好 `OBIT_HOME`，再跑 `claude -p "…" --model haiku --allowedTools Write`，检查 `obit proposals` 和 `hook.log`。
- `internal/guardrm`：删除护栏（`obit init --guard-rm` 才注册的 PreToolUse）。`rewrite.go` 的 `Suggest` 先解析命令（bash 在 `lex_bash.go`，用 shfmt 的解析器 `mvdan.cc/sh/v3/syntax`；PowerShell 在 `lex.go`，手写的引号感知分词），只认命令位置上的 `rm` / `rmdir` / `git clean` / `Remove-Item`，给出只换命令词、保留参数原文的等价 obit 命令，看不懂就放行；`options.go` / `powershell.go` 解析 `obit rm` / `rmdir` / `git-clean` / `remove-item` 接受的参数；`audit.go` 记拦截和放行的计数。每条 shell 命令前都会触发，所以不读账本、不起 git、不碰网络。
- `internal/integrate`：`obit init`。先算计划（记下原文），再 `Apply`（原文变了就拒绝、编号备份、临时文件加改名）。修改 `settings.json` 时保持键顺序、不做 HTML 转义；只有严格匹配 `obit … hook <agent>` 的命令才算我们的；卸载只删自己加的。`OBIT.md` 带归属标记，加 `@OBIT.md`。账本只有全局一份，记录里带 `Project` / `Session` / `Source`；`.obit/` 只放项目里的工作数据。
- `internal/integrate` 还负责 `--mcp` / `--skill`：`mcp.go`（Claude 的 `~/.claude.json` / `.mcp.json`）、`mcp_toml.go`（Codex `config.toml` 的标记块）、`skill.go`（`skill/SKILL.md` 用 go:embed 嵌入，归属记在旁边的 `.obituary-skill`）。`SKILL.md` 是给 agent 看的文档，由人来写。
- `internal/cli/mcp`：`obit mcp`（官方 Go SDK，stdio；`tools.go` 工具、`output.go` 回复与预算、`pagination.go` 分页）。stdout 只能写协议消息。工具清单由 `TestMCPToolInventory` 固定，用户的决定（approve / reject / trust / purge / 配置）不做成工具。变异检查用 `go test -overlay`：MCP 和核心关卡的变异表在 `internal/cli/mcp/mcp_test.go`（`OBIT_TEST_MCP_MUTATIONS=1 go test -timeout 30m -run TestMCPMutations ./internal/cli/mcp`），删除护栏的在 `internal/cli/remove/remove_test.go`（`OBIT_TEST_DELETE_MUTATIONS=1 go test -timeout 30m -run TestDeleteGuardMutations ./internal/cli/remove`）。整组 MCP 变异在空闲机器上约 6 分钟，机器忙时会超过 go test 默认的 10 分钟总时限，所以要带 `-timeout 30m`。变异条目的替换文本是拼进目标文件后编译的源码：把声明搬到别的包、改名或改成私有时，变异表里引用它的条目也要改（例如 core 拆包后要写 `containerIntentIdentity(p)`、不能再用已搬走的 `asciiPath`），并按名字把这些条目实际跑一遍；测试里往源文件末尾拼注入代码的夹具（如 `output_test.go`）同理。

## 原地升级与诊断

- `internal/update`：查新版和原地升级，窗口和 `obit update --apply` 共用 `update.Apply`。渠道按路径和同目录的安装回执判断，统计里的渠道名只来自 `DetectChannel`（不要让它多出新值）；没有回执的手动安装由 `manual.go` 的 `Inspect` 判断能否原地升级（正式发布版、有发布包、不在 `.app` 里、发布包里讣告自己的全部程序都是普通文件、当前用户能写），不满足就返回哪一条。`manual_install.go` 的事务只替换发布包里讣告自己的几个程序：**这个目录是共用的（可能在 PATH 上，有别的工具），不当作讣告独占，不写回执、不改 PATH 和开始菜单、不碰其他文件、不删任何东西**，旧程序改名放进目录旁的 `.obituary-update-*/old`，经 `ApplyHooks.Retire` 以 1 天寿命登记，任何一步失败整体改名回滚。下载先 GitHub、10 秒无响应才走国内镜像（HTTP 错误不回退），按 `checksums.txt` 核对 SHA-256 之后才解包，只取顶层同名条目。进度写 `update-progress.json`，窗口轮询到 `ready` 才退出，助手在窗口还开着时失败不重开窗口。`OBIT_UPDATE_BASE_URL` 只认回环 HTTP，供端到端测试。新增或改动一道关卡时，先在 `manual_test.go` / `manual_install_test.go` 写会失败的测试，改完把关卡临时改坏（必须能编译）确认测试抓得到；假发布服务器加无关文件的整套升级在 `internal/update`，真实构建的 1.4.3 → 1.4.4 在 `internal/e2e/manual_update_test.go`。发布说明按窗口语言取段落的逻辑在前端 `src/notes.ts`。
- `internal/cli/doctor`：`obit doctor`。默认只在终端打印精简结论（`summary.go`：检查按类别归并，一类一行，出问题的类别只写最严重的一项的原因和一条修复），同时把完整报告写成自包含的 HTML（`report.go`，`reports/` 在数据目录下，`app.ReportsDir`；用 `app.ExpireReport` 给 1 天寿命（`app.ReportLifespan`），Guard 的 `Exempt` 只放行这个文件夹里的内容；登记不上就把刚写的报告删掉，数据目录不存在时不替报告创建它）；`--verbose` 打印原来的逐项文字版（含 agent 能力，不写报告），`--open` 用系统浏览器打开报告，`--json` 不变也不写报告。检查本身仍然只读（`extensions.go` 是 MCP / Skill 的检查）。新增的安装环节要在这里加一项检查，并在 `summary.go` 的 `checkCategory` 里给它归类（`TestEveryCheckHasACategory` 会提醒）。

## 使用统计

统计默认开启，从首次真实使用起可上报；CLI、GUI 和协议入口均不提示、不请求确认。不要把空闲后台、健康检查或查更新算作活跃。各进程共享滚动 24 小时限额；失败也消耗限额，不重试。只发送随机安装 ID、版本、系统、架构、界面语言和安装渠道。持久关闭或设置 `OBIT_TELEMETRY=0` 应清除 ID 和待发状态；CI 和 `DO_NOT_TRACK=1` 禁止该进程上报。服务端不保存 IP 或 User-Agent。面向用户的说明放在官网中英文隐私页，不写进 README；`obit config telemetry --help` 列出关闭方式。
