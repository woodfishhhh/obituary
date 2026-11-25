# 平台构建与发布验收

Windows 与 Ubuntu 24.04 amd64 提供小窗口和托盘。资源管理器右键菜单和系统回收站模式仅限 Windows。

## Linux 桌面

Linux 桌面用 Ubuntu 24.04 amd64 原生构建：安装 `libgtk-3-dev libwebkit2gtk-4.1-dev`，前端改动先运行 `node scripts/build-frontend.mjs`。`CGO_ENABLED=1 go build -tags desktop,production,webkit2_41 -o bin/obit-gui ./cmd/obit-gui` 构建 Wails 小窗口。`goreleaser release -f .goreleaser-linux-desktop.yaml --snapshot --skip=publish` 生成 `bin/linux-desktop/*.deb`；六个原有归档仍用 `.goreleaser.yaml`。桌面作业的系统依赖见 `.github/workflows/linux-desktop.yml`；`bash scripts/test-linux-desktop-session.sh bin/linux-desktop/*.deb <证据目录>` 使用私有 Xvfb、会话总线和临时用户目录，验证窗口、托盘、复活和通知。不要在宿主显示器上验收。仅在一次性 Ubuntu 24.04 容器中用 `scripts/test-linux-desktop-package.sh` 验证安装与移除。

## macOS 桌面

macOS GUI 只能在 GitHub Actions 的 macOS runner 上构建和验收：`go test -tags desktop ./internal/app ./internal/gui ./internal/tray` 验证原生绑定，`bash scripts/build-macos-desktop.sh VERSION OUTPUT_DIR` 生成通用 `Obituary.app`、DMG 和 checksums。工作流 `.github/workflows/macos-desktop.yml` 还会用 `plutil`、`lipo`、ad-hoc `codesign`、`hdiutil` 和 app 内的 `--smoke` 检查包，并上传窗口与菜单栏截图；Windows 本地不要交叉构建 macOS GUI，也不要把 CI 收据当作真机通知授权或首次放行证明。

发布前用 `macos-acceptance.yml` 检查候选 DMG。发版工作流把同一次构建的 DMG 分发到 macOS 15、26 的 arm64 和 x86_64 runner，全部通过后才允许将该产物上传到草稿发布。核对每个切片的最低系统版本、首次打开的窗口和菜单栏、菜单只复活所选样本，以及卸载前后的账本、设置和保留样本哈希。隔离属性测试和桌面测试使用不同的干净用户环境；Gatekeeper 手动放行、通知授权和真实重新登录不能由截图代替。发布后用 `workflow_dispatch` 指定 `release_tag` 复核正式 DMG，不重新构建。

## 国内发布镜像

`mirror-latest.yml` 与 `pages.yml` 的 `deploy-cn` 共用 `cn-deploy`，不取消运行中的任务。正式 Release 发布时自动推送；重放用 `gh workflow run mirror-latest.yml --ref main -f tag=vX.Y.Z`。runner 用只读 `GITHUB_TOKEN` 下载 Release 的六个归档和 `checksums.txt`，从同一 Git 标签取两个安装脚本；核对归档清单和 SHA-256 后，经固定的 `CN_DEPLOY_KNOWN_HOSTS` 上传。`transport.sha256` 校验脚本和元数据的传输，不是独立的发布者签名。

专用仓库 Secret 是 `CN_RELEASE_DEPLOY_KEY`，ed25519 指纹 `SHA256:XDh6ylMRs/gpoyCC1T2TXzwFlHlIUOLIpmPzVpdoQ2E`。非 root 用户 `obit-deploy` 的 `authorized_keys` 只为这把钥匙增加 `restrict,command="/usr/local/bin/obituary-mirror-publish"`。该入口来自 `scripts/cn-mirror-pull.sh`，只接受以 `/home/obit-deploy/release-staging`（0700）为根的只写 rrsync，或 `publish vX.Y.Z`。两者都拿 `download/.pull.lock`；服务端从 `incoming/` 复核文件和哈希，在同一文件系统上改名发布完整版本，再按 runner 核对的最新正式版原子更新 `latest.json`。同标签内容不同则拒绝覆盖，相同则保留原文件。日志用 `journalctl -t obituary-mirror` 查。

两个上传任务的 `rsync --timeout=120` 是双向 I/O 超时，不能保证服务器发现消失的 runner。国内服务器的 `/etc/ssh/sshd_config.d/00-obit-client-alive.conf` 在账号 `Match` 之前全局设置 `ClientAliveInterval 30` 和 `ClientAliveCountMax 4`，让 sshd 断开不再回应探测的连接，接收端随之退出并放锁；能回应保活的慢连接不会因此断开。镜像任务给 60 分钟，国内站部署给 40 分钟，给慢链路留出传输时间。只有非公开的镜像 `incoming/` 用 `--partial`：重跑时利用已有数据，发布前仍须完整校验；不要把半成品留在公开站点。用 `bash scripts/cn-mirror-resume.test.sh` 验证只写 rrsync 确实复用了部分数据。

只回滚保活和超时：等 `cn-deploy` 空闲再回退相应工作流；服务器保留一条管理连接，核对 `/root/obit-sshd-<日期时间>-<后缀>/` 的主配置、drop-in 目录及 `before/` 基线。确认新增文件仍是上述两行且原来不存在后，只将它移出 Include 目录，不覆盖其他文件。`sshd -t` 通过，并按原连接条件逐账号比对 `sshd -T -C`，确认仅保活两项还原后 `systemctl reload ssh.service`；另开管理连接验证成功才关闭旧连接。不动钥匙、发布入口或已发布文件。

人工备用仍是原来的 `pull vX.Y.Z`（`CN_MIRROR_TRIGGER_KEY`、`/usr/local/bin/obituary-mirror-pull`，源码 `scripts/cn-mirror-pull-legacy.sh`）；只有这条备用路径从服务器直连 GitHub。若要回滚整个 runner 发布入口，先等 `cn-deploy` 空闲，回退发布工作流，再只移除上述指纹对应的新增授权行并撤销 `CN_RELEASE_DEPLOY_KEY`。核对原授权行和旧脚本与服务器 `/root/obituary-cn-mirror-<日期时间>-<后缀>/` 中的备份一致，不覆盖后续新增授权。需要恢复索引时，先核对备份的 `download.inventory` 和最新版本，持 `.pull.lock` 将备份的 `latest.json` 通过同目录临时文件改名恢复；不要删除任何已发布版本。最后停用新增入口和 staging，保留备份及旧 `pull` 入口。回滚不改 `CN_DEPLOY_KEY`、原站点钥匙或 `deploy-cn`。
