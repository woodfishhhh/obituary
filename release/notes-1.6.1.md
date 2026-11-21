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
