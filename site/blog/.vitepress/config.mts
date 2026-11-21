import { writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitepress';
import { devLocales, pageSuffix } from '../../docs/.vitepress/routes';
import { loadPosts } from './posts.mjs';
import squircleCorners from '../../../cmd/obit-gui/frontend/scripts/squircle-corners.mjs';

const site = `${process.env.SITE_URL ?? 'https://obituary.woodfish.site'}${process.env.SITE_BASE ?? '/'}`;
const base = `${process.env.SITE_BASE ?? '/'}blog/`;
const absolute = `${site}blog/`;
const xml = (value: string) => value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');

// The landing page's rule for its own faces: one that has not arrived in the first fraction of a second (the page preloads
// the Latin one, so it normally has) is left for the next visit instead of swapping in mid-reading and shifting the text.
const fontsOptional = {postcssPlugin:'obit-fonts-optional', AtRule:{'font-face':(rule:any) => { if (/Geist/.test(rule.toString())) rule.walkDecls('font-display', (d:any) => { d.value = 'optional'; }); }}};

export default defineConfig({
  base,
  head:[['link',{rel:'icon',type:'image/png',sizes:'32x32',href:`${base}icon-32.png`}], ['link',{rel:'icon',type:'image/png',sizes:'512x512',href:`${base}icon-512.png`}], ['link',{rel:'apple-touch-icon',sizes:'180x180',href:`${base}icon-180.png`}]],
  srcExclude:['README.md'], sitemap:{hostname:absolute},
  // Headings feed the article's contents list (the theme reads page.headers).
  markdown:{headers:{level:[2]}},
  vite:{
    publicDir:fileURLToPath(new URL('../../docs/public/',import.meta.url)),
    // Rounded rectangles become squircles where the engine can draw them, as on the landing page and in the docs.
    css:{postcss:{plugins:[fontsOptional, squircleCorners()]}},
    // The dev server may read the stylesheet the window shares (src/squircle.css, imported by the theme).
    server:{fs:{allow:[fileURLToPath(new URL('../../..',import.meta.url))]}},
  },
  locales: Object.fromEntries(devLocales.map(l => {
    const en = l.id === 'en';
    return [en ? 'en' : 'root', {label:l.name, lang:l.html, title:en ? 'Obituary · Development blog' : '讣告 · 开发博客', themeConfig:{
      langMenuLabel:en ? 'Language' : '语言', outline:{label:en ? 'On this page' : '本页内容'},
    }}];
  })),
  transformHead({pageData, assets}) {
    // The Latin face is preloaded where it is used (blog.css loads it from 768px up), so the first paint can use it.
    const font = assets.find(a => /geist-latin-wght-normal\.[\w-]+\.woff2$/.test(a));
    const preload = font ? [['link',{rel:'preload',as:'font',type:'font/woff2',href:font,crossorigin:'',media:'(min-width: 768px)'}] as ['link',Record<string,string>]] : [];
    if (pageData.isNotFound) return preload;
    const suffix = pageSuffix(pageData.relativePath);
    const dir = pageData.relativePath.startsWith('en/') ? 'en/' : '';
    return [...preload, ['link',{rel:'canonical',href:absolute+dir+suffix}], ['link',{rel:'alternate',type:'application/rss+xml',href:absolute+dir+'feed.xml'}],
      ...devLocales.map(l => ['link',{rel:'alternate',hreflang:l.html,href:absolute+l.path+suffix}] as ['link',Record<string,string>])];
  },
  async buildEnd({outDir}) {
    const posts = await loadPosts().load();
    for (const l of devLocales) {
      const en = l.id === 'en';
      const title = en ? 'Obituary development blog' : '讣告开发博客';
      const items = posts.filter(p => p.en === en).map(p => {
        const url = xml(absolute + p.url.slice(1));
        return `<item><title>${xml(p.title)}</title><link>${url}</link><guid>${url}</guid><description>${xml(p.description)}</description><pubDate>${new Date(p.date).toUTCString()}</pubDate></item>`;
      });
      await writeFile(join(outDir,l.path,'feed.xml'), `<?xml version="1.0" encoding="UTF-8"?>\n<rss version="2.0"><channel><title>${title}</title><link>${xml(absolute+l.path)}</link><language>${l.html}</language><description>${title}</description>${items.join('')}</channel></rss>\n`);
    }
  },
});
