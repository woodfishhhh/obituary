# Docker コンテナーとボリューム

エージェントのセッション中に起動したコンテナにも寿命を設定できます。

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## ボリュームに寿命を設定する・すぐに埋葬する

ボリュームは自動登録されません。明示的に登録または埋葬したものだけが対象です。イメージやネットワークは削除しません。

`obit mark --docker-volume cache 3d`（`add` も可）で `cache` を3日後に埋葬できます。すぐに埋葬するには `obit bury --docker-volume -- cache` を実行します。

Docker は Linux コンテナーモードで使ってください。対象は追加オプションのない `local` ドライバーのローカルボリュームだけです。初回は補助イメージが手元になければ、固定バージョンの Debian イメージを取得します。

墓地には完全なアーカイブを保存できる空き容量が必要です。アーカイブを作成して検証できた場合にのみボリュームを削除します。停止中も含めてコンテナーから参照されている場合や、パスが bind マウントと重なる場合はそのまま残します。

`obit resurrect <ID>` は元の名前でボリュームを作り直します。復活したボリュームには `io.obituary.restore` ラベルが恒久的に付きます。同名のボリュームがあれば、空でも上書きしません。

削除の前後に Docker のイベントを照合します。削除を確認できなければ、検証済みのアーカイブを残して `obit doctor` を確認してください。埋葬が済んだとはみなせません。

削除ガードは `docker volume rm`、`docker volume remove`、`docker volume prune`、`docker compose down -v`（または `--volumes`）、`docker system prune --volumes` を認識します。単純な `docker volume rm` で名前が明示されている場合に限り、`obit bury --docker-volume` を提案します。

## 期限が切れたら停止

- `docker stop` の後もコンテナとその書き込み可能なレイヤーは残ります。`obit resurrect <id>` は `docker start` を実行し、寿命の追跡を終了します。
- `docker rm` でコンテナを削除するのは、保持期間（`[graveyard] retention_days`、既定は 7 日間）が過ぎてからです。`-v` と `-f` は使いません。
- その時点ですでにコンテナが動いていれば、誰が起動したかにかかわらず復活済みとみなし、削除しません。
- `obit purge` や `--all` を指定してもコンテナの保持期間は短縮できません。ごみ箱モードも対象外で、まず停止し、保持期間後に削除します。

## 完全な ID で識別

- 名前は表示にのみ使います。停止、起動、削除の前には必ず完全な ID と作成日時を照合し、記録と一致しなければ操作しません。
- コンテナが属する Docker コンテキストとデーモンを記録し、そのデーモンにだけ操作を行います。`docker context` を切り替えても操作先は変わりません。
- Obituary 以外でコンテナを削除した場合は、外部で削除されたことを記録してから、追跡対象から外します。
- `--rm`（AutoRemove）付きのコンテナは停止時に Docker 自身が削除するため、登録できません。

## セッション中のコンテナを自動登録する

自動登録は既定で無効です。Obituary はコンテナを探しませんが、`obit docker add` で個別に登録できます。有効にすると、セッション終了時に、その間に作成されたコンテナをバックグラウンドで探します。対象は Compose の作業ディレクトリかバインドマウントがプロジェクト内にあるものだけです。それ以外は別の場所で起動した可能性があるため何もしません。該当するコンテナは寿命 3 日で登録し、後から通知します。`obit spare <id>` で取り消すと、同じセッションでは再登録しません。セッションを Git プロジェクトの外（ホームディレクトリなど）で開始した場合や、複数のセッションが同じコンテナを作成したと判断された場合は、起動元を特定できないため提案にとどめ、承認を待ちます。`[proposals] mode = "ask"` の場合も提案のみで、`obit approve` の後に追跡を始めます。

提案はコンテナごとに一件までです。フックが Docker を呼び出すことはありません。Docker が未インストールまたは停止中なら通知せずに処理をスキップし、理由は `obit doctor` で確認できます。停止中はコンテナの提案を承認できません。`obit proposals` とウィンドウの「For you」にその旨が表示され、Docker を起動すれば承認できます。
