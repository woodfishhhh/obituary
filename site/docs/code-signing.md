# 代码签名政策

Free code signing provided by [SignPath.io](https://about.signpath.io/), certificate by [SignPath Foundation](https://signpath.org/).

讣告的 Windows 程序由 [SignPath.io](https://about.signpath.io/) 免费签名，证书来自 [SignPath Foundation](https://signpath.org/)。

## 签名的是什么

- 发布压缩包里的 `obit.exe`、`obitd.exe` 和 `obit-gui.exe`。
- 它们只从本仓库打了版本标签的提交构建，由 GitHub Actions 上的发布流程（[`release.yml`](https://github.com/woodfishhhh/obituary/blob/main/.github/workflows/release.yml)）自动完成，构建脚本和 CI 配置都在仓库里。
- 每一次签名请求都由下面的批准人逐个人工批准。
- 程序里写有产品名「Obituary 讣告」和版本号，可以在文件属性的「详细信息」里看到。

## 团队和角色

| 角色 | 成员 |
| --- | --- |
| 提交者和审阅者（Committers and reviewers） | [woodfishhhh](https://github.com/woodfishhhh) |
| 批准人（Approvers） | [woodfishhhh](https://github.com/woodfishhhh) |

维护者以外的人提交的每一处改动，都要由维护者审阅后才会合并。所有成员的 GitHub 账号都开启了两步验证。

## 隐私

讣告联网只做一件事：每天最多一次检查新版本，可以随时关掉。详见[隐私政策](privacy)。
