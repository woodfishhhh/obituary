<script setup lang="ts">
import { computed } from 'vue';
import { coverUrl } from './blog';

// A post's first picture on a tinted mat. Pictures much wider than tall (terminal strips) sit whole on the mat
// instead of being cropped to a sliver. The picture is decoration here: the link around it carries the title.
const props = defineProps<{ post: { cover: { src: string; width: number; height: number } | null }; eager?: boolean }>();
const wide = computed(() => !!props.post.cover && props.post.cover.width / props.post.cover.height > 2.2);
</script>

<template>
  <div v-if="post.cover" class="cover" :class="{ wide }" aria-hidden="true">
    <img :src="coverUrl(post.cover.src)" alt="" :width="post.cover.width" :height="post.cover.height" :loading="eager ? 'eager' : 'lazy'" :fetchpriority="eager ? 'high' : undefined" decoding="async">
  </div>
</template>

<style scoped>
.cover { position: relative; overflow: hidden; background: linear-gradient(135deg, var(--b-mat-1), var(--b-mat-2)); }
/* The same dots the landing page's first screen is drawn with. */
.cover::before { content: ''; position: absolute; inset: 0; background: radial-gradient(circle at 1px 1px, var(--b-dot) 1px, transparent 0) 0 0 / 18px 18px; }
img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; object-position: 0 0; transition: transform 700ms var(--b-ease); }
.wide img { inset: 0; margin: auto; width: 88%; height: auto; border-radius: 10px; box-shadow: 0 18px 40px -18px rgb(17 21 34 / 0.5), 0 0 0 1px rgb(17 21 34 / 0.08); object-fit: contain; }
:global(.card:hover) img { transform: scale(1.035); }
@media (prefers-reduced-motion: reduce) { img { transition: none; } :global(.card:hover) img { transform: none; } }
</style>
