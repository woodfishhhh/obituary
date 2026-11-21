# 仓库结构

| 路径 | 职责 |
| --- | --- |
| `cmd/` | 组装 CLI、后台进程和小窗口。命令实现在 `internal/cli/`，入口只注册命令。 |
| `internal/` | `app` 打开服务，`core` 管生命周期，`gui` 提供桌面绑定。用户目录统一经 `userdirs` 解析。 |
| `site/` | React 首页在 `landing`，VitePress 用户和开发文档在 `docs`，组装脚本在 `scripts`。 |
| `docs/` | 架构契约、使用手册、agent 覆盖表、文档图片。索引在 `docs/README.md`。 |
| `scripts/`、`packaging/` | 构建与验收脚本、安装器、桌面包材料。 |
| `.github/workflows/`、`release/` | CI 和部署的执行定义，以及各版本的公开说明。 |

测试跟着实现：`bury.go` 的行为放在 `bury_test.go`，跨模块的真实协议或子进程测试放在 `internal/e2e`。新测试包沿用相邻包的 `TestMain`，让 `internal/testenv` 隔离账本、用户目录和平台集成状态。
