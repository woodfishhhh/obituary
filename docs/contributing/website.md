# 官网与小窗口

## 构建与分发

`site/`：官网，pnpm workspace，和 Go 代码互不依赖。`landing/` 是落地页（Vite + React + Tailwind，组件用 Magic UI、React Bits 和自己写的；React Bits 放在 `components/reactbits/`，许可是 MIT + Commons Clause，只能随网站使用，不能单独再分发），`docs/` 是文档站（VitePress，挂在 `/docs/`）。`pnpm build` 产出 GitHub Pages 版 `dist/`，`pnpm build:cn` 产出国内镜像版 `dist-cn/`：链接、给 agent 的那句话和安装脚本都指向 obituary.woodfish.site，不发任何站外请求，页脚有备案号。`scripts/assemble.mjs` 会核对 `docs/agent-install.md` 的原文，替换不上就让构建失败。

贡献者墙在构建时拉取，头像存进产物。第三方图标的来源和许可写在 `site/THIRD_PARTY_NOTICES.md`，正文和注释里不提出处。`pages.yml` 只在仓库公开后部署（Pages 和镜像都是）。

## 小窗口与多语言

首页的小窗口（第一屏的日历和「小窗口」那组标签）是 GUI 前端的真组件：`landing/vite.config.ts` 用别名 `@gui` 引入 `cmd/obit-gui/frontend/src`（去掉它样式里的整页规则，图标路径指向官网的 `icon-128.png`），`components/window-demo.tsx` 套上 `src/demo/` 的示例数据（`data.ts` 的文件、寿命和备注，`api.ts` 包一层 `@gui/mock`）和悬停卡片，所以改 GUI 官网会跟着变；演示在浏览器里懒加载，服务端和水合前都渲染同样大小的占位。小窗口的样式表放进低优先级的 `gui` 样式层，免得它的全局 `button` 规则压住官网的工具类。

首页有和 README 一样的 10 种语言（`src/locales.ts` 是语言表：简中在 `/`、英文在 `/en/`，其余在 `/<语言码>/`；繁中也用中国国旗）。文案在 `src/copy/<语言>.ts`，都得和 `zh.ts` 同形（类型检查保证不缺句），术语跟 README 的术语表一致；小窗口里的界面只有中英文，其他语言显示英文，示例备注（`src/demo/data.ts`）每种语言都有。新增用户可见文字要在全部 10 份文案里加。繁中、日、韩用访客的系统字体，只有简中用裁剪过的 Noto Sans SC。

构建时 `scripts/prerender.mjs` 用一份模板给每种语言写出完整 HTML（`<html lang>`、hreflang 互链、canonical、sitemap.xml、robots.txt；`vite build --ssr` 出 `dist-ssr`，用完删除），浏览器再水合：新加的组件不要在渲染期读 `window`、`navigator`、随机数或时间，这些放进 effect（访客的系统、浏览器语言提示、复制按钮都是这样，先出不依赖检测的版本）。

## 首屏、锚点与动画

首屏性能的几条线（用 Lighthouse 移动端和降速 4 倍的 trace 量过）：首屏文字在 HTML 里立刻可见，不能被依赖脚本的入场动画（opacity 0）挡住，不然 LCP 要等脚本跑完；入场用纯 CSS（`.rise`、`.reveal`、`.rw-in`），不用 motion；用到 motion 的区块（Flow、Story）是懒加载的，预渲染用 `react-dom/static` 的 `prerender` 等它们出完 HTML；每个页面只加载自己语言的文案（`copy/index.ts` 的 `loaders`，预渲染给该语言的模块和首屏字体写 modulepreload / preload，字体不预加载会在换字体时整块位移）；大面积柔光用径向渐变，不用 `blur-3xl`；小窗口的字体用 `font-display: block`，不然它加载字体时会位移。

首屏以下的区块和页脚是 HTML 从一开始就在、脚本延后加载的（`App.tsx` 的 `Below`，`lib/idle.ts`：访客第一次滚动、触摸、按键、移动指针或点 #链接时，或加载完 4.5 秒后）；首屏只水合 Nav 和 Hero。要在首屏以下加依赖脚本的新东西，放进 `Below`；`Reveal` 的隐藏态靠它自己加的 `html.js`，所以没水合的区块一直可见。

首屏以下的 section 不加 `content-visibility: auto`：折叠区的估算高度和真实高度差几百像素，页内锚点会落偏（测过，导航跳转偏差最多 290 px，换来的 TBT 只少 7 ms 左右）；`.rise` 和 `.reveal` 在 index.css 里，`.reveal` 只在组件的脚本真的开始观察它时才隐藏（`.armed`），脚本没到的区块一直是可见的。

页面自己的字体用 `font-display: optional`（vite.config.ts 的 postcss 插件改写），首屏字体由预渲染 preload，晚到的字体留给下次访问，不在阅读中换字体；轮播字所在的标签行在手机上预留两行高度，换词不推动页面。轮播字的动画只动 transform 和 opacity：字母用 `.rw-in` / `.rw-out`（index.css，改样式表时确认它们还在，缺了就没有动画、新旧词叠在一起），换词时药丸、两侧文字和词槽在布局上立刻变成新尺寸，再各用一个 transform 动画从原位滑过去，宽度不做动画，主线程忙也不会卡住；计时只在标签行在屏幕内且页面可见时走，回来重新数一个完整间隔；减少动态效果时直接换词。

绕边框的光（`components/ui/border-beam.tsx`）和其他持续动画只用 transform / opacity 这类合成层属性，离屏和标签页隐藏时暂停，开了减少动态效果就不显示；不要用脚本每帧改样式。新增效果前后都跑一遍 Lighthouse，不要把 LCP、TBT、CLS 拉回去。检查动画用无头浏览器按时间截帧（全局的 Playwright 即可）：chrome-devtools 打开的窗口被遮挡时 requestAnimationFrame 每秒只跑两帧，动画看起来像卡住。

## Agent 接入区

「接到你用的 agent 上」那一段是 `components/agent-stream.tsx`（流光带：三条滚动的药丸，每个药丸是指向该 agent 文档页的真链接，水合后普通点击改为选中并显示命令、复制和文档，带修饰键的点击仍然跳转；logo 直接放在药丸上不加底板，纯黑的单色标记在 `data-theme="dark"` 的暗色底上由 `agents.ts` 的 `tone` 反相（正式站目前只有亮色，暗色只在 playground 里定义），样式在 `index.css` 的 `.agent-*`，参数默认值是 `streamDefaults`）；`playground/`（`pnpm playground`、`pnpm build:playground`）是调这面墙的开发工具，不进正式站，`check-docs.mjs` 要求首页 HTML 里有每家 agent 文档页的链接。

命令条显示和复制的命令来自 `agents-data.ts` 的 `initCommand`（是否 `-g`、附加参数 `flags` 也在那里），文档站的接入、`--show`、`--uninstall` 三处也调用它，两边不各写一份；命令条上方「在哪里运行」那句的译文照搬各语言文档页的同一句（`copy/<语言>.ts` 的 `agents.project` / `agents.global`）。药丸会在页面里重复好几份，所以 agent 的标记不内联成 data URI（`vite.config.ts` 的 `assetsInlineLimit`），否则每份重复都进 HTML，首屏要先读完它们。

## 连续圆角

- **圆角写普通的 `border-radius`，连续曲率由构建加上（所有者 2026-11-21 定：苹果那种平滑曲线）。** 小窗口、官网首页、文档站的 Vite / VitePress 构建都挂 `cmd/obit-gui/frontend/scripts/squircle-corners.mjs`（PostCSS 插件，无依赖，测试在同目录）：每个圆角矩形的半径改写成 `calc(<半径> * var(--squircle-k, 1))` 并加 `corner-shape: var(--squircle-shape, round)`；这两个变量只在 `src/squircle.css` 的 `@supports (corner-shape: squircle)` 里定义（Chromium / WebView2 139 起），不支持的引擎（Safari / WKWebView、WebKitGTK、Firefox）算出的还是原来的半径和普通圆角，像素与改动前一致。`--squircle-k` 是 1.78：同一半径下 `squircle` 比圆角方得多，按苹果的角（Figma 圆角平滑 60%）在 13 种尺寸上叠图拟合得出，推导写在 `squircle.css` 的注释里。插件放过零、5px 以下和胶囊（百分比、100px 及以上、`calc(infinity * 1px)`），这些保持正圆；用小 px 半径写的胶囊（开关、徽标）直接写 `999px`，或在同一条规则里自己写 `corner-shape: round`（规则里有 `corner-shape` 就整条不动），文档站的第三方类名用插件的 `roundSelectors`。内框紧贴外框时（外半径 = 内半径 + 内缩）把外框手写成 `calc(var(--r) * var(--squircle-k, 1) + 内缩)`，别把同一个半径套到每一层；`border-radius: inherit` 会连形状一起继承。不要用 JS 算 SVG 路径做 clip-path 或 mask（会裁掉边框、阴影和焦点环）；canvas、`offset-path` 这类自己画的圆角继续用名义上的圆，曲线与补偿后的轮廓相差不到半径的 2%。
