# 开发文档

要改代码，先读仓库根目录的 [AGENTS.md](https://github.com/woodfishhhh/obituary/blob/main/AGENTS.md)，确认改动涉及哪些平台、该跑哪些测试。跨模块改动再读 [架构契约](https://github.com/woodfishhhh/obituary/blob/main/docs/architecture.md)。提 issue 或送审前看 [贡献指南](https://github.com/woodfishhhh/obituary/blob/main/CONTRIBUTING.md)。安装和日常操作请看[用户文档](../)。

| 想做什么 | 从这里开始 |
| --- | --- |
| 跟踪一个命令如何改变文件状态 | [架构](./architecture) · [安全模型](./safety) |
| 找到实现并在本地验证 | [仓库结构](./repository) · [三平台构建和测试](./build-test) |
| 修改桌面界面或适配一个宿主 | [小窗口](./gui) · [接入新 agent](./agent-integration) |
| 更新翻译、文档或发布产物 | [国际化](./i18n) · [文档与博客规范](./docs-and-blog) · [发版和镜像](./release) |

CLI 和后台进程面向 Windows、macOS 和 Linux。桌面包的范围更窄：Windows amd64、Ubuntu 24.04 amd64，以及 macOS 通用应用。资源管理器菜单仅限 Windows；系统回收站模式支持 Windows 和 Linux。
