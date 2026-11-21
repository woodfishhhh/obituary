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
