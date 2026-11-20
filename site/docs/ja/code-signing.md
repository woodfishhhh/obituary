# コード署名ポリシー

[SignPath.io](https://about.signpath.io/) がコード署名を無償で提供し、[SignPath Foundation](https://signpath.org/) が証明書を発行します。

## 署名の対象

- リリースの配布アーカイブに含まれる `obit.exe`、`obitd.exe`、`obit-gui.exe`。
- GitHub Actions のリリースワークフロー（[`release.yml`](https://github.com/woodfishhhh/obituary/blob/main/.github/workflows/release.yml)）が、このリポジトリのタグ付きコミットからのみ、これらのプログラムを自動ビルドします。ビルドスクリプトと CI の設定も同じリポジトリにあります。
- 署名要求は、以下の承認担当者が一件ずつ手動で承認します。
- ファイルの「プロパティ」→「詳細」で、製品名「Obituary 讣告」とバージョンを確認できます。

## チームの役割

| 役割 | メンバー |
| --- | --- |
| コミッターとレビュアー | [woodfishhhh](https://github.com/woodfishhhh) |
| 署名の承認担当者 | [woodfishhhh](https://github.com/woodfishhhh) |

チーム外からの変更は、メンテナーがレビューしてからマージします。メンバー全員が GitHub で多要素認証を使用しています。

## プライバシー

利用統計の送信と更新の自動確認は個別に無効にできます。詳しくは[プライバシーポリシー](privacy)をご覧ください。
