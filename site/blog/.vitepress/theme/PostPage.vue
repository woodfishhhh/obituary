<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { Content, useData, withBase } from 'vitepress';
import { dateParts } from './copy';
import { coverUrl, useBlog } from './blog';

const { page, theme } = useData();
const { en, c, list, index, post } = useBlog();
// The list is newest first: the post after this one in the list is the one written before it.
const prev = () => list.value[index.value + 1];
const next = () => (index.value > 0 ? list.value[index.value - 1] : undefined);
// The picture's box is known before the file arrives (width capped so it is at most 36rem tall), so the text below does not move when it loads.
const bannerSize = (p: { width: number; height: number }) => ({ aspectRatio: `${p.width} / ${p.height}`, width: `min(100%, calc(36rem * ${p.width} / ${p.height}), ${p.width}px)` });
const headings = () => (page.value.headers ?? []).filter((h: { level: number }) => h.level === 2) as { title: string; link: string }[];

// The contents list marks the section at the top of the page.
const active = ref('');
let stop = () => {};
onMounted(() => {
  const nodes = [...document.querySelectorAll<HTMLElement>('.post-body h2[id]')];
  let frame = 0;
  const update = () => {
    frame = 0;
    active.value = nodes.filter(h => h.getBoundingClientRect().top <= 140).at(-1)?.id ?? '';
  };
  const onScroll = () => { frame ||= requestAnimationFrame(update); };
  update();
  addEventListener('scroll', onScroll, { passive: true });
  stop = () => { removeEventListener('scroll', onScroll); cancelAnimationFrame(frame); };
});
onBeforeUnmount(() => stop());
</script>

<template>
  <article v-if="post" class="post">
    <header class="head">
      <div class="washes" aria-hidden="true">
        <div class="wash a" />
        <div class="wash b" />
      </div>
      <div class="head-text">
        <a class="back rise" style="--i: 0" :href="withBase(en ? '/en/' : '/')"><span aria-hidden="true">←</span> {{ c.back }}</a>
        <p class="meta rise" style="--i: 1">
          <time :datetime="post.date">{{ dateParts(post.date, en).long }}</time>
          <span>{{ c.minutes(post.minutes) }}</span>
          <span>{{ c.nth(list.length - index) }}</span>
          <span v-if="post.draft" class="chip">{{ c.draft }}</span>
        </p>
        <h1 class="rise" style="--i: 2">{{ post.title }}</h1>
        <p class="lede rise" style="--i: 3">{{ post.description }}</p>
      </div>
      <figure v-if="post.cover" class="banner rise" style="--i: 4">
        <div class="stage"><img :src="coverUrl(post.cover.src)" :alt="post.cover.alt" :width="post.cover.width" :height="post.cover.height" :style="bannerSize(post.cover)" loading="eager" fetchpriority="high" decoding="async"></div>
        <figcaption>{{ post.cover.caption }}</figcaption>
      </figure>
    </header>

    <details v-if="headings().length > 1" class="toc-narrow">
      <summary>{{ theme.outline.label }}</summary>
      <ol><li v-for="h in headings()" :key="h.link"><a :href="h.link">{{ h.title }}</a></li></ol>
    </details>

    <div class="body-wrap">
      <aside v-if="headings().length > 1" class="toc-wide" :aria-label="theme.outline.label">
        <div class="toc-sticky">
          <p>{{ theme.outline.label }}</p>
          <ol><li v-for="h in headings()" :key="h.link"><a :href="h.link" :class="{ on: active === h.link.slice(1) }" :aria-current="active === h.link.slice(1) ? 'location' : undefined">{{ h.title }}</a></li></ol>
        </div>
      </aside>
      <div class="post-body vp-doc" :class="{ 'has-banner': post.cover }"><Content /></div>
    </div>

    <nav class="pager" :aria-label="c.pager">
      <a v-if="prev()" class="pager-card prev" :href="withBase(prev().url)"><span class="dir"><span aria-hidden="true">←</span> {{ c.prev }}</span><strong>{{ prev().title }}</strong><time :datetime="prev().date">{{ dateParts(prev().date, en).long }}</time></a>
      <a v-if="next()" class="pager-card next" :href="withBase(next().url)"><span class="dir">{{ c.next }} <span aria-hidden="true">→</span></span><strong>{{ next().title }}</strong><time :datetime="next().date">{{ dateParts(next().date, en).long }}</time></a>
    </nav>
    <p class="all"><a :href="withBase(en ? '/en/' : '/')">{{ c.back }}</a></p>
  </article>
</template>

<style scoped>
.post { padding-bottom: 6rem; --column: 46rem; --bleed: min(4.5rem, max(0px, calc((100vw - var(--column)) / 2 - 1.5rem))); }

.head { position: relative; isolation: isolate; padding: 7rem 0 0; }
.washes { position: absolute; inset: 0; z-index: -1; overflow: hidden; pointer-events: none; }
.wash { position: absolute; }
.a { top: -14rem; left: -12rem; width: 50rem; height: 50rem; background: radial-gradient(closest-side, var(--b-wash-1), transparent); }
.b { top: -8rem; right: -12rem; width: 40rem; height: 40rem; background: radial-gradient(closest-side, var(--b-wash-2), transparent); }
.head-text { width: min(100% - 2.5rem, var(--column)); margin: 0 auto; }
.back { display: inline-flex; gap: 0.4rem; margin-bottom: 1.5rem; padding: 0.35rem 0.9rem 0.35rem 0.7rem; border: 1px solid var(--vp-c-divider); border-radius: 999px; background: color-mix(in oklab, var(--b-card) 75%, transparent); font-size: 0.875rem; font-weight: 500; color: var(--vp-c-text-2); transition: color 200ms, border-color 200ms; }
.back:hover { color: var(--vp-c-brand-1); border-color: var(--b-line-hover); }
.meta { display: flex; flex-wrap: wrap; align-items: center; gap: 0.25rem 0.9rem; margin: 0 0 0.9rem; font-size: 0.875rem; color: var(--vp-c-text-3); }
.meta > :not(:first-child)::before { content: ''; display: inline-block; width: 4px; height: 4px; margin: 0 0.9rem 0.2em -0.1rem; border-radius: 50%; background: var(--vp-c-divider); vertical-align: middle; }
.meta .chip::before { display: none; }
.chip { padding: 0 0.6rem; border-radius: 999px; font-size: 0.75rem; font-weight: 600; background: var(--vp-c-bg-soft); }
h1 { margin: 0; font-size: clamp(2rem, 4.8vw, 3.1rem); font-weight: 600; line-height: 1.2; letter-spacing: -0.03em; text-wrap: balance; color: var(--vp-c-text-1); }
.lede { margin: 1.1rem 0 0; font-size: 1.125rem; line-height: 1.8; text-wrap: pretty; color: var(--vp-c-text-2); }
/* The post's first picture, whole, on the same dotted mat as the list's covers. The first figure in the text is hidden (prose.css) because this is it. */
.banner { width: min(100% - 2.5rem, 62rem); margin: 2.5rem auto 0; }
.stage { display: grid; place-items: center; padding: clamp(0.75rem, 4.5vw, 3rem); border: 1px solid var(--vp-c-divider); border-radius: 28px; background: radial-gradient(circle at 1px 1px, var(--b-dot) 1px, transparent 0) 0 0 / 18px 18px, linear-gradient(135deg, var(--b-mat-1), var(--b-mat-2)); box-shadow: var(--b-shadow-shot); }
.stage img { display: block; height: auto; border-radius: 12px; box-shadow: 0 18px 40px -18px rgb(17 21 34 / 0.5), 0 0 0 1px rgb(17 21 34 / 0.08); }
.banner figcaption { max-width: 42rem; margin: 0.85rem auto 0; padding: 0 1rem; font-size: 0.875rem; line-height: 1.7; text-align: center; text-wrap: balance; color: var(--vp-c-text-2); }
@media (max-width: 639px) { .stage { border-radius: 20px; } .stage img { border-radius: 8px; } .lede { font-size: 1.05rem; } }

.body-wrap { position: relative; width: min(100% - 2.5rem, var(--column)); margin: 0 auto; }
.post-body { padding-top: 2.5rem; }

/* The contents: a collapsed list on narrow screens, a column hanging off the text's right edge where there is room. */
.toc-narrow { width: min(100% - 2.5rem, var(--column)); margin: 2rem auto 0; padding: 0.15rem 1rem; border: 1px solid var(--vp-c-divider); border-radius: 16px; background: var(--b-card); font-size: 0.9375rem; }
.toc-narrow summary { min-height: 2.75rem; display: flex; align-items: center; font-weight: 600; cursor: pointer; }
.toc-narrow ol, .toc-wide ol { margin: 0; padding: 0; list-style: none; }
.toc-narrow ol { padding-bottom: 0.6rem; }
.toc-narrow li a { display: block; padding: 0.4rem 0; color: var(--vp-c-text-2); }
.toc-wide { display: none; }
@media (min-width: 1440px) {
  .toc-narrow { display: none; }
  .toc-wide { position: absolute; top: 0; bottom: 0; left: calc(100% + 4.5rem + 2rem); display: block; width: 13rem; }
  .toc-sticky { position: sticky; top: 6.5rem; }
  .toc-wide p { margin: 0 0 0.6rem; font-size: 0.8125rem; font-weight: 600; letter-spacing: 0.02em; color: var(--vp-c-text-1); }
  .toc-wide ol { border-left: 1px solid var(--vp-c-divider); }
  .toc-wide a { position: relative; display: block; padding: 0.3rem 0 0.3rem 0.9rem; font-size: 0.8125rem; line-height: 1.5; color: var(--vp-c-text-3); transition: color 200ms; }
  .toc-wide a:hover, .toc-wide a.on { color: var(--vp-c-text-1); }
  .toc-wide a.on::before { content: ''; position: absolute; left: -1px; top: 0.25rem; bottom: 0.25rem; width: 2px; border-radius: 2px; background: var(--obit-gold); }
}

.pager { display: grid; gap: 1rem; width: min(100% - 2.5rem, calc(var(--column) + 2 * 4.5rem)); margin: 4.5rem auto 0; }
@media (min-width: 720px) { .pager { grid-template-columns: repeat(2, minmax(0, 1fr)); } .pager-card.next:only-child { grid-column: 2; } }
.pager-card { display: flex; flex-direction: column; gap: 0.4rem; padding: 1.15rem 1.4rem 1.3rem; border: 1px solid var(--vp-c-divider); border-radius: 20px; background: var(--b-card); box-shadow: var(--b-shadow-card); transition: transform 350ms var(--b-ease), border-color 350ms, box-shadow 350ms var(--b-ease); }
.pager-card:hover { transform: translateY(-2px); border-color: var(--b-line-hover); box-shadow: var(--b-shadow-card-hover); }
.pager-card.next { text-align: right; }
.dir { font-size: 0.8125rem; font-weight: 600; color: var(--vp-c-brand-1); }
.pager-card strong { font-size: 1.0625rem; font-weight: 600; line-height: 1.5; text-wrap: balance; color: var(--vp-c-text-1); }
.pager-card time { font-size: 0.8125rem; color: var(--vp-c-text-3); }
.all { margin: 1.75rem 0 0; text-align: center; font-size: 0.9375rem; font-weight: 500; }
.all a { color: var(--vp-c-text-2); text-decoration: underline; text-decoration-color: var(--vp-c-divider); text-underline-offset: 0.25em; }
.all a:hover { color: var(--vp-c-brand-1); }
@media (prefers-reduced-motion: reduce) { .pager-card { transition: none; } .pager-card:hover { transform: none; } }
</style>
