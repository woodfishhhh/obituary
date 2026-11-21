# プライバシーポリシー

Obituary の台帳、墓地、ログ、設定は端末内に保存されます。クラッシュレポート、ファイル、パス、ファイルの内容をアップロードすることはありません。

## 利用統計

統計は初期設定で有効です。利用中のインストール数とバージョン分布の把握にのみ使います。実際に利用したとき、24 時間に 1 回を上限として、ランダムなインストール ID、バージョン、OS、アーキテクチャ、表示言語、インストール経路を送信することがあります。ファイル、パス、コマンドは送信せず、IP アドレスや User-Agent ヘッダーも保存しません。`obit config telemetry off` を実行するか `OBIT_TELEMETRY=0` を設定すると送信を停止できます。CI 環境や `DO_NOT_TRACK=1` を設定した環境では送信しません。

サーバーには IP アドレス、User-Agent、元のインストール ID を保存しません。独立した鍵で計算したダイジェストで同じインストールからの報告を照合できますが、個人を特定するものではありません。日ごとの利用記録は 400 日で削除されます。重複を除くため、ダイジェストと最初・最後に観測した日付だけを長期保存します。送信に成功した件数は経路ごとの合計だけを記録します。認証が必要なダッシュボードを閲覧できるのはメンテナーだけです。

統計を無効にすると、端末内のインストール ID と未送信の報告を削除します。更新の確認は別の設定です。

## 新しいバージョンの確認

- 1 日に最大 1 回、`https://api.github.com` から Obituary リポジトリの最新リリースの公開情報を取得します。GitHub が応答しない場合は中国本土のミラーの `https://cn.obituary.woodfish.site/download/latest.json` を参照します。
- リクエストに含まれるのは通常のウェブリクエストと同じ情報だけです。IP アドレスと、Obituary のバージョンを示す User-Agent（例：`Obituary/1.0.0`）です。
- 自動確認はいつでも停止できます。ウィンドウの設定で「Check for updates automatically」をオフにするか、`%APPDATA%\obituary\config.toml` に次の設定を追加してください。

  ```toml
  [update]
  check = false
  ```

  停止後は、自分で `obit update --check` を実行したときだけ確認します。

## インストールとダウンロード

- Scoop、winget、インストールスクリプトは [GitHub Releases](https://github.com/woodfishhhh/obituary/releases) からアーカイブを取得します。[GitHub のプライバシー声明](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement)が適用されます。
- ミラー（`-Mirror cn`）を使うと、取得元は `cn.obituary.woodfish.site` になります。このサーバーは障害調査のために通常のアクセスログ（時刻、IP アドレス、要求された URL）を保存します。
- アプリ内の更新（ウィンドウの「Update now」または「Download and restart to update」、`obit update --apply`）は、リリースのアーカイブと `checksums.txt` を GitHub Releases から取得します。GitHub が 10 秒以内に応答しない場合は `cn.obituary.woodfish.site` から取得します。リクエストに含まれるのは IP アドレスと Obituary のバージョンを示す User-Agent だけで、利用データは送信しません。
- リリース時に GitHub Actions がミラーにファイルを転送します。ミラーは自身のサーバーでファイルを検証して配信し、利用者のダウンロード要求を GitHub に転送しません。

## エージェントとの接続

`obit init` は変更予定を先に示し、端末内のエージェントの設定ファイルだけを変更します。提供元など外部への送信はありません。

## ウェブサイト

- <https://obituary.woodfish.site/> はアクセス解析や追跡スクリプトを使いません。中国本土では当方のサーバー、それ以外では GitHub Pages が配信し、どちらも通常のアクセスログを記録します。
- サイトで登録したメールアドレスは、新しいプラットフォームやバージョンの案内だけに使います。当方のサーバーに保存し、第三者に提供しません。すべてのメールに配信停止リンクがあります。

## お問い合わせ

プライバシーに関する質問は [GitHub の Issue](https://github.com/woodfishhhh/obituary/issues) へお寄せください。セキュリティ上の問題は [SECURITY.md](https://github.com/woodfishhhh/obituary/blob/main/SECURITY.md) の手順で非公開で報告してください。
