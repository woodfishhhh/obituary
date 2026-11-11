# 命令参考

`<编号>` 换成 `obit add`、`obit bury` 回执里或 `obit list`、`obit graveyard` 列出的编号。时长单位：`s` `m` `h` `d` `w`，可以组合（`1d12h`）。

## 寿命

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
obit purge                                  # 火化保留期已过的（--all 全部，--dry-run 预览）
obit purge <编号>                           # 立即火化这一座（永久删除）
```

`resurrect` 和 `purge` 里的纯数字（`5`、`#5`）一律当编号，找不到就报错，不会退回去当成文件名；文件名本身是数字时写成 `./5` 或绝对路径。

## 删除包装

```powershell
obit rm -rf build            # 和 rm 一样的常用参数，每个路径都走下葬
obit rmdir empty-dir
obit git-clean -fdx
obit remove-item .\build     # 只接受一个字面路径
```

## 自动登记与提议

```powershell
obit rules                       # 当前项目生效的规则
obit rules check debug.log out/  # 预览：会登记什么、按哪条规则
obit rules apply debug.log       # 按规则登记
obit trust                       # 信任这个项目的 .obitrules（非交互环境加 --yes）
obit proposals                   # 等你点头的提议
obit approve 3                   # 同意
obit reject 3                    # 拒绝
$dir = obit mktemp 1d            # 会自动下葬的临时目录
```

## 文件夹策略与 Docker

```powershell
obit watch C:\Users\你\Downloads 2w
obit watch --list
obit watch --remove C:\Users\你\Downloads
obit docker add web-1 3d
obit docker list
```

## 接入与后台

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

## 查询与体检

```powershell
obit doctor              # 只读体检（支持 --json）
obit stats               # 悼词：埋了多少、腾出多少空间、复活过几次（同 eulogy）
obit log                 # 生平记录：做过的所有事
obit config              # 查看配置；--create 生成带注释的默认配置
obit hook audit          # 设了 OBIT_HOOK_AUDIT=1 时，查看每次 hook 收到了什么、做了什么
obit uninstall           # 撤掉讣告装过的所有东西（先 --dry-run 看看）
```

查询和结果类命令（`list`、`graveyard`、`proposals`、`stats`、`mark`、`bury`、`resurrect` 等）支持 `--json`，方便脚本和 AI agent 调用；`obit rm` 这类删除包装器照原命令的格式输出，不支持 `--json`。

## 环境变量

| 变量 | 作用 |
|---|---|
| `OBIT_DISABLED=1` | 临时关掉 hook |
| `OBIT_GUARD_RM=0` | 临时关掉删除护栏 |
| `OBIT_HOOK_AUDIT=1` | 记下每次 hook 收到了什么、做了什么 |
| `OBIT_TRUST_PROJECT_RULES=1` | CI 里信任项目的 `.obitrules` |
| `OBIT_HOME`、`OBIT_CONFIG_DIR`、`OBIT_USER_HOME` | 换数据目录、配置目录、用户主目录 |
