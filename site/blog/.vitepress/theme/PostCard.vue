<script setup lang="ts">
import { withBase } from 'vitepress';
import Cover from './Cover.vue';
import { dateParts } from './copy';
import { useBlog } from './blog';

// One post. The title's link stretches over the whole card, so the card is one target for a pointer and one stop for the keyboard.
defineProps<{ post: any; n: number; featured?: boolean; latest?: boolean; eager?: boolean }>();
const { en, c } = useBlog();
</script>

<template>
  <article class="card" :class="{ featured }">
    <Cover :post="post" :eager="eager" />
    <div class="body">
      <p v-if="featured" class="chips">
        <span v-if="latest" class="chip gold">{{ c.latest }}</span>
        <time :datetime="post.date" class="chip">{{ dateParts(post.date, en).short }}</time>
      </p>
      <h3 class="title"><a :href="withBase(post.url)">{{ post.title }}</a></h3>
      <p class="desc">{{ post.description }}</p>
      <p class="meta">
        <span class="nth">{{ c.nth(n) }}</span>
        <span>{{ c.minutes(post.minutes) }}</span>
        <span v-if="post.draft" class="chip">{{ c.draft }}</span>
        <span class="go" aria-hidden="true">{{ c.read }} →</span>
      </p>
    </div>
  </article>
</template>

<style scoped>
.card { position: relative; display: flex; flex-direction: column; overflow: hidden; border: 1px solid var(--vp-c-divider); border-radius: 24px; background: var(--b-card); box-shadow: var(--b-shadow-card); transition: transform 350ms var(--b-ease), box-shadow 350ms var(--b-ease), border-color 350ms; }
.card:hover { transform: translateY(-3px); border-color: var(--b-line-hover); box-shadow: var(--b-shadow-card-hover); }
.card:has(.title a:focus-visible) { outline: 2px solid var(--vp-c-brand-1); outline-offset: 3px; }
.title a::after { content: ''; position: absolute; inset: 0; }
.title a:focus-visible { outline: none; }
.cover { aspect-ratio: 16 / 9; }
.body { display: flex; flex: 1; flex-direction: column; padding: 1.1rem 1.35rem 1.2rem; }
.chips { display: flex; align-items: center; gap: 0.5rem; margin: 0 0 0.6rem; }
.chip { display: inline-flex; align-items: center; height: 1.5rem; padding: 0 0.65rem; border-radius: 999px; font-size: 0.75rem; font-weight: 600; background: var(--vp-c-bg-soft); color: var(--vp-c-text-2); }
.chip.gold { background: var(--obit-gold); color: #3a3000; }
.title { margin: 0; font-size: 1.1875rem; font-weight: 600; line-height: 1.4; letter-spacing: -0.012em; text-wrap: balance; color: var(--vp-c-text-1); }
.card:hover .title { color: var(--vp-c-brand-1); }
.desc { display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 3; margin: 0.55rem 0 0; overflow: hidden; font-size: 0.9375rem; line-height: 1.75; color: var(--vp-c-text-2); }
.meta { display: flex; flex-wrap: wrap; align-items: center; gap: 0.25rem 0.9rem; margin: auto 0 0; padding-top: 0.9rem; font-size: 0.8125rem; color: var(--vp-c-text-3); }
.nth { font-variant-numeric: tabular-nums; }
.go { margin-left: auto; font-weight: 600; color: var(--vp-c-brand-1); opacity: 0; transform: translateX(-6px); transition: opacity 250ms, transform 350ms var(--b-ease); }
.card:hover .go, .card:has(.title a:focus-visible) .go { opacity: 1; transform: none; }
@media (hover: none) { .go { opacity: 1; transform: none; } }

.featured .cover { aspect-ratio: 16 / 9; }
.featured .body { padding: 1.35rem 1.6rem 1.5rem; }
.featured .title { font-size: clamp(1.3rem, 2.2vw, 1.55rem); line-height: 1.35; }
.featured .desc { -webkit-line-clamp: 4; font-size: 1rem; }

/* In the timeline a card is a row: the picture on the left, as tall as the text beside it. */
@media (min-width: 640px) {
  .card:not(.featured) { flex-direction: row; }
  .card:not(.featured) .cover { flex: none; width: 15rem; aspect-ratio: auto; }
  .card:not(.featured) .body { min-width: 0; padding: 1.2rem 1.5rem; }
}
@media (prefers-reduced-motion: reduce) { .card, .go { transition: none; } .card:hover { transform: none; } }
</style>
