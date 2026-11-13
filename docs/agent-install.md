# 让 agent 帮你装讣告

> 如果你是 agent，被要求照这一页安装讣告：执行下面代码块里的步骤（用户说英文就用 [In English](#in-english) 那一段），遵守里面的规则。

把下面这段话原样发给你正在用的 agent（Claude Code、Codex、Cursor、DeepSeek、WorkBuddy……都行）。它会装好讣告，把讣告接到它自己身上，最后告诉你结果；要你拍板的地方，它会停下来问你。

```text
请帮我安装讣告（Obituary，命令是 obit），并把它接入你自己。按顺序做，每一步把命令和输出给我看：

1. 先运行 obit --version。已经装了就跳到第 3 步。
2. 先确认操作系统。Windows 安装，按这个顺序选第一个可用的：
   - 有 scoop：scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket，再 scoop install obituary
   - 没有 scoop：在 PowerShell 里 irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1，把脚本内容给我过目，我同意后再运行 .\install.ps1 -Version 1.1.1
   - macOS / Linux：curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh，把脚本内容给我过目。我同意后运行 bash install.sh --version 1.1.1（macOS 上有 Homebrew 时也可以改用 brew install --cask woodfishhhh/tap/obituary）。默认装到 ~/.local/bin，可用 --install-dir 指定目录；按输出提示配置 PATH。
   装完新开一个终端，确认 obit --version 能运行。
3. 运行 obit init --show，找到你自己对应的 agent 名字。
4. 运行 obit init -g --agent <你的名字> --dry-run，把计划给我看。默认会装所选 agent 的 Agent Skill；Windows 还会装右键菜单、开始菜单和桌面快捷方式，可用 --no-skill / --no-context-menu / --no-shortcuts 跳过。问我要不要同时打开删除护栏（--guard-rm：拦下 rm -rf 这类删除，改用可以复活的 obit 命令）。我同意后，去掉 --dry-run、加上 --yes 再执行一次。macOS / Linux 再运行 obit daemon enable --dry-run，确认后用 obit daemon enable 启用用户级 launchd / systemd 定时检查；Linux 没有 systemd 时给我用户 crontab 的 obitd --once 配置步骤。
5. 运行 obit doctor，把 FAIL 和 WARN 逐条告诉我，说明哪些是正常的。
6. 告诉我接下来要做什么：通常是重启你自己；如果你是 Codex，要在 Codex 里运行 /hooks 信任讣告的 hook。

规则：
- 不要手改任何 hook 或配置文件，一律通过 obit init。
- obit approve、obit reject、obit trust、obit purge 和 --purge-data 都由我决定，不要替我运行。
- 任何一步报错就停下来，把完整输出给我，不要自己换别的办法绕过去。
```

装好以后，讣告会这样陪着 agent 干活：它生成的调试脚本、日志、截图按规则自动带上寿命；规则覆盖不到的，会话结束后列成提议，等你批准；它用到一个已经下葬的文件时，会被当场提醒去复活；打开删除护栏后，它的 `rm -rf` 会改成可以复活的 `obit rm -rf`。各家 agent 能做到哪一步，见 [agent 覆盖表](agents.md)。

## In English

Paste this to the agent you use and it will install Obituary, connect it to itself and report back, stopping wherever a decision is yours:

```text
Please install Obituary (the command is obit) and integrate it with yourself. Go step by step and show me each command and its output:

1. Run obit --version. If it is already installed, skip to step 3.
2. Identify the operating system first. On Windows install it, using the first option that is available:
   - scoop: scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket, then scoop install obituary
   - no scoop: in PowerShell, irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1, show me the script, and only after I agree run .\install.ps1 -Version 1.1.1
   - macOS / Linux: curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh, show me the script, and only after I agree run bash install.sh --version 1.1.1 (on macOS with Homebrew, brew install --cask woodfishhhh/tap/obituary works too). The default directory is ~/.local/bin; --install-dir overrides it. Follow the printed PATH instructions.
   Then open a new terminal and confirm that obit --version runs.
3. Run obit init --show and find your own agent name.
4. Run obit init -g --agent <your name> --dry-run and show me the plan. The selected agent's Agent Skill is installed by default; Windows also gets context menus and Start menu / desktop shortcuts. Skip them with --no-skill / --no-context-menu / --no-shortcuts. Ask me whether to turn on the delete guard as well (--guard-rm: deletions such as rm -rf are stopped and replaced by restorable obit commands). After I agree, run it again without --dry-run and with --yes. On macOS / Linux also preview obit daemon enable --dry-run, then run obit daemon enable after confirmation for the user launchd / systemd scheduler; on Linux without systemd, show me user crontab steps for obitd --once.
5. Run obit doctor and go through every FAIL and WARN with me, saying which ones are expected.
6. Tell me what to do next: usually restart yourself; if you are Codex, run /hooks in Codex and trust Obituary's hooks.

Rules:
- Never edit hook or configuration files by hand; always go through obit init.
- obit approve, obit reject, obit trust, obit purge and --purge-data are my decisions; never run them for me.
- If any step fails, stop and show me the full output instead of working around it.
```
