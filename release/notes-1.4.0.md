# 讣告 1.4.0

有正式新版时，可以直接在讣告里安装。

- **在「关于与更新」里一键升级**：Windows 小窗口的「设置 → 关于与更新」新增「立即更新」。终端也能用 `obit update --apply`。升级走原来的安装渠道，结果和日志路径会保留。（#15）

升级：Windows 用 `scoop update obituary`，或再运行一次安装脚本（默认装最新版，国内加 `-Mirror cn`）；macOS 用 `brew upgrade --cask obituary`；macOS / Linux 也可以再运行一次 `bash install.sh`（国内加 `--mirror cn`）。

---

# Obituary 1.4.0

When a new stable release is available, you can install it from Obituary.

- **Update from About & Updates.** Choose Update now in Settings → About & Updates in the Windows window, or run `obit update --apply` in a terminal. Obituary updates through the original install channel and keeps the result and its log path. (#15)

Upgrade on Windows with `scoop update obituary`, or run the install script again (it installs the latest release; add `-Mirror cn` in mainland China). On macOS, `brew upgrade --cask obituary`; on macOS and Linux, running `bash install.sh` again works too (`--mirror cn` in mainland China).
