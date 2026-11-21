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
## アンインストール範囲とデータの場所

`obit uninstall --dry-run` は現在のユーザーと Git プロジェクトから削除できる項目を表示します。実行すると hook、MCP、Skill、トレイ、自動起動、スケジューラーを外しますが、他のプロジェクトの設定は変更しません。各プロジェクトで実行してください。

| 場所または要素 | Windows | macOS | Linux |
| --- | --- | --- | --- |
| プログラム | `obit`、`obitd`、任意の `obit-gui` | `/Applications/Obituary.app` または CLI ディレクトリ | CLI ディレクトリ、デスクトップは `.deb` |
| hook、MCP、Skill | ユーザーまたはプロジェクトの agent 設定 | agent 設定 | agent 設定 |
| スケジューラー | タスク、常駐プロセス、トレイ | LaunchAgent とメニューバー | systemd ユーザータイマー、XDG 自動起動 |
| プロジェクト設定 | `.claude/`、`.codex/` の Obituary 部分 | プロジェクト設定 | プロジェクト設定 |

データは Windows の `%LOCALAPPDATA%\obituary`、macOS の `~/Library/Application Support/obituary`、Linux の `$XDG_DATA_HOME/obituary`（未設定時 `~/.local/share/obituary`）です。設定ディレクトリは `OBIT_CONFIG_DIR`、データは `OBIT_HOME`、ユーザーホームは `OBIT_USER_HOME` で変更できます。同じボリュームの `.obituary` に墓地がある場合もあります。

墓地、台帳、監査ログ、設定は既定で残ります。`--purge-data` は所有を確認できる墓地データだけを削除し、未知のファイルは残します。recycle モードではシステムのごみ箱から戻し、`resurrect` は使えません。
