# 設定參考

讖告會從設定目錄讀取 `config.toml`。所有鍵都可省略；省略時採用下表的預設值。`obit config` 會顯示檔案位置與目前生效的值。

## 設定鍵

| 鍵 | 預設值 | 可選值或格式 | 作用 |
| --- | --- | --- | --- |
| `codex_windows_shell` | 未設定 | `bash`、`powershell` | 宣告 Windows 上 Codex 使用的 shell；未設定時不改寫含糊的刪除命令。 |
| `agent_shells.<agent>` | 未設定 | `bash`、`powershell` | 為指定 hook agent 宣告 Windows shell。 |
| `graveyard.mode` | `graveyard` | `graveyard`、`recycle` | 到期後移入墓地，或交給系統資源回收筒。 |
| `graveyard.retention_days` | `7` | `1` 到 `3650` 的整數 | 墓地項目可復原的天數。 |
| `watch.stable_window` | `10m` | Go 時長格式，例如 `30s`、`10m`、`1h` | 資料夾政策等待檔案穩定後才移動的時間，必須為正數。 |
| `explorer.enabled` | `true` | `true`、`false` | 是否寫入檔案總管日期標籤。 |
| `explorer.everywhere` | `false` | `true`、`false` | 是否在所有支援的資料夾顯示日期。 |
| `explorer.in_git` | `false` | `true`、`false` | 是否在 Git 工作區內顯示日期。 |
| `ui.language` | `auto` | `auto`、`en`、`zh-CN` | CLI、視窗與托盤使用的語言。 |
| `ui.tray` | `true` | `true`、`false` | 是否啟用托盤或選單列入口。 |
| `ui.taskbar` | `true` | `true`、`false` | Windows 視窗是否顯示獨立工作列圖示。 |
| `hooks.session_summary` | `false` | `true`、`false` | 是否在工作階段開始時顯示摘要。 |
| `hooks.session_cleanup` | `true` | `true`、`false` | 是否整理本次新增且尚未分類的檔案。 |
| `proposals.mode` | `off` | `off`、`tiered`、`auto`、`ask` | 自動登記等待核准的範圍。 |
| `update.check` | `true` | `true`、`false` | 是否自動檢查正式版本更新。 |
| `telemetry.enabled` | `true` | `true`、`false` | 是否允許使用統計上報，與更新檢查獨立。 |

`agent_shells` 是依 agent 名稱展開的表，例如 `[agent_shells] codex = "powershell"`。`codex_windows_shell` 是 Codex 的舊寫法；兩者同時設定且值不同會報錯。

## 格式與錯誤

時長使用 Go 格式：數字後接 `ns`、`us`、`ms`、`s`、`m` 或 `h`，可組合成 `1h30m`。保留天數必須介於 `1` 到 `3650`。`graveyard.mode`、`ui.language` 和 `proposals.mode` 只能使用表中的值。

未知鍵不會靜默忽略。讖告會回報設定檔路徑、行號和完整鍵名，拼字接近時也會提供建議。TOML 語法錯誤同樣會使 `obit config` 失敗。

## 常用操作

```powershell
obit config
obit config --create
obit config --language en
obit config telemetry off
obit config telemetry on
```

`--create` 只在設定檔不存在時寫入帶註解的預設範本。`--language` 修改介面語言；`auto`、`en` 和 `zh-CN` 是可接受的值。`telemetry on|off` 只切換 `telemetry.enabled`，不會改變 `[update] check`。
