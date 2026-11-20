# 墓地からの通知

埋葬しても、後からその事実が分からなければ復活できません。エージェントのツールが、埋葬済みでまだ復活できるパスや、埋葬したディレクトリ内の項目にアクセスすると、Obituary は次の情報を伝えます。

- 墓の番号
- 埋葬した日時
- 復活できる残り時間
- 復活コマンド `obit resurrect <id>`

## 通知するだけです

- ツールの呼び出しをブロックしたり書き換えたりはしません。
- 同じ墓について通知するのは、セッションごとに一度だけです。
- Claude Code と Codex にはツールの実行前に通知します。他のエージェントへの通知タイミングは `obit init --show` で確認できます。
- それ以外のときは作業を妨げません。エージェントは Obituary が使えることだけ知っていればよく、作業の前に一覧を読む必要はありません。

すでにインストールしている場合は `obit init` を再実行すると通知が有効になります。

## セッションの概要（任意）

トークンを節約するため、セッション開始時に伝えるのは、判断が必要な提案と直近に自動登録した項目だけです。概要も表示するには、小ウィンドウの設定で「Session summary」を有効にするか、次のように設定してください。

```toml
[hooks]
session_summary = true
```

これで、このプロジェクトで 24 時間以内に期限が切れる項目と、最近埋葬した項目のうちまだ復活できるものも通知されます。`obit mktemp` の一時ディレクトリは対象外です。

## 自分で確認する

```powershell
obit graveyard           # what is in the graveyard and when it will be purged
obit graveyard --here    # this folder only
obit resurrect <id>      # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last    # undo the last deletion: everything one obit bury / obit rm buried comes back
```
