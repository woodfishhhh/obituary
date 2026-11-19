# Changelog / 更新日志

## Unreleased

- 修复：Windows 上的 Codex 命令原文里，反引号或 `$(…)` 片段包含删除时，删除护栏会拦下这一次执行，并说明命令可能再经过一层 shell。
- Fix: the delete guard now denies Windows Codex commands whose raw backtick or `$(…)` fragments contain deletion, because Codex may pass the command through another shell.

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
