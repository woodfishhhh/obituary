# 接入 agent

## Claude Code

```powershell
obit init -g             # 所有项目（~/.claude）；obit init 只装当前项目（.claude/）
obit init -g --show --agent claude  # 装了什么
obit init -g --uninstall # 只删讣告加的东西
```

`obit init` 在 `settings.json` 里注册 `obit hook claude`（SessionStart、每次调用工具前、写文件后、Stop、SessionEnd），写一份给 agent 看的 `OBIT.md`，并在 `CLAUDE.md` 里加一行引用：用户级（`-g`）是 `~/.claude/CLAUDE.md` 里的 `@OBIT.md`，项目级是项目根 `CLAUDE.md` 里的 `@.claude/OBIT.md`。

- 你原来的设置和键顺序保持不变，重复执行不会重复添加；`obit.exe` 换了位置，再执行一次就会刷新路径。
- 每次改之前都先备份：第一次是 `.bak`，之后是 `.bak.1`、`.bak.2` …，旧备份永远不会被覆盖。
- hook 出任何错都静默放行（错误写进 `hook.log`）。设置 `OBIT_DISABLED=1` 可以临时关掉。

## Codex

```powershell
obit init -g --agent codex   # ~/.codex/hooks.json + AGENTS.md 里的说明块；obit init --agent codex 只装当前项目
```

- **Codex 只执行你信任过的 hook**：装完后打开 Codex，运行 `/hooks`，信任讣告的 hook。项目级的 hook 还需要这个项目本身是受信任的。
- 同一份 `hooks.json` 同时支持 Windows 和 WSL。WSL 传来的 `/mnt/<盘>/…` 路径会自动换成 `<盘>:\…`；WSL 自己文件系统里的路径（`/home/…`）暂时静默跳过。
- Codex 改文件用 `apply_patch`，讣告从补丁里解析出新增、修改、移动后的文件（删除的不算）。

## 其他 agent

```powershell
obit init -g --show                         # 列出全部可接入的 agent、各自能做到哪一步、装没装
obit init -g --agent gemini,cursor          # 装一家或几家；不带 -g 只装当前项目
obit init -g --agent gemini --mcp --skill   # 同时注册 MCP 服务、安装 Skill（看各家是否支持）
obit init -g --agent gemini --uninstall     # 只删讣告加的东西
```

不带 `--agent` 时只装 Claude Code，不会替你给别的 agent 装东西。

| 方式 | agent |
|---|---|
| 原生 hook | GitHub Copilot（VS Code 和 Copilot CLI）、Cursor、Gemini CLI、Factory Droid、Trae / Trae CN、Kimi、Mistral Vibe、Google Antigravity；国内的腾讯 CodeBuddy（CLI 和 IDE）、Qoder / Qoder CN / 灵码、智谱 ZCode |
| 插件（讣告装一个小插件，插件调用 `obit hook`） | OpenCode、OpenClaw、Pi、Oh My Pi、Hermes、Cline、Kilo Code、MiniMax Code、DeepSeek Harness |
| 只有规则文件和 MCP | Roo Code、Windsurf、WorkBuddy、QoderWork |

::: tip 各家能做到哪一步
各家能做到的程度不一样（写文件后的登记、会话开头的提示、删除护栏、MCP、Skill），逐项的「支持 / 有限 / 不支持」和原因见仓库里的 [agent 覆盖表](https://github.com/woodfishhhh/obituary/blob/main/docs/agents.md)。
:::

公开资料不足以安全地自动写入的地方，`obit init` 打印完整的手动步骤，结果标成「未完成，需要手动步骤」，不会假装装好了。

## MCP 服务与 Agent Skill

```powershell
obit init -g --mcp --skill                 # Claude Code；Codex 加 --agent codex。两项都默认不装
```

- **`obit mcp`** 是一个 stdio MCP 服务，把讣告的操作做成 agent 可以直接调用的工具：`obit_mark`、`obit_mktemp`、`obit_bury`、`obit_resurrect`、`obit_extend`、`obit_spare`，以及查询用的 `obit_list`、`obit_graveyard`、`obit_proposals`、`obit_stats`。
- **你的决定不交给 agent。** 批准或拒绝提议、信任 `.obitrules`、火化墓地、改配置，MCP 里都没有对应的工具。agent 只能把提议的内容转述给你。
- **Agent Skill** 是一份 `SKILL.md`，告诉 agent 什么时候给产物设寿命、怎么可复活地删除、哪些事要交给你决定。你改过这份文件，讣告就不再覆盖或删除它。
- 装完重启 agent。`obit doctor` 会检查两项是否装好、指向的 `obit.exe` 是否是当前这个。
