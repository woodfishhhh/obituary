# 发版和镜像

从版本标签核对三件事：`CHANGELOG.md` 记录行为变化，`release/notes-版本.md` 说明用户拿到什么，`.goreleaser.yaml` 决定归档实际包含什么。不要从某一平台的包推断另一平台也附带 GUI。

`release.yml` 响应 `v` 开头的版本标签。GoReleaser 为 Windows、Linux、macOS 的 amd64 和 arm64 生成六个归档及 SHA-256 清单；Windows GUI 只随 amd64 归档。Linux 桌面 `.deb` 和 macOS 的通用 DMG 有独立的桌面工作流。包生成、上传 Release 和包管理器收录是不同状态：Scoop、WinGet 清单和 Homebrew cask 要各自核对版本和下载校验值。

网站与二进制分开部署。`pages.yml` 从同一份源码构建 `dist/` 和 `dist-cn/`；国内安装示例使用镜像脚本。`mirror-latest.yml` 校验正式 Release 的归档和哈希再发布镜像索引。传输校验不等于独立的发布者签名；发布验收要读回实际下载和索引，不能只看网站能否打开。
