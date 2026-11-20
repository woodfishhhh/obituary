# Changelog / 更新日志

## 1.5.0 - 2026-11-21

- 命令行、Windows 托盘、小窗口和资源管理器菜单共用同一个语言设置，可选择英文或简体中文。
- The CLI, Windows tray, desktop window and File Explorer menu now share one language setting for English or Simplified Chinese.
- Windows 小窗口以 GB 或 TB 显示大小时保留两位小数；修复了日历弹窗顶部线条在圆角处突出的显示问题。
- The Windows desktop window shows two decimal places for sizes in GB or TB. The top line of the calendar dialog now follows its rounded corners.
- 新增匿名使用统计，默认开启，可用 `obit config telemetry off` 关闭；详情见[隐私政策](https://obituary.woodfish.site/docs/privacy.html)。
- Anonymous usage statistics are on by default. To turn them off, run `obit config telemetry off`. See the [privacy policy](https://obituary.woodfish.site/docs/privacy.html).

## 1.4.3 - 2026-11-20

- 修复：Windows 上的 Codex 命令里，反引号或 `$(…)` 片段即使在引号或带引号的 heredoc 里，只要包含删除，删除护栏也会拦下：Codex 会把命令再交给另一层 shell，引号可能失效。
- Fix: on Windows, the delete guard now denies a Codex command with a deletion inside backticks or `$(…)`, even within quotes or a quoted heredoc, because Codex passes the command through another shell where quoting may fail.

## 1.4.2 - 2026-11-19

- 修复：删除护栏现在会拦下 Bash 的 `find -delete`、`find -exec rm` 和 `xargs rm`，并提示先列出目标，再用 `obit rm` 下葬。
- Fix: the delete guard now denies Bash `find -delete`, `find -exec rm`, and `xargs rm`, and explains how to list the targets first and bury them with `obit rm`.
- 国内镜像：新版本发布后由国内服务器多线程从 GitHub 拉取，几分钟内就能从国内源下载。
- Mainland China mirror: after a release is published, the mirror server pulls it from GitHub over parallel connections, so it is available from the China source within minutes.

## 1.4.1 - 2026-11-19

- 修复：后台进程刚启动时执行 `obit daemon stop`（包括「立即更新」先停后台的那一步），偶尔会误报无法核实进程已退出。
- Fix: stopping the background process right after it started (including the first step of Update now) could wrongly report that its exit could not be verified.

## 1.4.0 - 2026-11-19

- 「关于与更新」支持一键安装正式新版；终端可用 `obit update --apply`，升级结果和日志路径会保留。（#15）
- Install a newer stable release in place from About & Updates, or with `obit update --apply`. The result and its log path are kept. (#15)
