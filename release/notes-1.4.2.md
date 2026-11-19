# 讣告 1.4.2

删除护栏现在也拦得住绕路的删除。

- **删除护栏拦下 `find -delete`、`find -exec rm` 和 `xargs rm`**：之前护栏只认出现在命令位置上的 `rm`、`rmdir`、`git clean` 和 `Remove-Item`。agent 被拦下以后，改用 `find … -delete` 或 `xargs rm` 就能绕过去。现在这几种写法也会被拦下，并提示先列出要删的目标，再用 `obit rm` 下葬。
- **国内镜像更快**：新版本发布以后，国内服务器直接从 GitHub 多线程拉取安装包，几分钟内就能从国内源下载和更新。

升级：Windows 可以在「设置 → 关于与更新」点「立即更新」，也可以用 `scoop update obituary` 或再运行一次安装脚本（国内加 `-Mirror cn`）；macOS 用 `brew upgrade --cask obituary`；macOS / Linux 也可以再运行一次 `bash install.sh`（国内加 `--mirror cn`）。

---

# Obituary 1.4.2

The delete guard now catches indirect deletions.

- **The delete guard denies `find -delete`, `find -exec rm`, and `xargs rm`.** It used to recognize only `rm`, `rmdir`, `git clean`, and `Remove-Item` in command position, so an agent that was denied could switch to `find … -delete` or `xargs rm`. These forms are now denied too, with a hint to list the targets first and bury them with `obit rm`.
- **Faster mainland China mirror.** After a release is published, the mirror server pulls the packages from GitHub over parallel connections, so they are available from the China source within minutes.

On Windows, choose Update now in Settings → About & Updates, run `scoop update obituary`, or run the install script again (`-Mirror cn` in mainland China). On macOS, `brew upgrade --cask obituary`; on macOS and Linux, running `bash install.sh` again works too (`--mirror cn` in mainland China).
