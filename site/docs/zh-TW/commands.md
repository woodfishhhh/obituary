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

以下表格由目前原始碼建置的 `obit --help` 輸出產生。

### `obit add`
別名: `add`, `mark`
用法: `obit add <path>... <duration> \| add <path>... --at <time> [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--at` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--force` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |
| `--no-daemon` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--note` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

範例：

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`
用法: `obit approve <proposal id>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

### `obit bury`
用法: `obit bury <path>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--force` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |
| `--note` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit cancel`
別名: `cancel`, `spare`
用法: `obit cancel <id\|path>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

### `obit completion`
用法: `obit completion [command]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit completion bash`
用法: `obit completion bash`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--no-descriptions` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit completion fish`
用法: `obit completion fish [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--no-descriptions` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit completion powershell`
用法: `obit completion powershell [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--no-descriptions` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit completion zsh`
用法: `obit completion zsh [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--no-descriptions` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit config`
用法: `obit config [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--create` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--language` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit config telemetry`
用法: `obit config telemetry on\|off [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit daemon`
用法: `obit daemon [command]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit daemon disable`
用法: `obit daemon disable [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--dry-run` | 無 | `false` | 只顯示將執行的操作，不修改檔案或服務。 |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit daemon enable`
用法: `obit daemon enable [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--dry-run` | 無 | `false` | 只顯示將執行的操作，不修改檔案或服務。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--resident` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit daemon start`
用法: `obit daemon start [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit daemon status`
用法: `obit daemon status [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

### `obit daemon stop`
用法: `obit daemon stop [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit docker`
用法: `obit docker [command]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit docker add`
用法: `obit docker add <container name\|ID> <lifespan> [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

### `obit docker list`
用法: `obit docker list [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

### `obit doctor`
用法: `obit doctor [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |
| `--verbose` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit extend`
用法: `obit extend <id\|path>... <duration> [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

範例：

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`
用法: `obit git-clean [options] [--] [pathspec]... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit graveyard`
別名: `graveyard`, `graves`
用法: `obit graveyard [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--here` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

### `obit help`
用法: `obit help [command] [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit hook`
用法: `obit hook [command]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit hook antigravity`
用法: `obit hook antigravity [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook audit`
用法: `obit hook audit [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--lines` | `-n` | `40` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook claude`
用法: `obit hook claude [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook cline`
用法: `obit hook cline [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook codebuddy`
用法: `obit hook codebuddy [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook codebuddy-ide`
用法: `obit hook codebuddy-ide [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook codex`
用法: `obit hook codex [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook copilot`
用法: `obit hook copilot [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook cursor`
用法: `obit hook cursor [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook droid`
用法: `obit hook droid [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook gemini`
用法: `obit hook gemini [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook hermes`
用法: `obit hook hermes [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook kilocode`
用法: `obit hook kilocode [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook kimi`
用法: `obit hook kimi [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook lingma`
用法: `obit hook lingma [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook minimax-code`
用法: `obit hook minimax-code [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook omp`
用法: `obit hook omp [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook openclaw`
用法: `obit hook openclaw [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook opencode`
用法: `obit hook opencode [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook pi`
用法: `obit hook pi [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook qoder`
用法: `obit hook qoder [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook qoder-cn`
用法: `obit hook qoder-cn [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook trae`
用法: `obit hook trae [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook trae-cn`
用法: `obit hook trae-cn [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook vibe`
用法: `obit hook vibe [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit hook zcode`
用法: `obit hook zcode [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--event` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--notice-only` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit init`
用法: `obit init [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--agent` | 無 | `[claude]` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--context-menu` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--dry-run` | 無 | `false` | 只顯示將執行的操作，不修改檔案或服務。 |
| `--format` | 無 | `"text"` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--global` | `-g` | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--guard-rm` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--mcp` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--no-context-menu` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--no-shortcuts` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--no-skill` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--show` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--skill` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--uninstall` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--yes` | 無 | `false` | 確認執行，不再詢問。 |

範例：

```text
obit init -g                 # Claude Code, for every project (~/.claude)
obit init                    # Claude Code, this project only (.claude/)
obit init -g --agent codex   # Codex (~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall     # remove only what Obituary added
obit init -g --show          # what is installed
obit init --show --format markdown > docs/agents.md
```

### `obit list`
別名: `list`, `ls`
用法: `obit list [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

### `obit log`
用法: `obit log [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |
| `--lines` | `-n` | `20` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit mcp`
用法: `obit mcp [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit mktemp`
用法: `obit mktemp <lifespan> [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |
| `--name` | 無 | `"tmp"` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--session` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

範例：

```text
obit mktemp 2h --name screenshots
```

### `obit proposals`
用法: `obit proposals [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

### `obit purge`
別名: `purge`, `cremate`
用法: `obit purge [id\|original path]... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--all` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--dry-run` | 無 | `false` | 只顯示將執行的操作，不修改檔案或服務。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |
| `--yes` | 無 | `false` | 確認執行，不再詢問。 |

### `obit reject`
用法: `obit reject <proposal id>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

### `obit remove-item`
用法: `obit remove-item [-Recurse] [-Force] [-Path\|-LiteralPath] <path> [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit resurrect`
別名: `resurrect`, `raise`
用法: `obit resurrect <id\|original path>... \| --last [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |
| `--last` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit rm`
用法: `obit rm [options] <path>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit rmdir`
用法: `obit rmdir [options] <path>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit rules`
用法: `obit rules [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

### `obit rules apply`
用法: `obit rules apply <path>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |
| `--session` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit rules check`
用法: `obit rules check <path>... [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |
| `--session` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit stats`
別名: `stats`, `eulogy`
用法: `obit stats [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

### `obit sweep`
別名: `sweep`, `reap`
用法: `obit sweep [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--dry-run` | 無 | `false` | 只顯示將執行的操作，不修改檔案或服務。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |

### `obit trust`
用法: `obit trust [rules file] [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--yes` | 無 | `false` | 確認執行，不再詢問。 |

### `obit uninstall`
用法: `obit uninstall [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--dry-run` | 無 | `false` | 只顯示將執行的操作，不修改檔案或服務。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--purge-data` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--yes` | 無 | `false` | 確認執行，不再詢問。 |

### `obit untrust`
用法: `obit untrust [rules file] [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 顯示說明。 |

### `obit update`
用法: `obit update [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--apply` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--check` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--relaunch-gui` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--wait-pid` | 無 | `0` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

### `obit watch`
用法: `obit watch <directory> <lifespan> \| watch --list \| watch --remove <directory> [flags]`

| 參數 | 簡寫 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `--age` | 無 | `"first-seen"` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--clear-exclusion` | 無 | `0` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--help` | `-h` | `false` | 顯示說明。 |
| `--json` | 無 | `false` | 以 JSON 輸出。 |
| `--list` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--no-daemon` | 無 | `false` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--policy` | 無 | `0` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--remove` | 無 | `""` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |
| `--remove-id` | 無 | `0` | 此參數的目前行為由原始碼產生；請同時查看命令列 help。 |

<!-- generated-command-reference:end -->
