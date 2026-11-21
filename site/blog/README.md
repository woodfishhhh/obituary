# 开发博客投稿

从 `site/` 执行 `pnpm install --frozen-lockfile`，再用 `pnpm --filter blog dev` 预览 `/blog/`。列表、文章、语言切换和 RSS 都由 Markdown 生成，不需要改导航或索引。RSS 和 sitemap 在正式构建后生成。

中文放 `blog/posts/<slug>.md`，英文放 `blog/en/posts/<slug>.md`。文件名用短的 ASCII slug；两种语言文件名相同。正文不用再写一级标题，页面会渲染 `title`。示例 frontmatter：

```yaml
---
title: '先是一个小问题：给文件设个存活倒计时'
description: '一句能独立阅读的摘要。'
developmentDate: '2026-11-07'
translationKey: why-obituary
sequence: 1
draft: true
---
```

`developmentDate` 是可核实的开发日期，不是成稿、翻译或提交文章的日期。两种语言必须一致。`translationKey` 等于文件名（不含 `.md`），用来配对；`description` 用于列表和 RSS。阅读时间按正文词数估算。

`sequence` 可选，是非负整数，用来固定同一个开发日期内的先后：数字越大，发生得越晚。两种语言必须一致；不写按 0 处理。列表按开发日期倒序（新的在前）；同一天先按 `sequence` 大的在前，再按 slug 排序。列表页顶部有“从第一篇读起”的入口，指向最早的一篇；文章页底部的“上一篇／下一篇”也按这个顺序。

新文章先写成 `draft: true`，审定后改为 `draft: false`。`draft: true` 只显示草稿标记，不隐藏 URL、RSS 或 sitemap，不是隐私开关，所以不要提交私密素材。合并前在 `site/` 运行 `pnpm build` 和 `pnpm build:cn`，核对 `dist/blog/` 和 `dist-cn/blog/`。只有维护者把经审定的内容合入 `main` 后才会触发网站部署；推送博客分支本身不会发布。

## 图片

每篇文章至少有一张图，来自开发过程里真实的东西：终端里实际跑出的输出、构建或测试的结果、用真实数字画的图表、站点和小窗口的截图、仓库里的图解。不用聊天记录截图，图里不放用户名、邮箱、令牌、IP 和公司相关的名字，需要时换成演示数据重新截，不打码。

- 文件放在 `blog/images/<slug>/`，WebP，宽不超过 1600 px，单张不超过 300 KB。语言无关的图中英共用，界面或文字随语言变的图写成 `<名字>.zh.webp` 和 `<名字>.en.webp`。
- 正文里用原生 HTML，中文页的路径是 `../images/<slug>/…`，英文页是 `../../images/<slug>/…`：

```html
<figure>
  <img src="../images/agents-30/capabilities.webp" alt="图里有什么" width="1600" height="206" loading="lazy" decoding="async">
  <figcaption>一句话说明这张图说明了什么。</figcaption>
</figure>
```

- `alt` 和 `figcaption` 各写成对应语言的完整句子；`width` 和 `height` 必须等于文件的真实尺寸，首屏以下一律 `loading="lazy"`。
- `pnpm --filter blog build` 会先运行 `node --test .vitepress/posts.test.mjs`，检查：每篇至少一张图，每张图在 `<figure>` 里并带 `figcaption`，`alt` 足够长且不含本机路径，文件存在、不超过 300 KB、宽不超过 1600 px，宽高属性和文件一致。
- 终端图里的路径用中性的演示目录，运行会写系统的命令（`obit init`、注册表、开始菜单、计划任务）时用 `OBIT_TEST_REGISTRY_ROOT` 和临时目录隔离。
