# 命令參考

把 `<编号>` 換成 `obit add` 或 `obit bury` 執行結果中的編號，也可以從 `obit list` 或 `obit graveyard` 的清單查到。時間單位有 `s`、`m`、`h`、`d` 和 `w`，可以組合使用，例如 `1d12h`。

## 壽命

```powershell
obit add debug.log 2h                       # 2 小时后收走（别名 mark）
obit add output\shots 1d12h                 # 文件夹也可以
obit add report.zip --at "2026-12-01 18:00" # 指定时刻
obit list                                   # 谁还剩多久
obit extend <编号> 1d                       # 续命 1 天
obit spare <编号>                           # 饶它一命：不再跟踪，文件保留（同 cancel）
obit reap                                   # 现在就把已到期的下葬（同 sweep，--dry-run 只预览）
```

## 墓地

```powershell
obit bury old-build                         # 立刻下葬
obit graveyard                              # 墓地里有什么、还剩几天被火化（--here 只看当前目录下的）
obit resurrect <编号>                       # 复活：放回原处；原处已有同名的，改名放回，绝不覆盖
obit resurrect --last                       # 撤销上一次删除：一条 obit bury / obit rm 埋下的几项一起回来
obit purge                                  # 火化保留期已过的（--all 全部，先确认；--dry-run 预览）
obit purge <编号>                           # 立即火化这一座（永久删除，先确认；非终端要加 --yes）
```

在 `resurrect` 和 `purge` 中，單獨的數字（`5`、`#5`）一律當作編號。編號不存在就會報錯，不會改以檔名查找。檔名本身是數字時，請寫成 `./5` 或使用絕對路徑。

## 可復原的刪除指令

```powershell
obit rm -rf build            # 和 rm 一样的常用参数，每个路径都走下葬
obit rmdir empty-dir
obit git-clean -fdx
obit remove-item .\build     # 只接受一个字面路径
```

## 規則與提議

```powershell
obit rules                       # 当前项目生效的规则
obit rules check debug.log out/  # 预览：会登记什么、按哪条规则
obit rules apply debug.log       # 按规则登记
obit trust                       # 信任这个项目的 .obitrules（非交互环境加 --yes）
obit proposals                   # 等你点头的提议，和没问你就登记了的东西
obit approve 3                   # 同意
obit reject 3                    # 拒绝
$dir = obit mktemp 1d            # 会自动下葬的临时目录
```

## 資料夾策略與 Docker

```powershell
obit watch C:\Users\你\Downloads 2w
obit watch --list
obit watch --remove C:\Users\你\Downloads
obit docker add web-1 3d
obit docker list
```

## 整合與背景服務

```powershell
obit init                       # Claude Code 的 hook 和 Skill、右键菜单、开始菜单和桌面快捷方式，托盘随登录启动
obit init -g --agent codex      # Codex
obit init -g --guard-rm         # 删除护栏
obit init -g --mcp              # 加装 MCP 服务
obit init --no-context-menu --no-shortcuts --no-skill   # 不装右键菜单、快捷方式或 Skill
obit init -g --show             # 装了什么
obit daemon enable              # 托盘随登录启动，并装兜底的定时任务
obit daemon status
obit daemon disable
```

## 查詢與檢查

```powershell
obit doctor              # 只读体检（支持 --json）
obit stats               # 悼词：埋了多少、腾出多少空间、复活过几次（同 eulogy）
obit log                 # 生平记录：做过的所有事
obit config              # 查看配置；--create 生成带注释的默认配置
obit config telemetry off # 关闭使用统计并清除本地安装 ID；不影响查更新
obit hook audit          # 设了 OBIT_HOOK_AUDIT=1 时，查看每次 hook 收到了什么、做了什么
obit uninstall           # 撤掉讣告装过的所有东西（先 --dry-run 看看）
```

查詢及回傳結果的指令（`list`、`graveyard`、`proposals`、`stats`、`mark`、`bury`、`resurrect` 等）支援 `--json`，供指令稿和 agent 讀取。`obit rm` 這類包裝原有刪除指令的命令會保留原指令的輸出格式，不支援 `--json`。

## 環境變數

| 變數 | 作用 |
|---|---|
| `OBIT_DISABLED=1` | 暫時停用 hook |
| `OBIT_TELEMETRY=0`、`DO_NOT_TRACK=1` | 停用目前程序的使用統計回報；CI 不會回報 |
| `OBIT_GUARD_RM=0` | 暫時停用刪除護欄 |
| `OBIT_HOOK_AUDIT=1` | 記下每次 hook 收到了什麼、做了什麼 |
| `OBIT_TRUST_PROJECT_RULES=1` | CI 裡信任專案的 `.obitrules` |
| `OBIT_HOME`、`OBIT_CONFIG_DIR`、`OBIT_USER_HOME` | 分別指定資料目錄、設定目錄及使用者家目錄的位置 |



## 刪除相容參數

- git-clean 接受 Git clean 的 -d、-f、-n、-q、-x、-X、-e pattern、--exclude pattern 和 --，也接受 --dry-run、--force、--quiet。
- rm 接受常用的 -r、-R、-f、-d、-v、--recursive、--force、--dir、--verbose、--preserve-root、-- 和其後路徑。
- rmdir 只接受 -v 或 --verbose 與路徑，不接受 rmdir -p。
- Windows remove-item 只接受 -Recurse、-Force、-Path 或 -LiteralPath，而且只能有一個字面路徑；需要萬用字元時在 Bash 使用 obit rm。

<!-- generated-command-reference:start -->
## 參數參考

### `obit`

訃告（Obituary），AI agent 的資源回收筒：為檔案設定壽命，壽命用盡就下葬進墓地，墓地裡的東西可以復活

用法: `obit [command]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | obit 的說明 |
| `--version` | `-v` | `false` | obit 的版本資訊 |

### `obit add`

為路徑設定壽命（對已追蹤的路徑再次登記，會重設它的到期時間）

別名: `add`, `mark`
用法: `obit add <path>... <duration> | add <path>... --at <time> [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--at` | 無 | `""` | 在指定的本機時間到期："2006-01-02 15:04"、"2006-01-02" 或 "15:04" |
| `--force` | 無 | `false` | 允許處理含有 .git 儲存庫的資料夾 |
| `--help` | `-h` | `false` | add 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |
| `--no-daemon` | 無 | `false` | 不啟動背景程式 |
| `--note` | 無 | `""` | 自由文字備註，顯示在清單中 |

範例：

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`

核准提議：登記提議中列出的項目

用法: `obit approve <proposal id>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | approve 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |

### `obit bury`

立刻把路徑移入墓地（可用 obit resurrect 還原）

用法: `obit bury <path>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--force` | 無 | `false` | 允許處理含有 .git 儲存庫的資料夾 |
| `--help` | `-h` | `false` | bury 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |
| `--note` | 無 | `""` | 隨這次下葬一併保存的自由文字備註 |

### `obit cancel`

放過路徑或容器 ID：停止追蹤，物件原樣保留

別名: `cancel`, `spare`
用法: `obit cancel <id|path>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | cancel 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |

### `obit completion`

為指定的 shell 產生 obit 的自動完成指令碼。

用法: `obit completion [command]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | completion 的說明 |

### `obit completion bash`

為 bash shell 產生自動完成指令碼。

用法: `obit completion bash`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | bash 的說明 |
| `--no-descriptions` | 無 | `false` | 關閉自動完成項目的說明 |

### `obit completion fish`

為 fish shell 產生自動完成指令碼。

用法: `obit completion fish [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | fish 的說明 |
| `--no-descriptions` | 無 | `false` | 關閉自動完成項目的說明 |

### `obit completion powershell`

為 powershell 產生自動完成指令碼。

用法: `obit completion powershell [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | powershell 的說明 |
| `--no-descriptions` | 無 | `false` | 關閉自動完成項目的說明 |

### `obit completion zsh`

為 zsh shell 產生自動完成指令碼。

用法: `obit completion zsh [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | zsh 的說明 |
| `--no-descriptions` | 無 | `false` | 關閉自動完成項目的說明 |

### `obit config`

顯示設定檔和目前生效的設定（--create 會寫入預設值）

用法: `obit config [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--create` | 無 | `false` | 依預設值寫出設定檔 |
| `--help` | `-h` | `false` | config 的說明 |
| `--language` | 無 | `""` | 設定語言：auto、en 或 zh-CN |

### `obit config telemetry`

使用統計：隨機產生的安裝 ID、版本、作業系統、架構、介面語言和安裝管道，用來統計安裝數量和各版本的使用情況。不會傳送檔案、路徑或指令，也不會儲存 IP 位址和 User-Agent。可用 obit config telemetry off 或 OBIT_TELEMETRY=0 關閉。

用法: `obit config telemetry on|off [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | telemetry 的說明 |

### `obit daemon`

控制背景程式（obitd）

用法: `obit daemon [command]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | daemon 的說明 |

### `obit daemon disable`

移除排程工作和常駐程式

用法: `obit daemon disable [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--dry-run` | 無 | `false` | 只列印變更內容 |
| `--help` | `-h` | `false` | disable 的說明 |

### `obit daemon enable`

啟用每 5 分鐘執行一次的使用者層級排程（Windows 上同時啟用隨登入啟動的系統匣）

用法: `obit daemon enable [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--dry-run` | 無 | `false` | 只列印變更內容 |
| `--help` | `-h` | `false` | enable 的說明 |
| `--resident` | 無 | `false` | 只啟用系統匣，不更動備援的排程工作 |

### `obit daemon start`

立即啟動背景程式

用法: `obit daemon start [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | start 的說明 |

### `obit daemon status`

顯示背景程式的狀態和下一次到期時間

用法: `obit daemon status [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | status 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |

### `obit daemon stop`

停止背景程式

用法: `obit daemon stop [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | stop 的說明 |

### `obit docker`

依不可變的完整 ID 和建立時間追蹤 Docker 容器。到期時停止容器並保留它的可寫層，復活時啟動它。超過 [graveyard] retention_days 之後，永久刪除會移除已停止的容器，但不會移除磁碟區；資源回收筒模式下也一樣。映像檔、磁碟區和網路都會保留。

用法: `obit docker [command]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | docker 的說明 |

### `obit docker add`

登記一個 Docker 容器；到期時停止它

用法: `obit docker add <container name|ID> <lifespan> [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | add 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |

### `obit docker list`

列出已登記的 Docker 容器

用法: `obit docker list [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | list 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |

### `obit doctor`

檢查安裝、整合和資料，不做任何變更

用法: `obit doctor [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | doctor 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |
| `--verbose` | 無 | `false` | 包含 agent 的功能細節 |

### `obit extend`

延長壽命：把到期時間往後推（已經到期的從現在起算）

用法: `obit extend <id|path>... <duration> [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | extend 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |

範例：

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`

把 git clean 試執行所選出的路徑下葬

用法: `obit git-clean [options] [--] [pathspec]... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | git-clean 的說明 |

### `obit graveyard`

列出墓地裡仍可復活的項目

別名: `graveyard`, `graves`
用法: `obit graveyard [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | graveyard 的說明 |
| `--here` | 無 | `false` | 只顯示原路徑位於目前目錄及其所有子目錄中的項目 |
| `--json` | 無 | `false` | 輸出 JSON |

### `obit help`

顯示本程式中任一指令的說明。

用法: `obit help [command] [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help 的說明 |

### `obit hook`

agent hook 的進入點（由 agent 呼叫，不是給人用的）

用法: `obit hook [command]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | hook 的說明 |

### `obit hook antigravity`

處理來自 stdin 的一個 antigravity hook 事件；發生錯誤時放行

用法: `obit hook antigravity [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | antigravity 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook audit`

顯示 hook 和刪除護欄的稽核記錄

用法: `obit hook audit [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | audit 的說明 |
| `--lines` | `-n` | `40` | 每個來源顯示的行數 |

### `obit hook claude`

處理來自 stdin 的一個 claude hook 事件；發生錯誤時放行

用法: `obit hook claude [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | claude 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook cline`

處理來自 stdin 的一個 cline hook 事件；發生錯誤時放行

用法: `obit hook cline [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | cline 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook codebuddy`

處理來自 stdin 的一個 codebuddy hook 事件；發生錯誤時放行

用法: `obit hook codebuddy [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | codebuddy 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook codebuddy-ide`

處理來自 stdin 的一個 codebuddy-ide hook 事件；發生錯誤時放行

用法: `obit hook codebuddy-ide [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | codebuddy-ide 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook codex`

處理來自 stdin 的一個 codex hook 事件；發生錯誤時放行

用法: `obit hook codex [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | codex 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook copilot`

處理來自 stdin 的一個 copilot hook 事件；發生錯誤時放行

用法: `obit hook copilot [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | copilot 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook cursor`

處理來自 stdin 的一個 cursor hook 事件；發生錯誤時放行

用法: `obit hook cursor [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | cursor 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook deepseek-harness`

處理來自 stdin 的一個 deepseek-harness hook 事件；發生錯誤時放行

用法: `obit hook deepseek-harness [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | deepseek-harness 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook droid`

處理來自 stdin 的一個 droid hook 事件；發生錯誤時放行

用法: `obit hook droid [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | droid 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook gemini`

處理來自 stdin 的一個 gemini hook 事件；發生錯誤時放行

用法: `obit hook gemini [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | gemini 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook hermes`

處理來自 stdin 的一個 hermes hook 事件；發生錯誤時放行

用法: `obit hook hermes [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | hermes 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook kilocode`

處理來自 stdin 的一個 kilocode hook 事件；發生錯誤時放行

用法: `obit hook kilocode [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | kilocode 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook kimi`

處理來自 stdin 的一個 kimi hook 事件；發生錯誤時放行

用法: `obit hook kimi [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | kimi 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook lingma`

處理來自 stdin 的一個 lingma hook 事件；發生錯誤時放行

用法: `obit hook lingma [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | lingma 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook minimax-code`

處理來自 stdin 的一個 minimax-code hook 事件；發生錯誤時放行

用法: `obit hook minimax-code [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | minimax-code 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook omp`

處理來自 stdin 的一個 omp hook 事件；發生錯誤時放行

用法: `obit hook omp [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | omp 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook openclaw`

處理來自 stdin 的一個 openclaw hook 事件；發生錯誤時放行

用法: `obit hook openclaw [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | openclaw 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook opencode`

處理來自 stdin 的一個 opencode hook 事件；發生錯誤時放行

用法: `obit hook opencode [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | opencode 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook pi`

處理來自 stdin 的一個 pi hook 事件；發生錯誤時放行

用法: `obit hook pi [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | pi 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook qoder`

處理來自 stdin 的一個 qoder hook 事件；發生錯誤時放行

用法: `obit hook qoder [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | qoder 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook qoder-cn`

處理來自 stdin 的一個 qoder-cn hook 事件；發生錯誤時放行

用法: `obit hook qoder-cn [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | qoder-cn 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook trae`

處理來自 stdin 的一個 trae hook 事件；發生錯誤時放行

用法: `obit hook trae [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | trae 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook trae-cn`

處理來自 stdin 的一個 trae-cn hook 事件；發生錯誤時放行

用法: `obit hook trae-cn [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | trae-cn 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook vibe`

處理來自 stdin 的一個 vibe hook 事件；發生錯誤時放行

用法: `obit hook vibe [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | vibe 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook zcode`

處理來自 stdin 的一個 zcode hook 事件；發生錯誤時放行

用法: `obit hook zcode [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | payload 中沒有事件名稱時使用的宿主事件名稱 |
| `--help` | `-h` | `false` | zcode 的說明 |
| `--notice-only` | 無 | `false` | 只傳遞提示性的上下文，不啟用刪除護欄 |

### `obit hook finish`

計算工作階段留下的檔案（由 hook 在背景啟動）

用法: `obit hook finish [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | finish 的說明 |
| `--project` | 無 | `""` | 專案根目錄 |
| `--session` | 無 | `""` | agent 工作階段 ID |

### `obit init`

為 agent 安裝（或用 --uninstall 解除安裝）訃告的 hook 和說明

用法: `obit init [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--agent` | 無 | `[claude]` | 要整合的 agent（可重複指定，也可用逗號分隔；預設為 claude） |
| `--context-menu` | 無 | `false` | 安裝傳統的檔案總管右鍵選單（預設啟用） |
| `--dry-run` | 無 | `false` | 只列印變更內容 |
| `--format` | 無 | `"text"` | --show 的輸出格式：text 或 markdown（agent 支援範圍） |
| `--global` | `-g` | `false` | 對目前使用者生效，而不是只對這個專案生效 |
| `--guard-rm` | 無 | `false` | 拒絕受支援的刪除指令，並提供可以復活的 obit 替代指令 |
| `--help` | `-h` | `false` | init 的說明 |
| `--mcp` | 無 | `false` | 註冊訃告的 MCP stdio 伺服器 |
| `--no-context-menu` | 無 | `false` | 不安裝檔案總管右鍵選單 |
| `--no-shortcuts` | 無 | `false` | 不建立開始功能表和桌面捷徑 |
| `--no-skill` | 無 | `false` | 不安裝訃告的 Agent Skill |
| `--show` | 無 | `false` | 顯示已安裝的項目 |
| `--skill` | 無 | `false` | 安裝訃告的 Agent Skill（預設啟用） |
| `--uninstall` | 無 | `false` | 移除訃告加入的項目 |
| `--yes` | 無 | `false` | 不詢問，直接套用 |

範例：

```text
obit init -g  # Claude Code，對所有專案生效（~/.claude）
obit init  # Claude Code，只對這個專案生效（.claude/）
obit init -g --agent codex  # Codex（~/.codex/hooks.json + AGENTS.md）
obit init -g --uninstall  # 只移除訃告加入的項目
obit init -g --show  # 已安裝的項目
obit init --show --format markdown > docs/agents.md
```

### `obit list`

顯示已追蹤的路徑和 Docker 容器，最先到期的排在最前面

別名: `list`, `ls`
用法: `obit list [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | list 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |

### `obit log`

顯示訃告所做的一切的稽核日誌

用法: `obit log [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | log 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |
| `--lines` | `-n` | `20` | 顯示的事件筆數 |

### `obit mcp`

透過 MCP stdio 提供訃告的工具

用法: `obit mcp [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | mcp 的說明 |

### `obit mktemp`

建立一個會自行下葬的暫存目錄，並列印它的路徑

用法: `obit mktemp <lifespan> [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | mktemp 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |
| `--name` | 無 | `"tmp"` | 目錄名稱的前綴 |
| `--session` | 無 | `""` | 目錄所屬的 agent 工作階段 |

範例：

```text
dir=$(obit mktemp 1d)
obit mktemp 2h --name screenshots
```

### `obit proposals`

等你核准的登記，以及沒有詢問你就已完成的登記

用法: `obit proposals [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | proposals 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |

### `obit purge`

永久刪除選定的或保留期已過的已下葬檔案。--all 和明確指定的編號或路徑會略過保留期，因此會先詢問；不在終端機中執行時必須加上 --yes。Docker 容器必須等 [graveyard] retention_days 屆滿才能永久刪除，使用 --all 或明確指定 ID 也一樣；執行中的容器視為已復活。移除 Docker 容器時會保留磁碟區，也從不強制移除。資源回收筒模式下，容器的處理方式完全相同。

別名: `purge`, `cremate`
用法: `obit purge [id|original path]... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--all` | 無 | `false` | 永久刪除所有已下葬的項目，不只是已過保留期的 |
| `--dry-run` | 無 | `false` | 只顯示會發生什麼 |
| `--help` | `-h` | `false` | purge 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |
| `--yes` | 無 | `false` | 不詢問，直接永久刪除 --all 或所選的已下葬項目（agent：請先詢問使用者） |

### `obit reject`

拒絕提議

用法: `obit reject <proposal id>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | reject 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |

### `obit remove-item`

使用有限的一部分 PowerShell 參數，把檔案系統項目下葬。

用法: `obit remove-item [-Recurse] [-Force] [-Path|-LiteralPath] <path> [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | remove-item 的說明 |

### `obit resurrect`

還原檔案，不覆寫既有內容；依 ID 啟動已下葬的 Docker 容器

別名: `resurrect`, `raise`
用法: `obit resurrect <id|original path>... | --last [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | resurrect 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |
| `--last` | 無 | `false` | 還原目前工作階段或 Git 專案中最近一批刪除的項目 |

### `obit rm`

立刻把路徑移入墓地（可用 obit resurrect 還原）

用法: `obit rm [options] <path>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | rm 的說明 |

### `obit rmdir`

立刻把路徑移入墓地（可用 obit resurrect 還原）

用法: `obit rmdir [options] <path>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | rmdir 的說明 |

### `obit rules`

顯示這裡生效的規則：內建規則，以及信任之後專案的 .obitrules

用法: `obit rules [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | rules 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |

### `obit rules apply`

依規則登記路徑（被 Git 追蹤的檔案永遠不會登記）

用法: `obit rules apply <path>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | apply 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |
| `--session` | 無 | `""` | 這些路徑來自的 agent 工作階段 |

### `obit rules check`

顯示規則會為這些路徑登記什麼（不做任何變更）

用法: `obit rules check <path>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | check 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |
| `--session` | 無 | `""` | 這些路徑來自的 agent 工作階段 |

### `obit stats`

訃告做過的事：追蹤了多少、下葬了多少、釋出多少空間、復活了幾次

別名: `stats`, `eulogy`
用法: `obit stats [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | stats 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |

### `obit sweep`

立刻收割：不等背景程式，把所有到期的項目都下葬

別名: `sweep`, `reap`
用法: `obit sweep [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--dry-run` | 無 | `false` | 只顯示會發生什麼 |
| `--help` | `-h` | `false` | sweep 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |

### `obit trust`

檢視並信任專案的 .obitrules（修改後需要重新檢視）

用法: `obit trust [rules file] [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | trust 的說明 |
| `--yes` | 無 | `false` | 不詢問，直接信任（只在檢視過規則之後使用） |

### `obit uninstall`

移除訃告的整合和背景服務；預設保留資料

用法: `obit uninstall [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--dry-run` | 無 | `false` | 只列印解除安裝計畫，不變更檔案和服務 |
| `--help` | `-h` | `false` | uninstall 的說明 |
| `--purge-data` | 無 | `false` | 永久刪除列出的墳墓，並移除訃告自己的資料；不認識的檔案保留 |
| `--yes` | 無 | `false` | 確認計畫（只有同時給了 --purge-data，才會一併確認清除資料） |

### `obit untrust`

不再套用專案的 .obitrules

用法: `obit untrust [rules file] [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | untrust 的說明 |

### `obit update`

檢查或安裝穩定版更新

用法: `obit update [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--apply` | 無 | `false` | 安裝已快取的較新穩定版 |
| `--check` | 無 | `false` | 立即檢查 GitHub，自動檢查被關閉時也會檢查 |
| `--help` | `-h` | `false` | update 的說明 |
| `--relaunch-gui` | 無 | `false` | 更新後重新開啟小視窗（Windows） |
| `--wait-pid` | 無 | `0` | 等待該程序結束後再更新（Windows） |

### `obit watch`

為資料夾中的每個第一層子項目各自設定壽命

用法: `obit watch <directory> <lifespan> | watch --list | watch --remove <directory> [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--age` | 無 | `"first-seen"` | 計齡基準：first-seen 或 modified（需要核准） |
| `--clear-exclusion` | 無 | `0` | 明確清除這個排除項 ID（須與 --policy 一起使用） |
| `--help` | `-h` | `false` | watch 的說明 |
| `--json` | 無 | `false` | 輸出 JSON |
| `--list` | 無 | `false` | 列出資料夾策略 |
| `--no-daemon` | 無 | `false` | 不啟動背景程式 |
| `--policy` | 無 | `0` | 要清除排除項的策略 ID |
| `--remove` | 無 | `""` | 移除策略，並取消它登記的壽命 |
| `--remove-id` | 無 | `0` | 依 ID 移除資料夾策略，路徑已不存在的也可以 |

<!-- generated-command-reference:end -->
