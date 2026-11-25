# 界面与 README 的多语言维护

## 界面消息

- 新增用户可见文字时，先在 internal/i18n/messages.go 增加稳定的点分消息 ID 和英文默认文案，再在 messages/active.zh-CN.json 增加对应翻译；不要把用户文字散落回 CLI 或托盘实现。
- 英文源文案由 Go 中的 go-i18n Message 定义维护，active.en.json 是可提交的提取结果；新增或修改消息后运行 goi18n extract 并检查两种语言的键和占位符。
- 新增语言时复制 active.zh-CN.json 的键集合，使用规范 BCP 47 标签，运行语言目录一致性测试，并在语言解析测试中覆盖显式设置、环境变量、区域变量和回退路径。
- OBIT_LANG、配置项 ui.language、LC_ALL、LC_MESSAGES、LANG 和操作系统界面语言按既定优先级解析；zh-TW、zh-HK、zh-MO 等繁体标签不能落到简体中文。
- 永远不翻译守卫和 hook 的拒绝提示、等价命令、宿主协议输出、MCP 工具名与描述、JSON 字段、退出码、路径、版本前缀、日志标记、注册表键和 Wails DTO 字段；hook 路径不加载语言目录。

## README、配图与术语

- 英文 `README.md` 是唯一源文件；每次修改必须在同一次提交中同步 `README.zh-CN.md`、`README.zh-TW.md`、`README.ja.md`、`README.ko.md`、`README.es.md`、`README.fr.md`、`README.de.md`、`README.pt-BR.md`、`README.ru.md`。
- 10 份 README 的第一行语言栏必须相同，每种语言都链接到根目录对应文件。顺序及国旗对应为：English（美国 `us`）、简体中文（中国 `cn`）、繁體中文（中国 `cn`）、日本語（日本 `jp`）、한국어（韩国 `kr`）、Español（西班牙 `es`）、Français（法国 `fr`）、Deutsch（德国 `de`）、Português (Brasil)（巴西 `br`）、Русский（俄罗斯 `ru`）。每个语言名之前用 `<img>` 引用 `docs/assets/flags/` 下的 SVG，高 14 px，垂直居中，`alt` 留空；不用国旗 emoji。
- 图片变体位于 `docs/assets/readme/i18n/<语言代码>/`，文案在 `strings.json`，排版引擎是 `layout.js`。生成和检查脚本要在无头浏览器里量字宽，依赖固定版本的 Playwright，装在 `docs/assets/readme/i18n/`（自己的 `package.json` 和 `pnpm-lock.yaml`，不依赖全局装的 Playwright）。第一次：`cd docs/assets/readme/i18n && pnpm install --frozen-lockfile && pnpm browser`（后一步是 `playwright install chromium`）。之后在同一目录：`pnpm generate` 重新生成 40 张图，`pnpm check` 检查版面，`pnpm test` 确认检查脚本仍能抓到故意弄坏的图。改文案或排版必须重新生成并提交 SVG；SVG 是用本机字体量的，CI 上的字体不同，所以 CI 只检查、不重新生成。
- 图片生成结果要与 README 语言同步；简体中文和繁體中文使用中文 demo GIF，其余语言使用英文 demo GIF。软件本身完成多语言后再重做 GIF 和右键菜单截图。
- 图的文字宽度一律在浏览器里用真实字体量出来（同时按更宽的 Verdana 回退字体留余量），不靠估算字宽；不使用横向压缩，说明文字在 880 px 宽度下不小于 14 px。`pnpm check`（即 `scripts/check-readme-diagrams.mjs`）取每段文字和每个卡片、命令框、箭头的真实边界框，查文字重叠、越出卡片或命令框、命令框不在自己的卡片里、间距与内边距、碰到箭头、对比度、孤字、行首标点和折行后与 `strings.json` 不一致，并用回退字体再查一遍。新增或放宽检查规则时，在 `scripts/check-readme-diagrams.test.mjs` 里加一条对应的故意弄坏的图，并把规则临时关掉确认测试会失败。检查通过后仍要用无头浏览器逐张看图，脚本不替代眼睛。`.github/workflows/readme-diagrams.yml` 在 README、`docs/assets/readme/` 或检查脚本改动时安装 Chromium 并跑 `check-readme-i18n.py`、`pnpm check`、`pnpm test`，必须通过。README 里演示动图在前、生命周期图在后（10 个语言版本一致）。
- 维护术语表：

| English | zh-CN | zh-TW | ja | ko | es | fr | de | pt-BR | ru |
|---|---|---|---|---|---|---|---|---|---|
| bury | 下葬 | 下葬 | 埋葬 | 매장 | enterrar | enterrer | begraben | enterrar | похоронить |
| graveyard | 墓地 | 墓地 | 墓地 | 묘지 | cementerio | cimetière | Friedhof | cemitério | кладбище |
| resurrect | 复活 | 復活 | 復活 | 부활 | resucitar | ressusciter | wiederbeleben | ressuscitar | воскресить |
| lifespan | 寿命 | 壽命 | 寿命 | 수명 | vida útil | durée de vie | Lebensdauer | vida útil | срок жизни |
| expire | 到期 | 到期 | 期限切れ | 만료 | caducar | expirer | ablaufen | expirar | истекать |
| register | 登记 | 登記 | 登録 | 등록 | registrar | enregistrer | registrieren | registrar | зарегистрировать |

`purge` 在非中文译文中表示永久删除；中文 README 保留现有的“火化”隐喻。

- 本地检查运行 `python scripts/check-readme-i18n.py`；它检查相对链接、图片路径、页内锚点、标题数量、代码块和 README 结构。图的版面由 `node scripts/check-readme-diagrams.mjs` 检查。每次更新还要同步官网、`docs/` 文档和这些译文。
