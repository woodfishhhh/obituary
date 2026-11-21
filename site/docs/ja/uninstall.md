# アンインストール

アンインストールは 2 段階です。まず Obituary が設定したものを取り除き、次にプログラム本体を削除します。墓地と台帳は既定で残るので、埋葬したファイルはその後も復活できます。

```powershell
obit uninstall --dry-run      # 取り除くものを一覧するだけで、何も変更しない
obit uninstall                # 計画を表示し、確認後に実行する
```

Windows でインストールスクリプトを使った場合は、次の 1 つで済みます。

```powershell
.\install.ps1 -Uninstall      # obit uninstall --yes を実行したあと、プログラムのファイル、追加した PATH のエントリ、スタートメニューのショートカットを削除する
```

## 取り除かれるもの

`obit uninstall` が扱うのは、ユーザー単位と現在の Git プロジェクトにあるものです。各エージェントのフック、MCP、Skill、5 分ごとに動く定期ジョブ（Windows ではタスク スケジューラ、macOS では launchd のユーザーエージェント、Linux では systemd のユーザータイマー）、常駐プロセスとトレイ、そしてフォルダーポリシー（ポリシーの下のファイルはそのまま残ります）です。Windows では、ログイン時の自動起動、エクスプローラーのコンテキストメニュー、日付ラベルも取り除きます。

触れるのは、Obituary 自身が書いたものだけです。あなたが編集したエントリや、ほかのインストールが残したエントリはそのまま残り、どれを残したかとその理由が計画に書かれます。ターミナルのない環境（スクリプトやエージェントのコマンドライン）では `--yes` を付けてください。付けないと、ターミナルが必要だと表示して止まります。

自動では行われないことが 3 つあります。プログラムのファイルは、インストールした方法に合わせて削除します（下記）。ほかのプロジェクトにプロジェクト単位のフックを入れた場合は、プログラムを削除する前に、それぞれのプロジェクトで `obit uninstall` を実行してください。そうしないと、`obit` がなくなってそちらのフックが動かなくなります。そして macOS と Linux では、トレイの自動起動のエントリは一覧に含まれません。アンインストールの前に、小窓の Settings で「Start the tray at sign-in」をオフにしてください。Obituary が書いたエントリが削除されます。Windows ではトレイの自動起動を `obit uninstall` が処理しますが、`obit init` が作ったスタートメニューとデスクトップのショートカットは一覧に入っていません。これらは `obit init --uninstall` で削除されますが、`--no-shortcuts` を付けると残せます。

## 墓地、台帳、設定

既定で残るのは 3 つです。墓地のファイル、台帳と監査ログ、そして `config.toml` です。`obit uninstall` は、墓地にまだどれだけ入っているか、どこにあるかを表示します。あとで再インストールすれば、それらをそのまま使えます。ごみ箱モードを使っている場合、期限が切れたものはすでにシステムのごみ箱にあります。アンインストールはそれに触れず、戻すときもごみ箱から行います。`obit resurrect` は届きません。

すべて消したいときは、`--purge-data` を付けます。

```powershell
obit uninstall --purge-data
```

まず墓地の全項目を一覧し、アンインストール計画を確認したあとで、もう一度確認します。確認のあと、何かを取り除く前に、Obituary は触れるものすべてを検査します。データディレクトリが Obituary 自身のものか、中のファイルがすべて認識できるものか、墓地フォルダーと削除対象の墓の一つひとつを確認できるか、です。どれか一つでも通らなければ、そこで止まって理由を示し、墓地、台帳、サービスはそのままなので、途中まで削除されることはありません。すべて通ったときにはじめて、それらのファイルを完全に削除し、続けて台帳、監査ログ、そのほか Obituary 自身のファイルを削除します。スクリプトでは `--yes --purge-data` を一緒に指定すると、2 回の確認が省かれます。

削除するのは Obituary が認識できるファイルだけです。データディレクトリにほかのファイル、リンク、知らないサブフォルダーが混ざっていると検査が止まり、どれが原因かが表示されます。`config.toml` は削除の対象に入らず、台帳と同じフォルダーにあっても（macOS の既定）移す必要はありません。削除のあと、そのフォルダーには `config.toml` だけが残ります。設定が別の場所にあるときは、データディレクトリごと削除されます。

## プログラムの削除

プログラムの削除方法は、インストールした方法によって決まります。

| インストール方法 | 削除方法 |
| --- | --- |
| Windows のインストールスクリプト | `.\install.ps1 -Uninstall` |
| Scoop | `obit uninstall` のあと `scoop uninstall obituary` |
| winget | `obit uninstall` のあと `winget uninstall woodfishhhh.Obituary` |
| macOS の小窓（DMG） | `obit uninstall` を実行して「Start the tray at sign-in」をオフにし、「アプリケーション」の Obituary.app をゴミ箱へドラッグ |
| Homebrew | `obit uninstall` のあと `brew uninstall --cask obituary` |
| macOS と Linux の `install.sh` | `obit uninstall` のあと、`~/.local/bin` の `obit` と `obitd` を削除 |
| Ubuntu のデスクトップパッケージ（`.deb`） | 「Start the tray at sign-in」をオフにしてトレイを終了し、`sudo apt remove obituary-desktop` |

macOS の app と Ubuntu の `.deb` は、それぞれ独自の `obit` を含んでいますが、どちらも PATH には入りません。コマンドラインを別にインストールしていない場合は、アンインストールの前にフルパスで実行してください。app の中のものは `/Applications/Obituary.app/Contents/MacOS/obit`、`.deb` のものは `/usr/libexec/obituary/obit` です。`apt remove` はそのフォルダーのプログラムを削除しますが、墓地、設定、ほかの方法で入れたコマンドラインには触れません。

## データの場所

| | データ（台帳、墓地、ログ） | 設定（`config.toml`） |
| --- | --- | --- |
| Windows | `%LOCALAPPDATA%\obituary` | `%APPDATA%\obituary` |
| macOS | `~/Library/Application Support/obituary` | 同じフォルダー |
| Linux | `~/.local/share/obituary` | `~/.config/obituary` |

Linux で `XDG_DATA_HOME` や `XDG_CONFIG_HOME` を設定している場合は、それぞれの下の `obituary` フォルダーです。環境変数 `OBIT_HOME` と `OBIT_CONFIG_DIR` で、データと設定を別の場所に移せます。どちらを使っているかは `obit config` が表示します。データディレクトリがあるドライブでは、墓地はその中の `graveyard` フォルダーです。ほかのドライブでは、そのドライブの直下にある隠しフォルダー `.obituary` が墓地になります。こうすることで、埋葬は常に同じドライブ内での名前変更になり、コピーは発生しません。
