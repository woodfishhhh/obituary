# 讣告 Obituary

给文件和文件夹设定寿命，到期下葬进墓地，面向 AI agent 设计。命令行是 `obit`，后台进程是 `obitd`。Go 1.26，支持 Windows、macOS 和 Linux。Windows 与 Ubuntu 24.04 amd64 有小窗口和托盘；资源管理器右键菜单和系统回收站模式只在 Windows 上。用法见 `README.md`，流程、关卡和 `core.FS` 契约见 `docs/architecture.md`（改动跨模块时先读它）。

## 使用统计

统计默认开启，从首次真实使用起可上报；CLI、GUI 和协议入口均不提示、不请求确认。不要把空闲后台、健康检查或查更新算作活跃。各进程共享滚动 24 小时限额；失败也消耗限额，不重试。只发送随机安装 ID、版本、系统、架构、界面语言和安装渠道。持久关闭或设置 `OBIT_TELEMETRY=0` 应清除 ID 和待发状态；CI 和 `DO_NOT_TRACK=1` 禁止该进程上报。服务端不保存 IP 或 User-Agent。面向用户的说明放在官网中英文隐私页，不写进 README；`obit config telemetry --help` 列出关闭方式。

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
- `internal/testenv`：每个测试包的 `TestMain` 都调用 `testenv.Main(m)`，把上面这些目录和临时目录指到一次性目录，清掉继承来的 `GIT_*`，换上专用 gitconfig；用 `OBIT_TEST_REGISTRY_ROOT` 把本进程和它启动的 obit/obitd 子进程的右键菜单、开机启动项写到一次性的 `HKCU\Software\obituary-test-*` 下（结束时删掉），并在前后只读比较真实的讣告右键菜单、Run 项和开始菜单/桌面快捷方式，有变化就让测试失败；还会在测试前后比较 TEMP 所在卷根 `.obituary\graveyard` 下的顶层条目（账本键目录，以及 `<键>.owner`、`.owner.lock` 这类文件），多出新条目就让测试失败（说明东西漏到了真实卷根；不递归进其他账本）。测试里下葬时，对象和 Home 要用同一种卷根写法，否则坟墓会建到真实卷根的 `.obituary` 下。新建测试包时照抄一个 `main_test.go`。编译一次性源码（`-overlay` 变异体和夹具）的 go 命令要带上 `testenv.MutantGoEnv(t)` 给的环境（`gocache.go`）：GOCACHEPROG 指回测试二进制，只读命中用户的 GOCACHE，新条目写进随测试删除的目录。否则每个变异体连同它上面的包都会在用户 GOCACHE 里留下再也命中不了的条目（一轮 MCP 变异约 6 GB）。`TestOverlayBuildsUseMutantGoEnv` 扫描源码强制这一点。
- `internal/pathutil`：卷根（`VolumeRoot`，扩展 UNC 带上服务器和共享）和引用路径的绝对化（`Abs`：`\\?\` 写法只在 Windows 的规范化不改变它时接受，并转成普通写法；拒绝备用数据流；保留结尾分隔符表示的「必须是目录」）。文件系统读出来的名字再当普通路径用之前，先过 `Unchanged`。路径代码不要直接用 `filepath.VolumeName` 求卷根，也不要对 `\\?\` 写法直接调 `filepath.Abs`。
- `internal/config`：`config.toml`（TOML，所有键都可以省略，有内置默认值）。
- `internal/rules`：`.gitignore` 写法加寿命的规则、内置规则、信任存储（先信任再加载）。否定规则优先：目录里有受保护的东西时，不把整个目录当成一个单元。
- `internal/project`：Git 工作区。`Find` 只向上找 `.git`，不启动 git 进程（hook 每次都要调用它，而且 git 出错时必须得到「重试」而不是「不在项目里」），遵守 `GIT_CEILING_DIRECTORIES`。是否被跟踪只问 `git`（每次调用最多 15 秒，超时算错误），问不出来就当作已被跟踪。`.obit/` 会写进 `info/exclude`。
- `internal/auto`：按规则自动登记（Apply）、`MkTemp`；`observe.go` 给 hook 用：Observe（还没问过的项目先提议，已同意就按规则登记，太大的改为提议）、Snapshot / Leftovers（会话差分，新路径交给规则，规则认不得的不登记也不提议）。`[proposals] mode` 默认 `off`，这些都不做；依赖、构建输出和缓存（`NeverAutomatic`）任何模式都不自动登记。
- `internal/hook`：各 agent 的薄壳，只把 payload 换成通用的 `Event`，具体处理全在 `Handle`。`claude.go`、`codex.go`（解析 apply_patch 补丁，只认第 0 列的文件头，把 WSL 的 `/mnt/x` 路径换成 `X:`）。永远退出 0，错误写进 `hook.log`；返回给 agent 的文字不超过 2000 字符。慢活通过 `Runner` 交给后台进程（`internal/cli/cmdutil/runner.go`），hook 里不做。
- `internal/guardrm`：删除护栏（`obit init --guard-rm` 才注册的 PreToolUse）。`rewrite.go` 的 `Suggest` 先解析命令（bash 在 `lex_bash.go`，用 shfmt 的解析器 `mvdan.cc/sh/v3/syntax`；PowerShell 在 `lex.go`，手写的引号感知分词），只认命令位置上的 `rm` / `rmdir` / `git clean` / `Remove-Item`，给出只换命令词、保留参数原文的等价 obit 命令，看不懂就放行；`options.go` / `powershell.go` 解析 `obit rm` / `rmdir` / `git-clean` / `remove-item` 接受的参数；`audit.go` 记拦截和放行的计数。每条 shell 命令前都会触发，所以不读账本、不起 git、不碰网络。
- `internal/e2e`：跨模块的端到端测试（外部包）：真实 shell 执行、hook 与 MCP 的协议抓取、安装、后台进程。
- `internal/integrate`：`obit init`。先算计划（记下原文），再 `Apply`（原文变了就拒绝、编号备份、临时文件加改名）。修改 `settings.json` 时保持键顺序、不做 HTML 转义；只有严格匹配 `obit … hook <agent>` 的命令才算我们的；卸载只删自己加的。`OBIT.md` 带归属标记，加 `@OBIT.md`。账本只有全局一份，记录里带 `Project` / `Session` / `Source`；`.obit/` 只放项目里的工作数据。
- `internal/integrate` 还负责 `--mcp` / `--skill`：`mcp.go`（Claude 的 `~/.claude.json` / `.mcp.json`）、`mcp_toml.go`（Codex `config.toml` 的标记块）、`skill.go`（`skill/SKILL.md` 用 go:embed 嵌入，归属记在旁边的 `.obituary-skill`）。`SKILL.md` 是给 agent 看的文档，由人来写。
- `internal/cli/mcp`：`obit mcp`（官方 Go SDK，stdio；`tools.go` 工具、`output.go` 回复与预算、`pagination.go` 分页）。stdout 只能写协议消息。工具清单由 `TestMCPToolInventory` 固定，用户的决定（approve / reject / trust / purge / 配置）不做成工具。变异检查用 `go test -overlay`：MCP 和核心关卡的变异表在 `internal/cli/mcp/mcp_test.go`（`OBIT_TEST_MCP_MUTATIONS=1 go test -timeout 30m -run TestMCPMutations ./internal/cli/mcp`），删除护栏的在 `internal/cli/remove/remove_test.go`（`OBIT_TEST_DELETE_MUTATIONS=1 go test -timeout 30m -run TestDeleteGuardMutations ./internal/cli/remove`）。整组 MCP 变异在空闲机器上约 6 分钟，机器忙时会超过 go test 默认的 10 分钟总时限，所以要带 `-timeout 30m`。变异条目的替换文本是拼进目标文件后编译的源码：把声明搬到别的包、改名或改成私有时，变异表里引用它的条目也要改（例如 core 拆包后要写 `containerIntentIdentity(p)`、不能再用已搬走的 `asciiPath`），并按名字把这些条目实际跑一遍；测试里往源文件末尾拼注入代码的夹具（如 `output_test.go`）同理。
- `internal/cli/doctor`：`obit doctor`，只读体检（`extensions.go` 是 MCP / Skill 的检查）。新增的安装环节要在这里加一项检查。
- `site/`：官网，pnpm workspace，和 Go 代码互不依赖。`landing/` 是落地页（Vite + React + Tailwind，组件用 Magic UI、React Bits 和自己写的；React Bits 放在 `components/reactbits/`，许可是 MIT + Commons Clause，只能随网站使用，不能单独再分发），`docs/` 是文档站（VitePress，挂在 `/docs/`）。`pnpm build` 产出 GitHub Pages 版 `dist/`，`pnpm build:cn` 产出国内镜像版 `dist-cn/`：链接、给 agent 的那句话和安装脚本都指向 obituary.woodfish.site，不发任何站外请求，页脚有备案号。`scripts/assemble.mjs` 会核对 `docs/agent-install.md` 的原文，替换不上就让构建失败。贡献者墙在构建时拉取，头像存进产物。第三方图标的来源和许可写在 `site/THIRD_PARTY_NOTICES.md`，正文和注释里不提出处。`pages.yml` 只在仓库公开后部署（Pages 和镜像都是）。首页的小窗口（第一屏的日历和「小窗口」那组标签）是 GUI 前端的真组件：`landing/vite.config.ts` 用别名 `@gui` 引入 `cmd/obit-gui/frontend/src`（去掉它样式里的整页规则，图标路径指向官网的 `icon-128.png`），`components/window-demo.tsx` 套上 `src/demo/` 的示例数据（`data.ts` 的文件、寿命和备注，`api.ts` 包一层 `@gui/mock`）和悬停卡片，所以改 GUI 官网会跟着变；演示在浏览器里懒加载，服务端和水合前都渲染同样大小的占位。首页有和 README 一样的 10 种语言（`src/locales.ts` 是语言表：简中在 `/`、英文在 `/en/`，其余在 `/<语言码>/`；繁中也用中国国旗）。文案在 `src/copy/<语言>.ts`，都得和 `zh.ts` 同形（类型检查保证不缺句），术语跟 README 的术语表一致；小窗口里的界面只有中英文，其他语言显示英文，示例备注（`src/demo/data.ts`）每种语言都有。构建时 `scripts/prerender.mjs` 用一份模板给每种语言写出完整 HTML（`<html lang>`、hreflang 互链、canonical、sitemap.xml、robots.txt；`vite build --ssr` 出 `dist-ssr`，用完删除），浏览器再水合：新加的组件不要在渲染期读 `window`、`navigator`、随机数或时间，这些放进 effect（访客的系统、浏览器语言提示、复制按钮都是这样，先出不依赖检测的版本）。新增用户可见文字要在全部 10 份文案里加。繁中、日、韩用访客的系统字体，只有简中用裁剪过的 Noto Sans SC。小窗口的样式表放进低优先级的 `gui` 样式层，免得它的全局 `button` 规则压住官网的工具类。绕边框的光（`components/ui/border-beam.tsx`）和其他持续动画只用 transform / opacity 这类合成层属性，离屏和标签页隐藏时暂停，开了减少动态效果就不显示；不要用脚本每帧改样式。首屏性能的几条线（用 Lighthouse 移动端和降速 4 倍的 trace 量过）：首屏文字在 HTML 里立刻可见，不能被依赖脚本的入场动画（opacity 0）挡住，不然 LCP 要等脚本跑完；入场用纯 CSS（`.rise`、`.reveal`、`.rw-in`），不用 motion；用到 motion 的区块（Flow、Story）是懒加载的，预渲染用 `react-dom/static` 的 `prerender` 等它们出完 HTML；每个页面只加载自己语言的文案（`copy/index.ts` 的 `loaders`，预渲染给该语言的模块和首屏字体写 modulepreload / preload，字体不预加载会在换字体时整块位移）；首屏以下的区块用 `content-visibility: auto`；大面积柔光用径向渐变，不用 `blur-3xl`；小窗口的字体用 `font-display: block`，不然它加载字体时会位移。首屏以下的区块和页脚是 HTML 从一开始就在、脚本延后加载的（`App.tsx` 的 `Below`，`lib/idle.ts`：访客第一次滚动、触摸、按键、移动指针或点 #链接时，或加载完 4.5 秒后）；首屏只水合 Nav 和 Hero。要在首屏以下加依赖脚本的新东西，放进 `Below`；`Reveal` 的隐藏态靠它自己加的 `html.js`，所以没水合的区块一直可见。页面自己的字体用 `font-display: optional`（vite.config.ts 的 postcss 插件改写），首屏字体由预渲染 preload，晚到的字体留给下次访问，不在阅读中换字体；轮播字所在的标签行在手机上预留两行高度，换词不推动页面。轮播字的动画只动 transform 和 opacity：字母用 `.rw-in` / `.rw-out`（index.css，改样式表时确认它们还在，缺了就没有动画、新旧词叠在一起），换词时药丸、两侧文字和词槽在布局上立刻变成新尺寸，再各用一个 transform 动画从原位滑过去，宽度不做动画，主线程忙也不会卡住；计时只在标签行在屏幕内且页面可见时走，回来重新数一个完整间隔；减少动态效果时直接换词。首屏以下的 section 不加 `content-visibility: auto`：折叠区的估算高度和真实高度差几百像素，页内锚点会落偏（测过，导航跳转偏差最多 290 px，换来的 TBT 只少 7 ms 左右）；`.rise` 和 `.reveal` 在 index.css 里，`.reveal` 只在组件的脚本真的开始观察它时才隐藏（`.armed`），脚本没到的区块一直是可见的。新增效果前后都跑一遍 Lighthouse，不要把 LCP、TBT、CLS 拉回去。检查动画用无头浏览器按时间截帧（全局的 Playwright 即可）：chrome-devtools 打开的窗口被遮挡时 requestAnimationFrame 每秒只跑两帧，动画看起来像卡住。

## 规则

- **安全第一。** 把东西挪走只能走 `FS.MovePinned`（下葬：在同一个句柄上核对身份并改名）、`FS.MovePinnedInto`（复活：同样按句柄核对，移进已固定的父目录）或 `FS.Recycle`。用户内容的永久删除只有 `Purge` 一处，而且只能删除 `graveDir` 认可的坟墓目录。讣告清理自己的东西不算在内，但只能删自己能证明的：墓地清单和临时文件、核实过身份且为空的坟墓（下葬中止、复活提交之后）、本次自己新建且仍为空的目录（复活被拒绝时按创建句柄撤销、`mktemp` 登记失败时撤销）。用户的东西禁止直接 `os.Remove` / `RemoveAll`。回收失败必须保留原文件并重试，不能退化成永久删除。只有确定的 `ErrNotFound` 才能放弃一条记录；`ErrUnsafe` 算拒绝；其他错误一律重试。
- 改 Guard、`locate`、`settle`、`commit` 或回收逻辑时，先写一个能证明不会误删的失败测试；改完后把修复临时改回去，确认测试真的能抓到。改回去的版本必须能编译，编译失败不算验证。
- 自动登记相关的改动，测试要跑在真的临时 Git 仓库里（参考 `internal/auto` 的 `newRepo`），不要假装 Git 的回答。
- Claude Code 的 Stop hook 每个回合都会触发，只要返回 additionalContext 或 block，Claude 就会接着跑下去，所以 Stop / SessionEnd 默认静默。唯一的例外是收工整理（`[hooks] session_cleanup`）：只对能接着干的宿主（Claude Code、Codex 的 Stop，由适配器设 `CanContinue`）、每个会话最多一次（先独占写下 `sessions/*.cleanup` 再输出，宁可丢一次也不循环）、只列 agent 自己这次新建且没归类的文件，`stop_hook_active` 时不出声。SessionEnd 默认只有 1.5 秒，重活交给 `obitd --once`。给 agent 的文字写成陈述，不写成命令（官方文档的建议，祈使句可能触发提示注入防护）。
- hook 的真机验收：在临时 Git 仓库里执行 `obit init --yes`（只写项目里的 `.claude/`，不碰全局），设好 `OBIT_HOME`，再跑 `claude -p "…" --model haiku --allowedTools Write`，检查 `obit proposals` 和 `hook.log`。
- **常驻进程和托盘不定时轮询（2026-11-14 定）。** 只靠事件醒来：数据目录和配置目录的文件变化、下一个到期时间，唯一的定时是 5 分钟一次的全量对账。一串连续事件合并成一次处理（安静 300 毫秒，最多推迟 2 秒），自己的提交不唤醒自己。新增唤醒来源时，同时加一条「空闲零唤醒」测试（参考 `internal/app/resident_test.go` 的 `TestIdleResidentHasZeroWakeups`）。
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

## 国内发布镜像

`mirror-latest.yml` 与 `pages.yml` 的 `deploy-cn` 共用 `cn-deploy`，不取消运行中的任务。正式 Release 发布时自动推送；重放用 `gh workflow run mirror-latest.yml --ref main -f tag=vX.Y.Z`。runner 用只读 `GITHUB_TOKEN` 下载 Release 的六个归档和 `checksums.txt`，从同一 Git 标签取两个安装脚本；核对归档清单和 SHA-256 后，经固定的 `CN_DEPLOY_KNOWN_HOSTS` 上传。`transport.sha256` 校验脚本和元数据的传输，不是独立的发布者签名。

专用仓库 Secret 是 `CN_RELEASE_DEPLOY_KEY`，ed25519 指纹 `SHA256:XDh6ylMRs/gpoyCC1T2TXzwFlHlIUOLIpmPzVpdoQ2E`。非 root 用户 `obit-deploy` 的 `authorized_keys` 只为这把钥匙增加 `restrict,command="/usr/local/bin/obituary-mirror-publish"`。该入口来自 `scripts/cn-mirror-pull.sh`，只接受以 `/home/obit-deploy/release-staging`（0700）为根的只写 rrsync，或 `publish vX.Y.Z`。两者都拿 `download/.pull.lock`；服务端从 `incoming/` 复核文件和哈希，在同一文件系统上改名发布完整版本，再按 runner 核对的最新正式版原子更新 `latest.json`。同标签内容不同则拒绝覆盖，相同则保留原文件。日志用 `journalctl -t obituary-mirror` 查。

人工备用仍是原来的 `pull vX.Y.Z`（`CN_MIRROR_TRIGGER_KEY`、`/usr/local/bin/obituary-mirror-pull`，源码 `scripts/cn-mirror-pull-legacy.sh`）；只有这条备用路径从服务器访问 GitHub。回滚时先等 `cn-deploy` 空闲，回退本次工作流，再只移除上述指纹对应的新增授权行并撤销 `CN_RELEASE_DEPLOY_KEY`。核对原授权行和旧脚本与服务器 `/root/obituary-cn-mirror-<日期时间>-<后缀>/` 中的备份一致，不覆盖后续新增授权。需要恢复索引时，先核对备份的 `download.inventory` 和最新版本，持 `.pull.lock` 将备份的 `latest.json` 通过同目录临时文件改名恢复；不要删除任何已发布版本。最后停用新增入口和 staging，保留备份及旧 `pull` 入口。回滚不改 `CN_DEPLOY_KEY`、原站点钥匙或 `deploy-cn`。

## 命令

```bash
go vet ./... && go test ./...
golangci-lint run ./...   # 只报相对 main 新增的问题；装法：go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
go build -o bin/obit.exe ./cmd/obit
go build -ldflags=-H=windowsgui -o bin/obitd.exe ./cmd/obitd
```

`bin/` 已忽略。只格式化改过的文件（`gofmt -w <files>`）。

Linux 桌面用 Ubuntu 24.04 amd64 原生构建：安装 `libgtk-3-dev libwebkit2gtk-4.1-dev`，前端改动先运行 `node scripts/build-frontend.mjs`。`CGO_ENABLED=1 go build -tags desktop,production,webkit2_41 -o bin/obit-gui ./cmd/obit-gui` 构建 Wails 小窗口。`goreleaser release -f .goreleaser-linux-desktop.yaml --snapshot --skip=publish` 生成 `bin/linux-desktop/*.deb`；六个原有归档仍用 `.goreleaser.yaml`。桌面作业的系统依赖见 `.github/workflows/linux-desktop.yml`；`bash scripts/test-linux-desktop-session.sh bin/linux-desktop/*.deb <证据目录>` 使用私有 Xvfb、会话总线和临时用户目录，验证窗口、托盘、复活和通知。不要在宿主显示器上验收。仅在一次性 Ubuntu 24.04 容器中用 `scripts/test-linux-desktop-package.sh` 验证安装与移除。

macOS GUI 只能在 GitHub Actions 的 macOS runner 上构建和验收：`go test -tags desktop ./internal/app ./internal/gui ./internal/tray` 验证原生绑定，`bash scripts/build-macos-desktop.sh VERSION OUTPUT_DIR` 生成通用 `Obituary.app`、DMG 和 checksums。工作流 `.github/workflows/macos-desktop.yml` 还会用 `plutil`、`lipo`、ad-hoc `codesign`、`hdiutil` 和 app 内的 `--smoke` 检查包，并上传窗口与菜单栏截图；Windows 本地不要交叉构建 macOS GUI，也不要把 CI 收据当作真机通知授权或首次放行证明。

**静态检查（所有者 2026-11-10 定）。** `.golangci.yml` 只开 staticcheck、errorlint、funlen（50 行）、nestif（复杂度 4），对应上面「函数不超过 50 行、嵌套不超过 4 层」的规则。只拦新增的问题（`new-from-merge-base: main`）；已有的超长函数和深嵌套是重构清单，记在 docs/milestones/m3-speed.md。交付前和 vet 一起跑，新代码不许新增告警；确需例外用 `//nolint:<linter> // 原因` 写明理由。`unused` 不在 CI 里：Linux 构建下，只被 Windows 代码调用的共用函数会被误报。拆包或删掉调用方之后，手动跑一次 `golangci-lint run --new-from-merge-base= --enable-only unused ./...` 查死代码，Windows 构建下应当是 0。

**测试分两档，全量只在 CI 上跑（所有者 2026-11-10 定，决策 D-0038）。** 测试大多碰真实文件系统、Git 和子进程。几个 worker 在同一台机器上同时跑全量时，全部慢二三十倍，所以：

- 本机（worker 和协调员都一样）：只跑改动涉及的包，不加 `-count=1`（让 `go test` 的缓存跳过没变的包），例如 `go test ./internal/core -run TestBury`；想快速摸一遍全仓库用 `go test -short ./...`。变异只跑相关的几项，子测试按名字过滤：`OBIT_TEST_MCP_MUTATIONS=1 go test -count=1 -run 'TestMCPMutations/purge-' ./internal/cli/mcp`，删除护栏同理（`TestDeleteGuardMutations/<名字>`）。
- 交付前：`gofmt -l`、`go vet ./...`、`golangci-lint run ./...`，加上改动涉及的包和相关变异。本机不跑全量和两组全量变异。
- 合并把关分三层：worker 交付前把改动的包用完整模式跑一遍、上层包跑 -short；协调员合并后直接推送，看 CI 的全量结果（推送就跑全量，见下一条），不必先在本机跑整树全量；发版前、账本格式升级后、大规模机械改动后手动触发一次带变异的 CI（`gh workflow run ci.yml -f full=true -f mutations=true`；变异测试在 CI 上按编号拆到 8 台机器并行，本地可以用 `OBIT_MUTATION_SHARD=k/n` 只跑其中一份）。本机确实要跑整树全量时，同一时间只跑一处。
- 全量和变异在 CI 上跑。仓库公开，标准机器不计分钟：每次推送和 PR 都跑全量（按包拆成四个并行任务，列出最慢的 30 个测试）和 Linux（带 `-race`）、macOS 上的 POSIX 测试，大约 10 分钟出结果；变异每周跑一次，也可以手动触发。打包和安装测试只在打包相关文件改动时跑（最慢的一项，为省时间）。推送不用攒批，推完看一眼 CI。
- 新写的测试：只要会起子进程、建 Git 仓库或要等超过 1 秒，就在开头写 `if testing.Short() { t.Skip("…") }`，并保证同一处逻辑在 `-short` 下还有一个快的测试覆盖。测试里默认不 fsync，只有专门检查落盘顺序的测试打开。

## 多语言

- 新增用户可见文字时，先在 internal/i18n/messages.go 增加稳定的点分消息 ID 和英文默认文案，再在 messages/active.zh-CN.json 增加对应翻译；不要把用户文字散落回 CLI 或托盘实现。
- 英文源文案由 Go 中的 go-i18n Message 定义维护，active.en.json 是可提交的提取结果；新增或修改消息后运行 goi18n extract 并检查两种语言的键和占位符。
- 新增语言时复制 active.zh-CN.json 的键集合，使用规范 BCP 47 标签，运行语言目录一致性测试，并在语言解析测试中覆盖显式设置、环境变量、区域变量和回退路径。
- OBIT_LANG、配置项 ui.language、LC_ALL、LC_MESSAGES、LANG 和操作系统界面语言按既定优先级解析；zh-TW、zh-HK、zh-MO 等繁体标签不能落到简体中文。
- 永远不翻译守卫和 hook 的拒绝提示、等价命令、宿主协议输出、MCP 工具名与描述、JSON 字段、退出码、路径、版本前缀、日志标记、注册表键和 Wails DTO 字段；hook 路径不加载语言目录。

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

## README 多语言维护

- 英文 `README.md` 是唯一源文件；每次修改必须在同一次提交中同步 `README.zh-CN.md`、`README.zh-TW.md`、`README.ja.md`、`README.ko.md`、`README.es.md`、`README.fr.md`、`README.de.md`、`README.pt-BR.md`、`README.ru.md`。
- 10 份 README 的第一行语言栏必须相同，每种语言都链接到根目录对应文件。顺序及国旗对应为：English（美国 `us`）、简体中文（中国 `cn`）、繁體中文（中国 `cn`）、日本語（日本 `jp`）、한국어（韩国 `kr`）、Español（西班牙 `es`）、Français（法国 `fr`）、Deutsch（德国 `de`）、Português (Brasil)（巴西 `br`）、Русский（俄罗斯 `ru`）。每个语言名之前用 `<img>` 引用 `docs/assets/flags/` 下的 SVG，高 14 px，垂直居中，`alt` 留空；不用国旗 emoji。
- 图片变体位于 `docs/assets/readme/i18n/<语言代码>/`，文案在 `strings.json`，排版引擎是 `layout.js`。生成和检查脚本要在无头浏览器里量字宽，依赖固定版本的 Playwright，装在 `docs/assets/readme/i18n/`（自己的 `package.json` 和 `pnpm-lock.yaml`，不依赖全局装的 Playwright）。第一次：`cd docs/assets/readme/i18n && pnpm install --frozen-lockfile && pnpm browser`（后一步是 `playwright install chromium`）。之后在同一目录：`pnpm generate` 重新生成 40 张图，`pnpm check` 检查版面，`pnpm test` 确认检查脚本仍能抓到故意弄坏的图。改文案或排版必须重新生成并提交 SVG；SVG 是用本机字体量的，CI 上的字体不同，所以 CI 只检查、不重新生成。
- 图片生成结果要与 README 语言同步；简体中文和繁體中文使用中文 demo GIF，其余语言使用英文 demo GIF。软件本身完成多语言后再重做 GIF 和右键菜单截图。
- 图的文字宽度一律在浏览器里用真实字体量出来（同时按更宽的 Verdana 回退字体留余量），不靠估算字宽；不使用横向压缩，说明文字在 880 px 宽度下不小于 14 px。`pnpm check`（即 `scripts/check-readme-diagrams.mjs`）取每段文字和每个卡片、命令框、箭头的真实边界框，查文字重叠、越出卡片或命令框、命令框不在自己的卡片里、间距与内边距、碰到箭头、对比度、孤字、行首标点和折行后与 `strings.json` 不一致，并用回退字体再查一遍。新增或放宽检查规则时，在 `scripts/check-readme-diagrams.test.mjs` 里加一条对应的故意弄坏的图，并把规则临时关掉确认测试会失败。检查通过后仍要用无头浏览器逐张看图，脚本不替代眼睛。`.github/workflows/readme-diagrams.yml` 在 README、`docs/assets/readme/` 或检查脚本改动时安装 Chromium 并跑 `check-readme-i18n.py`、`pnpm check`、`pnpm test`，必须通过。README 里演示动图在前、生命周期图在后（10 个语言版本一致）。
- 维护术语表：

| English | zh-CN | zh-TW | ja | ko | es | fr | de | pt-BR | ru |
|---|---|---|---|---|---|---|---|---|---|
| bury | 下葬 | 下葬 | 埋葬 | 매장 | enterrar | enterrer | begraben | enterrar | похоронить |
| graveyard | 墓地 | 墓地 | 墓地 | 묘지 | cementerio | cimetière | Friedhof | cemitério | кладбище |
| resurrect | 复活 | 復活 | 復活 | 부활 | resucitar | ressusciter | wiederbeleben | ressuscitar | воскресить |
| lifespan | 寿命 | 壽命 | 寿命 | 수명 | vida útil | durée de vie | Lebensdauer | vida útil | срок жизни |
| expire | 到期 | 到期 | 期限切れ | 만료 | caducar | expirer | ablaufen | expirar | истекать |
| register | 登记 | 登記 | 登録 | 등록 | registrar | enregistrer | registrieren | registrar | зарегистрировать |

`purge` 在非中文译文中表示永久删除；中文 README 保留现有的“火化”隐喻。
- 本地检查运行 `python scripts/check-readme-i18n.py`；它检查相对链接、图片路径、页内锚点、标题数量、代码块和 README 结构。图的版面由 `node scripts/check-readme-diagrams.mjs` 检查。每次更新还要同步官网、`docs/` 文档和这些译文。
