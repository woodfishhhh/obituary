# 讣告 1.6.2

- **中文界面下，命令行给人看的输出换成了中文**：表头、提示、确认、报错说明和后台进程的提示都译好了，中文表格按显示宽度对齐。给 agent 看的内容和以前完全一样：hook 的回执、`--json`、MCP 和 Skill。英文界面里原来夹着中文的几处，比如 `obit init` 的安装状态、资源管理器右键菜单的结果，现在都是英文。
- **名叫 NUL、CON、COM1 的文件也能处理了**：Windows 本地盘上这类和系统保留设备同名的文件（包括 `con.txt` 这种带扩展名的），现在可以正常登记、下葬、复活和火化；`obit rm ./NUL` 只处理当前目录里的真实文件，不会碰到同名的设备。

---

# Obituary 1.6.2

- **In the Chinese interface, what the command line prints for people is now in Chinese**: table headers, prompts, confirmations, error explanations and background-process notices are translated, and Chinese tables line up by display width. Output meant for agents is exactly as before: hook replies, `--json`, MCP and the skill. A few places that printed Chinese in the English interface, such as `obit init`'s installation status and the results of Explorer context menu actions, now print English.
- **Files named NUL, CON or COM1 work too**: on local Windows drives, files that share a name with a reserved system device (including ones with an extension, like `con.txt`) can now be marked, buried, resurrected and purged, and `obit rm ./NUL` handles the real file in the current directory, never the device.
