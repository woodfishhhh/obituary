# 讣告 1.4.3

Windows 上的 Codex，命令执行前还会再经过一层 shell。删除护栏现在把这一层也算进去。

- **引号里的替换也查**：Windows 上的 Codex 会把命令再交给另一层 shell，引号可能在那一层失效，按 Bash 规则只是文字的内容也可能被执行，比如带引号的 heredoc 正文里写的 `find -delete`。现在 Windows 上 Codex 的命令里，反引号片段和 `$(…)` 片段不论写在引号、带引号的 heredoc 还是注释里，都按 Bash 和 PowerShell 两种读法检查，里面有删除就拦下。agent 要写含反引号的文字（提交说明、补丁、文档）时，用编辑工具写进文件、再把文件路径交给命令，就不会被拦。
- 其他 agent，以及其他系统上的 Codex，行为不变。

升级：Windows 可以在「设置 → 关于与更新」点「立即更新」，也可以用 `scoop update obituary` 或再运行一次安装脚本（国内加 `-Mirror cn`）；macOS 用 `brew upgrade --cask obituary`；macOS / Linux 也可以再运行一次 `bash install.sh`（国内加 `--mirror cn`）。

---

# Obituary 1.4.3

On Windows, Codex passes each command through one more shell before it runs. The delete guard now accounts for that layer.

- **Substitutions count even inside quotes.** On Windows, Codex hands the command to another shell where quoting may fail, so text that Bash would keep literal can run, such as `find -delete` written inside a quoted heredoc. In a Codex command on Windows, every backtick span and `$(…)` is now read as Bash and as PowerShell wherever it appears, including quotes, quoted heredocs and comments, and a deletion inside one is denied. When an agent needs text with backticks (commit messages, patches, docs), writing it to a file with an editing tool and passing the file path to the command avoids the denial.
- Other agents, and Codex on other systems, behave as before.

On Windows, choose Update now in Settings → About & Updates, run `scoop update obituary`, or run the install script again (`-Mirror cn` in mainland China). On macOS, `brew upgrade --cask obituary`; on macOS and Linux, running `bash install.sh` again works too (`--mirror cn` in mainland China).
