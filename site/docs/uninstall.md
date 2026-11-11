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
