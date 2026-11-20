# 刪除護欄

agent 把使用者目錄或整個磁碟刪掉已不是假設；這種事人們不只發布過一次。開啟刪除護欄後，如果 agent 執行 `rm -rf build`、`rmdir`、`git clean -fdx`、`Remove-Item -Recurse` 或類似刪除，訃告會攔下執行，並告訴 agent 等價命令，例如 `obit rm -rf build`。agent 改用等價命令後，東西會進墓地而不是消失，`obit resurrect <id>` 可以把它們放回來。

```powershell
obit init -g --guard-rm                 # Claude Code；Codex 加 --agent codex。默认不开
```

## 一次真實的執行

開啟刪除護欄後，hook 收到一個 Claude Code Bash 呼叫 `rm -rf build`，回覆如下；`rm` 從未執行：

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Obituary's delete guard is on: `'C:/work/obit.exe' rm -rf build` provides restorable deletion; `obit resurrect` restores graveyard entries."}}
```

agent 接著改用訃告；資料夾進入墓地，再用一條命令放回。

```console
$ obit rm -rf build
$ obit graveyard --here
ID  BURIED            PURGE  SIZE  PATH
#2  2026-11-11 00:36  in 7d  23 B  C:\work\build
$ obit resurrect --last
#2 resurrected  C:\work\build
```

## 原則

- **攔截，不改寫。** 訃告不改寫 agent 的命令：Claude Code 會按改寫後的命令檢查權限，改寫可能使原本針對命令的 `deny` 規則失效。攔下原命令，讓 agent 重新發出等價命令，才能保留所有權限規則。
- **字面刪除一律攔下。** `rm`、`rmdir`、`git clean` 或 `Remove-Item`（包括 `del`、`rd`、`ri` 等別名）只要出現在任何命令位置，不論位於 `&&`、`||`、`;` 之後、管道、`$(…)`、子 shell、條件或迴圈中，都會攔下這次執行，例如 `rm -rf build && npm run build`、`rm -rf build 2>/dev/null` 或 `git clean -fdx && npm ci`。字串、註釋或 `echo` 引數裡的 `rm` 不算；`rm --help`、`git clean -n` 和 `Remove-Item -WhatIf` 等預覽也不算。
- **先列出執行時才選出的目標，再下葬。** Bash 的 `find -delete`、`find -exec rm`（包括 `-execdir`）、`xargs rm` 和 PowerShell 的 `Get-ChildItem … | Remove-Item` 會被攔下，不提供改寫。先列出並核對匹配項，再把明確路徑逐個或批次交給 `obit rm -- <路径>...`。只搜尋或列出路徑的 `find` 仍會執行。護欄不檢查 `sudo rm` 或 `bash -c '…'` 之類任意包裝。
- **不會退回真正刪除。** 交給 `obit rm`、`obit remove-item` 或 `obit git-clean` 的每個路徑都會下葬；如果路徑被拒絕（磁碟根目錄、系統資料夾、使用者目錄本身等），錯誤沿用 `rm` 格式，退出碼非零。
- **防止命令在途中變形。** agent 想執行的命令和真正執行的命令之間有 shell 引號、轉義，有時還有會改寫命令的代理。少一個字元，`rm -rf "$OUT"/` 就可能變成 `rm -rf /`。所以 `obit rm` 動手前會核對磁碟上的真實路徑。
- **Windows 上 Codex 的引號內替換也算。** Windows Codex 會把命令交給另一層 shell，引號可能失效，Bash 原本視為文字的內容也可能執行。因此命令中的 `$(…)` 片段會同時按 Bash 和 PowerShell 檢查，包括引號、帶引號 heredoc 和註釋；其中出現刪除就攔下。帶反引號的文字若由編輯工具寫入檔案，再把檔案路徑交給命令，就能原樣傳遞。
- Git 追蹤的檔案也會下葬，因此未提交的變更仍可復原；這是 agent 主動刪除，與自動登記不碰追蹤檔案不同。
- `obit rm -rf` 遇到含 `.git` 的資料夾會把整個儲存庫下葬；真正的 `rm -rf` 會摧毀它，而下葬至少可以撤回。不能強制的系統資料夾和使用者資料夾保護仍然有效。
- **等價命令刻意收窄。** 它絕不能比原命令選中更多目標，所以訃告不跨 shell 翻譯，只對下表形式提供一個等價命令。其他情況仍會攔下，原因會說明為何沒有等價命令，並指出 `obit bury <路径>`（字面路徑）和 `obit git-clean` 可以可復原地刪除，讓 agent 依此改寫。

## 各場景下攔下和放行的寫法

| 主體 | 攔下並提供等價命令 | 攔下並說明原因 | 放行 |
|---|---|---|---|
| Claude Code（Bash 工具） | `rm`、`rmdir`、`git clean`，前面可有字面的 `cd x`（以 `&&`、`;` 或換行連接）；`rm -f *.log`（資料夾內沒有以 `-` 開頭的檔案時）、`rm -rf ../dist`、`rm -- "$target"`（變數在 `--` 後） | 其他字面刪除：前後有其他語句（`npm …`、`trap`、`hash`）、`--` 前有變數、以 `/` 開頭的 POSIX 絕對路徑、重定向、`\|\|`、管道、`$(…)`、互動式刪除、帶路徑的 `/usr/bin/rm` | 不會刪除的命令 |
| Codex，在 WSL 的 `/mnt/<盘>` 下 | 單獨一個 `rm` 或 `rmdir`，引數是沒有 `..`、`~`、通配符或引號的相對路徑 | 其他字面刪除，包括 `git clean` 和指向 WSL 自有檔案系統的路徑 | WSL 自有檔案系統內的工作目錄 |
| Codex，宣告使用 Windows PowerShell | 單獨一個 `Remove-Item` 或別名，只帶一個字面路徑（可有 `-Recurse`、`-Force`） | 其他字面刪除：多條語句、管道、`-ErrorAction` 等其他引數、provider 路徑、`~`、通配符、反引號 | 彎引號字串中的文字 |
| Codex，Windows 上但 shell 不明 | — | 原始反引號或 `$(…)` 片段中的刪除 | 其他命令 |
| Linux 上的 pwsh、`git rm` | — | — | 全部放行 |

提供等價命令時，刪除必須是整條命令的**最後一段**，前面只能用 `&&`、`;` 或換行連接；以點或空格結尾的檔名不提供等價命令，因為 Windows 會把它們正規化成另一個名字。這些情況中的字面刪除仍會被攔下。

## 你自己也可以用

`obit rm`、`obit rmdir` 和 `obit git-clean` 接受 `rm`、`rmdir`、`git clean` 常用的非互動引數；不支援的 `-i` 或 `rmdir -p` 會報錯，不會悄悄忽略。`obit remove-item` 只接受一個字面路徑，不是完整的 `Remove-Item`；要用通配符，在 Bash 中使用 `obit rm *.log`。

用 `OBIT_GUARD_RM=0` 暫時關閉。`obit stats` 會顯示攔截了多少次刪除，以及下葬了多少資料。
