# 配置参考

讣告读取配置目录中的 `config.toml`。所有键都可以省略；省略时使用下表的默认值。`obit config` 显示文件位置和当前生效值。

## 配置键

| 键 | 默认值 | 可选值或格式 | 作用 |
| --- | --- | --- | --- |
| `codex_windows_shell` | 未设置 | `bash`、`powershell` | 声明 Windows 上 Codex 使用的 shell；未设置时不改写含糊的删除命令。 |
| `agent_shells.<agent>` | 未设置 | `bash`、`powershell` | 为指定 hook agent 声明 Windows shell。 |
| `graveyard.mode` | `graveyard` | `graveyard`、`recycle` | 到期后移入墓地，或交给系统回收站。 |
| `graveyard.retention_days` | `7` | `1` 到 `3650` 的整数 | 墓地项目可复活的天数。 |
| `watch.stable_window` | `10m` | Go 时长格式，例如 `30s`、`10m`、`1h` | 文件夹策略等待文件稳定后再移动的时间。必须为正数。 |
| `explorer.enabled` | `true` | `true`、`false` | 是否写入资源管理器日期标签。 |
| `explorer.everywhere` | `false` | `true`、`false` | 是否在所有受支持文件夹中显示日期。 |
| `explorer.in_git` | `false` | `true`、`false` | 是否在 Git 工作区内显示日期。 |
| `ui.language` | `auto` | `auto`、`en`、`zh-CN` | CLI、窗口和托盘使用的语言。 |
| `ui.tray` | `true` | `true`、`false` | 是否启用托盘或菜单栏入口。 |
| `ui.taskbar` | `true` | `true`、`false` | Windows 窗口是否在任务栏显示独立图标。 |
| `hooks.session_summary` | `false` | `true`、`false` | 是否在会话开始时显示摘要。 |
| `hooks.session_cleanup` | `true` | `true`、`false` | 是否在会话结束时整理本次新建且未归类的文件。 |
| `proposals.mode` | `off` | `off`、`tiered`、`auto`、`ask` | 自动登记等待批准的范围。 |
| `update.check` | `true` | `true`、`false` | 是否自动检查正式版本更新。 |
| `telemetry.enabled` | `true` | `true`、`false` | 是否允许使用统计上报。与更新检查独立。 |

`agent_shells` 是按 agent 名称展开的表，例如 `[agent_shells] codex = "powershell"`。`codex_windows_shell` 是兼容 Codex 的旧写法；两者同时设置且值不同会报错。

## 格式和错误

时长使用 Go 的格式：数字后接 `ns`、`us`、`ms`、`s`、`m` 或 `h`，可以组合，例如 `1h30m`。墓地保留天数必须在 `1` 到 `3650` 之间。`graveyard.mode`、`ui.language` 和 `proposals.mode` 只能使用表中的值。

未知键不会静默忽略。讣告会报告配置文件路径、行号和完整键名；拼写接近已知键时还会给出建议。TOML 语法错误也会让 `obit config` 失败，请修正文件后再试。

## 常用操作

```powershell
obit config
obit config --create
obit config --language en
obit config telemetry off
obit config telemetry on
```

`--create` 只在配置文件不存在时写入带注释的默认模板。`--language` 修改界面语言；`auto`、`en` 和 `zh-CN` 是唯一接受的值。`telemetry on|off` 只切换 `telemetry.enabled`，不会改变 `update.check`；需要分别编辑 `[update] check`。
