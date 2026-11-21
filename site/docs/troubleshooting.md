# 排障

先运行 `obit doctor`，它只读检查安装、集成、后台服务和数据，不会修改文件。需要更多细节时加 `--json` 或 `--verbose`。

## 安装、路径和校验

**症状：** 终端提示找不到 `obit`，或下载的安装包被改动。

**原因：** 程序目录没有加入 `PATH`，或者归档没有通过 SHA-256 校验。

**处理：** 重新下载同一版本的 `checksums.txt`，在下载目录运行：

```bash
sha256sum --check --strict checksums.txt
```

Windows PowerShell 使用 `Get-FileHash .\obit-*.zip -Algorithm SHA256`，把结果与校验文件比较；校验不一致时不要运行程序。安装后重新打开终端，让 `PATH` 生效。

## Windows 桌面

**SmartScreen：** 未签名程序可能显示“已保护你的电脑”。先核对 SHA-256，选择“更多信息 → 仍要运行”；无法核对或提示恶意软件时停止。

**WebView2：** 小窗口打不开或提示运行时缺失时，安装 Microsoft Edge WebView2 Runtime，再运行 `obit doctor`。Windows 小窗口目前只提供 x64。

## macOS 桌面

**Gatekeeper：** DMG 内的 app 未 Developer ID 签名或公证。核对校验和后第一次打开；若提示无法验证开发者，到“系统设置 → 隐私与安全性”选择“仍要打开”。不要关闭 Gatekeeper，也不要放行显示损坏或恶意软件的 app。

**launchd：** 菜单栏托盘未随登录启动时，先在设置中关闭再打开“随登录启动托盘”，然后检查：

```bash
obit daemon status
launchctl list | grep obituary
```

## Linux 桌面和后台

**WebKitGTK：** Ubuntu 24.04 小窗口需要 GTK 3 和 WebKitGTK 4.1。安装桌面包后仍打不开时运行：

```bash
sudo apt update
sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
obit doctor
```

**systemd / crontab：** 用户定时器未运行时查看：

```bash
systemctl --user status obituary.timer
systemctl --user list-timers | grep obituary
crontab -l
```

没有桌面包的 Linux 仍可直接运行 CLI；`obit daemon enable` 使用用户级调度，`obit daemon status` 查看下一次到期时间。

## 后台进程和更新

**后台没有运行：** 运行 `obit daemon status`。需要立即处理到期项目可运行 `obit sweep`；需要恢复后台调度可运行 `obit daemon enable`。

**更新失败或国内网络无法下载：** 先运行 `obit update --check`，再从同一发布版本下载归档和 `checksums.txt`。国内镜像安装脚本使用：

```bash
bash install.sh --mirror cn
```

不要混用不同版本的归档和校验文件。

## 文件和配置问题

**WSL 挂载盘改名：** 在 `/mnt/<盘符>` 下移动或改名后，如果路径不再对应原文件，先在 Windows 路径中确认真实位置，再用字面路径运行 `obit list`、`obit graveyard` 或重新 `obit mark`；不要用旧路径强行复活。

**权限、占用和重试：** 文件被编辑器或同步软件占用时，下葬会保留原文件并安排重试。关闭占用程序后运行 `obit sweep`；仍失败可在小窗口的“待你决定”中选择“重试”或“不再跟踪”。不要手动删除失败记录。

**TOML 写错：** `obit config` 会报告文件、行号和未知键。修正拼写、引号或时长后再次运行；未知键不会被忽略。

## 集成和容器

**hook 没生效：** 在当前项目或全局重新运行对应安装：

```powershell
obit init --show
obit init --dry-run
obit doctor --verbose
```

确认 agent 使用的是已安装的配置，且没有 `OBIT_DISABLED=1`。启用 hook 审计后可查看收到的事件：`OBIT_HOOK_AUDIT=1 obit hook audit`。

**Docker 没启动：** `obit docker list` 或窗口会显示连接失败。启动 Docker Desktop 或 Docker Engine 后，再批准容器提议；镜像、卷和网络不会由讣告删除。

**复活时目标已存在：** `obit resurrect` 不覆盖现有文件；它会使用不冲突的名称放回。先检查原目录，再移动或改名冲突目标，最后重试复活。
