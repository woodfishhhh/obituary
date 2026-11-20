<script setup lang="ts">
import { guides } from './catalog';
import { useData, withBase } from 'vitepress';
const { localeIndex } = useData();
</script>

<template>
  <ul>
    <li v-for="{ id, name } in guides" :key="id">
      <a :href="withBase('/' + (localeIndex === 'root' ? '' : localeIndex + '/') + 'agents/' + id + '.html')">{{ name }}</a>
    </li>
  </ul>
</template>
