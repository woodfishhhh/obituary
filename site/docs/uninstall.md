# 卸载

```powershell
obit uninstall --dry-run      # 先看会撤掉什么
obit uninstall                # 撤掉讣告装过的所有东西，确认后执行
.\install.ps1 -Uninstall      # 用安装脚本装的：再删掉程序文件、PATH 和开始菜单快捷方式
```

`obit uninstall` 撤掉用户级和当前 Git 项目里各家 agent 的 hook、MCP 和 Skill，登录自启、定时任务、常驻进程、右键菜单、资源管理器里的日期标签和文件夹策略，只撤讣告自己加的东西。

## 墓地和账本默认保留

卸载时墓地和账本默认保留，并告诉你墓地里还有多少东西、在哪。确实不要了再加 `--purge-data`，它会先列出墓地里的东西并再确认一次。

## 装在别的项目里的

在别的项目里也装过项目级 hook 的，先到那些项目里各执行一次 `obit uninstall`，再删程序，否则那些项目里的 hook 会因为找不到 `obit` 而失效。

## Scoop 和 winget

先执行 `obit uninstall`，再用各自的卸载命令。
## 卸载清单和数据位置

`obit uninstall --dry-run` 先列出当前用户和当前 Git 项目的可撤销项。正式执行会移除程序集成、hook、MCP、Skill、托盘、自启和调度，但不会替你删除其他项目中的项目级配置；请到那些项目分别执行卸载。

| 位置或组件 | Windows | macOS | Linux |
| --- | --- | --- | --- |
| 程序 | 安装目录中的 `obit`、`obitd`、可选 `obit-gui` | `/Applications/Obituary.app` 或 CLI 安装目录 | CLI 安装目录；桌面版为 `.deb` |
| hook、MCP、Skill | 用户目录或项目里的 agent 配置 | 同样由 agent 配置目录决定 | 同样由 agent 配置目录决定 |
| 后台调度 | 计划任务、可选常驻进程和托盘 | 用户 LaunchAgent、可选菜单栏托盘 | systemd 用户定时器；桌面托盘可有 XDG 自启 |
| 项目配置 | 项目 `.claude/`、`.codex/` 等讣告写入的部分 | 项目配置 | 项目配置 |

默认数据位置：Windows 为 `%LOCALAPPDATA%\obituary`，配置为 `%APPDATA%\obituary`；macOS 为 `~/Library/Application Support/obituary` 和 `~/Library/Application Support/obituary`；Linux 为 `$XDG_DATA_HOME/obituary`（未设置时 `~/.local/share/obituary`）和 `$XDG_CONFIG_HOME/obituary`（未设置时 `~/.config/obituary`）。`OBIT_HOME`、`OBIT_CONFIG_DIR`、`OBIT_USER_HOME` 可分别重定向数据、配置和用户主目录；同一卷还可能有 `.obituary` 墓地目录。

墓地、账本、审计日志和配置默认保留。`--purge-data` 只清理由讣告拥有且能核实的墓地数据，并保留未知文件；它不是删除用户主目录或其他项目的通行证。墓地模式下项目可用 `resurrect`，回收站模式下必须从系统回收站恢复，不能用 `resurrect`。
