# 登録の対象

## 登録はユーザーとエージェントが決め、埋葬は期限どおりに

既定では、何を登録するかはユーザーとエージェントが決めます（`[proposals] mode = "off"`）。

- **エージェントは自分の成果物を登録します。** デバッグスクリプト、ログ、スクリーンショットには `obit mark <path> 2d` で寿命を付けます。一時作業用の場所は `obit mktemp 1d` で作り、必要か判断できない項目は `obit bury` で埋葬します。`obit init` がエージェント用に作成する説明にも、この手順が書かれています。
- **ユーザーはコンテキストメニューから追加できます。** エクスプローラー、Finder、Nautilus、Dolphin でファイルやフォルダーを右クリックし（複数選択も可）、「Add to Obituary」から 1 日、7 日、30 日、または「Bury now (recoverable)」を選びます。[ウィンドウ、トレイ、右クリックメニュー](./desktop)を参照してください。
- **フォルダーポリシー**で「ダウンロード」などのフォルダーに寿命を設定できます。詳しくは[フォルダーポリシー](./folder-policies)を参照してください。

登録した項目は期限が来ると埋葬され、その後 7 日間は墓地から元の状態に戻せます。それ以外のものが勝手に登録されることはありません。

- **セッションの終わりに整理します。** Claude Code や Codex のエージェントが終了しようとすると、そのセッションで作成した未分類のファイルが一覧になります。デバッグスクリプト、ログ、スクリーンショットに寿命を設定するか埋葬し、成果物は残すようエージェントに知らせます。通知はセッションにつき一度までで、対象がなければ通知しません。他のエージェントでは次のセッション開始時に通知します。`[hooks] session_cleanup = false` で無効にできます。
- **旧版からアップグレードすると**、自動登録は一度だけ無効になります。その後の設定変更は維持されます。以前自動登録された依存ファイル、ビルド出力、キャッシュの登録は解除され、寿命のない状態で元の場所に残ります。

## 必要に応じて自動登録を有効にする

常駐プロセスは書き込み後に自動登録を処理するため、新しいファイルが `obit list` に表示されるまで少し時間がかかることがあります。

ウィンドウの Settings または For you で切り替えるか、`config.toml` の `[proposals] mode` を設定します。

- `"tiered"`：セッション中にエージェントが新規作成したファイルと、プロジェクト用に起動した [Docker コンテナ](./docker)を直接登録します。ファイルはフックが書き込みを検知し、開始時には存在せず、ファイルシステム上の作成時刻が開始後であるものに限ります。登録後にエージェントに通知し、`obit proposals` と For you の「Handled automatically」に表示します。すでにあったファイルやフックが書き込みを確認できなかったファイルは提案にとどまります。`obit approve <id>` で提案の項目を登録すると、そのプロジェクトで以後ルールに一致するものは直接登録できます。`obit reject <id>` はこのプロジェクトで今後自動登録しないという指定です。
- `"auto"`：自動登録で見つかった項目をすべて直接登録し、後から通知します。
- `"ask"`：まず提案として表示し、すべて承認を待ちます。

```powershell
obit proposals           # proposals waiting for you, and what was registered without asking
obit approve 3           # approve: register what it lists
obit reject 3            # reject
obit spare 12            # undo an automatic registration: it stays where it is, without a lifespan
obit extend 12 1w        # keep it longer
```

- **依存ファイル、ビルド出力、キャッシュ**（`node_modules`、`dist`、`.turbo`、`__pycache__` など）は、どのモードでも自動登録も提案もしません。整理の対象にするには自分で `obit mark` を実行します。
- **大きな項目**（1 GB 超または 200 ファイル超）をエージェント以外が作成した場合も、まず提案します。
- **セッション前後を比較します。** 開始時にスナップショットをデータディレクトリに保存し、終了時に比較します。リポジトリには何も追加しません。ルールに一致したものはモードに従って処理し、一致しない新規ファイルは登録も提案もしません。
- **モードを切り替えると**、未処理の提案をバックグラウンドで新しいモードに従って再分類します。
- **埋葬前にもう一度 Git に確認します。** ルールや提案で登録した項目が期限までにコミットされた場合は埋葬しません。
- **提案は自動的に失効します。** コミットまたは削除された項目は除かれ、空になった提案は取り下げます。監査ログには記録しますが、却下には数えません。

ボリュームは自動登録されません。明示的に登録または埋葬したものだけが対象です。イメージやネットワークは削除しません。

## ルール

```powershell
obit rules                       # the rules in effect here (built-in plus a trusted .obitrules)
obit rules check debug.log out/  # preview what would be registered and by which rule (changes nothing)
obit rules apply debug.log       # register by rule
```

`obit rules apply` はいつでもルールに沿って登録できます。自動登録が有効ならフックも同じルールを使います。組み込みルールは `.playwright-mcp/`、`test-results/`、`*.log` など、エージェントがよく残す作業ファイルを対象にします。編集時のバックアップ（`*.bak`、`*.bak.*`、`*.orig`、`*.rej`、`*.old`、`*.backup`、`*~`）は 7 日間保持しますが、LevelDB の `LOG.old` には触れません。プロジェクト直下の `.obitrules` では `.gitignore` と同じ書式でパターンを書き、その後ろに寿命を指定します。

```
output/**        7d      # every item under output gets its own lifespan
!output/keep/**          # never touch (negation wins, even over registering a parent folder)
test-results/    off     # turn off a built-in rule
```

`.gitignore` とは異なり、最後に一致したルールよりも除外ルールが優先されます。Windows では大文字と小文字を区別しません。閉じていない `[` など無効なパターンがあると、エラーにファイル名と行番号が表示され、そのプロジェクトの自動登録が一時停止します。

期間の単位は `s`、`m`、`h`、`d`、`w` です。`1d12h` のように組み合わせられます。

## 自動登録で必ず守られる条件

- 対象は Git の作業ツリー内の未追跡項目だけです。
- Git の追跡対象を一つでも含むフォルダーには手を加えません。
- Git の回答を得られなければスキップします。
- すでに設定された寿命を短くすることはありません。
- 一度却下したプロジェクトは、どのモードでも直接登録しません。フォルダーポリシーの適用と `.obitrules` を信頼するかどうかは必ずユーザーが決めます。

## プロジェクトのルールには明示的な信頼が必要

リポジトリに含まれる `.obitrules` は、内容を確認して `obit trust` を実行するまでは無効です。他人のリポジトリのルールで私用ファイルが移動されるのを防ぎます。信頼したファイルの SHA-256 を記録し、変更があれば再確認を求めます。非対話セッションでは `--yes` が必要です。CI では `OBIT_TRUST_PROJECT_RULES=1` を設定できます。

## 期限が来ると埋葬される一時フォルダー

```powershell
$dir = obit mktemp 1d            # .obit/tmp/ in a project (not in Git), otherwise the system temp folder
```

Bash の場合は `dir=$(obit mktemp 1d)` と書きます。
