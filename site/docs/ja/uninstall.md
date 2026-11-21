# アンインストール

```powershell
obit uninstall --dry-run      # see what would be removed
obit uninstall                # remove everything Obituary set up, after you confirm
.\install.ps1 -Uninstall      # if you used the install script: also remove the program files, PATH entry and Start menu shortcut
```

`obit uninstall` が取り除くのは、Obituary が追加したものだけです。対象はユーザーと現在の Git プロジェクトに設定したエージェントのフック、MCP、Skill のほか、ログイン時の自動起動、スケジュールされたタスク、実行中のデーモン、コンテキストメニュー、エクスプローラーの日付表示、フォルダーポリシーです。

## 墓地と台帳は残ります

既定では墓地と台帳を残し、墓地の場所と保管量を表示します。これらも削除する場合に限り `--purge-data` を付けてください。墓地の内容が一覧表示され、もう一度確認を求められます。

## 他のプロジェクトのフック

他のプロジェクトにもプロジェクト単位でフックを設定した場合は、プログラムを削除する前に、それぞれのプロジェクトで `obit uninstall` を実行してください。先に `obit` を削除すると、それらのフックが動作しなくなります。

## Scoop と winget

先に `obit uninstall` を実行してから、インストールに使ったパッケージマネージャーのアンインストールコマンドを実行してください。
