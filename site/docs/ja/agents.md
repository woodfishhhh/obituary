# エージェントの接続

## Claude Code

```powershell
obit init -g                         # all projects (~/.claude); obit init alone installs into this project (.claude/)
obit init -g --show --agent claude   # what is installed
obit init -g --uninstall             # remove only what Obituary added
```

`obit init` は `settings.json` に `obit hook claude` を登録します。対象は SessionStart、各ツールの呼び出し前、ファイル書き込み後、Stop、SessionEnd です。エージェント用の `OBIT.md` を作成し、`CLAUDE.md` に参照を 1 行追加します。ユーザー単位（`-g`）なら `~/.claude/CLAUDE.md` に `@OBIT.md`、プロジェクト単位ならプロジェクトの `CLAUDE.md` に `@.claude/OBIT.md` を追加します。

- 既存の設定とキーの並び順は変わりません。再実行しても項目は重複しません。`obit.exe` を移動した場合は再実行するとパスが更新されます。
- 編集の前に必ずバックアップを取ります。初回は `.bak`、それ以降は `.bak.1`、`.bak.2` …と番号を付け、以前のバックアップを上書きしません。
- フックでエラーが起きてもエージェントは中断されません。エラーは `hook.log` に記録されます。`OBIT_DISABLED=1` でフックを一時的に無効にできます。

## Codex

```powershell
obit init -g --agent codex   # ~/.codex/hooks.json plus a block in AGENTS.md; obit init --agent codex installs into this project
```

- **Codex はユーザーが信頼したフックだけを実行します。** インストール後に Codex で `/hooks` を開き、Obituary のフックを信頼してください。プロジェクトのフックには、そのプロジェクト自体への信頼も必要です。
- Windows と WSL で同じ `hooks.json` を使います。WSL の `/mnt/<drive>/…` 形式のパスは `<drive>:\…` に変換します。WSL 自身のファイルシステム内のパス（`/home/…`）は現時点では対象外です。
- Codex は `apply_patch` でファイルを編集します。Obituary はパッチを読んで追加、変更、移動されたファイルを把握します。削除されたものは数えません。

## その他のエージェント

```powershell
obit init -g --show                         # every supported agent, how far each integration goes, what is installed
obit init -g --agent gemini,cursor          # install one or several; without -g, into this project only
obit init -g --agent gemini --mcp --skill   # also register the MCP server and install the Skill (where supported)
obit init -g --agent gemini --uninstall     # remove only what Obituary added
```

`--agent` を省略すると Claude Code だけを設定します。他のエージェントに勝手にインストールすることはありません。

| 連携方法 | エージェント |
|---|---|
| ネイティブフック | GitHub Copilot（VS Code と Copilot CLI）、Cursor、Gemini CLI、Factory Droid、Trae / Trae CN、Kimi、Mistral Vibe、Google Antigravity、Tencent CodeBuddy（CLI と IDE）、Qoder / Qoder CN / Lingma、Zhipu ZCode |
| プラグイン（Obituary が `obit hook` を呼び出す小さなプラグインをインストール） | OpenCode、OpenClaw、Pi、Oh My Pi、Hermes、Cline、Kilo Code、MiniMax Code、DeepSeek Harness |
| ルールファイルと MCPのみ | Roo Code, Windsurf, WorkBuddy, QoderWork |

::: tip エージェント別の対応状況
書き込み後の登録、セッション開始時の通知、削除ガード、MCP、Skill の対応状況はエージェントごとに異なります。リポジトリの[対応表](https://github.com/woodfishhhh/obituary/blob/main/docs/agents.md)に「対応／制限あり／非対応」とその理由を記載しています。
:::

公開ドキュメントだけでは安全に設定を書き込めない場合、`obit init` は手動設定の全手順を示し、「未完了、手動操作が必要」と報告します。未設定のものをインストール済みと報告することはありません。

## MCP サーバーと Agent Skill

```powershell
obit init -g --mcp --skill                 # Claude Code; add --agent codex for Codex. Neither is installed by default
```

- **`obit mcp`** は標準入出力（stdio）で動く MCP サーバーです。エージェントは `obit_mark`、`obit_mktemp`、`obit_bury`、`obit_resurrect`、`obit_extend`、`obit_spare` と、照会用の `obit_list`、`obit_graveyard`、`obit_proposals`、`obit_stats` を呼び出せます。
- **判断はユーザーに委ねられます。** 提案の承認・却下、`.obitrules` の信頼、墓地の項目の永久削除、設定変更のための MCP ツールはありません。エージェントにできるのは提案を取り次ぐことだけです。
- **Agent Skill** は、成果物にいつ寿命を設定するか、復活できる形で削除するにはどうするか、何をユーザーの判断に委ねるかを説明した `SKILL.md` です。自分で編集した後は、Obituary が上書きしたり削除したりすることはありません。
- インストール後はエージェントを再起動してください。`obit doctor` で両方がインストールされ、現在の `obit.exe` を参照しているか確認できます。
