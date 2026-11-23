# 讣告 1.7.0

- 新增：Docker 本地卷可以用 `obit mark --docker-volume` 设寿命，也可以用 `obit bury --docker-volume` 立即下葬；讣告先归档并校验，再删除未被使用的卷，复活时不覆盖同名卷。
- 改进：桌面版托盘异常退出后由系统自动重启，15 分钟内第三次异常退出就不再重试，并在 `obit doctor` 里报告；启用监督以后，独立的兜底任务改为每小时运行一次，没有托盘的版本仍每 5 分钟检查一次。
- 改进：hook 只加载自己的命令、用到时才解析词库，`obit list` 和摘要只读展示所需的数据；在一台 Windows 机器上，普通 PreToolUse 中位数约从 53 降到 48 毫秒，1 万条活动记录加 1 万条墓地记录时 `obit list` 中位数约从 372 降到 243 毫秒，95 分位约从 428 降到 265 毫秒。
- 改进：删除护栏只在真要拒绝一条删除命令时才去查找 `obit` 的路径，`git status` 这类普通工具调用不再查 PATH，hook 在进程内的耗时从毫秒级降到二十微秒左右。
- 修复：从旧版升级、界面语言和右键菜单标签的语言不一样时，`obit doctor` 不再把右键菜单报成「不完整或被改过」，而是说明菜单能正常用，并给出换成当前语言的命令。

---

# Obituary 1.7.0

- New: give a local Docker volume a lifespan with `obit mark --docker-volume` or bury it now with `obit bury --docker-volume`; Obituary archives and verifies an unused volume before removing it and never overwrites a volume of the same name on restore.
- Improvement: the system now restarts the desktop tray after a crash and stops after the third crash within 15 minutes, which `obit doctor` reports; with supervision enabled, the independent fallback runs hourly, while editions without a tray still check every five minutes.
- Improvement: hooks load only their own commands and parse messages on first use, while `obit list` and its summary read only what they display; on one Windows machine, median ordinary PreToolUse time fell from about 53 to 48 ms, and with 10,000 active entries and 10,000 graves, median `obit list` time fell from about 372 to 243 ms and its 95th percentile from about 428 to 265 ms.
- Improvement: the delete guard looks up the path to `obit` only when it is about to deny a deletion, so ordinary tool calls such as `git status` no longer search PATH, and the hook's in-process time for them drops from milliseconds to about 20 microseconds.
- Fix: when the Explorer context menu's labels are in a different language from the interface (after upgrading from an older version, for example), `obit doctor` no longer reports the menu as incomplete or modified. It says the menu works and shows the command that switches its labels to the current language.
