# 墓地提醒

先下葬、再火化的前提，是下葬的东西要能被想起来。agent 的工具调用用到一个已经下葬、还能复活的路径（或下葬目录里的东西）时，讣告告诉它：

- 墓地里的编号；
- 什么时候下葬的；
- 还能复活到什么时候；
- 一条 `obit resurrect <编号>`。

## 只提醒，不打扰

- 只提醒，不拦也不改它的操作。
- 同一个会话里每座坟只提一次。
- Claude Code 和 Codex 在调用工具前当场提醒，其他家按 `obit init --show` 里写的时机。
- 平时不打扰：agent 开工时只知道有讣告在，不用先读一遍清单。

以前装过的，重新执行一次 `obit init` 才有墓地提醒。

## 开工清单（可选）

为了省 agent 的 token，会话开始时默认只转告需要你拍板的提议和刚自动登记的东西。在小窗口「设置」里打开「开工清单」，或者在 `config.toml` 里设：

```toml
[hooks]
session_summary = true
```

agent 开工时还会看到这个项目 24 小时内要到期的条目，和最近下葬、还能复活的东西（`obit mktemp` 的临时目录不算）。

## 自己查墓地

```powershell
obit graveyard           # 墓地里有什么、还剩几天被火化
obit graveyard --here    # 只看当前目录下的
obit resurrect <编号>    # 放回原处；原处已有同名的，改名放回，绝不覆盖
obit resurrect --last    # 撤销上一次删除：一条 obit bury / obit rm 埋下的几项一起回来
```
