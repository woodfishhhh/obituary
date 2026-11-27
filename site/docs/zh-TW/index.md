# 快速開始

Obituary 為檔案和資料夾設定壽命。壽命到期後，它們會被下葬到墓地；在保留期內（預設 7 天）隨時可以復活，只有過了保留期才會真正刪除。它是為 AI agent 打造的：agent 產生的除錯指令碼、日誌、截圖和測試輸出，從建立的那一刻起就帶有壽命。沒有人需要記得清理，agent 的 `rm -rf` 也不再把任何東西永久摧毀。

## 1. 安裝

Windows 10 / 11、macOS 和 Linux（x64 和 ARM64）：

| | Windows | macOS | Linux |
|---|---|---|---|
| 安裝方式 | Scoop、`install.ps1` 指令碼（winget 審核中） | Homebrew、`install.sh` 指令碼 | `install.sh` 指令碼 |
| 到期檢查 | 常駐系統匣程序，工作排程器作為備援 | 使用者的 launchd 代理程式 | `systemd --user` 計時器；沒有 systemd 時使用使用者 crontab |
| 命令列、墓地與復活、自動登記（可選）、刪除護欄、agent 整合（hook、Skill、MCP）、Docker 容器、資料夾策略 | 有 | 有 | 有 |
| 系統匣、小視窗、開始功能表和桌面捷徑 | 有 | macOS 通用 DMG：[小視窗與選單列系統匣](./desktop) | Ubuntu 24.04 amd64：[小視窗與系統匣](./desktop) |
| 檔案管理員的右鍵選單與下葬日期 | 檔案總管 | Finder：快速動作、日期標籤 | Nautilus：指令稿 · Dolphin：選單、日期標籤 |
| 資源回收筒模式 | 有 | 僅墓地 | 有 |

**最簡單的方法：讓 agent 幫你處理。** 把這句話傳給你使用的 agent（Claude Code、Codex、Cursor……都可以）。它會安裝 Obituary、連接到自己，並在需要你決定的地方停下來：

```text
Read https://obituary.woodfish.site/agent-install.md and follow it to install Obituary and connect it to yourself.
```

**手動安裝：** 在下方選擇你的作業系統，然後執行 `obit doctor` 檢查安裝結果。

### macOS / Linux

在 macOS 上，Homebrew 一行就能完成安裝：

```bash
brew install --cask woodfishhhh/tap/obituary
```

或者下載並閱讀安裝指令碼，再執行它（macOS 和 Linux）：

```bash
curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh
bash install.sh
# Custom directory: --install-dir "$HOME/bin"; preview only: --dry-run
```

指令碼預設安裝最新發行版（`--version <version>` 可選擇其他版本）。壓縮檔會同時分成四個部分下載（有些路線會限制每個連線的速度），失敗時會退回單一連線。它會偵測作業系統和架構，從同一個下載來源核對 SHA-256，並將 `obit` 和 `obitd` 安裝到 `~/.local/bin`。請依照 PATH 指示操作，然後執行 `obit --version`。中國大陸鏡像請使用 `--mirror cn`；指令碼位於 `https://cn.obituary.woodfish.site/install.sh`。

在 macOS 首次使用時，請執行 `obit --version` 和 `obitd --help`。如果 Gatekeeper 要求核准，請開啟「系統設定 → 隱私權與安全性」，對每個已核對來源的執行檔選擇「強制打開」，然後重試命令。

在 WSL 中，請把專案和 agent 設定放在 Linux 原生目錄（如 `~/projects`）。`/mnt/c` 等 DrvFS 掛載磁碟可能不支援安裝所需的安全重新命名。磁碟上的檔案只用 Windows 版 Obituary 設定壽命、下葬與復活，不要交給兩份帳本。

在 Docker 中，若容器的 overlayfs 無法提供持久的檔案識別碼，`obit mark` 會拒絕登記，檔案留在原處。若要讓到期下葬的檔案能夠復活，請把檔案和 Obituary 資料目錄放在支援持久識別碼的磁碟區或繫結掛載中，例如 ext4 目錄。

### Windows

::: code-group

```powershell [Scoop]
scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket
scoop install obituary
```

```powershell [winget]
# In review, available once approved
winget install woodfishhhh.Obituary
```

```powershell [Install script]
# Install script: download it, read it, then run it
# (installs to %LOCALAPPDATA%\Programs\obituary, adds it to your user PATH, creates a Start menu shortcut)
irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1
.\install.ps1          # installs the latest release; -WhatIf previews, -NoPath leaves PATH alone
```

:::

- 三種方式都會從 GitHub 發行版下載同一個壓縮檔，並在解壓縮前核對 SHA-256；不一致就會停止安裝。
- 程式碼簽署：由 [SignPath.io](https://about.signpath.io/) 提供免費程式碼簽署，憑證由 [SignPath Foundation](https://signpath.org/) 提供。請參閱[程式碼簽署政策](code-signing)和[隱私政策](privacy)。
- Obituary 目前尚未簽署程式碼。如果用瀏覽器下載壓縮檔，Windows 可能顯示「Windows 已保護你的電腦」；請選擇「更多資訊 → 仍要執行」。Scoop、winget 和安裝指令碼通常不會觸發這個提示。
- 小視窗 `obit-gui.exe` 需要 WebView2 執行階段（Windows 11 內建，多數 Windows 10 電腦也已具備），目前只有 x64 版本；ARM64 版本提供命令列和系統匣。
然後檢查安裝：

```powershell
obit doctor              # health check: short summary, full HTML report (--open, --verbose, --json)
```

## 2. 連接你的 agent

```powershell
obit init -g                 # Claude Code, all projects; also the Skill (and desktop integration on Windows)
obit init -g --agent codex   # Codex
```

之後重新啟動 agent。其他 agent、MCP 服務和 Agent Skill 請參閱[接入 agent](./agents)。

在 macOS / Linux 上，Claude Code 使用 `~/.claude`，Codex 使用 `~/.codex`，共用 Skill 使用 `~/.agents/skills`。也會遵守 `CLAUDE_CONFIG_DIR` 和 `CODEX_HOME` 覆寫設定。

桌面版啟用監督後，由常駐托盤在到期時處理，另有獨立工作每小時檢查一次。資料目錄未啟用系統匣監督時，每 5 分鐘檢查一次。

```bash
obit daemon enable --dry-run
obit daemon enable
obit doctor
```

macOS 使用已登入使用者的 launchd；Linux 使用 `systemd --user`。沒有 systemd 的 Linux，可以在使用者 crontab 中每 5 分鐘排程一次 `obitd --once`，並提供 `obitd`、`--home` 資料目錄和 `--config-dir` 設定目錄的絕對路徑。agent hook 也會在工作到期時啟動背景檢查。

## 3. 常用命令

將 `<id>` 替換成 `obit add` 或 `obit bury` 回執中的編號，或 `obit list` / `obit graveyard` 顯示的編號。

```powershell
obit add debug.log 2h        # bury it in 2 hours (alias: mark)
obit list                    # what is left and for how long
obit extend <id> 1d          # one more day
obit bury old-build          # bury right away
obit graveyard               # what is in the graveyard and when it will be purged
obit resurrect <id>          # put it back, never overwriting
obit resurrect --last        # undo the last deletion
```

其他功能請參閱[命令](./commands)。

## 接下來

| 想了解 | 閱讀 |
|---|---|
| 登記、下葬、復活和永久刪除的運作方式，以及墓地的位置 | [運作方式](./how-it-works) |
| 連接 Claude Code、Codex 和其他 agent | [接入 agent](./agents) |
| 會納入哪些內容：agent 登記、內容功能表、可選的自動登記 | [哪些項目會納入](./auto-register) |
| 把 agent 的 `rm -rf` 變成可以復原的下葬 | [刪除護欄](./delete-guard) |
| agent 觸及已下葬檔案時會發生什麼 | [墓地通知](./grave-notices) |
| 為下載和桌面等資料夾設定壽命 | [資料夾策略](./folder-policies) |
| 不使用命令列也能使用 Obituary | [小視窗、系統匣和內容功能表](./desktop) |
| 工作階段中啟動的 Docker 容器 | [Docker 容器](./docker) |
| 所有命令 | [命令](./commands) |
| 移除 Obituary | [解除安裝](./uninstall) |
| 其他內容 | [常見問題](./faq) |
