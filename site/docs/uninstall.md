# 卸载

卸载分两步：先让讣告撤掉它装过的东西，再删程序本身。墓地和账本默认留着，你放进墓地的文件之后仍然能复活。

```powershell
obit uninstall --dry-run      # 只列出会撤掉什么，什么都不改
obit uninstall                # 列出计划，确认后执行
```

在 Windows 上用安装脚本装的，一条命令就够了：

```powershell
.\install.ps1 -Uninstall      # 先替你运行 obit uninstall --yes，再删程序文件、它加的 PATH 项和开始菜单快捷方式
```

## 会撤掉什么

`obit uninstall` 处理的是用户级的和当前 Git 项目里的东西：各家 agent 的 hook、MCP 和 Skill，定时任务（Windows 的任务计划、macOS 的 launchd 用户代理、Linux 的 systemd 用户定时器），后台常驻进程和托盘，以及文件夹策略（策略下的文件留在原处）。Windows 上还会撤掉登录自启、资源管理器的右键菜单和日期标签。

它只动讣告自己写下的东西。你改动过的条目，或者别的安装留下的条目，会原样保留，计划里写着保留了哪一项、为什么。在没有终端的环境里（脚本、agent 的命令行）要加 `--yes`，否则它会提示需要终端。

有三样它不会替你做。程序文件要按你的安装方式去删（见下文）。在别的项目里装过项目级 hook 的，要先到那些项目里各执行一次 `obit uninstall`，再删程序，不然那里的 hook 会因为找不到 `obit` 而失效。macOS 和 Linux 上的托盘自启项也不在清单里：卸载前先在小窗口的设置里关掉「开机启动托盘」，它会连同讣告自己写的自启项一起撤掉；Windows 上托盘自启由 `obit uninstall` 负责。Windows 上 `obit init` 建的开始菜单和桌面快捷方式同样不在清单里，`obit init --uninstall` 会删掉它们，加上 `--no-shortcuts` 就留下。

## 墓地、账本和配置

默认保留的有三样：墓地里的文件、账本和审计日志、`config.toml`。`obit uninstall` 会告诉你墓地里还有多少东西、放在哪儿；以后重新安装，它们接着可用。如果你用的是回收站模式，到期的东西本来就在系统回收站里，卸载不碰它，也要从回收站里还原，`obit resurrect` 够不着。

确实不要了，再加 `--purge-data`：

```powershell
obit uninstall --purge-data
```

它会先列出墓地里的每一项，在你确认卸载计划之后再确认一次。确认之后、撤掉任何东西之前，讣告先检查一遍：数据目录是不是讣告自己的，里面的文件认不认得，墓地文件夹和要清除的每一座坟能不能核实。哪一项过不了，就在这里停下并说明原因，墓地、账本和服务都原封不动，不会删到一半。检查全部通过，才把这些文件永久删除，再删账本、审计日志等讣告自己的文件。脚本里同时写 `--yes --purge-data` 才会跳过这两次确认。

它只删讣告认得的文件。数据目录里混着别的文件、链接，或有它不认识的子文件夹，都会让检查停下，并指出是哪一个。`config.toml` 永远不在清除范围内，也不必为此移走它，哪怕它和账本在同一个文件夹（macOS 默认如此）：清除之后，那个文件夹里只剩 `config.toml`。配置在另一个位置时，数据目录整个删掉。

## 删掉程序

删程序的方法取决于你怎么装的。

| 安装方式 | 删除 |
| --- | --- |
| Windows 安装脚本 | `.\install.ps1 -Uninstall` |
| Scoop | 先 `obit uninstall`，再 `scoop uninstall obituary` |
| winget | 先 `obit uninstall`，再 `winget uninstall woodfishhhh.Obituary` |
| macOS 小窗口（DMG） | 先 `obit uninstall` 并关掉「开机启动托盘」，再把「应用程序」里的 Obituary.app 拖进废纸篓 |
| Homebrew | 先 `obit uninstall`，再 `brew uninstall --cask obituary` |
| macOS 和 Linux 的 `install.sh` | 先 `obit uninstall`，再删 `~/.local/bin` 里的 `obit` 和 `obitd` |
| Ubuntu 桌面包（`.deb`） | 先关掉「开机启动托盘」并退出托盘，再 `sudo apt remove obituary-desktop` |

macOS 的 app 和 Ubuntu 的 `.deb` 都自带 `obit`，但没有放进 PATH。没有另外装过命令行的话，卸载前用完整路径运行它：app 里的是 `/Applications/Obituary.app/Contents/MacOS/obit`，`.deb` 装的是 `/usr/libexec/obituary/obit`。`apt remove` 会删掉那个文件夹里的程序，但不碰你的墓地、设置和用别的方式装的命令行。

## 数据放在哪儿

| | 数据（账本、墓地、日志） | 配置（`config.toml`） |
| --- | --- | --- |
| Windows | `%LOCALAPPDATA%\obituary` | `%APPDATA%\obituary` |
| macOS | `~/Library/Application Support/obituary` | 同一个文件夹 |
| Linux | `~/.local/share/obituary` | `~/.config/obituary` |

Linux 上设了 `XDG_DATA_HOME` 或 `XDG_CONFIG_HOME` 的话，各自在它们下面的 `obituary`。环境变量 `OBIT_HOME` 和 `OBIT_CONFIG_DIR` 可以把数据和配置挪到别处；`obit config` 会打印当前用的是哪一个。数据目录所在的那块盘，墓地就在它的 `graveyard` 里；别的盘上的墓地在那块盘根目录的隐藏文件夹 `.obituary` 里，这样下葬始终是同一块盘上的改名，不用拷贝。
