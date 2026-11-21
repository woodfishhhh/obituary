# 开发博客投稿

从 `site/` 执行 `pnpm install --frozen-lockfile`，再用 `pnpm --filter blog dev` 预览 `/blog/`。列表、文章、语言切换和 RSS 都由 Markdown 生成，不需要改导航或索引。RSS 和 sitemap 在正式构建后生成。

中文放 `blog/posts/<slug>.md`，英文放 `blog/en/posts/<slug>.md`。文件名用短的 ASCII slug；两种语言文件名相同。正文不用再写一级标题，页面会渲染 `title`。示例 frontmatter：

```yaml
---
title: '示例：登记不是下葬'
description: '一句能独立阅读的摘要。'
developmentDate: '2026-11-21'
translationKey: example
draft: true
---
```

`developmentDate` 是可核实的开发日期，不是成稿、翻译或提交文章的日期。两种语言必须一致。`translationKey` 等于文件名（不含 `.md`），用来配对；`description` 用于列表和 RSS。文章按开发日期倒序，同日按 slug 排序。阅读时间按正文词数估算。

当前为分支预览：`draft: true` 会显示草稿标记，但不隐藏 URL、RSS 或 sitemap，不是隐私开关。不要提交私密素材。正式发布前移除示例文章、将审定稿标为 `draft: false`，在 `site/` 运行 `pnpm build` 和 `pnpm build:cn`，核对 `dist/blog/` 和 `dist-cn/blog/`。只有维护者把经审定的内容合入 `main` 后才会触发网站部署；推送博客分支本身不会发布。
