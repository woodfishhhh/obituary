import DefaultTheme from "vitepress/theme";
import type { Theme } from "vitepress";
import { h } from "vue";
import LanguageMenu from './LanguageMenu.vue';
import "./styles/docs.css";

export default {
  extends: DefaultTheme,
  Layout: () => h(DefaultTheme.Layout, null, { 'nav-bar-content-after': () => h(LanguageMenu) }),
} satisfies Theme;
