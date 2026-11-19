# Changelog / 更新日志

## Unreleased

## 1.4.1 - 2026-11-19

- 修复：后台进程刚启动时执行 `obit daemon stop`（包括「立即更新」先停后台的那一步），偶尔会误报无法核实进程已退出。
- Fix: stopping the background process right after it started (including the first step of Update now) could wrongly report that its exit could not be verified.

## 1.4.0 - 2026-11-19

- 「关于与更新」支持一键安装正式新版；终端可用 `obit update --apply`，升级结果和日志路径会保留。（#15）
- Install a newer stable release in place from About & Updates, or with `obit update --apply`. The result and its log path are kept. (#15)
