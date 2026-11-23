# フォルダーポリシー

フォルダーポリシーを設定すると、そのフォルダーに入った項目にそれぞれ寿命が付きます。よく使うのは「ダウンロード」フォルダーです。

```powershell
obit watch C:\Users\you\Downloads 2w        # files that land in Downloads are buried two weeks later
obit watch --list                           # folder policies, their exemptions and quiet periods
obit watch --remove C:\Users\you\Downloads  # remove a policy and the lifespans it registered
```

macOS と Linux でも、承認後は直下の項目が個別に登録され、既定では最初に検出した時点から期限を数えます。監視フォルダー自体は保護され、期限の切れたサブフォルダーには提案だけが作られます。ダウンロード中の一時ファイルは対象外です。書き込みを排除できない場合は、10 分間変更がなくても元の場所に残ります。

**現在、macOS と Linux ではポリシーによる自動埋葬に対応していません。期限切れのファイルも、承認したサブフォルダーも移動されません。**

```sh
obit watch "$HOME/Downloads" 2w
```

## 適用条件

- **ポリシー自体に承認が必要です。** `obit watch` は提案を作成します。`obit approve` を実行するか、小ウィンドウの「For you」で承認すると有効になります。
- **10 分間変更されていないことが条件です。** 寿命が切れ、かつ直近 10 分間変更されていないファイルだけを埋葬します。
- **サブフォルダーは個別に承認します。** 期限が切れても提案が作られるだけで、承認するまでは埋葬されません。
- ポリシーを削除すると、そのポリシーが登録した寿命も取り消されます。

## エクスプローラーの日付表示 {#dates-in-explorer}

「ダウンロード」、デスクトップ、ポリシーの対象フォルダーでは、追跡中の項目がエクスプローラーで「name (11-13 burial)」のように表示されます。実際のファイル名は変わりません。日付の表記は Obituary の表示言語に従い、簡体字中国語では「name（11-13 下葬）」、それ以外では英語になります。

- 日付の表示には、フォルダー内の隠しファイル `desktop.ini` を使います。
- `config.toml` の `[explorer]` セクションで、`enabled = false` にすると日付を非表示に、`everywhere = true` にするとすべての場所で表示できます。
- エクスプローラーはフォルダーの表示をキャッシュするため、寿命を延長しても同じウィンドウに最長 1 分ほど古い日付が残ることがあります。
