# 接入 agent

## Claude Code

```powershell
obit init -g             # 所有项目（~/.claude）；obit init 只装当前项目（.claude/）
obit init -g --show --agent claude  # 装了什么
obit init -g --uninstall # 只删讣告加的东西
```

`obit init` 會在 `settings.json` 中註冊 `obit hook claude`，分別在 SessionStart、每次呼叫工具前、寫入檔案後、Stop 和 SessionEnd 時執行。它也會為 agent 寫入 `OBIT.md`，並在 `CLAUDE.md` 加上一行引用：使用者層級（`-g`）是在 `~/.claude/CLAUDE.md` 中加入 `@OBIT.md`，專案層級則是在專案的 `CLAUDE.md` 中加入 `@.claude/OBIT.md`。

- 現有設定和鍵的順序不變。重複執行不會新增重複項目；如果 `obit.exe` 移到別處，再執行一次就會更新路徑。
- 每次改之前都先備份：第一次是 `.bak`，之後是 `.bak.1`、`.bak.2` …，舊備份永遠不會被覆蓋。
- hook 發生錯誤時，agent 可以繼續執行，不會被打斷；錯誤會記在 `hook.log`。設定 `OBIT_DISABLED=1` 可暫時停用 hook。

## Codex

```powershell
obit init -g --agent codex   # ~/.codex/hooks.json + AGENTS.md 里的说明块；obit init --agent codex 只装当前项目
```

- **Codex 只執行你信任過的 hook**：裝完後開啟 Codex，執行 `/hooks`，信任訃告的 hook。專案級的 hook 還需要這個專案本身是受信任的。
- 同一份 `hooks.json` 同時支援 Windows 和 WSL。WSL 傳來的 `/mnt/<盘>/…` 路徑會自動換成 `<盘>:\…`；WSL 自己檔案系統裡的路徑（`/home/…`）暫時靜默跳過。
- Codex 改檔案用 `apply_patch`，訃告從補丁裡解析出新增、修改、移動後的檔案（刪除的不算）。

## 其他 agent

```powershell
obit init -g --show                         # 列出全部可接入的 agent、各自能做到哪一步、装没装
obit init -g --agent gemini,cursor          # 装一家或几家；不带 -g 只装当前项目
obit init -g --agent gemini --mcp --skill   # 同时注册 MCP 服务、安装 Skill（看各家是否支持）
obit init -g --agent gemini --uninstall     # 只删讣告加的东西
```

省略 `--agent` 時只會設定 Claude Code。訃告不會自行安裝到其他 agent。

| 方式 | agent |
|---|---|
| 原生 hook | GitHub Copilot（VS Code 和 Copilot CLI）、Cursor、Gemini CLI、Factory Droid、Trae / Trae CN、Kimi、Mistral Vibe、Google Antigravity；Tencent CodeBuddy（CLI 和 IDE）、Qoder / Qoder CN / Lingma、Zhipu ZCode |
| 外掛（訃告裝一個小外掛，外掛呼叫 `obit hook`） | OpenCode、OpenClaw、Pi、Oh My Pi、Hermes、Cline、Kilo Code、MiniMax Code、DeepSeek Harness |
| 只有規則檔案和 MCP | Roo Code、Windsurf、WorkBuddy、QoderWork |

::: tip 各 agent 支援哪些功能
各 agent 支援的功能不同，包括寫入後登記、工作階段開始通知、刪除護欄、MCP 和 Skill。儲存庫的 [agent 功能對照表](https://github.com/woodfishhhh/obituary/blob/main/docs/agents.md) 逐項列出「支援／有限支援／不支援」及原因。
:::

如果公開文件不足以確認如何安全地寫入設定，`obit init` 會列出完整的手動步驟，並標示「未完成，需要手動操作」，不會報成安裝成功。

## MCP 服務與 Agent Skill

```powershell
obit init -g --mcp --skill                 # Claude Code；Codex 加 --agent codex。两项都默认不装
```

- **`obit mcp`** 是一個 stdio MCP 服務，把訃告的操作做成 agent 可以直接呼叫的工具：`obit_mark`、`obit_mktemp`、`obit_bury`、`obit_resurrect`、`obit_extend`、`obit_spare`，以及查詢用的 `obit_list`、`obit_graveyard`、`obit_proposals`、`obit_stats`。
- **這些決定由你來做。** MCP 沒有核准或拒絕提議、信任 `.obitrules`、永久刪除墓地項目或變更設定的工具。agent 只能把提議轉交給你。
- **Agent Skill** 是一份 `SKILL.md`，說明何時應為產出設定壽命、如何以可復活的方式刪除，以及哪些決定須由你來做。你編輯過這份檔案後，訃告就不會再覆寫或移除它。
- 裝完重啟 agent。`obit doctor` 會檢查兩項是否裝好、指向的 `obit.exe` 是否是當前這個。
