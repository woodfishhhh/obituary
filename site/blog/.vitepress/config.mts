import { writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitepress';
import { devLocales, pageSuffix } from '../../docs/.vitepress/routes';
import { loadPosts } from './posts.mjs';

const site = `${process.env.SITE_URL ?? 'https://obituary.woodfish.site'}${process.env.SITE_BASE ?? '/'}`;
const base = `${process.env.SITE_BASE ?? '/'}blog/`;
const absolute = `${site}blog/`;
const xml = (value: string) => value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');

export default defineConfig({
  base, srcExclude:['README.md'], sitemap:{hostname:absolute},
  vite:{publicDir:fileURLToPath(new URL('../../docs/public/',import.meta.url))},
  locales: Object.fromEntries(devLocales.map(l => {
    const en = l.id === 'en';
    return [en ? 'en' : 'root', {label:l.name, lang:l.html, title:en ? 'Obituary · Development blog' : '讣告 · 开发博客', themeConfig:{
      siteTitle:en ? 'Obituary' : '讣告', langMenuLabel:en ? 'Language' : '语言', outline:{label:en ? 'On this page' : '本页内容'},
      nav:[{text:en ? 'Posts' : '文章',link:`/${l.path}`}, {text:en ? 'Developer guide' : '开发文档',link:`${site}docs/${l.path}dev/`,target:'_self',noIcon:true}],
    }}];
  })),
  themeConfig:{logo:'/icon-128.png', docFooter:{prev:false,next:false}},
  transformHead({pageData}) {
    if (pageData.isNotFound) return [];
    const suffix = pageSuffix(pageData.relativePath);
    const dir = pageData.relativePath.startsWith('en/') ? 'en/' : '';
    return [['link',{rel:'canonical',href:absolute+dir+suffix}], ['link',{rel:'alternate',type:'application/rss+xml',href:absolute+dir+'feed.xml'}],
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
