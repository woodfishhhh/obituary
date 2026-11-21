# 排障

出了问题，先运行 `obit doctor`。它只读不写，逐项检查安装、agent 集成、后台定时任务和数据，每一行以 `通过`、`信息`、`警告` 或 `失败` 开头（英文界面是 `PASS`、`INFO`、`WARN`、`FAIL`），后面写着原因和该怎么做；有 `失败` 时命令以非零退出码结束。要把结果发给别人看，加 `--json`；想看每个 agent 具体接上了哪些能力，加 `--verbose`。

下面每一条按同样的顺序写：你会看到什么，为什么，怎么办，怎么确认已经好了。

## 终端里找不到 `obit`

终端提示 `command not found`，PowerShell 说无法识别 `obit`，原因是程序在磁盘上，但它所在的文件夹不在 PATH 里。Windows 的安装脚本会把 `%LOCALAPPDATA%\Programs\obituary` 加进用户 PATH，不过已经打开的终端读不到新的 PATH。`install.sh` 把程序放进 `~/.local/bin`，从来不改 PATH，有的系统默认也没有这个文件夹。Ubuntu 的 `.deb` 把程序放在 `/usr/libexec/obituary/`，同样不在 PATH 里。

Windows 上关掉终端重新打开就行。macOS 和 Linux 要把 `~/.local/bin` 加进 shell 配置，比如 zsh 在 `~/.zshrc` 里加一行 `export PATH="$HOME/.local/bin:$PATH"`，再开一个新终端。只装了 `.deb` 的话，用完整路径 `/usr/libexec/obituary/obit`，或者另外用安装脚本装一份命令行。

`obit --version` 能打印版本号，`obit doctor` 里的 `path` 一行不再是 `警告`，就是好了。

## `obit doctor` 说找不到 `obitd`

`obit doctor` 里出现 `失败 obitd`，说明 `obit` 旁边没有后台程序 `obitd`。讣告到同一个文件夹里找它，所以只拷走 `obit` 一个文件是不够的。把 `obit` 和 `obitd` 放在一起，或者重新运行安装方式对应的安装；之后再运行 `obit doctor`，这一行变成 `通过` 就是好了。

## 下载的文件校验不通过

用安装脚本时，这一步由脚本自己做，哈希对不上它就停下，什么都不会装。只有手动下载压缩包、DMG 或 `.deb` 时，才需要自己核对。对不上的原因通常是下载不完整、文件在路上被改了，或者 `checksums.txt` 取自另一个版本，因为哈希是按版本算的。

从同一个发布页重新下载安装包和 `checksums.txt`（在国内可以从镜像下载），再核对一次，仍然对不上就不要运行它。把下面的 `VERSION` 和文件名换成你下载的那个。Linux：

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict -
```

macOS 把最后一行里的 `sha256sum --check --strict -` 换成 `shasum -a 256 -c -`。Windows PowerShell：

```powershell
$file = "obituary_VERSION_windows_amd64.zip"
(Get-FileHash $file -Algorithm SHA256).Hash -eq ((Select-String -Path checksums.txt -SimpleMatch $file).Line -split '\s+')[0]
```

Linux 和 macOS 打印文件名加 `OK`，PowerShell 打印 `True`，就是对上了。

## Windows 弹出「已保护你的电脑」

用浏览器手动下载的压缩包，Windows SmartScreen 可能拦一下，原因是讣告的程序还没有代码签名。用 Scoop、winget 或安装脚本装一般遇不到。先按上一条核对 SHA-256，对上了再点「更多信息」，然后点「仍要运行」。核对不了，或者系统报告的是恶意软件，而不只是「未知发布者」，就不要运行。

## Windows 的小窗口打不开

从开始菜单或托盘点「讣告」没有反应，或者提示缺少运行时，是因为窗口靠 Microsoft Edge WebView2 运行时绘制。Windows 11 自带它，Windows 10 多数已经装好，没有的机器窗口就起不来。去 Microsoft 官网下载并安装 WebView2 运行时（选常青版），再点一次「讣告」，窗口打开就是好了。

窗口目前只有 x64 版，ARM64 的 Windows 压缩包里没有 `obit-gui.exe`，只能用命令行。托盘菜单里的「打开讣告」弹出通知、说找不到窗口程序，是因为 `obit-gui.exe` 不在 `obitd.exe` 旁边，ARM64 压缩包就是这样；x64 的把它们放进同一个文件夹即可。

## macOS：app 提示与当前系统版本不兼容

1.6.0 的 macOS 小窗口（DMG 里的 `Obituary.app`）需要 macOS 26。在 macOS 15 及更早的系统上，双击它时系统会提示这个 app 与当前版本不兼容，打不开。这是 1.6.0 打包时留下的问题，1.6.1 修复。在那之前，要用这个窗口就得升级到 macOS 26。问题只出在 DMG 里的小窗口上。装上 1.6.1 或更新的版本，app 能正常打开，就是好了。

## macOS：「无法验证开发者」

DMG 里的 app 没有 Developer ID 签名，也没有公证，所以第一次打开会被 Gatekeeper 拦下。核对 SHA-256 之后，先正常打开一次；系统说无法验证开发者，就到「系统设置 → 隐私与安全性」，在页面下方找到 Obituary，点「仍要打开」，再确认一次。步骤在[第一次打开未签名 app](./desktop#第一次打开未签名-app) 里有详细说明。不要为此全局关掉 Gatekeeper。系统如果说 app 已损坏或含有恶意软件，那是另一回事，不要放行。

## 托盘没有随登录启动（macOS、Ubuntu）

登录之后菜单栏或托盘里没有讣告的图标。随登录启动靠一个自启项：macOS 上是用户 LaunchAgent `site.woodfish.obituary.tray`，Ubuntu 上是 `~/.config/autostart/obituary.desktop`。小窗口设置里的开关没开，或者 app 挪过位置、是在 DMG 里直接运行时开的，自启项里记的路径就失效了，登录时找不到程序。

打开小窗口，在设置里把「开机启动托盘」关掉再打开，让讣告重写这一项。macOS 上运行 `launchctl list | grep obituary`，能看到 `site.woodfish.obituary.tray`；Ubuntu 上 `ls ~/.config/autostart/obituary.desktop` 能找到文件，下次登录托盘就会出现。

## Ubuntu：小窗口打不开，或没有托盘图标

从应用列表点「讣告」没有反应，一般是缺 GTK 3 或 WebKitGTK 4.1。用 `apt install ./….deb` 安装时它们会自动装上，被后来删掉或者装到一半出错才会缺。Ubuntu 24.04 里 GTK 3 的包叫 `libgtk-3-0t64`，名字里不带 `t64` 的 `libgtk-3-0` 在这个版本里不存在，照着旧教程敲会失败：

```bash
sudo apt update
sudo apt install libgtk-3-0t64 libwebkit2gtk-4.1-0 xdg-utils
```

装完再从应用列表打开，窗口出现就是好了。窗口能开、只是没有托盘图标，是因为托盘要靠桌面提供 StatusNotifierItem，GNOME 默认没有。不装扩展也能从应用列表打开同一个窗口；想要图标，就给 GNOME 装一个支持它的扩展，比如 AppIndicator。

## 到期的东西没有被下葬

文件寿命早到了，却还在原处，`obit daemon status` 里「运行方式」一行写着「未开启」。到期清理靠一个每 5 分钟运行一次的定时任务：Windows 是任务计划里的「Obituary Runner」，macOS 是 launchd 用户代理，Linux 是 systemd 用户定时器 `obituary.timer`。它没装、被关了，或者装给了另一个数据目录，时间到了就没人动手；这时到期的东西要等到下一次 `obit sweep`，或者 agent 的 hook 触发，才会被处理。

运行 `obit daemon enable` 装上定时任务（Windows 上同时装登录托盘）；眼前就要清理，运行 `obit sweep`。Linux 上如果没有 systemd 用户服务（有些容器和精简系统是这样），`obit daemon enable` 装不上，就改用 cron，每 5 分钟运行一次 `obitd --once --home <数据目录> --config-dir <配置目录>`，这两个目录 `obit config` 都会告诉你。`obit daemon status` 里 `runner` 不再是 `off`，就是好了；Linux 上还可以用 `systemctl --user list-timers | grep obituary` 看到它。

## 到期了，但没有下葬成功

小窗口「待你决定」页的「没埋成的」里有这一项，旁边写着原因，常见的是文件正被编辑器、同步盘或别的程序占着，文件还留在原处。下葬要把文件挪走，被占用或没有权限时挪不动。讣告不会硬来，原文件保持原样，过一阵再试，间隔越来越长，多次失败后停下，等你决定。

先关掉占用文件的程序，再点「重试」，或者运行 `obit sweep`。不想再管它，就点「不再跟踪」，文件留在原处，不再计时。这一项从「没埋成的」里消失，并且出现在 `obit graveyard` 里，就是下葬成功了。

## 复活之后文件名变了

`obit resurrect` 没有报错，放回来的文件却叫 `report (resurrected).txt`，是因为原位置已经有同名的东西了。复活从不覆盖，名字被占时就在同一个文件夹里换个名字放回来，两份内容都完好。自己比较一下，留下要的那份，把名字改回去。

## 配置文件写错了

`obit config` 报「配置文件有错误」（英文界面是 `config file has errors`），后面跟着文件路径、行号和键名，是因为 `config.toml` 里有讣告不认识的键，或者取值不合法。讣告不会悄悄忽略它们，否则 `retention = 30` 会让 7 天的保留期照旧生效，你却以为已经改了。按提示里的行号改正拼写、引号或时长，再运行 `obit config`；不再报错，`obit doctor` 里的 `config` 一行是 `通过`，就是好了。各项的写法见[配置参考](./config)。

## hook 没有生效

`obit init` 装完，agent 删文件时没有被拦下，或者什么提示都没出现。常见的原因有三个：agent 没有重启，还在用旧配置；这个 agent 的集成装在了另一个范围（全局还是项目）；agent 的环境里设了 `OBIT_DISABLED=1`（关掉所有 hook）或 `OBIT_GUARD_RM=0`（关掉删除护栏）。

先看装了什么：

```bash
obit init -g --show        # 全局装了什么
obit init --show           # 当前项目装了什么
obit doctor --verbose      # 每个 agent 的接入情况
```

没有装，就用 `obit init -g --agent <名字>` 装上，想先看计划就加 `--dry-run`。装完重启 agent；Codex 还要在里面运行 `/hooks`，信任讣告的 hook。想知道 hook 到底收到了什么，在启动 agent 的终端里先设 `OBIT_HOOK_AUDIT=1`，用一阵之后运行 `obit hook audit`。被拦下的删除命令不需要这个开关，本来就会记录。hook 自己的报错写在数据目录的 `hook.log` 里，`obit doctor` 的 `hook-log` 一行会数最近 24 小时的错误。

`obit doctor` 里这个 agent 的那一行（比如 `claude`）是 `通过`、写着「已安装」（英文界面是 `PASS` 和 `integration is current`），就是装好了；用删除护栏的话，`claude-guard-rm` 这样的行要写「已注册：是」（英文界面是 `registered: yes`）。在启动 agent 的同一个终端里运行 `obit doctor`，如果环境变量把 hook 关了，这一行会变成 `警告`，并写出是哪个变量。

## Docker 容器的提议批准不了

小窗口里容器提议的「批准」点不了，卡片上写着「Docker 没有运行：先启动 Docker，才能处理这条提议」。登记容器要向 Docker 确认它完整的 ID 和创建时间，Docker 没运行就确认不了。启动 Docker Desktop 或 Docker Engine，再批准；「批准」可以点了，`obit docker list` 里列出这个容器，就是好了。

容器到期时讣告只是把它停掉，可写层留着，复活时重新启动；保留期过后才删除已停止的容器。镜像、卷和网络从来不动。

## 更新失败，或国内下载很慢

`obit update --check` 报错，或安装脚本下载很慢、失败，多半是这台电脑连不上 GitHub。`obit update --check` 和安装脚本在 GitHub 迟迟没有回应时，会自己改走国内镜像。想一开始就走镜像，macOS 和 Linux 运行 `bash install.sh --mirror cn`，Windows 运行 `.\install.ps1 -Mirror cn`。手动下载的话，压缩包和 `checksums.txt` 要取自同一个版本、同一个来源。`obit update --check` 能打印出 `Latest:` 一行，就是好了。
