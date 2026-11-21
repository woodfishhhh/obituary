# 設定リファレンス

Obituary の設定は、設定ディレクトリにある `config.toml` に書きます。このファイルは必須ではありません。ファイルがなくても、数項目しか書いていなくても、残りは既定値で動きます。`obit config` を実行すると、ファイルの場所、ファイルがあるかどうか、各項目が今どの値で動いているかが分かります。`obit config --create` は、すべて既定値のコメント付きファイルを書き出すので、ここから編集を始めるのがいちばん手軽です。

設定ディレクトリは、Windows では `%APPDATA%\obituary`、macOS では `~/Library/Application Support/obituary`、Linux では `~/.config/obituary` です（`XDG_CONFIG_HOME` を設定している場合は、その下の `obituary`）。別の場所に置きたいときは、環境変数 `OBIT_CONFIG_DIR` を設定します。

| キー | 既定値 | 指定できる値 |
| --- | --- | --- |
| `graveyard.mode` | `graveyard` | `graveyard`、`recycle` |
| `graveyard.retention_days` | `7` | 1 から 3650 までの整数 |
| `proposals.mode` | `off` | `off`、`tiered`、`auto`、`ask` |
| `hooks.session_cleanup` | `true` | `true`、`false` |
| `hooks.session_summary` | `false` | `true`、`false` |
| `watch.stable_window` | `10m` | 正の時間。例: `30s`、`10m`、`1h30m` |
| `explorer.enabled` | `true` | `true`、`false` |
| `explorer.everywhere` | `false` | `true`、`false` |
| `explorer.in_git` | `false` | `true`、`false` |
| `ui.language` | `auto` | `auto`、`en`、`zh-CN` |
| `ui.tray` | `true` | `true`、`false` |
| `ui.taskbar` | `true` | `true`、`false` |
| `update.check` | `true` | `true`、`false` |
| `telemetry.enabled` | `true` | `true`、`false` |
| `agent_shells` | 未設定 | テーブル。キーはエージェント名、値は `bash` または `powershell` |
| `codex_windows_shell` | 未設定 | `bash`、`powershell` |

## 期限が切れたものの行き先

既定では、寿命が尽きたものは Obituary 自身の墓地に入ります。`graveyard.retention_days` 日の間（既定は 7 日）は `obit resurrect` で元の場所に戻せて、それを過ぎてはじめて完全に削除されます。日数は 1 から 3650 の範囲で変えられます。

`graveyard.mode` を `recycle` にすると、期限が切れたものはシステムのごみ箱に渡され、Windows が管理します。この設定があるのは Windows だけで、macOS と Linux には墓地しかありません。ごみ箱に入ったものはごみ箱から元に戻します。`obit resurrect` では見つかりません。

## 自動登録とエージェントのセッション

`proposals.mode` は、Obituary がファイルに自分で寿命を付けるかどうかを決めます。既定の `off` では何も自動では行わず、エージェントやあなたがコマンド、コンテキストメニュー、フォルダーポリシーで登録したものだけが対象になります。`tiered` では、エージェントがそのセッションで作ったファイルと、セッションが起動したコンテナーがそのまま登録され、あとで知らされます。不要なら `obit spare` で取り消せます。それ以外は `obit proposals` で承認を待ちます。`auto` は、自動登録が見つけたものをすべてそのまま登録し、あとで知らせます。`ask` は、1 件ずつ先に確認します。どのモードでも、Git が追跡しているファイルや、依存関係、ビルド出力、キャッシュ（`node_modules`、`dist`、`__pycache__` など）は自動では登録されません。詳しくは[登録の対象](./auto-register)を参照してください。

`hooks.session_cleanup` は既定で有効です。各セッションの終わり近くに、エージェントは、そのセッションで作った、寿命がなく Git も追跡していないファイルを知らされ、作業用のファイルに寿命を付ける（`obit mark`）か埋葬するよう求められます。成果物はそのまま残ります。`hooks.session_summary` は既定で無効です。有効にすると、セッションの開始時に、プロジェクト内で 24 時間以内に期限が切れるものと、最近埋葬されたものも知らされます。既定で無効なのはトークンを節約するためです。エージェントが埋葬済みのパスに触れたときは、どのみち通知されます。

## フォルダーポリシーとエクスプローラーの日付

`watch.stable_window` は、フォルダーポリシーの待ち時間です。新しいファイルが現れてから、これだけの間変化がなかった場合にはじめて移動されるので、ダウンロード中のファイルが持ち去られることはありません。Go の時間の書式を使い、数字のあとに `s`、`m`、`h` を付けます。`1h30m` のようにつなげて書くこともでき、0 より大きくなければなりません。

`explorer.*` は Windows でのみ意味があります。Obituary は、エクスプローラーでファイル名の横に埋葬日を表示させますが、実際のファイル名は変わりません。既定では、ダウンロード、デスクトップ（サブフォルダーを含む）、フォルダーポリシーが管理する場所だけに表示されます。`explorer.everywhere` を有効にすると、追跡中のほかの場所にも表示されます。Git ワークスペースは既定では表示されず、表示したい場合は `explorer.in_git` を `true` にします。`explorer.enabled` を `false` にすると、まったく書き込まなくなります。[フォルダーポリシー](./folder-policies)を参照してください。

## インターフェース

`ui.language` は、コマンドライン、小窓、トレイ、コンテキストメニューの言語を決めます。今のところ `en` と `zh-CN` だけです。`auto` は、環境変数 `OBIT_LANG`、`LC_ALL`、`LC_MESSAGES`、`LANG` の順に見て、最後にシステムの表示言語を見ます。`ui.tray` は、トレイアイコン（macOS ではメニューバーのアイコン）の表示を切り替えます。非表示にしても、バックグラウンドのクリーンアップと更新確認は続きます。`ui.taskbar` は Windows だけで有効で、小窓を開いている間にタスクバーのボタンを出すかどうかを決めます。

## 更新確認と使用統計

`update.check` が有効なとき、Obituary は GitHub の公開リリース情報を 1 日に最大 1 回だけ読み、使用データは一切送りません。無効にしても、`obit update --check` でいつでも手動で確認できます。`telemetry.enabled` は匿名の使用統計を制御します。更新確認とは別のスイッチで、`obit config telemetry off` は統計だけを止め、このコンピューターにあるインストール ID と送信待ちのデータを消去します。詳しくは[プライバシーポリシー](./privacy)を参照してください。

## Windows でエージェントが使うシェル

削除ガードは、エージェントが実行しようとしている削除コマンドを読み取れて初めて、同等の `obit` コマンドを案内できます。Windows のエージェントは bash を使うことも PowerShell を使うこともあり、Obituary には区別できないため、あなたが宣言します。このエージェントはこの Windows マシン上で、このシェルでコマンドを実行する、という宣言です。宣言がない場合、意味があいまいなコマンドはブロックされずに通過します。

```toml
[agent_shells]
codex = "powershell"
```

`agent_shells` のキーはエージェント名です。現在これを読むのは、`codex`、`codebuddy`、`codebuddy-ide`、`minimax-code` です。`codex_windows_shell` は Codex だけに使う旧来の書き方で、`agent_shells.codex` と同時に異なる値で書くと、設定は拒否されます。

## 書き間違えたとき

Obituary は、知らないキーを黙って無視することはしません。無視してしまうと、`retention = 30` と書いても 7 日の保持期間がそのまま有効で、30 日になったと思い込むことになるからです。そのため `obit config` は、認識できないキーについて、ファイルのパス、行番号、キー名の全体を報告し、綴りが近いときは正しいキーを提案します。

```text
obit: config file has errors: …\config.toml:2: unknown key "graveyard.retention" (did you mean "graveyard.retention_days"?)
```

値が範囲外のときも同様に、どの項目で何が許されるかが示されます。たとえば `graveyard.retention_days must be between 1 and 3650` です。TOML の構文エラーでも `obit config` は失敗します。ファイルを直してもう一度実行し、エラーが出なくなれば直っています。

## よく使うコマンド

```powershell
obit config                  # ファイルの場所、データディレクトリ、有効な値
obit config --create         # コメント付きの既定ファイルを書き出す
obit config --language en    # 表示言語を保存する: auto、en、zh-CN
obit config telemetry off    # 使用統計をオフにする（on で再びオン）
```

`--create` は既存のファイルを上書きしません。ファイルがすでにあるときは、エラーを報告して終了します。`--language` は、ファイルがまだないときは既定の内容と一緒に作成します。
