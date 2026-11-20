# 墓地提醒

先下葬而不直接刪除，前提是日後找得到。如果 agent 呼叫工具時用到已下葬、仍可復活的路徑，或用到已下葬資料夾中的項目，訃告會告訴它：

- 墓地裡的編號；
- 什麼時候下葬的；
- 距離復活期限還有多久；
- 復活指令 `obit resurrect <编号>`。

## 只提醒，不攔截

- 訃告不會攔截或更改這次工具呼叫。
- 每次工作階段中，每座墳墓只提醒一次。
- Claude Code 和 Codex 會在執行工具前收到提醒，其他 agent 的提醒時機則列在 `obit init --show` 中。
- 平時不打擾：agent 開工時只知道有訃告在，不用先讀一遍清單。

如果已安裝訃告，重新執行 `obit init` 即可啟用墓地提醒。

## 開工清單（可選）

為了節省 agent 的 token，工作階段開始時預設只列出需要你決定的提議，以及剛自動登記的項目。若要顯示摘要，可在小視窗的「設定」中開啟「開工清單」，或在 `config.toml` 中設定：

```toml
[hooks]
session_summary = true
```

啟用後，agent 也會看到這個專案中 24 小時內到期的項目，以及最近下葬且仍可復活的項目（不含 `obit mktemp` 建立的暫存資料夾）。

## 自己查墓地

```powershell
obit graveyard           # 墓地里有什么、还剩几天被火化
obit graveyard --here    # 只看当前目录下的
obit resurrect <编号>    # 放回原处；原处已有同名的，改名放回，绝不覆盖
obit resurrect --last    # 撤销上一次删除：一条 obit bury / obit rm 埋下的几项一起回来
```
