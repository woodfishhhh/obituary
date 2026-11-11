# 安装讣告

[English](install.en.md) · [首页](../README.md)

最省事的办法是让 agent 帮你装：把 README 里那句话发给它就行。这一页写给想自己动手的人。

讣告支持 Windows 10 和 11（x64、ARM64）。小窗口 `obit-gui.exe` 用 WebView2 运行时，Windows 11 自带，Windows 10 多数已经装好。

## 1. 装程序

三种方式任选一种：

```powershell
# Scoop
scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket
scoop install obituary

# winget（审核中，通过后可用）
winget install woodfishhhh.Obituary

# 安装脚本：先下载、看一眼，再运行
# 装到 %LOCALAPPDATA%\Programs\obituary，加进用户 PATH，建开始菜单快捷方式
irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1
.\install.ps1 -Version 1.0.0          # -WhatIf 只预览，-NoPath 不改 PATH
```

在中国大陆，安装脚本从国内镜像下载：

```powershell
irm https://cn.obituary.woodfish.site/install.ps1 -OutFile install.ps1
.\install.ps1 -Version 1.0.0 -Mirror cn
```

- 三种方式装的是同一个压缩包，安装前都会核对 SHA-256，对上了才解压。
- 用浏览器手动下载压缩包时，Windows 可能弹出「Windows 已保护你的电脑」，点「更多信息 → 仍要运行」。用 Scoop、winget 或安装脚本装不会弹出。

装完新开一个终端，确认 `obit --version` 能运行。

## 2. 接到你用的 agent 上

```powershell
obit init --show                          # 列出能接的 agent 和它们的名字
obit init -g --agent claude --dry-run     # 先看会改什么
obit init -g --agent claude,codex --yes   # 确认后执行，一次可以接好几家
obit init -g --agent claude --guard-rm --yes   # 同时打开删除护栏：rm -rf 改成能复活的 obit rm -rf
```

`-g` 对所有项目生效；去掉 `-g` 只接当前项目。接好后重启 agent。Codex 要在里面运行一次 `/hooks`，信任讣告的 hook。

## 3. 体检

```powershell
obit doctor
```

逐项检查 PATH、后台进程、各家 agent 的接入、数据目录和配置，每一项写着该怎么处理。

## 更新

讣告每天最多联网一次，看有没有新的正式版。它只读公开的版本信息，不发送任何使用数据；连不上 GitHub 时会改从国内镜像检查。有新版时，托盘菜单和小窗口会提示你，设置页的「关于与更新」里给出对应你安装方式的升级命令：

- Scoop：`scoop update obituary`
- winget：`winget upgrade woodfishhhh.Obituary`
- 安装脚本：用新的版本号再运行一次安装脚本

讣告只负责检查和提醒，升级由你来运行。终端里用 `obit update` 看当前版本和升级命令，用 `obit update --check` 马上检查一次。

不想让它自动联网：在设置页关掉「自动检查更新」，或者在 `config.toml` 里写：

```toml
[update]
check = false
```

关掉后，手动检查照样可以用。更新检查的联网说明见[隐私政策](https://obituary.woodfish.site/docs/privacy)。

## 卸载

```powershell
obit uninstall --dry-run      # 先看会撤掉什么
obit uninstall                # 撤掉讣告装过的所有东西，确认后执行
.\install.ps1 -Uninstall      # 用安装脚本装的：再删掉程序文件、PATH 和开始菜单快捷方式
```

`obit uninstall` 会撤掉讣告装过的所有东西，而且只撤讣告自己加的：
- 用户级和当前 Git 项目里各家 agent 的 hook、MCP 和 Skill；
- 登录自启、定时任务、常驻进程；
- 右键菜单、资源管理器里的日期标签和文件夹策略。

墓地和账本默认保留，它会告诉你墓地里还有多少东西、放在哪里。确实不要了，再加 `--purge-data`，它会先列出墓地里的东西，再跟你确认一次。

在别的项目里也装过项目级 hook 的，先到那些项目里各运行一次 `obit uninstall`，再删程序。用 Scoop 或 winget 装的，先运行 `obit uninstall`，再用它们各自的卸载命令。
