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

以下の表は、現在のソースから構築した `obit --help` の出力をもとに生成されています。

### `obit add`
別名: `add`, `mark`
使い方: `obit add <path>... <duration> \| add <path>... --at <time> [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--at` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--force` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |
| `--no-daemon` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--note` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

例：

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`
使い方: `obit approve <proposal id>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

### `obit bury`
使い方: `obit bury <path>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--force` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |
| `--note` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit cancel`
別名: `cancel`, `spare`
使い方: `obit cancel <id\|path>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

### `obit completion`
使い方: `obit completion [command]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit completion bash`
使い方: `obit completion bash`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--no-descriptions` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit completion fish`
使い方: `obit completion fish [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--no-descriptions` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit completion powershell`
使い方: `obit completion powershell [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--no-descriptions` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit completion zsh`
使い方: `obit completion zsh [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--no-descriptions` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit config`
使い方: `obit config [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--create` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--language` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit config telemetry`
使い方: `obit config telemetry on\|off [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit daemon`
使い方: `obit daemon [command]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit daemon disable`
使い方: `obit daemon disable [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--dry-run` | なし | `false` | 実行内容だけを表示し、ファイルやサービスを変更しません。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit daemon enable`
使い方: `obit daemon enable [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--dry-run` | なし | `false` | 実行内容だけを表示し、ファイルやサービスを変更しません。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--resident` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit daemon start`
使い方: `obit daemon start [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit daemon status`
使い方: `obit daemon status [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

### `obit daemon stop`
使い方: `obit daemon stop [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit docker`
使い方: `obit docker [command]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit docker add`
使い方: `obit docker add <container name\|ID> <lifespan> [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

### `obit docker list`
使い方: `obit docker list [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

### `obit doctor`
使い方: `obit doctor [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |
| `--verbose` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit extend`
使い方: `obit extend <id\|path>... <duration> [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

例：

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`
使い方: `obit git-clean [options] [--] [pathspec]... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit graveyard`
別名: `graveyard`, `graves`
使い方: `obit graveyard [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--here` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

### `obit help`
使い方: `obit help [command] [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit hook`
使い方: `obit hook [command]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit hook antigravity`
使い方: `obit hook antigravity [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook audit`
使い方: `obit hook audit [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--lines` | `-n` | `40` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook claude`
使い方: `obit hook claude [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook cline`
使い方: `obit hook cline [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook codebuddy`
使い方: `obit hook codebuddy [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook codebuddy-ide`
使い方: `obit hook codebuddy-ide [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook codex`
使い方: `obit hook codex [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook copilot`
使い方: `obit hook copilot [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook cursor`
使い方: `obit hook cursor [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook droid`
使い方: `obit hook droid [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook gemini`
使い方: `obit hook gemini [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook hermes`
使い方: `obit hook hermes [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook kilocode`
使い方: `obit hook kilocode [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook kimi`
使い方: `obit hook kimi [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook lingma`
使い方: `obit hook lingma [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook minimax-code`
使い方: `obit hook minimax-code [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook omp`
使い方: `obit hook omp [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook openclaw`
使い方: `obit hook openclaw [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook opencode`
使い方: `obit hook opencode [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook pi`
使い方: `obit hook pi [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook qoder`
使い方: `obit hook qoder [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook qoder-cn`
使い方: `obit hook qoder-cn [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook trae`
使い方: `obit hook trae [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook trae-cn`
使い方: `obit hook trae-cn [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook vibe`
使い方: `obit hook vibe [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit hook zcode`
使い方: `obit hook zcode [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--event` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--notice-only` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit init`
使い方: `obit init [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--agent` | なし | `[claude]` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--context-menu` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--dry-run` | なし | `false` | 実行内容だけを表示し、ファイルやサービスを変更しません。 |
| `--format` | なし | `"text"` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--global` | `-g` | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--guard-rm` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--mcp` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--no-context-menu` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--no-shortcuts` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--no-skill` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--show` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--skill` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--uninstall` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--yes` | なし | `false` | 確認して実行し、再確認しません。 |

例：

```text
obit init -g                 # Claude Code, for every project (~/.claude)
obit init                    # Claude Code, this project only (.claude/)
obit init -g --agent codex   # Codex (~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall     # remove only what Obituary added
obit init -g --show          # what is installed
obit init --show --format markdown > docs/agents.md
```

### `obit list`
別名: `list`, `ls`
使い方: `obit list [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

### `obit log`
使い方: `obit log [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |
| `--lines` | `-n` | `20` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit mcp`
使い方: `obit mcp [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit mktemp`
使い方: `obit mktemp <lifespan> [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |
| `--name` | なし | `"tmp"` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--session` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

例：

```text
obit mktemp 2h --name screenshots
```

### `obit proposals`
使い方: `obit proposals [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

### `obit purge`
別名: `purge`, `cremate`
使い方: `obit purge [id\|original path]... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--all` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--dry-run` | なし | `false` | 実行内容だけを表示し、ファイルやサービスを変更しません。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |
| `--yes` | なし | `false` | 確認して実行し、再確認しません。 |

### `obit reject`
使い方: `obit reject <proposal id>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

### `obit remove-item`
使い方: `obit remove-item [-Recurse] [-Force] [-Path\|-LiteralPath] <path> [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit resurrect`
別名: `resurrect`, `raise`
使い方: `obit resurrect <id\|original path>... \| --last [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |
| `--last` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit rm`
使い方: `obit rm [options] <path>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit rmdir`
使い方: `obit rmdir [options] <path>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit rules`
使い方: `obit rules [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

### `obit rules apply`
使い方: `obit rules apply <path>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |
| `--session` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit rules check`
使い方: `obit rules check <path>... [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |
| `--session` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit stats`
別名: `stats`, `eulogy`
使い方: `obit stats [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

### `obit sweep`
別名: `sweep`, `reap`
使い方: `obit sweep [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--dry-run` | なし | `false` | 実行内容だけを表示し、ファイルやサービスを変更しません。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |

### `obit trust`
使い方: `obit trust [rules file] [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--yes` | なし | `false` | 確認して実行し、再確認しません。 |

### `obit uninstall`
使い方: `obit uninstall [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--dry-run` | なし | `false` | 実行内容だけを表示し、ファイルやサービスを変更しません。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--purge-data` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--yes` | なし | `false` | 確認して実行し、再確認しません。 |

### `obit untrust`
使い方: `obit untrust [rules file] [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | ヘルプを表示します。 |

### `obit update`
使い方: `obit update [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--apply` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--check` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--relaunch-gui` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--wait-pid` | なし | `0` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

### `obit watch`
使い方: `obit watch <directory> <lifespan> \| watch --list \| watch --remove <directory> [flags]`

| パラメーター | 短縮 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `--age` | なし | `"first-seen"` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--clear-exclusion` | なし | `0` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--help` | `-h` | `false` | ヘルプを表示します。 |
| `--json` | なし | `false` | JSON 形式で出力します。 |
| `--list` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--no-daemon` | なし | `false` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--policy` | なし | `0` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--remove` | なし | `""` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |
| `--remove-id` | なし | `0` | このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。 |

<!-- generated-command-reference:end -->
