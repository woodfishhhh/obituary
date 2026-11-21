# 解除安裝

解除安裝分兩步：先讓訃告撤掉它設定過的東西，再刪除程式本身。墓地和帳本預設會保留，你放進墓地的檔案之後仍然可以復活。

```powershell
obit uninstall --dry-run      # 只列出會撤掉什麼，什麼都不改
obit uninstall                # 列出計畫，確認後執行
```

在 Windows 上用安裝腳本安裝的，一條命令就夠了：

```powershell
.\install.ps1 -Uninstall      # 先替你執行 obit uninstall --yes，再刪除程式檔案、它加入的 PATH 項目和開始功能表捷徑
```

## 會撤掉什麼

`obit uninstall` 處理的是使用者層級和目前 Git 專案裡的東西：各家 agent 的 hook、MCP 和 Skill，每 5 分鐘執行一次的排程工作（Windows 的工作排程器、macOS 的 launchd 使用者代理、Linux 的 systemd 使用者計時器），常駐的背景程式和系統匣，以及資料夾策略（策略底下的檔案留在原處）。在 Windows 上還會撤掉登入時自動啟動的設定、檔案總管的右鍵選單和日期標籤。

它只動訃告自己寫下的東西。你改過的項目，或是別的安裝留下的項目，會原樣保留，計畫裡會寫出保留了哪一項、為什麼。在沒有終端機的環境（腳本、agent 的命令列）要加上 `--yes`，否則它會提示需要終端機。

有三件事它不會替你做。程式檔案要依照你的安裝方式去刪（見下文）。在其他專案裡裝過專案層級 hook 的，要先到那些專案裡各執行一次 `obit uninstall`，再刪程式，不然那裡的 hook 會因為找不到 `obit` 而失效。macOS 和 Linux 上系統匣的自動啟動項目也不在清單裡：解除安裝前先到小視窗的 Settings 裡關閉「Start the tray at sign-in」，它會連同訃告自己寫下的自動啟動項目一起撤掉；Windows 上系統匣的自動啟動由 `obit uninstall` 負責，不過 `obit init` 建立的開始功能表和桌面捷徑不在它的清單裡，`obit init --uninstall` 會刪掉它們。

## 墓地、帳本與設定

預設保留的有三樣：墓地裡的檔案、帳本和稽核記錄、`config.toml`。`obit uninstall` 會告訴你墓地裡還有多少東西、放在哪裡；之後重新安裝，它們可以接著使用。如果你用的是資源回收筒模式，到期的項目本來就在系統資源回收筒裡，解除安裝不會動它們，也要從資源回收筒還原，`obit resurrect` 碰不到。

確定都不要了，再加上 `--purge-data`：

```powershell
obit uninstall --purge-data
```

它會先列出墓地裡的每一項，在你確認解除安裝計畫之後再確認一次，然後把這些檔案永久刪除，並刪掉帳本、稽核記錄等訃告自己的檔案和資料目錄。在腳本裡同時寫 `--yes --purge-data` 才會跳過這兩次確認。它只刪訃告認得的檔案：資料目錄裡混有別的檔案，或有它不認得的子資料夾，清除就會停在那裡，資料目錄原樣保留，並告訴你是哪一個。`config.toml` 永遠不在清除範圍內；它在 macOS 上和帳本位於同一個資料夾，所以清除前要先把它移走，不然清除會因此停下。

## 刪除程式

刪除程式的方法取決於你當初怎麼安裝。

| 安裝方式 | 刪除方式 |
| --- | --- |
| Windows 安裝腳本 | `.\install.ps1 -Uninstall` |
| Scoop | 先 `obit uninstall`，再 `scoop uninstall obituary` |
| winget | 先 `obit uninstall`，再 `winget uninstall woodfishhhh.Obituary` |
| macOS 小視窗（DMG） | 先 `obit uninstall` 並關閉「Start the tray at sign-in」，再把「應用程式」裡的 Obituary.app 拖進垃圾桶 |
| Homebrew | 先 `obit uninstall`，再 `brew uninstall --cask obituary` |
| macOS 和 Linux 的 `install.sh` | 先 `obit uninstall`，再刪除 `~/.local/bin` 裡的 `obit` 和 `obitd` |
| Ubuntu 桌面套件（`.deb`） | 先關閉「Start the tray at sign-in」並結束系統匣程式，再 `sudo apt remove obituary-desktop` |

macOS 的 app 和 Ubuntu 的 `.deb` 都自帶 `obit`，但沒有放進 PATH。如果你沒有另外安裝過命令列，解除安裝前請用完整路徑執行它：app 裡的是 `/Applications/Obituary.app/Contents/MacOS/obit`，`.deb` 安裝的是 `/usr/libexec/obituary/obit`。`apt remove` 會刪掉那個資料夾裡的程式，但不會動你的墓地、設定，以及用別的方式安裝的命令列。

## 資料放在哪裡

| | 資料（帳本、墓地、記錄） | 設定（`config.toml`） |
| --- | --- | --- |
| Windows | `%LOCALAPPDATA%\obituary` | `%APPDATA%\obituary` |
| macOS | `~/Library/Application Support/obituary` | 同一個資料夾 |
| Linux | `~/.local/share/obituary` | `~/.config/obituary` |

Linux 上若設定了 `XDG_DATA_HOME` 或 `XDG_CONFIG_HOME`，各自就在它們底下的 `obituary`。環境變數 `OBIT_HOME` 和 `OBIT_CONFIG_DIR` 可以把資料和設定移到別處；`obit config` 會列出目前使用的是哪一個。資料目錄所在的那個磁碟，墓地就是它的 `graveyard` 資料夾；其他磁碟上的墓地，則在該磁碟根目錄的隱藏資料夾 `.obituary` 裡，這樣下葬永遠是同一個磁碟上的改名，不需要複製。
