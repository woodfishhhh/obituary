import { h } from 'vue';
import DefaultTheme from 'vitepress/theme';
import LanguageMenu from '../../../docs/.vitepress/theme/LanguageMenu.vue';
import { devLocales } from '../../../docs/.vitepress/routes';
import '../../../docs/.vitepress/theme/styles/docs.css';
import Posts from './Posts.vue';
import './figure.css';

export default { extends:DefaultTheme, Layout:() => h(DefaultTheme.Layout, null, {
  'nav-bar-content-after':() => h(LanguageMenu,{languages:devLocales}),
  'doc-before':() => h(Posts,{position:'header'}),
  'doc-after':() => h(Posts,{position:'navigation'}),
}), enhanceApp({app}) { app.component('PostList', Posts); } };
