# 发版和镜像

从版本标签核对三件事：`CHANGELOG.md` 记录行为变化，`release/notes-版本.md` 说明用户拿到什么，`.goreleaser.yaml` 决定归档实际包含什么。不要从某一平台的包推断另一平台也附带 GUI。

`release.yml` 响应 `v` 开头的版本标签。GoReleaser 为 Windows、Linux、macOS 的 amd64 和 arm64 生成六个归档及 SHA-256 清单；Windows GUI 只随 amd64 归档。Linux 桌面 `.deb` 和 macOS 的通用 DMG 有独立的桌面工作流。包生成、上传 Release 和包管理器收录是不同状态：Scoop、WinGet 清单和 Homebrew cask 要各自核对版本和下载校验值。

Scoop bucket 和 Homebrew tap 都指向已发布的新版本后，在 Actions 手动运行 **Scoop and Homebrew CLI acceptance**（`channel-acceptance.yml`），`version` 填 `1.6.2` 这样的版本号，不带 `v`。从各自渠道实装，核对索引和程序版本，登记、下葬并复活样本，再验收卸载。Scoop 分别验先运行 `obit uninstall --yes` 和直接卸载；Homebrew 验 CLI cask 按文档卸载及 LaunchAgent 清理，不验桌面 DMG。三项都必须确认程序移除、隔离账本与非空墓地原样保留；把日志产物链接记进发版收据。

网站与二进制分开部署。`pages.yml` 从同一份源码构建 `dist/` 和 `dist-cn/`；国内安装示例使用镜像脚本。`mirror-latest.yml` 校验正式 Release 的归档和哈希再发布镜像索引。传输校验不等于独立的发布者签名；发布验收要读回实际下载和索引，不能只看网站能否打开。
