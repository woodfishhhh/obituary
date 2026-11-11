# 删除护栏

agent 删光用户目录、删空整块盘，这样的事故已经有人贴出来过不止一次。打开删除护栏后，agent 执行 `rm -rf build`、`rmdir`、`git clean -fdx`、`Remove-Item -Recurse` 这类删除命令时，讣告会拦下这一次执行，并告诉 agent 一条等价命令，比如 `obit rm -rf build`。agent 改用它之后，东西进墓地而不是消失，`obit resurrect <id>` 一条命令放回原处。

```powershell
obit init -g --guard-rm                 # Claude Code；Codex 加 --agent codex。默认不开
```

## 一次真实的输出

删除护栏打开时，hook 收到 Claude Code 的一次 Bash 调用 `rm -rf build`，回复如下，`rm` 没有执行：

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Obituary's delete guard is on: `'C:/work/obit.exe' rm -rf build` provides restorable deletion; `obit resurrect` restores graveyard entries."}}
```

之后 agent 改用讣告，东西进墓地，再一条命令放回：

```console
$ obit rm -rf build
$ obit graveyard --here
ID  BURIED            PURGE  SIZE  PATH
#2  2026-11-11 00:36  in 7d  23 B  C:\work\build
$ obit resurrect --last
#2 resurrected  C:\work\build
```

## 原则

- **只拦不改。** 讣告不改写 agent 的命令：Claude Code 会按改写后的命令检查权限，改写会让你设的 `deny` 规则失效。拦下再让 agent 自己发出等价命令，你的所有权限规则照常生效。
- **宁可漏拦，不可误拦。** 只认处在命令开头（或 `&&`、`;`、换行之后）的删除命令；`||` 之后的、字符串里的 `rm`、heredoc、`$(…)`、`sudo`、`xargs rm`、`find -delete`、交互式删除一律放行（设了 `OBIT_HOOK_AUDIT=1` 会记进审计日志）。
- **不会退回成真删。** `obit rm` / `obit remove-item` / `obit git-clean` 的每个路径都走下葬；被拒绝（盘符根、系统目录、用户目录本身等）时照 `rm` 的格式报错、返回非零。
- **怕命令在路上变了样。** agent 想的命令和真正执行的命令之间，隔着 shell 的引号和转义，还可能隔着一层改写命令的代理。一个符号被吃掉，`rm -rf "$OUT"/` 就成了 `rm -rf /`。所以 `obit rm` 动手前按磁盘上的真实路径再核对一遍。
- 被 Git 跟踪的文件也照样下葬，这样未提交的修改也找得回来（这是 agent 主动删除，和「自动登记绝不碰被跟踪文件」不冲突）。
- `obit rm -rf` 一个含 `.git` 的目录会把整个仓库下葬：真正的 `rm -rf` 会把它永久删掉，下葬至少还能复活。系统目录、用户文件夹这些不能强制的保护照样生效。
- **覆盖范围刻意收窄**：等价命令绝不能比原命令选中更多东西，所以讣告不做跨 shell 的翻译，拿不准就放行。

## 各场景下拦下和放行的写法

| 场景 | 拦下并给等价命令 | 放行 |
|---|---|---|
| Claude Code（Bash 工具） | `rm`、`rmdir`、`git clean`，前面只能有字面的 `cd x`（用 `&&`、`;` 或换行连接）；`rm -f *.log`（当前目录里没有以 `-` 开头的文件时）、`rm -rf ../dist`、`rm -- "$target"`（变量在 `--` 之后） | 其他前置语句（`trap`、`hash`、`npm …` 等）、`--` 之前的变量、以 `/` 开头的 POSIX 绝对路径、复杂结构、交互式、带路径的 `/usr/bin/rm` |
| Codex，WSL 的 `/mnt/<盘>` 下 | 单独一条 `rm` / `rmdir`，参数都是不含 `..`、`~`、通配、引号的相对路径 | 其他一切，含 `git clean` 和 WSL 自己的文件系统 |
| Codex，明确是 Windows PowerShell | 单独一条 `Remove-Item` 或别名，只带一个字面路径（可加 `-Recurse`、`-Force`） | 多条语句、管道、provider 路径、`~`、通配、反引号、弯引号 |
| Codex 在 Windows 上但看不出是哪种 shell、Linux 上的 pwsh、`git rm` | — | 全部放行 |

删除命令必须是整条命令的**最后一段**，前面只能用 `&&`、`;` 或换行连接。文件名以点或空格结尾的一律放行（Windows 会把它们归一化成另一个名字）。

## 你自己也可以用

`obit rm`、`obit rmdir`、`obit git-clean` 接受 `rm`、`rmdir`、`git clean` 常用的非交互参数（`-i`、`rmdir -p` 这类不支持的参数会报错，不会悄悄忽略）；`obit remove-item` 只接受一个字面路径，不是完整的 `Remove-Item` 替代。要用通配，在 bash 里用 `obit rm *.log`。

临时关掉：`OBIT_GUARD_RM=0`。`obit stats` 会显示拦下了多少次删除、埋了多少东西。
