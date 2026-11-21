<script setup lang="ts">
import { computed } from 'vue';
import { withBase } from 'vitepress';
import PostCard from './PostCard.vue';
import { dateParts } from './copy';
import { useBlog } from './blog';

const { en, c, list } = useBlog();
const first = computed(() => list.value[list.value.length - 1]);
// The two newest posts get the big cards; the rest run down the timeline, grouped by development day.
const latest = computed(() => list.value.slice(0, 2));
const days = computed(() => {
  const out: { date: string; posts: { post: any; n: number }[] }[] = [];
  list.value.forEach((post, i) => {
    if (i < 2) return;
    const n = list.value.length - i;
    if (out.at(-1)?.date === post.date) out.at(-1)!.posts.push({ post, n });
    else out.push({ date: post.date, posts: [{ post, n }] });
  });
  return out;
});
const span = computed(() => `${dateParts(first.value.date, en.value).short} – ${dateParts(list.value[0].date, en.value).short}`);
const minutes = computed(() => list.value.reduce((sum, p) => sum + p.minutes, 0));
</script>

<template>
  <div v-if="list.length" class="list">
    <header class="hero">
      <div class="washes" aria-hidden="true">
        <div class="wash dots" />
        <div class="wash a" />
        <div class="wash b" />
      </div>
      <p class="eyebrow rise" style="--i: 0"><span class="star" aria-hidden="true" />{{ c.section }}</p>
      <h1 class="rise" style="--i: 1">{{ c.heroTitle[0] }}<span class="brand-text">{{ c.heroTitle[1] }}</span>{{ c.heroTitle[2] }}</h1>
      <p class="lede rise" style="--i: 2">{{ c.lede }}</p>
      <div class="actions rise" style="--i: 3">
        <a class="btn primary" :href="withBase(first.url)">{{ c.first }} <span aria-hidden="true">→</span></a>
        <a class="btn" :href="withBase(en ? '/en/feed.xml' : '/feed.xml')" target="_self">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" aria-hidden="true"><path d="M4 11a9 9 0 0 1 9 9M4 4a16 16 0 0 1 16 16" /><circle cx="5" cy="19" r="1" fill="currentColor" /></svg>{{ c.rss }}
        </a>
      </div>
      <ul class="stats rise" style="--i: 4">
        <li>{{ c.count(list.length) }}</li><li>{{ span }}</li><li>{{ c.total(minutes) }}</li>
      </ul>
    </header>

    <section class="latest" aria-labelledby="latest-head">
      <h2 id="latest-head" class="visually-hidden">{{ c.latest }}</h2>
      <PostCard v-for="(post, i) in latest" :key="post.key" :post="post" :n="list.length - i" featured :latest="i === 0" :eager="i === 0" />
    </section>

    <section v-if="days.length" class="timeline" aria-labelledby="timeline-head">
      <div class="timeline-head">
        <h2 id="timeline-head">{{ c.timeline }}</h2>
        <p>{{ c.timelineHint }}</p>
      </div>
      <ol class="days">
        <li v-for="day in days" :key="day.date" class="day">
          <div class="when">
            <time :datetime="day.date" :aria-label="dateParts(day.date, en).long">
              <b>{{ dateParts(day.date, en).day }}</b>
              <span>{{ dateParts(day.date, en).month }} · {{ dateParts(day.date, en).weekday }}</span>
            </time>
          </div>
          <ul class="posts">
            <li v-for="{ post, n } in day.posts" :key="post.key"><PostCard :post="post" :n="n" /></li>
          </ul>
        </li>
      </ol>
    </section>
  </div>
</template>

<style scoped>
.list { --page: min(100% - 2.5rem, 76rem); }

.hero { position: relative; isolation: isolate; display: flex; flex-direction: column; align-items: center; padding: 8.5rem 1.5rem 3.25rem; text-align: center; }
/* The washes are wider than a phone, so they sit in a box that clips them (a phone would otherwise zoom out to fit them), and fade out below the first screen. */
.washes { position: absolute; inset: 0 0 -6rem; z-index: -1; overflow: hidden; pointer-events: none; mask-image: linear-gradient(#000 calc(100% - 14rem), transparent); }
.wash { position: absolute; }
.dots { inset: 0; background: radial-gradient(circle at 1px 1px, var(--b-dot) 1px, transparent 0) 0 0 / 22px 22px; mask-image: radial-gradient(ellipse 70% 60% at 50% 40%, #000 30%, transparent 75%); }
.a { top: -12rem; left: -10rem; width: 56rem; height: 56rem; background: radial-gradient(closest-side, var(--b-wash-1), transparent); }
.b { top: -6rem; right: -10rem; width: 46rem; height: 46rem; background: radial-gradient(closest-side, var(--b-wash-2), transparent); }

.eyebrow { display: inline-flex; align-items: center; gap: 0.5rem; margin: 0; padding: 0.4rem 1rem 0.4rem 0.8rem; border: 1px solid var(--vp-c-divider); border-radius: 999px; background: color-mix(in oklab, var(--b-card) 80%, transparent); box-shadow: var(--b-shadow-node); font-size: 0.875rem; font-weight: 600; color: var(--vp-c-text-2); }
.star { width: 0.5rem; height: 0.5rem; border-radius: 50%; background: var(--obit-gold); box-shadow: 0 0 0 3px color-mix(in oklab, var(--obit-gold) 30%, transparent); }
h1 { max-width: 16ch; margin: 1.5rem 0 0; font-size: clamp(2.5rem, 6.4vw, 4.75rem); font-weight: 600; line-height: 1.06; letter-spacing: -0.035em; text-wrap: balance; color: var(--vp-c-text-1); }
.brand-text { background-image: linear-gradient(100deg, var(--vp-c-brand-1) 10%, var(--vp-c-brand-3) 90%); -webkit-background-clip: text; background-clip: text; color: transparent; }
.lede { max-width: 38rem; margin: 1.4rem 0 0; font-size: clamp(1.05rem, 1.6vw, 1.2rem); line-height: 1.75; text-wrap: balance; color: var(--vp-c-text-2); }
.actions { display: flex; flex-wrap: wrap; justify-content: center; gap: 0.75rem; margin-top: 2rem; }
.btn { display: inline-flex; align-items: center; gap: 0.5rem; height: 3rem; padding: 0 1.5rem; border: 1px solid var(--vp-c-divider); border-radius: 999px; background: color-mix(in oklab, var(--b-card) 70%, transparent); font-size: 0.9375rem; font-weight: 600; color: var(--vp-c-text-1); backdrop-filter: blur(8px); transition: transform 250ms var(--b-ease), border-color 250ms, background-color 250ms; }
.btn:hover { transform: translateY(-1px); border-color: var(--b-line-hover); }
.btn.primary { border-color: transparent; background: #0048e8; color: #fff; box-shadow: var(--b-shadow-shine); }
.btn.primary:hover { background: #0039bb; box-shadow: var(--b-shadow-shine), 0 14px 30px -14px rgb(0 72 232 / 0.7); }
.btn.primary span { transition: transform 250ms var(--b-ease); }
.btn.primary:hover span { transform: translateX(3px); }
.stats { display: flex; flex-wrap: wrap; justify-content: center; gap: 0.25rem 1.5rem; margin: 1.75rem 0 0; padding: 0; list-style: none; font-size: 0.875rem; color: var(--vp-c-text-3); }
@media (min-width: 640px) { .stats li + li::before { content: ''; display: inline-block; width: 4px; height: 4px; margin: 0 1.5rem 0.2em -1.5rem; border-radius: 50%; background: var(--vp-c-divider); } }

.latest { display: grid; gap: 1.25rem; width: var(--page); margin: 0 auto; }
@media (min-width: 800px) { .latest { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1.5rem; } }

.timeline { width: var(--page); margin: 4.5rem auto 6rem; }
.timeline-head { display: flex; flex-wrap: wrap; align-items: baseline; gap: 0.25rem 1rem; margin-bottom: 2rem; }
.timeline-head h2 { margin: 0; font-size: 1.5rem; font-weight: 600; letter-spacing: -0.02em; }
.timeline-head p { margin: 0; font-size: 0.9375rem; color: var(--vp-c-text-3); }
.days, .posts { margin: 0; padding: 0; list-style: none; }
.posts { display: grid; gap: 1.25rem; }
.day { position: relative; margin-bottom: 2.5rem; }
.when { margin-bottom: 0.9rem; }
.when time { display: flex; align-items: baseline; gap: 0.6rem; }
.when b { font-size: 1.75rem; font-weight: 600; line-height: 1; letter-spacing: -0.03em; font-variant-numeric: tabular-nums; color: var(--vp-c-text-1); }
.when span { font-size: 0.875rem; color: var(--vp-c-text-3); }
.when time::after { content: ''; flex: 1; height: 1px; background: var(--vp-c-divider); align-self: center; }

/* Wide: the days hang off a rail, the date in a column on its left, stuck to the top while that day's posts scroll by. */
@media (min-width: 900px) {
  .day { --when: 8rem; display: grid; grid-template-columns: var(--when) minmax(0, 1fr); column-gap: 3rem; margin-bottom: 1.75rem; }
  .day::before { content: ''; position: absolute; top: 0.75rem; bottom: -2.5rem; left: calc(var(--when) + 1.5rem - 1px); width: 2px; background: linear-gradient(var(--vp-c-divider), var(--vp-c-divider)); }
  .day:last-child::before { bottom: 0; background: linear-gradient(var(--vp-c-divider), transparent); }
  .day::after { content: ''; position: absolute; top: 0.6rem; left: calc(var(--when) + 1.5rem - 6px); width: 10px; height: 10px; border: 2px solid var(--vp-c-brand-1); border-radius: 50%; background: var(--vp-c-bg); box-sizing: content-box; translate: -1px -1px; }
  .when { position: sticky; top: 6rem; align-self: start; margin: 0; text-align: right; }
  .when time { flex-direction: column; align-items: flex-end; gap: 0.35rem; }
  .when time::after { display: none; }
  .when b { font-size: 2.6rem; }
}
</style>
