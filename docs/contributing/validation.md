# 构建与测试

## 隔离测试环境

- `internal/testenv`：每个测试包的 `TestMain` 都调用 `testenv.Main(m)`，把上面这些目录和临时目录指到一次性目录，清掉继承来的 `GIT_*`，换上专用 gitconfig；用 `OBIT_TEST_REGISTRY_ROOT` 把本进程和它启动的 obit/obitd 子进程的右键菜单、开机启动项写到一次性的 `HKCU\Software\obituary-test-*` 下（结束时删掉），并在前后只读比较真实的讣告右键菜单、Run 项和开始菜单/桌面快捷方式，有变化就让测试失败；还会在测试前后比较 TEMP 所在卷根 `.obituary\graveyard` 下的顶层条目（账本键目录，以及 `<键>.owner`、`.owner.lock` 这类文件），多出新条目就让测试失败（说明东西漏到了真实卷根；不递归进其他账本）。测试里下葬时，对象和 Home 要用同一种卷根写法，否则坟墓会建到真实卷根的 `.obituary` 下。新建测试包时照抄一个 `main_test.go`。编译一次性源码（`-overlay` 变异体和夹具）的 go 命令要带上 `testenv.MutantGoEnv(t)` 给的环境（`gocache.go`）：GOCACHEPROG 指回测试二进制，只读命中用户的 GOCACHE，新条目写进随测试删除的目录。否则每个变异体连同它上面的包都会在用户 GOCACHE 里留下再也命中不了的条目（一轮 MCP 变异约 6 GB）。`TestOverlayBuildsUseMutantGoEnv` 扫描源码强制这一点。
- `internal/e2e`：跨模块的端到端测试（外部包）：真实 shell 执行、hook 与 MCP 的协议抓取、安装、后台进程。
- 测试只测可观察的行为。`core` 的测试用真实文件 ID 加上不进回收站的 `testFS`，不碰用户的回收站。真实回收站测试要设 `OBIT_TEST_RECYCLE=1`，跑完从回收站清掉 `obit-*` 条目。
- 手动验收时 `OBIT_HOME` 和 `OBIT_CONFIG_DIR` 都要指向临时目录，不要污染 `%LOCALAPPDATA%\obituary` 和 `%APPDATA%\obituary`；结束后执行 `obit daemon stop`。

## 构建 CLI 与后台

```bash
golangci-lint run ./...   # 只报相对 main 新增的问题；装法：go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
go build -o bin/obit.exe ./cmd/obit
go build -ldflags=-H=windowsgui -o bin/obitd.exe ./cmd/obitd
```

`bin/` 已忽略。只格式化改过的文件（`gofmt -w <files>`）。

## 静态检查

**静态检查（所有者 2026-11-10 定）。** `.golangci.yml` 只开 staticcheck、errorlint、funlen（50 行）、nestif（复杂度 4），对应根目录 AGENTS.md「函数不超过 50 行、嵌套不超过 4 层」的规则。只拦新增的问题（`new-from-merge-base: main`）；已有的超长函数和深嵌套是重构清单，记在 私有留档库的 `docs/milestones/m3-speed.md`。交付前和 vet 一起跑，新代码不许新增告警；确需例外用 `//nolint:<linter> // 原因` 写明理由。`unused` 不在 CI 里：Linux 构建下，只被 Windows 代码调用的共用函数会被误报。拆包或删掉调用方之后，手动跑一次 `golangci-lint run --new-from-merge-base= --enable-only unused ./...` 查死代码，Windows 构建下应当是 0。

## 本机和 CI

**测试分两档，全量只在 CI 上跑（所有者 2026-11-10 定，决策 D-0038）。** 测试大多碰真实文件系统、Git 和子进程。几个 worker 在同一台机器上同时跑全量时，全部慢二三十倍，所以：

- 本机（worker 和协调员都一样）：只跑改动涉及的包，不加 `-count=1`（让 `go test` 的缓存跳过没变的包），例如 `go test ./internal/core -run TestBury`；想快速摸一遍全仓库用 `go test -short ./...`。变异只跑相关的几项，子测试按名字过滤：`OBIT_TEST_MCP_MUTATIONS=1 go test -count=1 -run 'TestMCPMutations/purge-' ./internal/cli/mcp`，删除护栏同理（`TestDeleteGuardMutations/<名字>`）。
- 交付前：`gofmt -l`、`go vet ./...`、`golangci-lint run ./...`，加上改动涉及的包和相关变异。本机不跑全量和两组全量变异。
- 合并把关分三层：worker 交付前把改动的包用完整模式跑一遍、上层包跑 -short；协调员合并后直接推送，看 CI 的全量结果，不必先在本机跑整树全量；发版前、账本格式升级后、大规模机械改动后手动触发一次带变异的 CI（`gh workflow run ci.yml -f full=true -f mutations=true`；变异测试在 CI 上按编号拆到 8 台机器并行，本地可以用 `OBIT_MUTATION_SHARD=k/n` 只跑其中一份）。本机确实要跑整树全量时，同一时间只跑一处。
- 全量和变异在 CI 上跑。仓库公开，标准机器不计分钟：PR 和满足路径条件的 main 推送跑全量（按包拆成四个并行任务，列出最慢的 30 个测试）和 Linux（带 `-race`）、macOS 上的 POSIX 测试，大约 10 分钟出结果；变异每周跑一次，也可以手动触发。main 推送按打包相关路径决定是否跑打包和安装测试，PR 和手动触发时也跑（最慢的一项，为省时间）。推送不用攒批，推完看一眼 CI。
- 新写的测试：只要会起子进程、建 Git 仓库或要等超过 1 秒，就在开头写 `if testing.Short() { t.Skip("…") }`，并保证同一处逻辑在 `-short` 下还有一个快的测试覆盖。测试里默认不 fsync，只有专门检查落盘顺序的测试打开。

工作分支或纯文档改动需要 CI 回执时，按分支手动运行 `ci.yml`。README 链接、锚点和配图检查可手动运行 `readme-diagrams.yml`。

## Windows 与 WSL

- 在 Git Bash 里直接敲 `schtasks /Delete /TN …` 这类命令时，`/Delete` 会被 MSYS 当成路径转换掉。手动操作用 PowerShell（`Get-ScheduledTask`、`Unregister-ScheduledTask`，它们也能看到隐藏的任务）；Go 代码里调用 `schtasks` 不受影响。
- `schtasks /Query /XML` 按控制台代码页（GBK）输出，任务 XML 里只放 ASCII。
- 真实的任务计划测试要设 `OBIT_TEST_SCHEDULER=1`。测试任务的名字是唯一的，清理时按这个名字删，不要依赖全局变量（defer 会比 t.Cleanup 先执行）。

- 从 WSL 启动 Windows 版 `obit.exe` 做隔离测试时，把 `OBIT_HOME` 和 `OBIT_CONFIG_DIR` 都导出为 WSL 临时目录路径，并设置 `WSLENV=OBIT_HOME/p:OBIT_CONFIG_DIR/p`。`/p` 会把两个路径转换为 Windows 路径。若已有 `WSLENV` 条目，请保留并用冒号分隔。
- 别在 heredoc 里用 Python 修改含 `\n` 的 Go 字符串字面量：`\n` 会变成真的换行，导致编译失败。改这类地方用 Edit 工具。

- 维护者的 Git Bash 会把 `rm` 送进回收站，排查额外回收站条目时先核对 shell 命令。可恢复删除用 `obit bury <确切路径>`。仅清理自己的一次性临时文件时，原生命令是 `/usr/bin/rm`。WSL 脚本与后续清理分开执行，核实确切路径并遵守宿主删除护栏。

- `CoInitializeEx` 返回 `S_FALSE`（已经初始化过）时，x/sys 会把它当成错误，要单独放行。
- `GetFinalPathNameByHandle` 返回的路径带 `\\?\` 前缀并且是长文件名。和 `t.TempDir()` 返回的短名比较之前，先用 `filepath.EvalSymlinks` 统一。去掉前缀之后，要再验证这个路径还能找回同一个对象（文件名以点结尾的情况）。
- `filepath.WalkDir` 里对非目录条目返回 `SkipDir`，会跳过它后面的所有同级条目，找 `.git` 时会漏掉，所以不要这样用。
- 这台机器上的 `python3` 是 Windows 应用商店的占位程序，要用 `python`。
- 不要把测试的 TEMP 指向 Python `tempfile.mkdtemp` 建的目录（Python 3.13 起它在 Windows 上带受保护的 ACL）：`TestOwnerReportsDeniedProjectDiscovery` 在那里恢复 `.git` 的 ACL 会失败，留下只有管理员能删的目录。清理办法：提权后对确切的目录执行 `takeown /f <目录> /r /d y` 和 `icacls <目录> /reset /t /c /q`，再用普通权限删掉。
