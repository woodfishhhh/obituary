# 解除安裝

```powershell
obit uninstall --dry-run      # 先看会撤掉什么
obit uninstall                # 撤掉讣告装过的所有东西，确认后执行
.\install.ps1 -Uninstall      # 用安装脚本装的：再删掉程序文件、PATH 和开始菜单快捷方式
```

`obit uninstall` 只會移除訃告自己加上的項目：使用者層級和目前 Git 專案的 agent hook、MCP 與 Skill，以及登入時自動啟動的設定、排程工作、執行中的背景程式、右鍵選單、檔案總管中的日期標籤和資料夾策略。

## 預設保留墓地和帳本

預設會保留墓地和帳本，並顯示墓地的位置與存放量。確定要一併刪除時才加上 `--purge-data`；訃告會先列出墓地內容，再請你確認一次。

## 其他專案的 hook

如果其他專案也安裝了專案層級的 hook，請先在那些專案裡分別執行 `obit uninstall`，再移除程式。否則這些 hook 會因找不到 `obit` 而失效。

## Scoop 和 winget

先執行 `obit uninstall`，再用各自的解除安裝命令。
## 解除安裝範圍與資料位置

`obit uninstall --dry-run` 會列出目前使用者與目前 Git 專案可移除的項目。真正執行會移除整合、hook、MCP、Skill、托盤自啟與排程；其他專案的專案級設定要到各專案分別解除安裝。

| 位置或元件 | Windows | macOS | Linux |
| --- | --- | --- | --- |
| 程式 | 安裝目錄中的 `obit`、`obitd`、可選 `obit-gui` | `/Applications/Obituary.app` 或 CLI 目錄 | CLI 目錄；桌面版為 `.deb` |
| hook、MCP、Skill | 使用者或專案範圍的 agent 設定 | agent 設定範圍 | agent 設定範圍 |
| 背景排程 | 工作排程、常駐程式與托盤 | 使用者 LaunchAgent 與選單列托盤 | systemd 使用者計時器；桌面托盤可有 XDG 自啟 |
| 專案設定 | `.claude/`、`.codex/` 等由讖告加入的部分 | 專案設定 | 專案設定 |

預設資料位於 Windows `%LOCALAPPDATA%\obituary`、設定為 `%APPDATA%\obituary\config.toml`；macOS 的 `~/Library/Application Support/obituary`；Linux 的 `$XDG_DATA_HOME/obituary`，未設定時為 `~/.local/share/obituary`。`OBIT_HOME`、`OBIT_CONFIG_DIR`、`OBIT_USER_HOME` 可分別重定向資料、設定與使用者主目錄；同卷可能還有 `.obituary` 墓地。

墓地、賬本、稽核記錄和設定預設保留。`--purge-data` 只清理讖告擁有且已核實的墓地資料，不刪未知檔案，也不是刪除使用者主目錄的授權。recycle 模式必須從系統資源回收筒復原，不能使用 `resurrect`。
