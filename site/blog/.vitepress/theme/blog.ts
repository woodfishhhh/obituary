import { computed } from 'vue';
import { useData } from 'vitepress';
import { data } from '../posts.data';
import { copy } from './copy';

// The pictures are imported where Vite can hash them; posts.data.ts only knows their paths under blog/.
const urls = import.meta.glob<string>('../../images/*/*.webp', { eager: true, query: '?url', import: 'default' });
export const coverUrl = (src: string) => urls[`../../${src}`];

// Which language's posts this page lists, and where this page sits among them (the list is newest first).
export function useBlog() {
  const { page, lang, site } = useData();
  const en = computed(() => lang.value === 'en');
  const list = computed(() => data.filter(p => p.en === en.value));
  const index = computed(() => list.value.findIndex(p => p.url === '/' + page.value.relativePath.replace(/\.md$/, '.html')));
  // The site root and the docs sit outside this build's base, so the SPA router must leave them to the browser (target="_self").
  const root = computed(() => site.value.base.replace(/blog\/$/, ''));
  return { en, c: computed(() => copy(en.value)), list, index, post: computed(() => list.value[index.value]), root };
}
