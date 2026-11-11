# 讣告 Obituary

给文件和文件夹设定寿命，到期下葬进墓地，面向 AI agent 设计。命令行是 `obit`，后台进程是 `obitd`。Go 1.26，目前只支持 Windows。用法见 `README.md`，流程、关卡和 `core.FS` 契约见 `docs/architecture.md`（改动跨模块时先读它）。

## 布局

- `cmd/obit`：命令行入口（cobra），`main.go` 只创建根命令、注册子命令、把错误映射成退出码。子命令带主题别名：`mark`=add、`spare`=cancel、`reap`=sweep。
- `internal/cli/<命令>/`：每个命令一个包，构造函数是 `NewCmd`；`internal/cli/cmdutil` 放共用的打开服务、输出与回执（`ReportGraves`、`ErrReported`）、拉起后台进程（`runner.go`）；`internal/cli/clitest` 是只给测试用的公共辅助（文件、Git、stdio、runner、墓地夹具、MCP 连接）。
- `cmd/obitd`：后台进程，必须用 `-ldflags=-H=windowsgui` 构建；`--home` 指定数据目录。
- `internal/core`：跟踪列表（带修订号 `Rev`）、到期清理（逐条认领、按修订号回写、同一个 store 同时只跑一个 sweep）、安全检查（Guard）、时长解析、审计日志。访问文件系统只走 `core.FS` 接口。
- `internal/winfs`：`core.FS` 的 Windows 实现：128 位文件 ID（`GetFileInformationByHandleEx` / `OpenFileById`）、硬链接检查、`Canonical`（真实路径）、逐级展开长名（`longpath_windows.go`）、按序数规则比较名字（`name_windows.go`）、复活用的父目录句柄、按句柄建目录和改名（`restore_windows.go`），以及通过 `IFileOperation` vtable 移进回收站。
- `internal/core` 的墓地按职责分文件：`bury.go` / `bury_move.go` / `burial_lifespans.go`（下葬：独占建坟墓、写清单、锁内复核后按句柄移动、随父目录下葬的子条目）、`resurrect.go`（复活，不覆盖）以及 `restore_intent.go` / `restore_parents.go` / `restore_creation.go` / `restore_cleanup.go`（复活意图、祖先核对、按句柄建目录与撤销、清理回执）、`abandon.go`（显式放弃核实不了的复活）、`purge.go`（火化）、`manifest.go` / `recovery.go`（清单与恢复）、`sweep.go` / `settle.go`（到期收割）、`register.go`（登记）、`lookup.go`（按编号或路径找条目和墓地记录）、`graveyard.go` / `graveyard_owner.go`（墓地根、归属标记与公共逻辑）。复活和火化之前都必须过 `verifyGrave`。
- `internal/core` 的账本与审计：`store.go`（读写账本）、`lock_windows.go`（文件锁、按目录句柄落盘替换）、`read_windows.go`（共享删除的读取）、`audit.go` / `audit_outbox.go`（审计日志、待补审计队列）、`observations.go`（hook 没拿到锁时延后处理的写入事件）。协议见 `docs/architecture.md` 的「账本与审计」。
- `internal/app`：组装服务和 Guard（含已知文件夹），控制 obitd：默认是定时任务（`schedule_windows.go`，用 XML 定义，因为 schtasks 的参数设不了电池、补跑、隐藏这些选项），可选常驻模式（命名互斥量、stop 文件、HKCU Run 开机自启）；`RunOnce` 是给 hook 用的一次性后台处理。
- `internal/userdirs`：解析数据目录、配置目录、用户主目录、`~/.claude`（`ClaudeDir`）、`~/.codex`（`CodexDir`）的唯一入口，可以用 `OBIT_HOME` / `OBIT_CONFIG_DIR` / `OBIT_USER_HOME` / `CLAUDE_CONFIG_DIR` / `CODEX_HOME` 重定向。其他代码不准自己去找这些目录，`source_test.go` 会扫描源码强制这一点；确实需要（比如 Guard 要保护真实的用户文件夹）就加进豁免表并写明理由。
- `internal/testenv`：每个测试包的 `TestMain` 都调用 `testenv.Main(m)`，把上面这些目录和临时目录指到一次性目录，清掉继承来的 `GIT_*`，换上专用 gitconfig；用 `OBIT_TEST_REGISTRY_ROOT` 把本进程和它启动的 obit/obitd 子进程的右键菜单、开机启动项写到一次性的 `HKCU\Software\obituary-test-*` 下（结束时删掉），并在前后只读比较真实的讣告右键菜单、Run 项和开始菜单/桌面快捷方式，有变化就让测试失败；还会在测试前后比较 TEMP 所在卷根 `.obituary\graveyard` 下的顶层条目（账本键目录，以及 `<键>.owner`、`.owner.lock` 这类文件），多出新条目就让测试失败（说明东西漏到了真实卷根；不递归进其他账本）。测试里下葬时，对象和 Home 要用同一种卷根写法，否则坟墓会建到真实卷根的 `.obituary` 下。新建测试包时照抄一个 `main_test.go`。
- `internal/pathutil`：卷根（`VolumeRoot`，扩展 UNC 带上服务器和共享）和引用路径的绝对化（`Abs`：`\\?\` 写法只在 Windows 的规范化不改变它时接受，并转成普通写法；拒绝备用数据流；保留结尾分隔符表示的「必须是目录」）。文件系统读出来的名字再当普通路径用之前，先过 `Unchanged`。路径代码不要直接用 `filepath.VolumeName` 求卷根，也不要对 `\\?\` 写法直接调 `filepath.Abs`。
- `internal/config`：`config.toml`（TOML，所有键都可以省略，有内置默认值）。
- `internal/rules`：`.gitignore` 写法加寿命的规则、内置规则、信任存储（先信任再加载）。否定规则优先：目录里有受保护的东西时，不把整个目录当成一个单元。
- `internal/project`：Git 工作区。`Find` 只向上找 `.git`，不启动 git 进程（hook 每次都要调用它，而且 git 出错时必须得到「重试」而不是「不在项目里」），遵守 `GIT_CEILING_DIRECTORIES`。是否被跟踪只问 `git`（每次调用最多 15 秒，超时算错误），问不出来就当作已被跟踪。`.obit/` 会写进 `info/exclude`。
- `internal/auto`：按规则自动登记（Apply）、`MkTemp`；`observe.go` 给 hook 用：Observe（还没问过的项目先提议，已同意就按规则登记，太大的改为提议）、Snapshot / Leftovers（会话差分，遗留物只提议、不登记）。
- `internal/hook`：各 agent 的薄壳，只把 payload 换成通用的 `Event`，具体处理全在 `Handle`。`claude.go`、`codex.go`（解析 apply_patch 补丁，只认第 0 列的文件头，把 WSL 的 `/mnt/x` 路径换成 `X:`）。永远退出 0，错误写进 `hook.log`；返回给 agent 的文字不超过 2000 字符。慢活通过 `Runner` 交给后台进程（`internal/cli/cmdutil/runner.go`），hook 里不做。
- `internal/guardrm`：删除护栏（`obit init --guard-rm` 才注册的 PreToolUse）。`rewrite.go` 的 `Suggest` 先解析命令（bash 在 `lex_bash.go`，用 shfmt 的解析器 `mvdan.cc/sh/v3/syntax`；PowerShell 在 `lex.go`，手写的引号感知分词），只认命令位置上的 `rm` / `rmdir` / `git clean` / `Remove-Item`，给出只换命令词、保留参数原文的等价 obit 命令，看不懂就放行；`options.go` / `powershell.go` 解析 `obit rm` / `rmdir` / `git-clean` / `remove-item` 接受的参数；`audit.go` 记拦截和放行的计数。每条 shell 命令前都会触发，所以不读账本、不起 git、不碰网络。
- `internal/e2e`：跨模块的端到端测试（外部包）：真实 shell 执行、hook 与 MCP 的协议抓取、安装、后台进程。
- `internal/integrate`：`obit init`。先算计划（记下原文），再 `Apply`（原文变了就拒绝、编号备份、临时文件加改名）。修改 `settings.json` 时保持键顺序、不做 HTML 转义；只有严格匹配 `obit … hook <agent>` 的命令才算我们的；卸载只删自己加的。`OBIT.md` 带归属标记，加 `@OBIT.md`。账本只有全局一份，记录里带 `Project` / `Session` / `Source`；`.obit/` 只放项目里的工作数据。
- `internal/integrate` 还负责 `--mcp` / `--skill`：`mcp.go`（Claude 的 `~/.claude.json` / `.mcp.json`）、`mcp_toml.go`（Codex `config.toml` 的标记块）、`skill.go`（`skill/SKILL.md` 用 go:embed 嵌入，归属记在旁边的 `.obituary-skill`）。`SKILL.md` 是给 agent 看的文档，由人来写。
- `internal/cli/mcp`：`obit mcp`（官方 Go SDK，stdio；`tools.go` 工具、`output.go` 回复与预算、`pagination.go` 分页）。stdout 只能写协议消息。工具清单由 `TestMCPToolInventory` 固定，用户的决定（approve / reject / trust / purge / 配置）不做成工具。变异检查用 `go test -overlay`：MCP 和核心关卡的变异表在 `internal/cli/mcp/mcp_test.go`（`OBIT_TEST_MCP_MUTATIONS=1 go test -timeout 30m -run TestMCPMutations ./internal/cli/mcp`），删除护栏的在 `internal/cli/remove/remove_test.go`（`OBIT_TEST_DELETE_MUTATIONS=1 go test -timeout 30m -run TestDeleteGuardMutations ./internal/cli/remove`）。整组 MCP 变异在空闲机器上约 6 分钟，机器忙时会超过 go test 默认的 10 分钟总时限，所以要带 `-timeout 30m`。
- `internal/cli/doctor`：`obit doctor`，只读体检（`extensions.go` 是 MCP / Skill 的检查）。新增的安装环节要在这里加一项检查。
- `site/`：官网，pnpm workspace，和 Go 代码互不依赖。`landing/` 是落地页（Vite + React + Tailwind，组件只用 Magic UI 和自己写的），`docs/` 是文档站（VitePress，挂在 `/docs/`）。`pnpm build` 产出 GitHub Pages 版 `dist/`，`pnpm build:cn` 产出国内镜像版 `dist-cn/`：链接、给 agent 的那句话和安装脚本都指向 obituary.woodfish.site，不发任何站外请求，页脚有备案号。`scripts/assemble.mjs` 会核对 `docs/agent-install.md` 的原文，替换不上就让构建失败。贡献者墙在构建时拉取，头像存进产物。第三方图标的来源和许可写在 `site/THIRD_PARTY_NOTICES.md`，正文和注释里不提出处。`pages.yml` 只在仓库公开后部署（Pages 和镜像都是）。

## 规则

- **安全第一。** 把东西挪走只能走 `FS.MovePinned`（下葬：在同一个句柄上核对身份并改名）、`FS.MovePinnedInto`（复活：同样按句柄核对，移进已固定的父目录）或 `FS.Recycle`。用户内容的永久删除只有 `Purge` 一处，而且只能删除 `graveDir` 认可的坟墓目录。讣告清理自己的东西不算在内，但只能删自己能证明的：墓地清单和临时文件、核实过身份且为空的坟墓（下葬中止、复活提交之后）、本次自己新建且仍为空的目录（复活被拒绝时按创建句柄撤销、`mktemp` 登记失败时撤销）。用户的东西禁止直接 `os.Remove` / `RemoveAll`。回收失败必须保留原文件并重试，不能退化成永久删除。只有确定的 `ErrNotFound` 才能放弃一条记录；`ErrUnsafe` 算拒绝；其他错误一律重试。
- 改 Guard、`locate`、`settle`、`commit` 或回收逻辑时，先写一个能证明不会误删的失败测试；改完后把修复临时改回去，确认测试真的能抓到。改回去的版本必须能编译，编译失败不算验证。
- 自动登记相关的改动，测试要跑在真的临时 Git 仓库里（参考 `internal/auto` 的 `newRepo`），不要假装 Git 的回答。
- Claude Code 的 Stop hook 每个回合都会触发，只要返回 additionalContext，Claude 就会接着跑下去，所以 Stop / SessionEnd 必须静默。SessionEnd 默认只有 1.5 秒，重活交给 `obitd --once`。给 agent 的文字写成陈述，不写成命令（官方文档的建议，祈使句可能触发提示注入防护）。
- hook 的真机验收：在临时 Git 仓库里执行 `obit init --yes`（只写项目里的 `.claude/`，不碰全局），设好 `OBIT_HOME`，再跑 `claude -p "…" --model haiku --allowedTools Write`，检查 `obit proposals` 和 `hook.log`。
- **进度不放在本仓库。** 路线、里程碑、任务拆分和决策记录在维护者的私有仓库里（维护者本机检出在被忽略的 `docs-internal/`），由维护者更新。GitHub 上只保留里程碑和少量 issue：不要把任务拆成一串 issue，一次最多新开一个（所有者 2026-11-09 定）；做完已有 issue 对应的事项时在提交说明里写 `Closes #<编号>`。
- **测试按功能命名，不按审查轮次或阶段命名。** 新测试放进它所测功能的文件（`graveyard_test.go`、`manifest_test.go`、`delete_guard_test.go`……），不要新建 `review5_test.go`、`m2a_test.go` 这类文件；每轮修了什么、为什么修，写进 commit message。跨模块的端到端测试（真实 shell 执行 → 下葬 → 复活、协议抓取）放在 `internal/e2e/`，以外部包的方式写。
- **测试文件与源文件一一配对。** `bury.go` 的测试放 `bury_test.go`；只有共用的 `helpers_test.go`、`main_test.go`、`export_test.go` 例外，以及跨模块的 `internal/e2e/`。外部测试包（`package x_test`）因为导入循环没法并进同名的 `_test.go` 时，用 `<源文件>_external_test.go`。不要新建找不到同名源文件的测试文件（如 `mcp_recycle_test.go`），把测试并进它所测函数所在源文件对应的 `_test.go`。
- **`cmd/` 只做组装。** `cmd/obit/main.go` 只注册命令；命令的实现放 `internal/cli/<命令>/`，共用的打开服务、输出、拉起后台进程放 `internal/cli/cmdutil`。新命令照这个结构加，`main.go` 里只多一行注册。
- **文件按职责拆分。** 一个文件超过约 500 行、或装了好几件事时，按职责拆开（例如墓地的下葬、复活、火化、恢复各自一个文件）。重组只改结构不改行为：前后 `go test ./...` 的测试数量与结果一致，原有的变异检查照样能抓到。
- 测试只测可观察的行为。`core` 的测试用真实文件 ID 加上不进回收站的 `testFS`，不碰用户的回收站。真实回收站测试要设 `OBIT_TEST_RECYCLE=1`，跑完从回收站清掉 `obit-*` 条目。
- 手动验收时 `OBIT_HOME` 和 `OBIT_CONFIG_DIR` 都要指向临时目录，不要污染 `%LOCALAPPDATA%\obituary` 和 `%APPDATA%\obituary`；结束后执行 `obit daemon stop`。
- 优先采用同类成熟工具验证过的做法，不要自己从头摸索：hook、init、配置、统计、安装、MCP 注册、Skill、测试和性能写法，动手前先调研同类工具怎么做。公开的文档和注释里只写这样做的理由（满足哪条安全或兼容要求），不写「照搬、借鉴、参考了某某项目」。
- **先用现成的代码，不重复造轮子。** 写底层代码之前先查 Go 标准库（例如 `os.Root` 的句柄相对操作：Go 1.25 起 `Root.Rename` 在 Windows 上就是带 POSIX 语义、按目录句柄的替换改名），再查维护活跃、许可证兼容（MIT / BSD / Apache-2.0）的成熟库（例如 `mvdan.cc/sh` 解析 bash、go-git 的 `plumbing/format/gitignore` 匹配 gitignore 模式、`tidwall/sjson` 保序改 JSON、`gofrs/flock` 文件锁）。确实要手写时，在代码注释和 commit message 里写明现成的为什么不行（缺什么能力、哪个安全要求满足不了）。换成现成的库时只换实现、不改行为：前后测试数量与结果一致，原有的变异检查照样能抓到。
- **临时目录和证据目录用讣告自己管。** 调试脚本、日志、复现目录、报告证据用 `dir=$(obit mktemp 2d)` 创建，或对已有的目录执行 `obit mark <路径> 2d`，到期自动下葬、可以复活，不靠记得去删。`obit mktemp` 失败（例如沙箱账户写不了账本）才退回 `%TEMP%` 下手建的 `obit-<任务名>-<随机>` 目录，并在报告里写明。含 `.git` 的目录讣告会拒绝登记，用完自己删。
- **文档写现在的规则和原因，不写过程。** `docs/architecture.md`、README 只写现在怎么做、为什么；「第几轮审查试过什么、又撤回了」这类经过写进提交说明，定期压缩成规则。
- 这台机器上 Git Bash 的 `rm` 是会把文件移进回收站的包装脚本。清理自己的临时文件用 `/usr/bin/rm`。排查回收站里多出来的条目时，先排除 `rm`，再怀疑代码。

## 命令

```bash
go vet ./... && go test ./...
golangci-lint run ./...   # 只报相对 main 新增的问题；装法：go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
go build -o bin/obit.exe ./cmd/obit
go build -ldflags=-H=windowsgui -o bin/obitd.exe ./cmd/obitd
```

`bin/` 已忽略。只格式化改过的文件（`gofmt -w <files>`）。

**静态检查（所有者 2026-11-10 定）。** `.golangci.yml` 只开 staticcheck、errorlint、funlen（50 行）、nestif（复杂度 4），对应上面「函数不超过 50 行、嵌套不超过 4 层」的规则。只拦新增的问题（`new-from-merge-base: main`）；已有的超长函数和深嵌套是重构清单，记在 docs/milestones/m3-speed.md。交付前和 vet 一起跑，新代码不许新增告警；确需例外用 `//nolint:<linter> // 原因` 写明理由。

**测试分两档，全量只在 CI 上跑（所有者 2026-11-10 定，决策 D-0038）。** 测试大多碰真实文件系统、Git 和子进程。几个 worker 在同一台机器上同时跑全量时，全部慢二三十倍，所以：

- 本机（worker 和协调员都一样）：只跑改动涉及的包，不加 `-count=1`（让 `go test` 的缓存跳过没变的包），例如 `go test ./internal/core -run TestBury`；想快速摸一遍全仓库用 `go test -short ./...`。变异只跑相关的几项，子测试按名字过滤：`OBIT_TEST_MCP_MUTATIONS=1 go test -count=1 -run 'TestMCPMutations/purge-' ./internal/cli/mcp`，删除护栏同理（`TestDeleteGuardMutations/<名字>`）。
- 交付前：`gofmt -l`、`go vet ./...`、`golangci-lint run ./...`，加上改动涉及的包和相关变异。本机不跑全量和两组全量变异。
- 合并把关分三层：worker 交付前把改动的包用完整模式跑一遍、上层包跑 -short；协调员每攒一批动到移动、下葬、删除护栏的合并（core、winfs、integrate、explorer、watch、hook）或三个以上分支，推送前在本机后台跑一次 `go test -count=1 -p 4 -timeout 30m ./...`；发版前、账本格式升级后、大规模机械改动后手动触发 CI 全量（`gh workflow run ci.yml -f full=true`，发版前再加 `-f mutations=true`；变异测试在 CI 上按编号拆到 8 台机器并行，本地可以用 `OBIT_MUTATION_SHARD=k/n` 只跑其中一份）。本机整树全量同一时间只跑一处。
- 全量和两组变异在 CI 上跑。仓库是私有的，Windows 机器按两倍计费，所以每次推送和 PR 只在一台机器上跑 `go test -short ./...`；全量（按包拆成四个并行任务，列出最慢的 30 个测试）每晚在 main 有新提交时跑，也可以手动触发；变异每周跑。打包和安装测试只在打包相关文件改动时跑。协调员把合并攒起来，大约一小时推一次 main，不是每合一个就推。
- 新写的测试：只要会起子进程、建 Git 仓库或要等超过 1 秒，就在开头写 `if testing.Short() { t.Skip("…") }`，并保证同一处逻辑在 `-short` 下还有一个快的测试覆盖。测试里默认不 fsync，只有专门检查落盘顺序的测试打开。

## 已知坑

- 在 Git Bash 里直接敲 `schtasks /Delete /TN …` 这类命令时，`/Delete` 会被 MSYS 当成路径转换掉。手动操作用 PowerShell（`Get-ScheduledTask`、`Unregister-ScheduledTask`，它们也能看到隐藏的任务）；Go 代码里调用 `schtasks` 不受影响。
- `schtasks /Query /XML` 按控制台代码页（GBK）输出，任务 XML 里只放 ASCII。
- 真实的任务计划测试要设 `OBIT_TEST_SCHEDULER=1`。测试任务的名字是唯一的，清理时按这个名字删，不要依赖全局变量（defer 会比 t.Cleanup 先执行）。

- 从 WSL 通过 interop 启动 `obit.exe` 时，Windows 的环境变量（`LOCALAPPDATA` 等）都能拿到；但 WSL 里设的变量要靠 `WSLENV` 才能传过去，测试时用 `WSLENV=OBIT_HOME/w`。
- 别在 heredoc 里用 Python 修改含 `\n` 的 Go 字符串字面量：`\n` 会变成真的换行，导致编译失败。改这类地方用 Edit 工具。
- 同一条命令里既有 `wsl … -c` 脚本又有 `rm` 时，Claude Code 的安全检查会把整条命令拦下。拆成两步：先跑脚本，再用确切的路径单独 `/usr/bin/rm`。

- `CoInitializeEx` 返回 `S_FALSE`（已经初始化过）时，x/sys 会把它当成错误，要单独放行。
- `GetFinalPathNameByHandle` 返回的路径带 `\\?\` 前缀并且是长文件名。和 `t.TempDir()` 返回的短名比较之前，先用 `filepath.EvalSymlinks` 统一。去掉前缀之后，要再验证这个路径还能找回同一个对象（文件名以点结尾的情况）。
- `filepath.WalkDir` 里对非目录条目返回 `SkipDir`，会跳过它后面的所有同级条目，找 `.git` 时会漏掉，所以不要这样用。
- 这台机器上的 `python3` 是 Windows 应用商店的占位程序，要用 `python`。
- 不要把测试的 TEMP 指向 Python `tempfile.mkdtemp` 建的目录（Python 3.13 起它在 Windows 上带受保护的 ACL）：`TestOwnerReportsDeniedProjectDiscovery` 在那里恢复 `.git` 的 ACL 会失败，留下只有管理员能删的目录。清理办法：提权后对确切的目录执行 `takeown /f <目录> /r /d y` 和 `icacls <目录> /reset /t /c /q`，再用普通权限删掉。
