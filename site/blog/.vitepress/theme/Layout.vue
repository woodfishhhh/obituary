<script setup lang="ts">
import { useData, withBase } from 'vitepress';
import BlogNav from './BlogNav.vue';
import PostList from './PostList.vue';
import PostPage from './PostPage.vue';
import { useBlog } from './blog';

const { page, frontmatter } = useData();
const { en, c, root } = useBlog();
const repo = 'https://github.com/woodfishhhh/obituary';
</script>

<template>
  <a class="skip" href="#main">{{ c.skip }}</a>
  <BlogNav />
  <main id="main" tabindex="-1">
    <section v-if="page.isNotFound" class="lost">
      <h1>{{ c.lostTitle }}</h1>
      <p>{{ c.lostText }}</p>
      <a :href="withBase(en ? '/en/' : '/')">{{ c.back }}</a>
    </section>
    <PostList v-else-if="frontmatter.layout === 'index'" />
    <PostPage v-else :key="page.relativePath" />
  </main>
  <footer class="foot">
    <p>{{ c.legal }}</p>
    <nav :aria-label="c.brand">
      <a :href="root + (en ? 'en/' : '')" target="_self">{{ c.home }}</a>
      <a :href="`${root}docs/${en ? 'en/' : ''}`" target="_self">{{ c.docs }}</a>
      <a :href="repo">GitHub</a>
      <a :href="withBase(en ? '/en/feed.xml' : '/feed.xml')" target="_self">RSS</a>
    </nav>
  </footer>
</template>

<style scoped>
.skip { position: fixed; top: 0.75rem; left: 0.75rem; z-index: 100; padding: 0.5rem 1rem; border-radius: 999px; background: var(--vp-c-brand-1); color: #fff; transform: translateY(-200%); }
.skip:focus { transform: none; }
main { min-height: 70vh; outline: none; }
.lost { display: flex; flex-direction: column; align-items: center; padding: 12rem 1.5rem 6rem; text-align: center; }
.lost h1 { margin: 0; font-size: clamp(2rem, 5vw, 3rem); font-weight: 600; letter-spacing: -0.03em; }
.lost p { margin: 1rem 0 2rem; color: var(--vp-c-text-2); }
.lost a { padding: 0.7rem 1.5rem; border-radius: 999px; background: #0048e8; color: #fff; font-weight: 600; }
.foot { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 0.75rem 2rem; width: 100%; padding: 1.75rem max(1.25rem, calc((100% - 76rem) / 2)); border-top: 1px solid var(--vp-c-divider); background: var(--vp-c-bg-alt); font-size: 0.875rem; color: var(--vp-c-text-3); }
.foot p { margin: 0; }
.foot nav { display: flex; flex-wrap: wrap; gap: 0.5rem 1.5rem; }
.foot a { color: var(--vp-c-text-2); transition: color 200ms; }
.foot a:hover { color: var(--vp-c-brand-1); }
</style>
