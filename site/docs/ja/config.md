# 設定リファレンス

Obituary は設定ディレクトリの `config.toml` を読み込みます。すべてのキーは省略でき、省略時は次の既定値を使います。`obit config` でファイルの場所と有効な値を確認できます。

## キー

| キー | 既定値 | 値または形式 | 動作 |
| --- | --- | --- | --- |
| `codex_windows_shell` | 未設定 | `bash`、`powershell` | Windows の Codex が使う shell を宣言します。未設定なら曖昧な削除コマンドを通します。 |
| `agent_shells.<agent>` | 未設定 | `bash`、`powershell` | hook agent ごとの Windows shell を宣言します。 |
| `graveyard.mode` | `graveyard` | `graveyard`、`recycle` | 期限切れの項目を墓地またはシステムのごみ箱へ移します。 |
| `graveyard.retention_days` | `7` | `1`〜`3650` の整数 | 墓地の項目を復元できる日数です。 |
| `watch.stable_window` | `10m` | `30s`、`10m`、`1h` などの Go duration | フォルダー方針で移動する前に待つ安定時間です。正数が必要です。 |
| `explorer.enabled` | `true` | `true`、`false` | Explorer の日付ラベルを書き込みます。 |
| `explorer.everywhere` | `false` | `true`、`false` | 対応するすべてのフォルダーで日付を表示します。 |
| `explorer.in_git` | `false` | `true`、`false` | Git worktree 内でも日付を表示します。 |
| `ui.language` | `auto` | `auto`、`en`、`zh-CN` | CLI、ウィンドウ、トレイの言語です。 |
| `ui.tray` | `true` | `true`、`false` | トレイまたはメニューバー入口を有効にします。 |
| `ui.taskbar` | `true` | `true`、`false` | Windows ウィンドウを個別のタスクバー項目にします。 |
| `hooks.session_summary` | `false` | `true`、`false` | セッション開始時に概要を表示します。 |
| `hooks.session_cleanup` | `true` | `true`、`false` | このセッションで作成され未分類のファイルを整理します。 |
| `proposals.mode` | `off` | `off`、`tiered`、`auto`、`ask` | 自動登録を承認待ちにする範囲です。 |
| `update.check` | `true` | `true`、`false` | 安定版の更新を自動確認します。 |
| `telemetry.enabled` | `true` | `true`、`false` | 使用統計の送信を許可します。更新確認とは独立しています。 |

`agent_shells` は agent 名をキーにしたテーブルです。例: `[agent_shells] codex = "powershell"`。`codex_windows_shell` は旧形式で、異なる値を同時に指定するとエラーになります。

## 形式とエラー

duration は Go の形式です。`ns`、`us`、`ms`、`s`、`m`、`h` を使い、`1h30m` のように組み合わせられます。保持日数は `1`〜`3650`、`graveyard.mode`、`ui.language`、`proposals.mode` は表の値だけを受け付けます。

未知のキーは無視されません。設定ファイルのパス、行番号、キー名を含むエラーになり、近い綴りなら候補も表示します。TOML の構文エラーでも `obit config` は失敗します。

## よく使う操作

```powershell
obit config
obit config --create
obit config --language en
obit config telemetry off
obit config telemetry on
```

`--create` はファイルが存在しない場合だけコメント付き既定テンプレートを書きます。`--language` は表示言語を変更し、値は `auto`、`en`、`zh-CN` です。`telemetry on|off` は `telemetry.enabled` だけを変更し、`[update] check` には影響しません。
