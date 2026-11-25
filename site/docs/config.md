# 配置参考

讣告的设置都写在配置目录里的 `config.toml`。这个文件不是必须的：没有它，或者里面只写了几项，其余的都用默认值。`obit config` 会告诉你文件在哪、有没有创建、每一项现在实际取什么值；`obit config --create` 写出一份带注释、取值全是默认值的文件，从它开始改最省事。

配置目录在 Windows 上是 `%APPDATA%\obituary`，在 macOS 上是 `~/Library/Application Support/obituary`，在 Linux 上是 `~/.config/obituary`（设了 `XDG_CONFIG_HOME` 就在它下面的 `obituary`）。想换个位置，设环境变量 `OBIT_CONFIG_DIR`。

| 键 | 默认值 | 可选值 |
| --- | --- | --- |
| `graveyard.mode` | `graveyard` | `graveyard`、`recycle` |
| `graveyard.retention_days` | `7` | 1 到 3650 的整数 |
| `proposals.mode` | `off` | `off`、`tiered`、`auto`、`ask` |
| `hooks.session_cleanup` | `true` | `true`、`false` |
| `hooks.session_summary` | `false` | `true`、`false` |
| `watch.stable_window` | `10m` | 正的时长，如 `30s`、`10m`、`1h30m` |
| `watch.file_expiry` (macOS/Linux) | `quiet` | `quiet`, `propose` |
| `explorer.enabled` | `true` | `true`、`false` |
| `explorer.everywhere` | `false` | `true`、`false` |
| `explorer.in_git` | `false` | `true`、`false` |
| `ui.language` | `auto` | `auto`、`en`、`zh-CN` |
| `ui.tray` | `true` | `true`、`false` |
| `ui.taskbar` | `true` | `true`、`false` |
| `update.check` | `true` | `true`、`false` |
| `telemetry.enabled` | `true` | `true`、`false` |
| `agent_shells` | 未设置 | 表，键是 agent 名，值是 `bash` 或 `powershell` |
| `codex_windows_shell` | 未设置 | `bash`、`powershell` |

## 到期之后去哪儿

默认情况下，寿命用完的东西进讣告自己的墓地，`graveyard.retention_days` 天内（默认 7 天）可以用 `obit resurrect` 放回原处，过了这个期限才被永久删除。这个天数可以在 1 到 3650 之间调整。

Windows 和 Linux 可把 `graveyard.mode` 设为 `recycle`。从系统回收站还原，不能用 `obit resurrect`。macOS 仅支持墓地。

## 自动登记和 agent 会话

`proposals.mode` 决定讣告会不会自己给文件登记寿命。默认的 `off` 什么都不自动做，只有 agent 或你用命令、右键菜单、文件夹策略登记的东西才进来。`tiered` 让 agent 在本次会话里新建的文件、会话启动的容器直接进来，事后告诉你，不想要可以用 `obit spare` 撤回；其余的先放进 `obit proposals`，等你批准。`auto` 把自动登记发现的一切直接登记，事后告诉你。`ask` 则每一项都先问你。不论哪一档，Git 已经跟踪的文件，以及依赖、构建产物和缓存（`node_modules`、`dist`、`__pycache__` 之类）都不会被自动登记。详细规则见[什么进讣告](./auto-register)。

`hooks.session_cleanup` 默认开着。agent 每个会话在快要结束时，会被告知它这次新建、还没有寿命、Git 也没有跟踪的文件，并请它给临时文件登记寿命（`obit mark`）或者下葬，交付物留着不动。`hooks.session_summary` 默认关着；打开后，agent 在会话开始时还会听到项目里 24 小时内到期的东西和最近下葬的东西。它默认关闭是为了省 token：agent 碰到已下葬的路径时，本来就会被告知。

## 文件夹策略和资源管理器里的日期

`watch.stable_window` 是文件夹策略的耐心：新文件出现后，要连续这么久没有变化，才会被移动，免得把还在下载的文件移走。它写成 Go 的时长格式，数字后面跟 `s`、`m` 或 `h`，可以连写，比如 `1h30m`，而且必须大于零。

自动模式必须能检查文件属主的所有进程。只有文件属于当前用户、组和其他人都没有写权限时，才忽略其他用户（包括 root）不可读的进程。想逐项确认时请选择 `propose`。批准可以覆盖进程信息不全，只要发现文件仍被打开，就会保留并显示原因。

Windows 在资源管理器的显示名旁标出日期；Finder 和 Dolphin 使用原生标签。三者都遵循 `explorer.*`，都不修改真实文件名。默认只在「下载」、桌面（含子文件夹）和文件夹策略管着的位置显示；`explorer.everywhere` 打开后，其他受跟踪的位置也显示；Git 工作区默认不显示，要显示就把 `explorer.in_git` 设为 `true`；`explorer.enabled` 设为 `false` 则完全不写。见[文件夹策略](./folder-policies)。

## 界面

`ui.language` 管命令行、小窗口、托盘和右键菜单的语言，目前只有 `en` 和 `zh-CN`；`auto` 依次看环境变量 `OBIT_LANG`、`LC_ALL`、`LC_MESSAGES`、`LANG` 和系统界面语言。`ui.tray` 控制托盘图标（macOS 上是菜单栏图标）的显示，隐藏之后后台清理和更新检查照常进行。`ui.taskbar` 只对 Windows 有效，决定小窗口打开时是否在任务栏占一个按钮。

## 更新检查和使用统计

`update.check` 开着时，讣告每天最多读一次 GitHub 上公开的发布信息，不发送任何使用数据；关掉后仍可以随时用 `obit update --check` 手动检查。`telemetry.enabled` 控制匿名的使用统计，它和更新检查是两个独立的开关，`obit config telemetry off` 只关统计，并清掉本机的安装 ID 和待发的数据。详情见[隐私政策](./privacy)。

## Windows 上 agent 用哪种 shell

删除护栏要读懂 agent 要执行的删除命令，才能给出等价的 `obit` 命令。Windows 上的 agent 可能用 bash，也可能用 PowerShell，讣告自己分辨不了，所以由你来声明：这个 agent 就在这台 Windows 电脑上，用这种 shell 执行命令。没有声明时，意思含糊的命令一律放行，不拦。

```toml
[agent_shells]
codex = "powershell"
```

`agent_shells` 里的键是 agent 的名字，目前读取它的有 `codex`、`codebuddy`、`codebuddy-ide` 和 `minimax-code`。`codex_windows_shell` 是只给 Codex 的旧写法，和 `agent_shells.codex` 同时出现且值不一样时，配置会被拒绝。

## 写错了会怎样

讣告不会悄悄忽略写错的键。如果忽略，`retention = 30` 会让 7 天的保留期原样生效，而你以为已经改成了 30。所以遇到不认识的键，`obit config` 会报出文件路径、行号和完整的键名，拼写接近时还会给出建议：

```text
obit: 配置文件有错误：…\config.toml:2: unknown key "graveyard.retention" (did you mean "graveyard.retention_days"?)
```

取值超出范围时同样会指出是哪一项、允许什么，比如 `graveyard.retention_days must be between 1 and 3650`。TOML 语法错误也一样让 `obit config` 失败。改好文件再运行一次，不再报错就是好了。

## 常用命令

```powershell
obit config                  # 文件位置、数据目录和当前生效的值
obit config --create         # 写出带注释的默认文件
obit config --language en    # 把界面语言写进配置：auto、en 或 zh-CN
obit config telemetry off    # 关闭使用统计（on 重新打开）
```

`--create` 不会覆盖已有的文件，文件已经存在时它会报错并退出。`--language` 在文件不存在时会连同默认内容一起创建。
