import type { Theme } from 'vitepress';
import '../../../../cmd/obit-gui/frontend/src/squircle.css';
// The default theme's design tokens and its Markdown styling (text, code blocks, code groups). Its pages, navigation and
// search are not used here, and not loading them keeps the script and the stylesheet to what the blog shows; its Inter
// face is left out because the blog uses Geist, the landing page's face (blog.css).
import 'vitepress/dist/client/theme-default/styles/vars.css';
import 'vitepress/dist/client/theme-default/styles/base.css';
import 'vitepress/dist/client/theme-default/styles/icons.css';
import 'vitepress/dist/client/theme-default/styles/utils.css';
import 'vitepress/dist/client/theme-default/styles/components/custom-block.css';
import 'vitepress/dist/client/theme-default/styles/components/vp-code.css';
import 'vitepress/dist/client/theme-default/styles/components/vp-code-group.css';
import 'vitepress/dist/client/theme-default/styles/components/vp-doc.css';
import '../../../docs/.vitepress/theme/styles/docs.css';
import './blog.css';
import './prose.css';
import Layout from './Layout.vue';

// VitePress supplies the Markdown pipeline, its code blocks and the routing; the page around the article is the blog's own.
export default { Layout } satisfies Theme;
