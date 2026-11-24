# クイックスタート

Obituary はファイルとフォルダーに寿命を設定します。寿命が切れると墓地に埋葬されます。保管期間（既定では 7 日間）中ならいつでも復活でき、本当に削除されるのはその後です。AI エージェント向けに作られているため、エージェントが作成するデバッグスクリプト、ログ、スクリーンショット、テスト出力には作成時点から寿命が付きます。片付けを覚えておく必要はなく、エージェントの `rm -rf` で何かが永久に失われることもありません。

## 1. インストール

Windows 10 / 11、macOS、Linux（x64 と ARM64）：

| | Windows | macOS | Linux |
|---|---|---|---|
| インストール方法 | Scoop、`install.ps1` スクリプト（winget は審査中） | Homebrew、`install.sh` スクリプト | `install.sh` スクリプト |
| 期限の確認 | 常駐トレイプロセス、タスク スケジューラが代替 | ユーザーの launchd エージェント | `systemd --user` タイマー、systemd がない場合はユーザーの crontab |
| CLI、墓地と復活、自動登録（任意）、削除ガード、エージェント連携（hook、Skill、MCP）、Docker コンテナ、フォルダーポリシー | あり | あり | あり |
| トレイ、小さなウィンドウ、コンテキストメニュー、Explorer の埋葬日時、スタートメニューとデスクトップショートカット | あり | CLI を使用：`obit list`、`obit graveyard`、`obit resurrect` | macOS と同じ |
| ごみ箱モード | あり | 墓地のみ | 墓地のみ |

**簡単な方法：エージェントに任せる。** 使用中のエージェント（Claude Code、Codex、Cursor など）にこの文を送ってください。Obituary をインストールして自分自身に接続し、あなたの判断が必要なところで停止します。

```text
Read https://obituary.woodfish.site/agent-install.md and follow it to install Obituary and connect it to yourself.
```

**手動で:** 以下からオペレーティングシステムを選び、`obit doctor` を実行してインストールを確認します。

### macOS / Linux

macOS では、Homebrew を使えば 1 行でインストールできます。

```bash
brew install --cask woodfishhhh/tap/obituary
```

または、インストールスクリプトをダウンロードして読み、実行します（macOS と Linux）。

```bash
curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh
bash install.sh
# Custom directory: --install-dir "$HOME/bin"; preview only: --dry-run
```

スクリプトは既定で最新リリースをインストールします（`--version <version>` で別のバージョンを選べます）。アーカイブは 4 つの部分に分けて同時にダウンロードされ（経路によっては各接続の速度が制限されます）、うまくいかない場合は 1 接続に戻ります。OS とアーキテクチャを検出し、同じダウンロード元の SHA-256 を検証して、`obit` と `obitd` を `~/.local/bin` にインストールします。PATH の案内に従ってから `obit --version` を実行してください。中国本土のミラーを使うには `--mirror cn` を指定します。スクリプトは `https://cn.obituary.woodfish.site/install.sh` にあります。

macOS で初めて使うときは、`obit --version` と `obitd --help` を実行してください。Gatekeeper が許可を求めたら、システム設定 → プライバシーとセキュリティを開き、出所を確認した各実行ファイルで「このまま開く」を選んでから、コマンドをもう一度実行します。

WSL では、プロジェクトとエージェントの設定を `~/projects` など Linux ネイティブのディレクトリに置いてください。`/mnt/c` などの DrvFS マウントでは、インストールに必要な安全な名前変更ができない場合があります。これらのドライブ上のファイルの寿命設定・埋葬・復活には Windows 版のみを使用してください。

Docker コンテナの overlayfs が永続的なファイル識別子を提供できない場合、`obit mark` は登録を拒否し、ファイルは元の場所に残ります。期限が切れたファイルを復活できるようにするには、ファイルと Obituary のデータディレクトリを、ext4 など永続的な識別子を提供するファイルシステムのボリュームまたはバインドマウントに置いてください。

### Windows

::: code-group

```powershell [Scoop]
scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket
scoop install obituary
```

```powershell [winget]
# In review, available once approved
winget install woodfishhhh.Obituary
```

```powershell [Install script]
# Install script: download it, read it, then run it
# (installs to %LOCALAPPDATA%\Programs\obituary, adds it to your user PATH, creates a Start menu shortcut)
irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1
.\install.ps1          # installs the latest release; -WhatIf previews, -NoPath leaves PATH alone
```

:::

- 3 つとも GitHub リリースから同じアーカイブをダウンロードし、展開前に SHA-256 を検証します。一致しなければインストールを停止します。
- コード署名：無料のコード署名は [SignPath.io](https://about.signpath.io/) が提供し、証明書は [SignPath Foundation](https://signpath.org/) が提供します。[コード署名ポリシー](code-signing)と[プライバシーポリシー](privacy)を参照してください。
- Obituary はまだコード署名されていません。ブラウザーでアーカイブをダウンロードすると、Windows が「Windows によって PC が保護されました」と表示することがあります。「詳細情報 → 実行」を選択してください。Scoop、winget、インストールスクリプトでは通常表示されません。
- 小さなウィンドウ `obit-gui.exe` には WebView2 ランタイムが必要です（Windows 11 には組み込まれており、ほとんどの Windows 10 にもあります）。x64 専用で、ARM64 ビルドにはコマンドラインとトレイがあります。
続いてインストールを確認します。

```powershell
obit doctor              # health check: short summary, full HTML report (--open, --verbose, --json)
```

## 2. エージェントを接続

```powershell
obit init -g                 # Claude Code, all projects; also the Skill (and desktop integration on Windows)
obit init -g --agent codex   # Codex
```

その後エージェントを再起動します。その他のエージェント、MCP サーバー、Agent Skill については[エージェントを接続](./agents)を参照してください。

macOS / Linux では、Claude Code は `~/.claude`、Codex は `~/.codex`、共有 Skill は `~/.agents/skills` を使用します。`CLAUDE_CONFIG_DIR` と `CODEX_HOME` による上書きも有効です。

デスクトップ版で監視が有効な場合、トレイの常駐プロセスが期限に合わせて処理し、独立したジョブが1時間ごとに取りこぼしを確認します。そのデータディレクトリでトレイの監視が無効なら、ジョブは5分ごとに実行されます。

```bash
obit daemon enable --dry-run
obit daemon enable
obit doctor
```

macOS はログイン中のユーザーの launchd を使い、Linux は `systemd --user` を使います。systemd のない Linux では、ユーザーの crontab で 5 分ごとに `obitd --once` を実行するよう、`obitd`、`--home` データディレクトリ、`--config-dir` 設定ディレクトリの絶対パスを指定してください。エージェントの hook も、作業の期限になるとバックグラウンドチェックを開始します。

## 3. よく使うコマンド

`<id>` は `obit add` または `obit bury` のレシート、あるいは `obit list` / `obit graveyard` に表示される番号に置き換えます。

```powershell
obit add debug.log 2h        # bury it in 2 hours (alias: mark)
obit list                    # what is left and for how long
obit extend <id> 1d          # one more day
obit bury old-build          # bury right away
obit graveyard               # what is in the graveyard and when it will be purged
obit resurrect <id>          # put it back, never overwriting
obit resurrect --last        # undo the last deletion
```

その他のコマンドは[コマンド一覧](./commands)を参照してください。

## 次に読む

| 知りたいこと | 読むページ |
|---|---|
| 登録、埋葬、復活、永久削除の仕組みと墓地の場所 | [仕組み](./how-it-works) |
| Claude Code、Codex、その他のエージェントとの接続 | [エージェントを接続](./agents) |
| 何が登録されるか：エージェントの登録、コンテキストメニュー、任意の自動登録 | [登録対象](./auto-register) |
| エージェントの `rm -rf` を取り消せる埋葬に変える方法 | [削除ガード](./delete-guard) |
| エージェントが埋葬されたファイルにアクセスしたときの動作 | [墓地からの通知](./grave-notices) |
| Downloads や Desktop などのフォルダーに設定する寿命 | [フォルダーポリシー](./folder-policies) |
| コマンドラインを使わずに Obituary を使う方法 | [ウィンドウ、トレイ、コンテキストメニュー](./desktop) |
| セッション中に起動した Docker コンテナ | [Docker コンテナ](./docker) |
| すべてのコマンド | [コマンド一覧](./commands) |
| Obituary の削除 | [アンインストール](./uninstall) |
| その他 | [よくある質問](./faq) |
