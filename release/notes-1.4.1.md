# 讣告 1.4.1

修复后台刚启动时停止命令偶尔误报的问题。

- **修复 `obit daemon stop` 的启动竞态**：后台刚启动时执行停止命令，偶尔会误报无法核实进程已退出。「立即更新」先停后台的那一步也受这个问题影响。

升级：Windows 可以在「设置 → 关于与更新」点「立即更新」，也可以用 `scoop update obituary` 或再运行一次安装脚本（国内加 `-Mirror cn`）；macOS 用 `brew upgrade --cask obituary`；macOS / Linux 也可以再运行一次 `bash install.sh`（国内加 `--mirror cn`）。

---

# Obituary 1.4.1

Fixes a false error when stopping the background process just after startup.

- **Fixed a startup race in `obit daemon stop`.** Stopping the daemon just after it started could wrongly report that its exit could not be verified. This also affected the first step of Update now, which stops the daemon.

On Windows, choose Update now in Settings → About & Updates, run `scoop update obituary`, or run the install script again (`-Mirror cn` in mainland China). On macOS, `brew upgrade --cask obituary`; on macOS and Linux, running `bash install.sh` again works too (`--mirror cn` in mainland China).
