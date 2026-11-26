# Docker コンテナーとボリューム

エージェントのセッション中に起動したコンテナにも寿命を設定できます。

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## コンテナー作成時に寿命を登録する

一時コンテナーは起動時に寿命を決めておかないと止め忘れがちです。`docker run -d` は完全なコンテナー ID を標準出力に返すため、そのまま `obit docker add` に渡せます。

```powershell
# PowerShell
$cid = docker run -d --name tmp-redis redis:7
if ($LASTEXITCODE -eq 0) { obit docker add $cid 4h }
```

```bash
# Bash / zsh (no name required; pass the returned ID directly)
cid=$(docker run -d redis:7) && obit docker add "$cid" 4h
```

`obit docker add <name-or-id> <lifespan>` はコンテナー名、短い ID、または完全な 64 文字 ID を受け取り、`--at` や `--note` オプションはありません。登録すると台帳番号と期限時刻（`#<id> docker:<name> (<12-char-id>)  expires <time>`）を表示します。`docker run --rm` は付けないでください。AutoRemove 付きのコンテナーは停止と同時に Docker が削除するため、`obit docker add` は登録を拒否します。期限前に延長または追跡解除する場合は、`obit docker list` か `obit list` に出た番号で `obit extend <id> <lifespan>` または `obit spare <id>` を実行します。

## 保持期間中に停止済みコンテナーを確認・復活する

期限切れで `docker stop` されたコンテナーは、稼働中の追跡一覧である `obit docker list` から墓地へ移り、`[graveyard] retention_days`（既定は 7 日間）の保持期間に入ります。保持期間中の停止済みコンテナーを確認・復活するには次のコマンドを使います。

```powershell
obit graveyard                 # list all restorable burials; containers show as docker:<name> (<12-char-id>) with purge countdown
obit graveyard --json          # machine-readable output; filter burials where kind == "container"
obit stats                     # total graveyard count and the next purge time
obit resurrect <id>            # start the stopped container by its numeric graveyard ID and stop tracking it
```

`obit docker add` で手動登録したコンテナーはプロジェクトディレクトリに紐付かないため、`obit graveyard --here` には表示されません。`--here` に一致するのはプロジェクト経由で自動登録または提案承認されたコンテナーだけです。デスクトップ版ではトレイメニューで墓地の件数と直近 10 件までの埋葬項目を確認でき（項目をクリックするとそのまま復活します）、ウィンドウの「Graveyard」ページでは名前で検索し、残りの保持期間を確認して選んで復活できます。コンテナー停止時や永久削除の直前にバックグラウンドからシステム通知は送られないため、保持期間中に見直すときは `obit graveyard` かウィンドウの墓地ページを確認してください。

## ボリュームに寿命を設定する・すぐに埋葬する

ボリュームは自動登録されません。明示的に登録または埋葬したものだけが対象です。イメージやネットワークは削除しません。

`obit mark --docker-volume cache 3d`（`add` も可）で `cache` を3日後に埋葬できます。すぐに埋葬するには `obit bury --docker-volume -- cache` を実行します。

Docker は Linux コンテナーモードで使ってください。対象は追加オプションのない `local` ドライバーのローカルボリュームだけです。初回は補助イメージが手元になければ、固定バージョンの Debian イメージを取得します。

墓地には完全なアーカイブを保存できる空き容量が必要です。アーカイブを作成して検証できた場合にのみボリュームを削除します。停止中も含めてコンテナーから参照されている場合や、パスが bind マウントと重なる場合はそのまま残します。

`obit resurrect <ID>` は元の名前でボリュームを作り直します。復活したボリュームには `io.obituary.restore` ラベルが恒久的に付きます。同名のボリュームがあれば、空でも上書きしません。

削除の前後に Docker のイベントを照合します。削除を確認できなければ、検証済みのアーカイブを残して `obit doctor` を確認してください。埋葬が済んだとはみなせません。 `docker-volume-recovery` の警告は要確認の記録ごとに表示されます。指示に従い、元の Docker デーモンでボリュームを確認してください。未記帳のアーカイブは `obit resurrect` や `obit purge` では扱えません。

削除ガードは `docker volume rm`、`docker volume remove`、`docker volume prune`、`docker compose down -v`（または `--volumes`）、`docker system prune --volumes` を認識します。単純な `docker volume rm` で名前が明示されている場合に限り、`obit bury --docker-volume` を提案します。

## 期限が切れたら停止

- `docker stop` の後もコンテナとその書き込み可能なレイヤーは残ります。`obit resurrect <id>` は `docker start` を実行し、寿命の追跡を終了します。
- `docker rm` でコンテナを削除するのは、保持期間（`[graveyard] retention_days`、既定は 7 日間）が過ぎてからです。`-v` と `-f` は使いません。
- 保持期間中にコンテナを起動すると、その後また停止しても削除されず、寿命の追跡が終了します。旧版が停止したコンテナには照合用の起動時刻がないため削除されません。引き続き `obit resurrect <id>` で復活できます。
- `obit purge` や `--all` を指定してもコンテナの保持期間は短縮できません。ごみ箱モードも対象外で、まず停止し、保持期間後に削除します。

## 完全な ID で識別

- 名前は表示にのみ使います。停止、起動、削除の前には必ず完全な ID と作成日時を照合し、記録と一致しなければ操作しません。
- コンテナが属する Docker コンテキストとデーモンを記録し、そのデーモンにだけ操作を行います。`docker context` を切り替えても操作先は変わりません。
- Obituary 以外でコンテナを削除した場合は、外部で削除されたことを記録してから、追跡対象から外します。
- `--rm`（AutoRemove）付きのコンテナは停止時に Docker 自身が削除するため、登録できません。

## セッション中のコンテナを自動登録する

自動登録は既定で無効です。Obituary はコンテナを探しませんが、`obit docker add` で個別に登録できます。有効にすると、セッション終了時に、その間に作成されたコンテナをバックグラウンドで探します。対象は Compose の作業ディレクトリかバインドマウントがプロジェクト内にあるものだけです。それ以外は別の場所で起動した可能性があるため何もしません。該当するコンテナは寿命 3 日で登録し、後から通知します。`obit spare <id>` で取り消すと、同じセッションでは再登録しません。セッションを Git プロジェクトの外（ホームディレクトリなど）で開始した場合や、複数のセッションが同じコンテナを作成したと判断された場合は、起動元を特定できないため提案にとどめ、承認を待ちます。`[proposals] mode = "ask"` の場合も提案のみで、`obit approve` の後に追跡を始めます。

提案はコンテナごとに一件までです。フックが Docker を呼び出すことはありません。Docker が未インストールまたは停止中なら通知せずに処理をスキップし、理由は `obit doctor` で確認できます。停止中はコンテナの提案を承認できません。`obit proposals` とウィンドウの「For you」にその旨が表示され、Docker を起動すれば承認できます。
