# 削除ガード

エージェントがホームフォルダーやドライブ全体を消去する事例は、もう仮説ではありません。ガードを有効にしているときに `rm -rf build`、`rmdir`、`git clean -fdx`、`Remove-Item -Recurse` などの削除が実行されると、Obituary は処理を止め、`obit rm -rf build` のような同等のコマンドを伝えます。エージェントがそれを使うと対象は墓地へ移され、`obit resurrect <id>` で戻せます。

```powershell
obit init -g --guard-rm                 # Claude Code; add --agent codex for Codex. Off by default
```

## 実際の実行例

ガードを有効にすると、フックは Claude Code の Bash 呼び出し `rm -rf build` を受け取り、次のように返しました。`rm` は実行されていません。

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Obituary's delete guard is on: `'C:/work/obit.exe' rm -rf build` provides restorable deletion; `obit resurrect` restores graveyard entries."}}
```

エージェントは Obituary のコマンドに切り替え、フォルダーは墓地へ移り、1 つのコマンドで戻せます。

```console
$ obit rm -rf build
$ obit graveyard --here
ID  BURIED            PURGE  SIZE  PATH
#2  2026-11-11 00:36  in 7d  23 B  C:\work\build
$ obit resurrect --last
#2 resurrected  C:\work\build
```

## 原則

- **拒否し、書き換えない。** Obituary はエージェントのコマンドを書き換えません。Claude Code は書き換え後のコマンドで権限を確認するため、元のコマンド向けの `deny` ルールが効かなくなるからです。元の実行を拒否し、エージェントに同等のコマンドを出し直させることで、権限ルールを維持します。
- **リテラルな削除は停止する。** `rm`、`rmdir`、`git clean`、`Remove-Item`（`del`、`rd`、`ri` などの別名を含む）が、`&&`、`||`、`;` の後、パイプ、`$(…)`、サブシェル、条件、ループなど、どのコマンド位置に現れても実行を停止します。たとえば `rm -rf build && npm run build`、`rm -rf build 2>/dev/null`、`git clean -fdx && npm ci` です。文字列、コメント、`echo` の引数内の `rm` は対象外です。`rm --help`、`git clean -n`、`Remove-Item -WhatIf` などのプレビューも対象外です。
- **実行時に選ばれた対象を先に一覧化する。** Bash の `find -delete`、`find -exec rm`（`-execdir` を含む）、`xargs rm`、PowerShell の `Get-ChildItem … | Remove-Item` は書き換えずに拒否します。先に一致した対象を確認し、明示的なパスを `obit rm -- <path>...` に個別またはまとめて渡してください。検索や一覧表示だけの `find` は実行できます。`sudo rm` や `bash -c '…'` のような任意のラッパーは調べません。
- **実際の削除にはフォールバックしない。** `obit rm`、`obit remove-item`、`obit git-clean` に渡したパスはすべて埋葬されます。ドライブのルートやシステムフォルダー、ユーザーフォルダーなどが拒否された場合は `rm` と同じ形式でエラーになり、終了コードは 0 以外です。
- **コマンドの変化を確認する。** エージェントが意図したコマンドと実行されるコマンドの間には、シェルの引用やエスケープ、コマンドを書き換えるプロキシが入ることがあります。1 文字失われると `rm -rf "$OUT"/` が `rm -rf /` になります。`obit rm` は実行前にディスク上の実パスを確認します。
- **Windows の Codex では引用符内の置換も対象です。** 別のシェルを経由するため、Bash では文字列でも実行されることがあります。Windows の Codex では `$(…)` を Bash と PowerShell の両方として読み、引用符、引用付き heredoc、コメント内でも削除を検出します。削除を含めば停止します。バックティックを含む文章は編集ツールでファイルに書き、そのパスをコマンドに渡せばそのまま扱えます。
- Git で追跡されるファイルも埋葬されるため、未コミットの変更を復元できます。これはエージェントが意図した削除であり、自動登録が追跡ファイルに触れないこととは別です。
- `.git` を含むフォルダーに対する `obit rm -rf` はリポジトリ全体を埋葬します。通常の `rm -rf` なら破壊しますが、埋葬なら取り消せます。強制できないシステムフォルダーやユーザーフォルダーの保護は残ります。
- **同等のコマンドは意図的に限定しています。** 元のコマンドより多くを選ばないため、shell 間の変換はせず、下表の形式だけに同等のコマンドを示します。それ以外は停止し、理由とともに `obit bury <path>`（リテラルパス）や `obit git-clean` が復元可能な削除であることを示します。

## 止まったものと通り過ぎたもの

| 実行元 | 同等のコマンドを示して停止 | 理由と一般的な案内を示して停止 | 通過 |
|---|---|---|---|
| Claude Code（Bash ツール） | `rm`、`rmdir`、`git clean`、必要ならリテラルの `cd x` を前置（`&&`、`;`、改行で接続）；`rm -f *.log`（`-` で始まるファイルがない場合）、`rm -rf ../dist`、`rm -- "$target"`（変数は `--` の後） | その他のリテラル削除：前後に他の文（`npm …`、`trap`、`hash`）、`--` 前の変数、`/` で始まる POSIX 絶対パス、リダイレクト、`\|\|`、パイプ、` $(…) `、対話的削除、パス付き | 削除しないコマンド |
| WSL の `/mnt/<drive>` 上の Codex | `..`、`~`、ワイルドカード、引用符を含まない相対パスを引数にする単一の `rm` / `rmdir` | その他のリテラル削除（`git clean` と WSL 自身のファイルシステムへのパスを含む） | WSL 自身のファイルシステム内の作業ディレクトリ |
| Windows PowerShell と明示された Codex | 1 つのリテラルパスだけを取る単一の `Remove-Item` または別名（`-Recurse`、`-Force` は可） | その他のリテラル削除：複数文、パイプ、`-ErrorAction` など、provider パス、`~`、ワイルドカード、バックティック | 引用符内の文章 |
| Windows だが shell 不明の Codex | — | 生のバックティックまたは `$(…)` 片の中の削除 | その他のコマンド |
| Linux の pwsh、`git rm` | — | — | すべて通過 |

同等のコマンドを出す場合、削除はコマンド全体の**最後の部分**であり、前の部分は `&&`、`;`、改行だけで接続されていなければなりません。末尾がドットや空白のファイル名には同等のコマンドを出しません。Windows が別名に正規化するためです。これらでもリテラル削除自体は停止します。

## あなたが使ってください

`obit rm`、`obit rmdir`、`obit git-clean` は `rm`、`rmdir`、`git clean` の一般的な非対話オプションを受け付けます。`-i` や `rmdir -p` など未対応の指定はエラーです。`obit remove-item` はリテラルパス 1 つだけを受け付ける `Remove-Item` の簡易版です。ワイルドカードには Bash で `obit rm *.log` を使います。

`OBIT_GUARD_RM=0` で一時的に無効にできます。`obit stats` では停止した削除回数と埋葬した容量を確認できます。
