# 讣告 Obituary

讣告为文件和文件夹设定寿命。命令行 `obit` 和后台 `obitd` 使用 Go 1.26，支持 Windows、macOS、Linux。用法见 [README](README.md)。跨模块改动先读[架构与 core.FS 契约](docs/architecture.md)。

## 数据安全

- `core` 经 `core.FS` 访问文件系统。下葬只用 `FS.MovePinned`，在同一句柄核对身份并改名。复活只用 `FS.MovePinnedInto`，目标父目录也必须固定，且不能覆盖。移进回收站只用 `FS.Recycle`。
- 复活和火化前必须通过 `verifyGrave`。用户内容只有 `Purge` 可永久删除，且只限 `graveDir` 认可的坟墓。禁止直接对用户内容调用 `os.Remove` / `RemoveAll`。
- 仅可清理能证明归属的清单和临时文件、下葬中止或复活提交后核实身份且为空的坟墓，以及本次新建且仍为空的目录（复活拒绝按创建句柄撤销，`mktemp` 登记失败时撤销）。
- 回收失败保留原文件并重试，禁止退化成永久删除。只有确定的 `ErrNotFound` 才放弃记录，`ErrUnsafe` 拒绝，其余错误重试。
- 用户目录统一由 `internal/userdirs` 解析，新增豁免须说明理由。自动登记先核对 Git，查不出按已跟踪处理。项目规则先信任再加载，依赖、构建输出、缓存不自动登记。
- 使用统计只计真实使用。保持跨进程限额、停报清理、字段白名单和服务端隐私约束，改动前读[使用统计](docs/contributing/subsystems.md#使用统计)。

## 代码和文档

- `cmd/` 只组装。`cmd/obit/main.go` 创建根命令、注册子命令并映射退出码。命令实现用 `internal/cli/<命令>/NewCmd`，共用逻辑放 `cmdutil`。
- 按职责拆文件，约 500 行或承载多种职责时拆分。函数不超过 50 行，嵌套不超过 4 层。重组或替换库保持行为、全量测试数及结果一致，原有变异检查仍能抓到错误。
- 改 hook、init、配置、统计、安装、MCP、Skill、测试和性能前，先调研成熟工具。写底层代码先查标准库，再查维护活跃、许可兼容（MIT / BSD / Apache-2.0）的库。必须手写时在代码注释及提交说明中交代缺失能力或安全要求。
- README、架构文档及公开注释写当前规则和理由。试错、修复经过写进提交说明，公开文字不写借鉴来源。路线、任务、里程碑规划及决策留在维护者的私库（本机 `docs-internal/`）。GitHub 保留里程碑和少量 issue，一次最多新开一个。完成已有 issue 时在提交说明写 `Closes #<编号>`。

## 测试与交付

- 改 Guard、`locate`、`settle`、`commit` 或回收逻辑，先写能证明不会误删的失败测试。修复后临时还原成能编译的错误版本，确认测试抓得到。自动登记用真实临时 Git 仓库。
- 每个测试包在 `TestMain` 调 `testenv.Main(m)`。只测可观察行为，按功能与源文件一一配对。例外是共用的 `helpers_test.go`、`main_test.go`、`export_test.go`，以及 `internal/e2e/` 的跨模块外部测试。遇导入循环用 `<源文件>_external_test.go`。
- 手动验收把 `OBIT_HOME`、`OBIT_CONFIG_DIR` 都指到临时目录，结束执行 `obit daemon stop`。常规测试不碰用户回收站、真实注册表或宿主桌面。临时产物和证据用 `obit mktemp 2d` 或 `obit mark <路径> 2d`。创建失败才退到 `%TEMP%/obit-<任务名>-<随机>` 并报告。含 `.git` 的目录不能登记，用后单独清理。
- Go 改动只格式化改过的文件。交付前运行 `gofmt -l`、`go vet ./...`、`golangci-lint run ./...`，完整测涉及包，上层跑 `-short`，只跑相关变异。本机复用测试缓存，全量留给 CI。拆包或删除调用方后在 Windows 加查 `unused`。推送后核对相关 CI。
- 发版前、账本格式升级后或大规模机械改动后，手动跑全量及变异 CI。平台发布需验同一次构建的产物。Linux 桌面用私有显示环境，macOS GUI 只在 macOS runner 构建验收。真机授权及登录另行验证。

## 按任务先读

| 做什么 | 先读哪里 |
|---|---|
| 核心、路径、账本、CLI/list、项目规则、自动登记 | [架构](docs/architecture.md) · [子系统维护](docs/contributing/subsystems.md) |
| 常驻和托盘、hook、init、删除护栏、MCP、原地升级、doctor | [常驻、hook 和接入](docs/contributing/subsystems.md#常驻hook-和接入) · [原地升级与诊断](docs/contributing/subsystems.md#原地升级与诊断) |
| 改测试、跑变异、静态检查、Windows 或 WSL 验收 | [构建与测试](docs/contributing/validation.md) |
| 平台构建、候选包验收、国内镜像发布与回滚 | [平台构建与发布验收](docs/contributing/release.md) |
| GUI、官网、文档站的构建、圆角、首屏性能和动画 | [官网与小窗口](docs/contributing/website.md) |
| 新增界面文字、改 README、翻译、配图和术语 | [多语言维护](docs/contributing/localization.md) |
