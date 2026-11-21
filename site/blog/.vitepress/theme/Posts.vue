<script setup>
import { computed } from 'vue';
import { useData, withBase } from 'vitepress';
import { data } from '../posts.data';
defineProps({ position:{type:String, default:'list'} });
const { page, lang } = useData();
const en = computed(() => lang.value === 'en');
const list = computed(() => data.filter(p => p.en === en.value));
const index = computed(() => list.value.findIndex(p => p.url === '/' + page.value.relativePath.replace(/\.md$/,'.html')));
const post = computed(() => list.value[index.value]);
</script>

<template>
  <ol v-if="position === 'list'" class="post-list">
    <li v-for="p in list" :key="p.key"><time :datetime="p.date">{{ p.date }}</time><h2><a :href="withBase(p.url)">{{ p.title }}</a></h2><p>{{ p.description }}</p></li>
  </ol>
  <header v-else-if="post && position === 'header'" class="vp-doc post-heading">
    <h1>{{ post.title }}</h1><p><time :datetime="post.date">{{ post.date }}</time> · {{ en ? `${post.minutes} min read` : `阅读约 ${post.minutes} 分钟` }}<span v-if="post.draft"> · {{ en ? 'Draft' : '草稿' }}</span></p>
  </header>
  <nav v-else-if="post && position === 'navigation'" class="post-links" :aria-label="en ? 'Article navigation' : '文章导航'">
    <a v-if="list[index+1]" :href="withBase(list[index+1].url)">{{ en ? 'Previous' : '上一篇' }}: {{ list[index+1].title }}</a>
    <a :href="withBase(en ? '/en/' : '/')">{{ en ? 'All posts' : '全部文章' }}</a>
    <a v-if="index > 0" :href="withBase(list[index-1].url)">{{ en ? 'Next' : '下一篇' }}: {{ list[index-1].title }}</a>
  </nav>
</template>

<style scoped>
.vp-doc .post-list { list-style:none; padding:0 }
.post-list li,.post-heading { margin:2rem 0 }
.post-list h2 { border:0; padding:0; margin:.4rem 0 }
.post-list time,.post-heading p { color:var(--vp-c-text-2) }
.post-links { display:flex; flex-wrap:wrap; gap:1rem; justify-content:space-between; border-top:1px solid var(--vp-c-divider); padding:1rem 0; color:var(--vp-c-brand-1) }
</style>
