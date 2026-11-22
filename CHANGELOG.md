# Changelog / 更新日志

## Unreleased / 未发布

- 新增：Docker 本地卷可以用 `obit mark --docker-volume` 设寿命，也可以用 `obit bury --docker-volume` 立即下葬；讣告先归档并校验，再删除未被使用的卷，复活时不覆盖同名卷。
- New: give a local Docker volume a lifespan with `obit mark --docker-volume` or bury it now with `obit bury --docker-volume`; Obituary archives and verifies an unused volume before removing it and never overwrites a volume of the same name on restore.
- 改进：桌面版托盘异常退出后由系统自动重启，15 分钟内第三次异常退出就不再重试，并在 `obit doctor` 里报告；启用监督以后，独立的兜底任务改为每小时运行一次，没有托盘的版本仍每 5 分钟检查一次。
- Improvement: the system now restarts the desktop tray after a crash and stops after the third crash within 15 minutes, which `obit doctor` reports; with supervision enabled, the independent fallback runs hourly, while editions without a tray still check every five minutes.
- 修复：从旧版升级、界面语言和右键菜单标签的语言不一样时，`obit doctor` 不再把右键菜单报成「不完整或被改过」，而是说明菜单能正常用，并给出换成当前语言的命令。
- Fix: when the Explorer context menu's labels are in a different language from the interface (after upgrading from an older version, for example), `obit doctor` no longer reports the menu as incomplete or modified. It says the menu works and shows the command that switches its labels to the current language.
- 改进：删除护栏只在真要拒绝一条删除命令时才去查找 `obit` 的路径，`git status` 这类普通工具调用不再查 PATH，hook 在进程内的耗时从毫秒级降到二十微秒左右。
- Improvement: the delete guard looks up the path to `obit` only when it is about to deny a deletion, so ordinary tool calls such as `git status` no longer search PATH, and the hook's in-process time for them drops from milliseconds to about 20 microseconds.
- 改进：hook 只加载自己的命令、用到时才解析词库，`obit list` 和摘要只读展示所需的数据；在一台 Windows 机器上，普通 PreToolUse 中位数约从 53 降到 48 毫秒，1 万条活动记录加 1 万条墓地记录时 `obit list` 中位数约从 372 降到 243 毫秒，95 分位约从 428 降到 265 毫秒。
- Improvement: hooks load only their own commands and parse messages on first use, while `obit list` and its summary read only what they display; on one Windows machine, median ordinary PreToolUse time fell from about 53 to 48 ms, and with 10,000 active entries and 10,000 graves, median `obit list` time fell from about 372 to 243 ms and its 95th percentile from about 428 to 265 ms.

## 1.6.2 - 2026-11-22

- 中文界面下，命令行给人看的输出换成了中文：表头、提示、确认、报错说明和后台进程的提示都译好了，中文表格按显示宽度对齐。给 agent 看的内容和以前完全一样：hook 的回执、`--json`、MCP 和 Skill。英文界面里原来夹着中文的几处，比如 `obit init` 的安装状态、资源管理器右键菜单的结果，现在都是英文。
- In the Chinese interface, what the command line prints for people is now in Chinese: table headers, prompts, confirmations, error explanations and background-process notices are translated, and Chinese tables line up by display width. Output meant for agents is exactly as before: hook replies, `--json`, MCP and the skill. A few places that printed Chinese in the English interface, such as `obit init`'s installation status and the results of Explorer context menu actions, now print English.
- 修复：Windows 本地盘上名叫 `NUL`、`con.txt`、`COM1` 等的文件可以正常登记、下葬、复活和火化；`obit rm ./NUL` 只处理当前目录的真实文件，不访问同名设备。
- Fix: files named `NUL`, `con.txt`, `COM1`, and other DOS-reserved names on local Windows drives can be marked, buried, restored, and purged. `obit rm ./NUL` addresses the file in the current directory, never the device.

## 1.6.1 - 2026-11-21

# 讣告 1.6.1

- **手动安装也能一键升级**：直接解压使用的讣告，现在也能在小窗口设置页里点"下载并重启更新"。升级只替换讣告自己的程序，同一目录里的其他文件不受影响；下载的安装包先核对 SHA-256，任何一步失败都会把原来的程序换回去，万一换回也失败，旧程序会原样留着，并告诉你它们在哪里。命令行用 `obit update --apply` 走同一套流程。更新说明只显示当前界面语言的那一段。
- **macOS 小窗口在 macOS 15 上也能打开了**：1.6.0 的 DMG 实际要求 macOS 26，在 macOS 15 及更早的系统上打不开。这一版把最低系统版本改成 macOS 12，并在 macOS 15 和 26 的 Apple 芯片、Intel 机器上都验证过。第一次打开 app，菜单栏图标会一起出现；通知改由单独的菜单栏辅助程序发出，系统会正常向你请求通知权限。app 仍未经 Developer ID 签名或公证，第一次打开请照[文档](https://obituary.woodfish.site/docs/desktop.html#第一次打开未签名-app)放行。
- **托盘菜单刷新时不会再误触发复活**：最近下葬的列表刷新、移除旧菜单行的时候，旧行可能被当成一次点击，复活一个你没有选的条目。Windows、macOS 和 Linux 的托盘都修好了。
- **卸载更稳妥**：`obit uninstall --purge-data` 现在先确认能完整清除才动手，不会再删空墓地以后又因为配置文件停下（macOS 的默认目录布局会碰到这个问题）；`obit init --uninstall` 也会照 `--no-context-menu`、`--no-shortcuts`、`--no-skill` 保留对应的部分。
- **日历每天一列，宽度相同**：卡片多的日子不会再把其他日子挤得看不清，窗口太窄时日历改为横向滚动。
- 小窗口的圆角换成了连续曲率的平滑圆角。

---

# Obituary 1.6.1

- **One-click updates for manual installs**: if you unpacked Obituary by hand, the window's settings page now offers "Download and restart to update" as well. The update replaces only Obituary's own programs and leaves every other file in that folder alone; the download is checked against its SHA-256 first, and if any step fails the previous programs are put back. If putting them back also fails, the old programs are kept as they were and you are told where they are. On the command line, `obit update --apply` follows the same steps. Release notes now show only the section in the window's language.
- **The macOS window opens on macOS 15 again**: the 1.6.0 DMG actually required macOS 26 and would not open on macOS 15 or earlier. This release sets the minimum system to macOS 12 and was checked on macOS 15 and 26, on both Apple silicon and Intel. The menu bar icon now appears the first time you open the app, and notifications come from a separate menu bar helper, so the system asks for notification permission as it should. The app is still not Developer ID signed or notarized; follow the [first-open instructions](https://obituary.woodfish.site/docs/en/desktop.html#first-opening-an-unsigned-app) the first time you open it.
- **Refreshing the tray menu no longer triggers a resurrect by itself**: when the recently buried list refreshed and old menu rows were removed, a removed row could be taken as a click and resurrect an item you had not chosen. Fixed for the tray on Windows, macOS and Linux.
- **Safer uninstall**: `obit uninstall --purge-data` now checks that it can finish before it deletes anything, so it no longer empties the graveyard and then stops over the settings file (the default macOS folder layout ran into this). `obit init --uninstall` also keeps whatever `--no-context-menu`, `--no-shortcuts` or `--no-skill` asks it to keep.
- **One column per day, all the same width**: a busy day no longer squeezes the other days until they cannot be read; when the window is too narrow, the calendar scrolls sideways.
- The window's rounded corners now use continuous-curvature (squircle) corners.

## 1.6.0 - 2026-11-21

- Linux 小窗口预览版：Ubuntu 24.04（amd64）用户可安装 `.deb`，从应用列表或托盘打开小窗口、接收桌面通知，并选择登录时启动托盘。
- Linux desktop preview: install the `.deb` on Ubuntu 24.04 (amd64) to open the window from the application menu or tray, receive desktop notifications, and optionally start the tray at login.
- macOS 小窗口预览版：通用 DMG 内的 `Obituary.app` 支持 Intel 和 Apple 芯片，提供菜单栏图标，可通过 LaunchAgent 随登录启动。应用未经 Developer ID 签名或公证，见[首次打开说明](https://obituary.woodfish.site/docs/desktop.html#第一次打开未签名-app)。
- macOS desktop preview: the universal DMG includes `Obituary.app` for Intel and Apple silicon, with a menu bar icon and optional login startup via LaunchAgent. The app is not Developer ID signed or notarized; see the [first-open instructions](https://obituary.woodfish.site/docs/en/desktop.html#first-opening-an-unsigned-app).
- Windows 安装脚本不再依赖 PowerShell 模块自动加载来校验 SHA-256 和解压安装包。
- The Windows installer can now verify SHA-256 and extract the archive without PowerShell module autoloading.

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
