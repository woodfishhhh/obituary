# 設定參考

訃告的設定都寫在設定目錄裡的 `config.toml`。這個檔案不是必要的：沒有它，或是裡面只寫了幾項，其餘的都使用預設值。`obit config` 會告訴你檔案在哪裡、有沒有建立，以及每一項目前實際採用的值；`obit config --create` 會寫出一份附有註解、所有值都是預設值的檔案，從它開始修改最省事。

設定目錄在 Windows 上是 `%APPDATA%\obituary`，在 macOS 上是 `~/Library/Application Support/obituary`，在 Linux 上是 `~/.config/obituary`（若設定了 `XDG_CONFIG_HOME`，就是它底下的 `obituary`）。想換位置的話，設定環境變數 `OBIT_CONFIG_DIR`。

| 鍵 | 預設值 | 可用的值 |
| --- | --- | --- |
| `graveyard.mode` | `graveyard` | `graveyard`、`recycle` |
| `graveyard.retention_days` | `7` | 1 到 3650 的整數 |
| `proposals.mode` | `off` | `off`、`tiered`、`auto`、`ask` |
| `hooks.session_cleanup` | `true` | `true`、`false` |
| `hooks.session_summary` | `false` | `true`、`false` |
| `watch.stable_window` | `10m` | 正的時間長度，例如 `30s`、`10m`、`1h30m` |
| `watch.file_expiry` (macOS/Linux) | `quiet` | `quiet`, `propose` |
| `explorer.enabled` | `true` | `true`、`false` |
| `explorer.everywhere` | `false` | `true`、`false` |
| `explorer.in_git` | `false` | `true`、`false` |
| `ui.language` | `auto` | `auto`、`en`、`zh-CN` |
| `ui.tray` | `true` | `true`、`false` |
| `ui.taskbar` | `true` | `true`、`false` |
| `update.check` | `true` | `true`、`false` |
| `telemetry.enabled` | `true` | `true`、`false` |
| `agent_shells` | 未設定 | 資料表，鍵是 agent 名稱，值是 `bash` 或 `powershell` |
| `codex_windows_shell` | 未設定 | `bash`、`powershell` |

## 到期之後去哪裡

預設情況下，壽命用完的項目會進訃告自己的墓地，在 `graveyard.retention_days` 天內（預設 7 天）可以用 `obit resurrect` 放回原處，超過這個期限才會被永久刪除。天數可以在 1 到 3650 之間調整。

Windows 和 Linux 可將 `graveyard.mode` 設為 `recycle`。請從系統資源回收筒還原，不能用 `obit resurrect`。macOS 僅支援墓地。

## 自動登記與 agent 工作階段

`proposals.mode` 決定訃告會不會自己替檔案登記壽命。預設的 `off` 什麼都不自動做，只有 agent 或你透過命令、右鍵選單、資料夾策略登記的項目才會進來。`tiered` 讓 agent 在這次工作階段中建立的檔案、工作階段啟動的容器直接進來，事後再告訴你，不想要的話可以用 `obit spare` 撤回；其餘的先放進 `obit proposals`，等你批准。`auto` 會把自動登記找到的一切直接登記，事後告訴你。`ask` 則是每一項都先問你。無論哪一種模式，Git 已經追蹤的檔案，以及相依套件、建置產物和快取（`node_modules`、`dist`、`__pycache__` 之類）都不會被自動登記。詳細規則見[哪些項目會納入訃告](./auto-register)。

`hooks.session_cleanup` 預設開啟。每個工作階段快結束時，agent 會被告知它這次建立、還沒有壽命、Git 也沒有追蹤的檔案，並被請求替暫存檔登記壽命（`obit mark`）或直接下葬，交付成果則保持不動。`hooks.session_summary` 預設關閉；開啟後，agent 在工作階段開始時還會聽到專案裡 24 小時內到期的項目和最近下葬的項目。預設關閉是為了節省 token：agent 碰到已下葬的路徑時，本來就會被告知。

## 資料夾策略與檔案總管裡的日期

`watch.stable_window` 是資料夾策略的耐心：新檔案出現後，必須連續這麼久沒有變動才會被移動，免得把還在下載的檔案搬走。它使用 Go 的時間長度格式，數字後面接 `s`、`m` 或 `h`，可以連著寫，例如 `1h30m`，而且必須大於零。

`explorer.*` 只對 Windows 有意義。訃告會讓檔案總管在檔名旁顯示下葬日期，實際的檔名不會改變。預設只在「下載」、桌面（含子資料夾）和資料夾策略管理的位置顯示；開啟 `explorer.everywhere` 後，其他被追蹤的位置也會顯示；Git 工作區預設不顯示，要顯示就把 `explorer.in_git` 設為 `true`；`explorer.enabled` 設為 `false` 則完全不寫入。見[資料夾策略](./folder-policies)。

## 介面

`ui.language` 管理命令列、小視窗、系統匣和右鍵選單的語言，目前只有 `en` 和 `zh-CN`；`auto` 會依序參考環境變數 `OBIT_LANG`、`LC_ALL`、`LC_MESSAGES`、`LANG`，最後是系統的介面語言。`ui.tray` 控制系統匣圖示（macOS 上是選單列圖示）的顯示，隱藏之後背景清理和更新檢查照常進行。`ui.taskbar` 只對 Windows 有效，決定小視窗開啟時是否在工作列上占一個按鈕。

## 更新檢查與使用統計

`update.check` 開啟時，訃告每天最多讀取一次 GitHub 上公開的發行資訊，不會傳送任何使用資料；關閉之後仍然可以隨時用 `obit update --check` 手動檢查。`telemetry.enabled` 控制匿名的使用統計，它和更新檢查是兩個獨立的開關：`obit config telemetry off` 只會關閉統計，並清除這台電腦上的安裝 ID 和尚未送出的資料。詳情見[隱私政策](./privacy)。

## Windows 上 agent 使用哪種 shell

刪除護欄必須讀懂 agent 即將執行的刪除命令，才能給出對等的 `obit` 命令。Windows 上的 agent 可能使用 bash，也可能使用 PowerShell，訃告自己分辨不出來，所以由你來宣告：這個 agent 就在這台 Windows 電腦上，用這種 shell 執行命令。沒有宣告時，意思不明確的命令一律放行，不攔截。

```toml
[agent_shells]
codex = "powershell"
```

`agent_shells` 裡的鍵是 agent 的名稱，目前會讀取它的有 `codex`、`codebuddy`、`codebuddy-ide` 和 `minimax-code`。`codex_windows_shell` 是只給 Codex 用的舊寫法；它和 `agent_shells.codex` 同時出現而且值不一樣時，設定會被拒絕。

## 寫錯了會怎麼樣

訃告不會默默忽略寫錯的鍵。如果忽略，`retention = 30` 會讓 7 天的保留期照舊生效，而你以為已經改成 30 了。所以遇到不認得的鍵，`obit config` 會回報檔案路徑、行號和完整的鍵名，拼法接近時還會給出建議：

```text
obit: config file has errors: …\config.toml:2: unknown key "graveyard.retention" (did you mean "graveyard.retention_days"?)
```

值超出範圍時同樣會指出是哪一項、允許什麼，例如 `graveyard.retention_days must be between 1 and 3650`。TOML 語法錯誤也一樣會讓 `obit config` 失敗。修正檔案後再執行一次，不再報錯就是好了。

## 常用命令

```powershell
obit config                  # 檔案位置、資料目錄，以及目前生效的值
obit config --create         # 寫出附有註解的預設檔案
obit config --language en    # 把介面語言寫進設定：auto、en 或 zh-CN
obit config telemetry off    # 關閉使用統計（on 則重新開啟）
```

`--create` 不會覆蓋既有的檔案，檔案已經存在時它會回報錯誤並結束。`--language` 在檔案不存在時，會連同預設內容一起建立。
