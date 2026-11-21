# コマンド一覧

`<id>` には `obit add`・`obit bury` の実行結果、または `obit list`・`obit graveyard` の一覧に表示される番号を指定します。期間の単位は `s`、`m`、`h`、`d`、`w` で、`1d12h` のように組み合わせられます。

## 寿命

```powershell
obit add debug.log 2h                       # bury it in 2 hours (alias: mark)
obit add output\shots 1d12h                 # folders work too
obit add report.zip --at "2026-12-01 18:00" # at a given moment
obit list                                   # what is left and for how long
obit extend <id> 1d                         # one more day
obit spare <id>                             # stop tracking, keep the file (alias: cancel)
obit reap                                   # bury everything that is due now (alias: sweep; --dry-run previews)
```

## 墓地

```powershell
obit bury old-build                         # bury right away
obit graveyard                              # what is in the graveyard and when it will be purged (--here: this folder only)
obit resurrect <id>                         # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last                       # undo the last deletion: everything one obit bury / obit rm buried comes back
obit purge                                  # purge what is past retention (--all for everything, after confirming; --dry-run previews)
obit purge <id>                             # purge this grave now (permanent; asks first, --yes outside a terminal)
```

`resurrect` と `purge` では、数字だけの指定（`5`、`#5`）は必ず ID として扱います。該当する ID がなければエラーになり、ファイル名として探し直すことはありません。名前が数字のファイルには `./5` または絶対パスを指定してください。

## 削除コマンドのラッパー

```powershell
obit rm -rf build            # rm's common options; every path is buried
obit rmdir empty-dir
obit git-clean -fdx
obit remove-item .\build     # one literal path only
```

## ルールと提案

```powershell
obit rules                       # the rules in effect here
obit rules check debug.log out/  # preview what would be registered and by which rule
obit rules apply debug.log       # register by rule
obit trust                       # trust this project's .obitrules (--yes when non-interactive)
obit proposals                   # proposals waiting for you, and what was registered without asking
obit approve 3                   # approve
obit reject 3                    # reject
$dir = obit mktemp 1d            # a temporary folder that buries itself
```

## フォルダーポリシーと Docker

```powershell
obit watch C:\Users\you\Downloads 2w
obit watch --list
obit watch --remove C:\Users\you\Downloads
obit docker add web-1 3d
obit docker list
```

## エージェント連携とバックグラウンド処理

```powershell
obit init                       # Claude Code hooks and Skill, context menu, Start menu and desktop shortcuts; the tray starts at login
obit init -g --agent codex      # Codex
obit init -g --guard-rm         # delete guard
obit init -g --mcp              # add the MCP server
obit init --no-context-menu --no-shortcuts --no-skill   # skip the context menu, shortcuts or Skill
obit init -g --uninstall --no-context-menu   # on uninstall keep the context menu and remove the rest; --no-shortcuts and --no-skill work the same way
obit init -g --show             # what is installed
obit daemon enable              # tray at login plus the fallback scheduled task
obit daemon status
obit daemon disable
```

## 状態の照会と診断

```powershell
obit doctor              # read-only check (--json)
obit stats               # eulogy: how much was buried, space freed, resurrections (alias: eulogy)
obit log                 # the full record of everything done
obit config              # show the configuration; --create writes a commented default
obit config telemetry off # disable usage reporting and clear the local installation ID; update checks are separate
obit hook audit          # with OBIT_HOOK_AUDIT=1: what each hook received and did
obit uninstall           # remove everything Obituary set up (try --dry-run first)
```

`list`、`graveyard`、`proposals`、`stats`、`mark`、`bury`、`resurrect` など、状態や処理結果を返すコマンドは、スクリプトやエージェント向けに `--json` をサポートします。`obit rm` などの削除ラッパーは元のコマンドの出力形式を維持するため、`--json` は使えません。

## 環境変数

| 変数 | 効果 |
|---|---|
| `OBIT_DISABLED=1` | フックを一時的に無効にする |
| `OBIT_TELEMETRY=0`、`DO_NOT_TRACK=1` | このプロセスから利用統計を送信しない。CI 環境からは送信されない |
| `OBIT_GUARD_RM=0` | 削除ガードを一時的に無効にする |
| `OBIT_HOOK_AUDIT=1` | 各フックが受け取った内容と実行した処理を記録する |
| `OBIT_TRUST_PROJECT_RULES=1` | CI でプロジェクトの `.obitrules` を信頼する |
| `OBIT_HOME`、`OBIT_CONFIG_DIR`、`OBIT_USER_HOME` | データ、設定、ユーザーのホームディレクトリの場所をそれぞれ変更する |

<!-- generated-command-reference:start -->
## パラメーターリファレンス

### `obit`

Obituary (讣告)。AI エージェント向けのごみ箱で、ファイルに寿命を設定する。寿命が尽きると墓地に埋葬され、そこから復活させられる

使い方: `obit [command]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | obit のヘルプ |
| `--version` | `-v` | `false` | obit のバージョン情報 |

### `obit add`

パスに寿命を設定する（追跡中のパスを再登録すると期限がリセットされる）

別名: `add`, `mark`
使い方: `obit add <path>... <duration> | add <path>... --at <time> [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--at` | なし | `""` | 指定したローカル時刻に期限切れにする："2006-01-02 15:04"、"2006-01-02"、"15:04" のいずれか |
| `--force` | なし | `false` | .git リポジトリを含むフォルダーを許可する |
| `--help` | `-h` | `false` | add のヘルプ |
| `--json` | なし | `false` | JSON を出力する |
| `--no-daemon` | なし | `false` | バックグラウンドのデーモンを起動しない |
| `--note` | なし | `""` | 一覧に表示する自由記述のメモ |

例：

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`

提案を承認する：提案に載っている項目を登録する

使い方: `obit approve <proposal id>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | approve のヘルプ |
| `--json` | なし | `false` | JSON を出力する |

### `obit bury`

パスをすぐに墓地へ移す（obit resurrect で復元できる）

使い方: `obit bury <path>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--force` | なし | `false` | .git リポジトリを含むフォルダーを許可する |
| `--help` | `-h` | `false` | bury のヘルプ |
| `--json` | なし | `false` | JSON を出力する |
| `--note` | なし | `""` | 埋葬に添えて保存する自由記述のメモ |

### `obit cancel`

パスまたはコンテナ ID を助命する：追跡をやめ、対象はそのまま残す

別名: `cancel`, `spare`
使い方: `obit cancel <id|path>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | cancel のヘルプ |
| `--json` | なし | `false` | JSON を出力する |

### `obit completion`

指定したシェル用の obit のオートコンプリートスクリプトを生成する。

使い方: `obit completion [command]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | completion のヘルプ |

### `obit completion bash`

bash シェル用のオートコンプリートスクリプトを生成する。

使い方: `obit completion bash`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | bash のヘルプ |
| `--no-descriptions` | なし | `false` | 補完候補の説明を無効にする |

### `obit completion fish`

fish シェル用のオートコンプリートスクリプトを生成する。

使い方: `obit completion fish [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | fish のヘルプ |
| `--no-descriptions` | なし | `false` | 補完候補の説明を無効にする |

### `obit completion powershell`

powershell 用のオートコンプリートスクリプトを生成する。

使い方: `obit completion powershell [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | powershell のヘルプ |
| `--no-descriptions` | なし | `false` | 補完候補の説明を無効にする |

### `obit completion zsh`

zsh シェル用のオートコンプリートスクリプトを生成する。

使い方: `obit completion zsh [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | zsh のヘルプ |
| `--no-descriptions` | なし | `false` | 補完候補の説明を無効にする |

### `obit config`

設定ファイルと有効な設定を表示する（--create でデフォルト値を書き出す）

使い方: `obit config [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--create` | なし | `false` | デフォルト値で設定ファイルを書き出す |
| `--help` | `-h` | `false` | config のヘルプ |
| `--language` | なし | `""` | 言語を設定する：auto、en、zh-CN のいずれか |

### `obit config telemetry`

利用統計：ランダムなインストール ID、バージョン、OS、アーキテクチャ、表示言語、インストール経路が、インストール数とバージョンごとの利用状況の把握に役立つ。ファイル、パス、コマンドは送信しない。IP アドレスとユーザーエージェントは保存しない。obit config telemetry off または OBIT_TELEMETRY=0 で無効にできる。

使い方: `obit config telemetry on|off [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | telemetry のヘルプ |

### `obit daemon`

バックグラウンドのデーモン（obitd）を制御する

使い方: `obit daemon [command]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | daemon のヘルプ |

### `obit daemon disable`

スケジュールされたタスクと常駐デーモンを削除する

使い方: `obit daemon disable [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--dry-run` | なし | `false` | 変更内容の表示だけを行う |
| `--help` | `-h` | `false` | disable のヘルプ |

### `obit daemon enable`

5 分ごとに動くユーザー単位のスケジューラーを有効にする（Windows ではログイン時に起動するトレイも有効にする）

使い方: `obit daemon enable [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--dry-run` | なし | `false` | 変更内容の表示だけを行う |
| `--help` | `-h` | `false` | enable のヘルプ |
| `--resident` | なし | `false` | 予備のスケジュール実行は変更せず、トレイだけを有効にする |

### `obit daemon start`

デーモンを今すぐ起動する

使い方: `obit daemon start [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | start のヘルプ |

### `obit daemon status`

デーモンの状態と次の期限を表示する

使い方: `obit daemon status [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | status のヘルプ |
| `--json` | なし | `false` | JSON を出力する |

### `obit daemon stop`

デーモンを停止する

使い方: `obit daemon stop [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | stop のヘルプ |

### `obit docker`

Docker コンテナを、不変の完全な ID と作成時刻で追跡する。期限が切れるとコンテナを停止し、書き込み可能なレイヤーは保持する。復活させると起動する。[graveyard] retention_days が過ぎると、永久削除で停止中のコンテナを削除するが、ボリュームは削除しない。ごみ箱モードでも同じ。イメージ、ボリューム、ネットワークは保持される。

使い方: `obit docker [command]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | docker のヘルプ |

### `obit docker add`

Docker コンテナを 1 つ登録する。期限が切れると停止する

使い方: `obit docker add <container name|ID> <lifespan> [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | add のヘルプ |
| `--json` | なし | `false` | JSON を出力する |

### `obit docker list`

登録済みの Docker コンテナを一覧表示する

使い方: `obit docker list [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | list のヘルプ |
| `--json` | なし | `false` | JSON を出力する |

### `obit doctor`

インストール、連携、データを確認する。何も変更しない

使い方: `obit doctor [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | doctor のヘルプ |
| `--json` | なし | `false` | JSON を出力する |
| `--verbose` | なし | `false` | エージェントの機能の詳細を含める |

### `obit extend`

延長する：期限を先へ延ばす（すでに期限を過ぎていれば現在から数える）

使い方: `obit extend <id|path>... <duration> [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | extend のヘルプ |
| `--json` | なし | `false` | JSON を出力する |

例：

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`

git clean のドライランで選ばれたパスを埋葬する

使い方: `obit git-clean [options] [--] [pathspec]... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | git-clean のヘルプ |

### `obit graveyard`

墓地にあり、まだ復活できるものを一覧表示する

別名: `graveyard`, `graves`
使い方: `obit graveyard [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | graveyard のヘルプ |
| `--here` | なし | `false` | 元のパスが現在のディレクトリとその下位にあるものだけを表示する |
| `--json` | なし | `false` | JSON を出力する |

### `obit help`

アプリケーション内の任意のコマンドのヘルプを表示する。

使い方: `obit help [command] [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help のヘルプ |

### `obit hook`

エージェントのフックの入口（エージェントが呼び出すもので、人が使うものではない）

使い方: `obit hook [command]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | hook のヘルプ |

### `obit hook antigravity`

stdin から antigravity のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook antigravity [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | antigravity のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook audit`

フックと削除ガードの監査記録を表示する

使い方: `obit hook audit [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | audit のヘルプ |
| `--lines` | `-n` | `40` | ソースごとの行数 |

### `obit hook claude`

stdin から claude のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook claude [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | claude のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook cline`

stdin から cline のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook cline [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | cline のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook codebuddy`

stdin から codebuddy のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook codebuddy [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | codebuddy のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook codebuddy-ide`

stdin から codebuddy-ide のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook codebuddy-ide [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | codebuddy-ide のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook codex`

stdin から codex のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook codex [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | codex のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook copilot`

stdin から copilot のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook copilot [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | copilot のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook cursor`

stdin から cursor のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook cursor [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | cursor のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook deepseek-harness`

stdin から deepseek-harness のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook deepseek-harness [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | deepseek-harness のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook droid`

stdin から droid のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook droid [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | droid のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook gemini`

stdin から gemini のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook gemini [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | gemini のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook hermes`

stdin から hermes のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook hermes [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | hermes のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook kilocode`

stdin から kilocode のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook kilocode [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | kilocode のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook kimi`

stdin から kimi のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook kimi [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | kimi のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook lingma`

stdin から lingma のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook lingma [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | lingma のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook minimax-code`

stdin から minimax-code のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook minimax-code [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | minimax-code のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook omp`

stdin から omp のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook omp [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | omp のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook openclaw`

stdin から openclaw のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook openclaw [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | openclaw のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook opencode`

stdin から opencode のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook opencode [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | opencode のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook pi`

stdin から pi のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook pi [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | pi のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook qoder`

stdin から qoder のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook qoder [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | qoder のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook qoder-cn`

stdin から qoder-cn のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook qoder-cn [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | qoder-cn のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook trae`

stdin から trae のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook trae [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | trae のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook trae-cn`

stdin から trae-cn のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook trae-cn [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | trae-cn のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook vibe`

stdin から vibe のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook vibe [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | vibe のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook zcode`

stdin から zcode のフックイベントを 1 件処理する。エラー時は止めずに通す

使い方: `obit hook zcode [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | ペイロードにイベント名がないときに使うホスト側のイベント名 |
| `--help` | `-h` | `false` | zcode のヘルプ |
| `--notice-only` | なし | `false` | 注意喚起のコンテキストだけを渡し、削除ガードは働かせない |

### `obit hook finish`

セッションが残したファイルを算出する（フックがバックグラウンドで起動する）

使い方: `obit hook finish [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | finish のヘルプ |
| `--project` | なし | `""` | プロジェクトのルート |
| `--session` | なし | `""` | エージェントのセッション ID |

### `obit init`

エージェント向けに Obituary のフックと指示をインストールする（--uninstall で削除）

使い方: `obit init [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--agent` | なし | `[claude]` | 連携するエージェント（繰り返し指定するか、カンマで区切る。既定は claude） |
| `--context-menu` | なし | `false` | クラシックなエクスプローラーのコンテキストメニューをインストールする（既定で有効） |
| `--dry-run` | なし | `false` | 変更内容の表示だけを行う |
| `--format` | なし | `"text"` | --show の出力形式：text または markdown（エージェントの対応範囲） |
| `--global` | `-g` | `false` | このプロジェクトだけでなく、ユーザー全体に適用する |
| `--guard-rm` | なし | `false` | 対応する削除コマンドを拒否し、復元できる obit のコマンドを代わりに示す |
| `--help` | `-h` | `false` | init のヘルプ |
| `--mcp` | なし | `false` | Obituary の MCP stdio サーバーを登録する |
| `--no-context-menu` | なし | `false` | エクスプローラーのコンテキストメニューをインストールしない |
| `--no-shortcuts` | なし | `false` | スタートメニューとデスクトップのショートカットを作らない |
| `--no-skill` | なし | `false` | Obituary の Agent Skill には手を加えない |
| `--show` | なし | `false` | インストール済みの内容を表示する |
| `--skill` | なし | `false` | Obituary の Agent Skill をインストールする（既定で有効） |
| `--uninstall` | なし | `false` | Obituary が追加したものを削除する |
| `--yes` | なし | `false` | 確認せずに適用する |

例：

```text
obit init -g  # Claude Code、すべてのプロジェクト向け（~/.claude）
obit init  # Claude Code、このプロジェクトだけ（.claude/）
obit init -g --agent codex  # Codex（~/.codex/hooks.json + AGENTS.md）
obit init -g --uninstall  # Obituary が追加したものだけを削除する
obit init -g --show  # インストール済みの内容
obit init --show --format markdown > docs/agents.md
```

### `obit list`

追跡中のパスと Docker コンテナを、期限が近い順に表示する

別名: `list`, `ls`
使い方: `obit list [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | list のヘルプ |
| `--json` | なし | `false` | JSON を出力する |

### `obit log`

Obituary が行ったすべての操作の監査ログを表示する

使い方: `obit log [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | log のヘルプ |
| `--json` | なし | `false` | JSON を出力する |
| `--lines` | `-n` | `20` | 表示するイベント数 |

### `obit mcp`

MCP stdio で Obituary のツールを提供する

使い方: `obit mcp [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | mcp のヘルプ |

### `obit mktemp`

自分で埋葬される一時ディレクトリを作成し、そのパスを出力する

使い方: `obit mktemp <lifespan> [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | mktemp のヘルプ |
| `--json` | なし | `false` | JSON を出力する |
| `--name` | なし | `"tmp"` | ディレクトリ名の接頭辞 |
| `--session` | なし | `""` | ディレクトリが属するエージェントのセッション |

例：

```text
dir=$(obit mktemp 1d)
obit mktemp 2h --name screenshots
```

### `obit proposals`

承認待ちの登録と、確認なしで行われた登録

使い方: `obit proposals [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | proposals のヘルプ |
| `--json` | なし | `false` | JSON を出力する |

### `obit purge`

選択した、または保持期間が過ぎた埋葬済みファイルを削除する。--all と明示的に指定した対象は保持期間を飛ばすため、先に確認する。ターミナル以外では --yes が必要。Docker コンテナは、--all や ID を明示した場合も、[graveyard] retention_days が経過していなければ削除できない。実行中のコンテナは復活済みとして扱う。Docker のコンテナ削除ではボリュームを残し、強制削除は行わない。ごみ箱モードでもコンテナの扱いは同じ。

別名: `purge`, `cremate`
使い方: `obit purge [id|original path]... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--all` | なし | `false` | 保持期間が過ぎたものだけでなく、すべての埋葬を永久削除する |
| `--dry-run` | なし | `false` | 実行される内容を表示するだけにする |
| `--help` | `-h` | `false` | purge のヘルプ |
| `--json` | なし | `false` | JSON を出力する |
| `--yes` | なし | `false` | --all または選択した埋葬を確認せずに永久削除する（エージェントは先にユーザーへ確認する） |

### `obit reject`

提案を却下する

使い方: `obit reject <proposal id>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | reject のヘルプ |
| `--json` | なし | `false` | JSON を出力する |

### `obit remove-item`

PowerShell のパラメーターの一部だけを使って、ファイルシステムの項目を埋葬する。

使い方: `obit remove-item [-Recurse] [-Force] [-Path|-LiteralPath] <path> [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | remove-item のヘルプ |

### `obit resurrect`

上書きせずにファイルを復元する。埋葬された Docker コンテナは ID を指定して起動する

別名: `resurrect`, `raise`
使い方: `obit resurrect <id|original path>... | --last [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | resurrect のヘルプ |
| `--json` | なし | `false` | JSON を出力する |
| `--last` | なし | `false` | 現在のセッションまたは Git プロジェクトで最後に行った削除をまとめて復元する |

### `obit rm`

パスをすぐに墓地へ移す（obit resurrect で復元できる）

使い方: `obit rm [options] <path>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | rm のヘルプ |

### `obit rmdir`

パスをすぐに墓地へ移す（obit resurrect で復元できる）

使い方: `obit rmdir [options] <path>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | rmdir のヘルプ |

### `obit rules`

ここで有効なルールを表示する：組み込みルールと、信頼済みの場合はプロジェクトの .obitrules

使い方: `obit rules [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | rules のヘルプ |
| `--json` | なし | `false` | JSON を出力する |

### `obit rules apply`

ルールに従ってパスを登録する（Git で追跡されているファイルは登録されない）

使い方: `obit rules apply <path>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | apply のヘルプ |
| `--json` | なし | `false` | JSON を出力する |
| `--session` | なし | `""` | パスの出どころのエージェントのセッション |

### `obit rules check`

ルールがパスに対して何を登録するかを表示する（何も変更しない）

使い方: `obit rules check <path>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | check のヘルプ |
| `--json` | なし | `false` | JSON を出力する |
| `--session` | なし | `""` | パスの出どころのエージェントのセッション |

### `obit stats`

Obituary がしたこと：追跡数、埋葬数、取り戻した容量、復活の回数

別名: `stats`, `eulogy`
使い方: `obit stats [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | stats のヘルプ |
| `--json` | なし | `false` | JSON を出力する |

### `obit sweep`

今すぐ刈り取る：バックグラウンド処理を待たず、期限が来たものをすべて埋葬する

別名: `sweep`, `reap`
使い方: `obit sweep [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--dry-run` | なし | `false` | 実行される内容を表示するだけにする |
| `--help` | `-h` | `false` | sweep のヘルプ |
| `--json` | なし | `false` | JSON を出力する |

### `obit trust`

プロジェクトの .obitrules を確認して信頼する（編集後は再確認が必要）

使い方: `obit trust [rules file] [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | trust のヘルプ |
| `--yes` | なし | `false` | 確認せずに信頼する（ルールを確認したあとだけ） |

### `obit uninstall`

Obituary の連携とバックグラウンドサービスを削除する。既定ではデータを残す

使い方: `obit uninstall [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--dry-run` | なし | `false` | アンインストール計画の表示だけを行い、ファイルやサービスは変更しない |
| `--help` | `-h` | `false` | uninstall のヘルプ |
| `--purge-data` | なし | `false` | 一覧にある墓を永久削除し、Obituary 所有のデータを削除する。不明なファイルは残す |
| `--yes` | なし | `false` | 計画を確認する（データの削除は --purge-data も指定した場合のみ） |

### `obit untrust`

プロジェクトの .obitrules を適用しない

使い方: `obit untrust [rules file] [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | untrust のヘルプ |

### `obit update`

安定版のアップデートを確認またはインストールする

使い方: `obit update [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--apply` | なし | `false` | キャッシュ済みの新しい安定版をダウンロードして検証し、インストールする（手動インストールでは Obituary 自身のプログラムだけを置き換える） |
| `--check` | なし | `false` | 今すぐ GitHub を確認する（自動チェックが無効でも確認する） |
| `--help` | `-h` | `false` | update のヘルプ |
| `--relaunch-gui` | なし | `false` | 更新後に小ウィンドウを開き直す（Windows） |
| `--wait-pid` | なし | `0` | 更新の前に、このプロセスが終了するのを待つ（Windows） |

### `obit watch`

フォルダー直下の各項目に、それぞれ寿命を設定する

使い方: `obit watch <directory> <lifespan> | watch --list | watch --remove <directory> [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--age` | なし | `"first-seen"` | 経過時間の基準：first-seen または modified（承認が必要） |
| `--clear-exclusion` | なし | `0` | この除外 ID を明示的に解除する（--policy と併用） |
| `--help` | `-h` | `false` | watch のヘルプ |
| `--json` | なし | `false` | JSON を出力する |
| `--list` | なし | `false` | フォルダーポリシーを一覧表示する |
| `--no-daemon` | なし | `false` | バックグラウンド処理を起動しない |
| `--policy` | なし | `0` | 除外を解除する対象のポリシー ID |
| `--remove` | なし | `""` | ポリシーを削除し、登録済みの寿命を取り消す |
| `--remove-id` | なし | `0` | ID を指定してフォルダーポリシーを削除する。存在しないパスも対象 |

<!-- generated-command-reference:end -->
