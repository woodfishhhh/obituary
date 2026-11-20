# 隐私政策

讣告的账本、墓地、日志和配置只存在本机。讣告不上传崩溃报告、文件、路径或文件内容。

## 使用统计

统计默认开启，只用于了解有多少安装在用，以及各版本的分布。实际使用时，每 24 小时最多发送一次随机安装 ID、版本、系统、架构、界面语言和安装渠道。不发送文件、路径或命令，不保存 IP 地址或 User-Agent。运行 `obit config telemetry off` 或设置 `OBIT_TELEMETRY=0` 即可关闭。在 CI 中或设置了 `DO_NOT_TRACK=1` 时不上报。

服务端不保存 IP 地址、User-Agent 或原始安装 ID。用独立密钥计算的安装摘要可以识别同一安装的后续报告，不代表一个人。每日明细保留 400 天；安装摘要及首次、末次出现日期长期保留，用于去重。每个渠道的成功次数只记总数。数据只在维护者的认证仪表盘查看。

关闭统计会清除本地安装 ID 和待发状态。统计和查更新可以分别关闭。

## 检查新版本

- 每天最多一次，读取公开的发布信息：先请求 `https://api.github.com` 上讣告仓库的最新发布；GitHub 没有回应时，改读国内镜像的 `https://cn.obituary.woodfish.site/download/latest.json`。
- 请求里只有一般网络请求都会带的信息：你的 IP 地址，以及写着讣告版本号的 User-Agent（例如 `Obituary/1.0.0`）。
- 可以随时关掉：在小窗口「设置」里关闭「自动检查更新」，或者在 `%APPDATA%\obituary\config.toml` 里写：

  ```toml
  [update]
  check = false
  ```

  关掉以后，讣告只在你手动运行 `obit update --check` 时才会查询。

## 安装和下载

- Scoop、winget 和安装脚本从 [GitHub Releases](https://github.com/woodfishhhh/obituary/releases) 下载程序包，适用 [GitHub 的隐私声明](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement)。
- 使用国内镜像（`-Mirror cn`）时从 `cn.obituary.woodfish.site` 下载；服务器只记录一般的访问日志（时间、IP、请求的地址），用于排查故障。

## 接入 agent

`obit init` 只修改你电脑上各家 agent 的配置文件（每一处都先给你看计划），不会把任何信息发给 agent 的服务商或其他地方。

## 官网

- 官网 <https://obituary.woodfish.site/> 不使用任何访问统计或跟踪脚本。国内访问由我们的服务器提供，境外访问由 GitHub Pages 提供，两者都会记录一般的访问日志。
- 在官网留下的邮箱只用于通知新平台和新版本，存在我们自己的服务器上，不会交给第三方；每封邮件都可以退订。

## 联系

隐私相关的问题请在 [GitHub](https://github.com/woodfishhhh/obituary/issues) 提 issue；涉及安全的问题请按 [SECURITY.md](https://github.com/woodfishhhh/obituary/blob/main/SECURITY.md) 私下报告。
