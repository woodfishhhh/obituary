# トラブルシューティング

困ったときは、まず `obit doctor` を実行してください。読み取りだけで何も書き込まず、インストール、エージェント連携、バックグラウンドのスケジュール、データを項目ごとに確認します。各行は `PASS`、`INFO`、`WARN`、`FAIL` のいずれかで始まり、そのあとに理由と対処が書かれています。`FAIL` があると、コマンドは 0 以外の終了コードで終わります。結果をほかの人に見せたいときは `--json` を、各エージェントでどの機能がつながっているかを見たいときは `--verbose` を付けてください。

以下の各項目は、見えること、理由、対処、直ったかどうかの確かめ方、の順に書いています。

## ターミナルで `obit` が見つからない

ターミナルに `command not found` と出る、あるいは PowerShell が `obit` を認識しない場合、プログラムはディスクにあるものの、そのフォルダーが PATH に入っていません。Windows のインストールスクリプトは `%LOCALAPPDATA%\Programs\obituary` をユーザーの PATH に追加しますが、すでに開いているターミナルには新しい PATH が反映されません。`install.sh` はプログラムを `~/.local/bin` に置き、PATH は一切変更しません。このフォルダーが最初から PATH に入っていないシステムもあります。Ubuntu の `.deb` は `/usr/libexec/obituary/` に置きますが、ここも PATH には入りません。

Windows では、ターミナルを閉じて開き直せば済みます。macOS と Linux では、`~/.local/bin` をシェルの設定に追加します。zsh なら `~/.zshrc` に `export PATH="$HOME/.local/bin:$PATH"` と 1 行書き、新しいターミナルを開いてください。`.deb` だけを入れた場合は、フルパスの `/usr/libexec/obituary/obit` を使うか、インストールスクリプトでコマンドラインを別に入れます。

`obit --version` がバージョンを表示し、`obit doctor` の `path` の行が `WARN` でなくなれば、直っています。

## `obit doctor` が `obitd` を見つけられない

`obit doctor` に `FAIL obitd` と出るのは、`obit` の隣にバックグラウンドプログラムの `obitd` がないということです。Obituary は同じフォルダーの中でこれを探すので、`obit` だけをコピーしても足りません。`obit` と `obitd` を一緒に置くか、インストールした方法に合わせてインストールをやり直してください。もう一度 `obit doctor` を実行して、その行が `PASS` になれば直っています。

## ダウンロードしたファイルがチェックサムに通らない

インストールスクリプトを使う場合、この確認はスクリプト自身が行い、ハッシュが合わなければ何もインストールせずに止まります。アーカイブ、DMG、`.deb` を自分でダウンロードしたときだけ、手動で確認が必要です。合わない原因は、ダウンロードが途中で切れた、途中でファイルが書き換えられた、`checksums.txt` が別のバージョンのものだった、のいずれかであることがほとんどです。ハッシュはバージョンごとに計算されるからです。

同じリリースのページからパッケージと `checksums.txt` をダウンロードし直して（中国本土ではミラーからも取得できます）、もう一度確認してください。それでも合わなければ実行しないでください。下の `VERSION` とファイル名は、実際にダウンロードしたものに置き換えます。Linux の場合:

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict -
```

macOS では、最後の行の `sha256sum --check --strict -` を `shasum -a 256 -c -` に置き換えます。Windows PowerShell の場合:

```powershell
$file = "obituary_VERSION_windows_amd64.zip"
(Get-FileHash $file -Algorithm SHA256).Hash -eq ((Select-String -Path checksums.txt -SimpleMatch $file).Line -split '\s+')[0]
```

ハッシュが一致すると、Linux と macOS ではファイル名のあとに `OK`、PowerShell では `True` と表示されます。

## Windows に「Windows によって PC が保護されました」と出る

ブラウザーで手動ダウンロードしたアーカイブは、Windows SmartScreen に止められることがあります。Obituary のプログラムにまだコード署名がないためです。Scoop、winget、インストールスクリプトで入れた場合は、まず出ません。先に上の方法で SHA-256 を確認し、一致していたら「詳細情報」を選んでから「実行」を選びます。確認できない場合や、「不明な発行元」というだけでなくマルウェアとして報告された場合は、実行しないでください。

## Windows の小窓が開かない

スタートメニューやトレイで Obituary をクリックしても何も起きない、またはランタイムが足りないと表示される場合です。ウィンドウは Microsoft Edge WebView2 ランタイムで描画されます。Windows 11 には標準で入っており、Windows 10 でもほとんどのマシンに入っていますが、ない環境ではウィンドウが起動しません。Microsoft のサイトから WebView2 ランタイム（Evergreen 版）をダウンロードしてインストールし、もう一度 Obituary をクリックしてください。ウィンドウが開けば直っています。

ウィンドウは今のところ x64 のみです。ARM64 の Windows 用アーカイブには `obit-gui.exe` が入っていないので、そちらではコマンドラインを使います。トレイメニューの「Open Obituary」で、ウィンドウのプログラムが見つからないという通知が出るのは、`obit-gui.exe` が `obitd.exe` の隣にないからで、ARM64 のアーカイブはまさにその構成です。x64 では、この 2 つを同じフォルダーに置いてください。

## macOS: app が現在の macOS と互換性がないと表示される

1.6.0 の macOS の小窓（DMG 内の `Obituary.app`）には macOS 26 が必要です。macOS 15 以前で開こうとすると、この app は現在のバージョンと互換性がないというメッセージが出て、起動しません。これは 1.6.0 のパッケージング時の問題で、1.6.1 で修正されます。それまでこのウィンドウを使うには macOS 26 が必要です。問題があるのは DMG 内の小窓だけです。1.6.1 以降を入れて app が開けば、直っています。

## macOS:「開発元を検証できません」

DMG の app には Developer ID の署名がなく、公証もされていないため、初回は Gatekeeper に止められます。SHA-256 を確認したうえで、まず普通に一度開いてください。開発元を検証できないと表示されたら、「システム設定 → プライバシーとセキュリティ」を開き、ページの下の方にある Obituary で「このまま開く」を選び、もう一度確認します。手順は[未署名 app を初めて開くとき](./desktop#未署名-app-を初めて開くとき)にあります。そのために Gatekeeper を全体で無効にしないでください。app が破損している、あるいはマルウェアを含むと表示される場合は別の問題なので、許可しないでください。

## トレイがログイン時に起動しない（macOS、Ubuntu）

ログインしても、メニューバーやトレイに Obituary のアイコンが出ません。ログイン時の起動は、起動エントリに頼っています。macOS ではユーザーの LaunchAgent `site.woodfish.obituary.tray`、Ubuntu では `~/.config/autostart/obituary.desktop` です。小窓の Settings のスイッチがオフだったり、app を移動したり、DMG の中から直接実行している間にオンにしたりすると、エントリに記録されたパスが古くなり、ログイン時に何も見つからなくなります。

小窓を開き、Settings で「Start the tray at sign-in」を一度オフにして、もう一度オンにしてください。Obituary がエントリを書き直します。macOS では `launchctl list | grep obituary` に `site.woodfish.obituary.tray` が表示されるはずです。Ubuntu では `ls ~/.config/autostart/obituary.desktop` でファイルが見つかり、次のログインでトレイが現れます。

## Ubuntu: 小窓が開かない、またはトレイアイコンがない

アプリケーション一覧で Obituary をクリックしても何も起きない場合、たいてい GTK 3 か WebKitGTK 4.1 が足りません。`apt install ./….deb` ならこれらも一緒に入るので、足りなくなるのは、あとから削除された場合か、インストールが途中で失敗した場合だけです。Ubuntu 24.04 では GTK 3 のパッケージ名は `libgtk-3-0t64` で、`t64` の付かない `libgtk-3-0` はこのリリースには存在しないため、古い手順をそのまま入力すると失敗します。

```bash
sudo apt update
sudo apt install libgtk-3-0t64 libwebkit2gtk-4.1-0 xdg-utils
```

もう一度アプリケーション一覧から開いて、ウィンドウが表示されれば直っています。ウィンドウは開くのにトレイアイコンがないのは、トレイにはデスクトップ側が StatusNotifierItem を提供している必要があり、GNOME は既定では提供していないからです。拡張機能を入れなくても、アプリケーション一覧から同じウィンドウを開けます。アイコンが欲しい場合は、AppIndicator など、それに対応した拡張機能を GNOME に入れてください。

## 期限が切れたものが埋葬されない

ファイルの寿命はとっくに切れているのに元の場所にあり、`obit daemon status` の `runner:` の行が `off` になっています。期限の処理は、5 分ごとに動く定期ジョブが担当します。Windows ではタスク スケジューラの「Obituary Runner」、macOS では launchd のユーザーエージェント、Linux では systemd のユーザータイマー `obituary.timer` です。これがインストールされていない、オフになっている、別のデータディレクトリ向けに入っている、という状態では、時間が来ても誰も動きません。その場合、期限切れのものは、次の `obit sweep` かエージェントのフックが動くまで待たされます。

`obit daemon enable` で定期ジョブをインストールしてください（Windows ではログイン時のトレイも入ります）。今すぐ片付けたい場合は `obit sweep` を実行します。systemd のユーザーサービスがない Linux（一部のコンテナーや最小構成のシステムなど）では `obit daemon enable` でインストールできないので、代わりに cron で 5 分ごとに `obitd --once --home <データディレクトリ> --config-dir <設定ディレクトリ>` を実行します。2 つのディレクトリは `obit config` で分かります。`obit daemon status` の `runner` の行が `off` でなくなれば直っています。Linux では `systemctl --user list-timers | grep obituary` でもタイマーを確認できます。

## 期限が切れたのに埋葬できなかった

小窓の「For you」ページの「Burials that failed」に、その項目が理由付きで表示されます。多くは、エディター、同期クライアント、ほかのプログラムがファイルを開いているためで、ファイルは元の場所に残っています。埋葬はファイルを移動することなので、使用中だったり権限がなかったりすると失敗します。Obituary は無理に進めません。元のファイルには手を付けず、しばらくしてから、間隔を少しずつ延ばしながら再試行し、失敗が続くと止まってあなたの判断を待ちます。

ファイルを開いているものを閉じて、「Retry」をクリックするか、`obit sweep` を実行してください。もう扱ってほしくないときは「Stop tracking」をクリックします。ファイルは元の場所に残り、カウントダウンは終わります。その項目が「Burials that failed」から消えて `obit graveyard` に現れれば、埋葬されています。

## 復活したファイルの名前が変わった

`obit resurrect` はエラーを出していないのに、戻ってきたファイルが `report (resurrected).txt` という名前になっています。元の場所に同じ名前のものがすでにあったためです。復活は決して上書きしないので、名前が使われているときは同じフォルダーに別の名前で戻し、どちらの中身も無傷です。両方を見比べて、必要なほうを残し、名前を付け直してください。

## 設定ファイルに間違いがある

`obit config` が `config file has errors` と表示し、そのあとにファイルのパス、行番号、キー名が続きます。`config.toml` に Obituary が知らないキーや、許されない値があります。Obituary はこれを黙って無視しません。無視すると、`retention = 30` と書いても 7 日の保持期間が有効なままで、変わったと思い込むことになるからです。示された行の綴り、引用符、時間の書式を直して、もう一度 `obit config` を実行してください。エラーが出なくなり、`obit doctor` の `config` の行が `PASS` になれば直っています。各項目の書き方は[設定リファレンス](./config)にあります。

## フックが効かない

`obit init` のあと、エージェントがファイルを削除するのを止められなかった、あるいは何も起きていないように見える場合です。原因として多いのは 3 つです。エージェントを再起動しておらず、古い設定のままである。このエージェントの連携が、別のスコープ（グローバルかプロジェクトか）にインストールされている。エージェントの環境に `OBIT_DISABLED=1`（すべてのフックを無効にする）や `OBIT_GUARD_RM=0`（削除ガードを無効にする）が設定されている。

まず、何がインストールされているかを確認します。

```bash
obit init -g --show        # グローバルにインストールされているもの
obit init --show           # 現在のプロジェクトにインストールされているもの
obit doctor --verbose      # 各エージェントの接続状況
```

インストールされていなければ、`obit init -g --agent <名前>` でインストールします。先に計画を見たいときは `--dry-run` を付けます。そのあとエージェントを再起動してください。Codex では、中で `/hooks` を実行し、Obituary のフックを信頼する必要もあります。フックが実際に何を受け取っているかを見るには、エージェントを起動するターミナルで `OBIT_HOOK_AUDIT=1` を設定し、しばらく使ってから `obit hook audit` を実行します。拒否された削除コマンドは、このスイッチがなくても常に記録されます。フック自身のエラーはデータディレクトリの `hook.log` に書かれ、`obit doctor` の `hook-log` の行が直近 24 時間のエラー数を数えます。

`obit doctor` のそのエージェントの行（たとえば `claude`）が `PASS` で `integration is current` と書かれていれば、インストールされています。削除ガードを使っている場合は、`claude-guard-rm` のような行に `registered: yes` と出るはずです。エージェントを起動するのと同じターミナルで `obit doctor` を実行してください。環境変数でフックが無効になっていると、その行が `WARN` になり、変数名が表示されます。

## Docker コンテナーの提案を承認できない

小窓で、コンテナーの提案の「Approve」がグレーアウトし、カードに「Docker is not running: start Docker to decide on this proposal.」と出ています。コンテナーを登録するには、Docker にそのフル ID と作成時刻を問い合わせる必要があり、Docker が止まっているとそれができません。Docker Desktop か Docker Engine を起動してから、もう一度承認してください。「Approve」がクリックでき、`obit docker list` にそのコンテナーが表示されれば、成功です。

コンテナーの期限が切れると、Obituary はそれを停止して書き込み可能レイヤーを残すだけで、復活すると再び起動します。停止したコンテナーが削除されるのは、保持期間が過ぎてからです。イメージ、ボリューム、ネットワークには一切触れません。

## 更新に失敗する、または中国本土でのダウンロードが遅い

`obit update --check` がエラーを報告する、あるいはインストールスクリプトが遅い、失敗する場合は、たいていこのコンピューターが GitHub に届いていません。GitHub の応答が遅いと、`obit update --check` とインストールスクリプトは自動的に中国本土のミラーに切り替わります。最初からミラーを使うには、macOS と Linux では `bash install.sh --mirror cn`、Windows では `.\install.ps1 -Mirror cn` を実行します。手動でダウンロードする場合は、アーカイブと `checksums.txt` を同じリリース、同じ入手先から取ってください。`obit update --check` が `Latest:` の行を表示すれば、直っています。
