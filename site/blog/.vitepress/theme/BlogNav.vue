<script setup lang="ts">
import { inject, ref, watchPostEffect } from 'vue';
import { useData, withBase } from 'vitepress';
import LanguageMenu from '../../../docs/.vitepress/theme/LanguageMenu.vue';
import { devLocales } from '../../../docs/.vitepress/routes';
import { useBlog } from './blog';

// The landing page's floating pill, with the blog's own links in it.
const { isDark } = useData();
const { en, c, root } = useBlog();
const toggle = inject('toggle-appearance', () => { isDark.value = !isDark.value; });
// Set after mounting, as the default theme's switch does: the server cannot know the reader's mode, so a label computed during rendering would not match.
const label = ref(c.value.themeToDark);
watchPostEffect(() => { label.value = isDark.value ? c.value.themeToLight : c.value.themeToDark; });
const dir = () => (en.value ? 'en/' : '');
const repo = 'https://github.com/woodfishhhh/obituary';
</script>

<template>
  <header class="nav-wrap">
    <nav class="nav" :aria-label="c.mainNav">
      <a class="brand" :href="root + dir()" target="_self">
        <img :src="withBase('/icon-128.png')" alt="" width="32" height="32">
        <span class="brand-name">{{ c.brand }}</span>
      </a>
      <a class="item section" :href="withBase(en ? '/en/' : '/')" aria-current="page">{{ c.section }}</a>
      <span class="grow" />
      <a class="item wide-only" :href="`${root}docs/${dir()}`" target="_self">{{ c.docs }}</a>
      <a class="item wide-only" :href="`${root}docs/${dir()}dev/`" target="_self">{{ c.dev }}</a>
      <LanguageMenu :languages="devLocales" />
      <button class="icon" type="button" :aria-label="label" :title="label" @click="toggle">
        <svg class="sun" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></svg>
        <svg class="moon" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" /></svg>
      </button>
      <a class="github" :href="repo">GitHub</a>
    </nav>
  </header>
</template>

<style scoped>
.nav-wrap { position: fixed; inset: 0.75rem 0 auto; z-index: 50; padding-inline: 0.75rem; pointer-events: none; }
.nav { pointer-events: auto; display: flex; align-items: center; gap: 0.25rem; max-width: 1240px; height: 3.5rem; margin-inline: auto; padding: 0 0.5rem 0 1rem; border: 1px solid color-mix(in oklab, var(--vp-c-divider) 80%, transparent); border-radius: 999px; background: color-mix(in oklab, var(--vp-c-bg) 75%, transparent); box-shadow: var(--b-shadow-nav); backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px); }
.nav { position: relative; }
.nav > * { flex: none; }
.nav > .grow { flex: 1; }
.nav > .section { flex: 0 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.nav :deep(.obit-language) { margin-inline: 0; }
.nav :deep(.obit-language summary) { min-height: 2.5rem; padding-inline: 0.7rem; border-radius: 999px; transition: background-color 200ms; }
.nav :deep(.obit-language summary:hover) { background: var(--vp-c-bg-soft); }
.brand { display: flex; align-items: center; gap: 0.5rem; margin-right: 0.25rem; padding: 0.25rem 0.5rem 0.25rem 0; border-radius: 999px; font-weight: 600; color: var(--vp-c-text-1); }
.brand img { width: 2rem; height: 2rem; }
.item { padding: 0.5rem 0.75rem; border-radius: 999px; font-size: 0.875rem; font-weight: 500; white-space: nowrap; color: var(--vp-c-text-2); transition: background-color 200ms, color 200ms; }
.item:hover { background: var(--vp-c-bg-soft); color: var(--vp-c-text-1); }
.section { color: var(--vp-c-text-1); background: var(--vp-c-bg-soft); }
.icon { display: grid; place-items: center; width: 2.5rem; height: 2.5rem; border: 0; border-radius: 999px; background: none; color: var(--vp-c-text-2); cursor: pointer; transition: background-color 200ms, color 200ms; }
.icon:hover { background: var(--vp-c-bg-soft); color: var(--vp-c-text-1); }
.moon, .dark .sun { display: none; }
.dark .moon { display: block; }
.github { margin-left: 0.25rem; padding: 0.5rem 1rem; border-radius: 999px; font-size: 0.875rem; font-weight: 500; background: var(--vp-c-text-1); color: var(--vp-c-bg); transition: opacity 200ms; }
.github:hover { opacity: 0.85; }
@media (max-width: 719px) { .wide-only { display: none; } .brand-name { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); } }
/* On a phone the language list hangs from the pill's edge, so it cannot run off the left of a narrow screen. */
@media (max-width: 559px) { .nav :deep(.obit-language) { position: static; } .nav :deep(.obit-language nav) { right: 0.5rem; } }
@media (max-width: 479px) { .section { display: none; } }
@media (max-width: 400px) { .nav { padding-left: 0.75rem; } .github { padding-inline: 0.75rem; } }
</style>
