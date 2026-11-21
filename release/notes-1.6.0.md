# 讣告 1.6.0

- **Linux 小窗口（预览版）**：Ubuntu 24.04（amd64）可用 `.deb` 安装。从应用列表或托盘打开小窗口，接收桌面通知，并可在设置中开启托盘随登录启动。
- **macOS 小窗口（预览版）**：通用 DMG 内的 `Obituary.app` 同时支持 Intel 和 Apple 芯片。可从菜单栏打开小窗口，也可通过 LaunchAgent 随登录启动。应用未经 Developer ID 签名或公证；请先核对 SHA-256，再按[第一次打开未签名 app 的步骤](https://obituary.woodfish.site/docs/desktop.html#第一次打开未签名-app)放行。
- **已知问题**：这一版的 macOS 小窗口（DMG 里的 `Obituary.app`）需要 macOS 26，在 macOS 15 及更早的系统上打不开，系统会提示与当前版本不兼容。1.6.1 修复。
- Windows 安装脚本现在直接校验 SHA-256 和解压安装包，即使 PowerShell 无法自动加载模块也能安装。

---

# Obituary 1.6.0

- **Linux desktop window (preview)**: install the `.deb` on Ubuntu 24.04 (amd64). Open the window from the application menu or tray, receive desktop notifications, and optionally start the tray at login.
- **macOS desktop window (preview)**: the universal DMG includes `Obituary.app` for both Intel and Apple silicon. Open the window from the menu bar and optionally start it at login via LaunchAgent. The app is not Developer ID signed or notarized; verify the SHA-256, then follow the [first-open instructions](https://obituary.woodfish.site/docs/en/desktop.html#first-opening-an-unsigned-app).
- **Known issue**: the macOS desktop window in this release (`Obituary.app` in the DMG) needs macOS 26 and does not open on macOS 15 or earlier; the system reports that it is incompatible with this version. Fixed in 1.6.1.
- The Windows installer now verifies SHA-256 and extracts the archive even when PowerShell cannot autoload modules.
