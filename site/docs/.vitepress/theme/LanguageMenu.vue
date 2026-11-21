<script setup lang="ts">
import { computed, ref } from 'vue';
import { useData, withBase } from 'vitepress';
import { locales, type Locale } from '../../../landing/src/locales';
import { pageLocales, pageSuffix } from '../routes';
import sprite from './flag-symbols.svg?raw';

const { page, lang, theme } = useData();
const current = computed(() => locales.find(l => l.html === lang.value) ?? locales[0]);
const menu = ref<HTMLDetailsElement>();
const summary = ref<HTMLElement>();
const width = ({flag}: Locale) => 16 * ({us:19/10, de:5/3, br:10/7}[flag] ?? 3/2);
function close(restoreFocus = false) {
  if (!menu.value?.open) return;
  menu.value.open = false;
  if (restoreFocus) summary.value?.focus();
}
</script>

<template>
  <span aria-hidden="true" v-html="sprite" />
  <details ref="menu" class="obit-language" @keydown.esc="close(true)" @focusout="!menu?.contains($event.relatedTarget) && close()">
    <summary ref="summary" :aria-label="theme.langMenuLabel + ': ' + current.name" :title="theme.langMenuLabel">
      <svg :width="width(current)" height="16" aria-hidden="true"><use :href="`#flag-${current.flag}`" /></svg>
      <span class="vpi-chevron-down" />
    </summary>
    <nav :aria-label="theme.langMenuLabel">
      <a v-for="l of pageLocales(page.relativePath)" :key="l.id" :href="withBase('/' + l.path + pageSuffix(page.relativePath))" :lang="l.html" :hreflang="l.html" :aria-current="l.id === current.id ? 'page' : undefined" @click="close()">
        <svg :width="width(l)" height="16" aria-hidden="true"><use :href="`#flag-${l.flag}`" /></svg><span>{{ l.name }}</span>
      </a>
    </nav>
  </details>
</template>

<style scoped>
.obit-language {position:relative;margin-inline:8px}
summary {display:flex;align-items:center;gap:5px;min-height:44px;cursor:pointer;list-style:none}
summary::-webkit-details-marker {display:none}
nav {position:absolute;right:0;top:100%;min-width:195px;max-height:70vh;overflow:auto;padding:6px;background:var(--vp-c-bg-elv);border:1px solid var(--vp-c-divider);border-radius:8px;box-shadow:var(--vp-shadow-2)}
a {display:grid;grid-template-columns:32px 1fr;align-items:center;gap:8px;padding:5px 9px;font-size:14px}
a:hover,a:focus-visible {background:var(--vp-c-bg-soft)}
a[aria-current] {color:var(--vp-c-brand-1);font-weight:600}
</style>
