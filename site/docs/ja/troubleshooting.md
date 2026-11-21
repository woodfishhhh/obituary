# トラブルシューティング

まず `obit doctor` を実行してください。インストール、連携、バックグラウンドサービス、データを変更せずに確認します。詳細には `--json` または `--verbose` を付けます。

## PATH とチェックサム

**症状:** `obit` が見つからない、またはアーカイブのチェックサムが一致しない。

**対処:** 同じリリースの `checksums.txt` を取得し、ダウンロード先で `sha256sum --check --strict checksums.txt` を実行します。Windows PowerShell では `Get-FileHash .\obit-*.zip -Algorithm SHA256` を比較します。一致しないファイルは実行せず、PATH を変更したら新しいターミナルを開きます。

## デスクトップ

Windows SmartScreen の未署名警告は SHA-256 を確認してから「詳細情報 → 実行」を選びます。WebView2 がない場合は Microsoft Edge WebView2 Runtime を入れます。Windows の小窓は現在 x64 のみです。

macOS アプリは Developer ID 署名と公証がありません。チェックサムを確認し、「システム設定 → プライバシーとセキュリティ → このまま開く」で初回起動を許可します。Gatekeeper を無効にしたり、破損・マルウェア警告のアプリを許可したりしないでください。メニューバーがログイン時に起動しない場合は `obit daemon status` と `launchctl list | grep obituary` を確認します。

Ubuntu 24.04 の小窓には GTK 3 と WebKitGTK 4.1 が必要です。

```bash
sudo apt update
sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
obit doctor
```

`systemctl --user status obituary.timer`、`systemctl --user list-timers | grep obituary`、`crontab -l` でスケジューラーを調べます。

## バックグラウンド、更新、ファイル

`obit daemon status` で状態を確認し、すぐ処理する場合は `obit sweep`、スケジューラーを戻す場合は `obit daemon enable` を使います。中国向けミラーは `bash install.sh --mirror cn` です。異なるリリースのアーカイブとチェックサムを組み合わせないでください。

WSL のマウントドライブ名を変更した後は、本当の Windows パスを確認してから操作します。占有中のファイルは元の場所に残り、再試行されます。TOML の誤りは `obit config` がファイルと行を示します。

## 連携とコンテナー

hook が動かない場合は `obit init --show`、`obit init --dry-run`、`obit doctor --verbose` を実行し、`OBIT_DISABLED=1` が設定されていないことを確認します。監査には `OBIT_HOOK_AUDIT=1 obit hook audit` を使います。Docker を起動してからコンテナーを承認してください。復元先が存在する場合、`obit resurrect` は上書きせず、衝突しない名前で戻します。
