import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import { locales } from '../landing/src/locales.ts';

const root = new URL('../docs/', import.meta.url);
const output = new URL(process.env.VITE_SITE === 'cn' ? '../dist-cn/docs/' : '../dist/docs/', import.meta.url);
const read = file => readFile(file, 'utf8');
const pageNames = async dir => (await readdir(dir)).filter(p => p.endsWith('.md')).sort();
const pages = await pageNames(root);
const blocks = s => [...s.matchAll(/^ *```[^\n]*\n[\s\S]*?^ *```/gm)].map(m => m[0]);
const prose = s => s.replace(/^ *```[^\n]*\n[\s\S]*?^ *```/gm, '').replace(/`[^`\n]+`|\]\([^)]*\)/g,'').split('\n').map(l => l.trim()).filter(l => (l.match(/\b[A-Za-z]{2,}\b/g)?.length ?? 0) >= 9 || /[\u4e00-\u9fff]{14}/u.test(l));
const fence = String.fromCharCode(96).repeat(3);
const anyText = '[' + String.fromCharCode(92) + 's' + String.fromCharCode(92) + 'S]*?';
const withoutCodeBlocks = s => s.replace(new RegExp(fence + anyText + fence, 'g'), '');
const han = /[㐀-䶿一-鿿]/u;
const simplified = /[为这们让从后还过发进开关与业专门问无现备复机连统传图录仅级译术览归盘线产带别认时该应证说读设计变网处广汉听启类页节块输编样欢标键锁扫预边层导态优严确际]/u;
const sitemap = await read(new URL('sitemap.xml', output));
for (const l of locales) {
  assert.deepEqual(await pageNames(new URL(l.path || '.', root)), pages, l.html);
  for (const page of pages) {
    const source = await read(new URL(l.path + page, root));
    assert.ok(!/<[0-9]+>/.test(withoutCodeBlocks(source)), 'placeholder marker in ' + l.path + page);
    if (page === 'index.md') {
      const text = withoutCodeBlocks(source);
      if (!['zh', 'zh-TW', 'ja'].includes(l.id)) assert.ok(!han.test(text), 'Han character in ' + l.path + page);
      if (l.id === 'ja' || l.id === 'zh-TW') assert.ok(!simplified.test(text), 'Simplified character in ' + l.path + page);
    }
    if (!['en','zh'].includes(l.id)) {
      const origin = await read(new URL((l.id === 'zh-TW' && page !== 'index.md' ? '' : 'en/') + page, root));
      assert.deepEqual(blocks(source), blocks(origin), `changed code: ${l.path}${page}`);
      const originals = new Set(prose(origin));
      for (const line of prose(source)) assert.ok(!originals.has(line), `untranslated: ${l.path}${page}: ${line}`);
    }
    const file = page.replace(/\.md$/, '.html');
    const href = l.path + (page === 'index.md' ? '' : file);
    const html = await read(new URL(l.path + file, output));
    assert.ok(sitemap.includes(`/docs/${href}</loc>`), `sitemap: ${href}`);
    for (const other of locales) {
      const dest = other.path + (page === 'index.md' ? '' : file);
      assert.ok(html.includes(`hreflang="${other.html}" href="${process.env.SITE_URL ?? 'https://obituary.woodfish.site'}${process.env.SITE_BASE ?? '/'}docs/${dest}"`), `alternate ${other.html} on ${href}`);
    }
    if (process.env.VITE_SITE === 'cn' && page === 'index.md') {
      const commands = [...html.matchAll(/<pre\b[^>]*>([\s\S]*?)<\/pre>/g)].map(m => m[1].replace(/<[^>]+>/g,'')).join('\n');
      for (const script of ['install.sh','install.ps1']) assert.ok(commands.includes(`https://cn.obituary.woodfish.site/${script}`), `mirror ${script}: ${l.html}`);
      for (const flag of ['bash install.sh --mirror cn', '.\\install.ps1 -Mirror cn']) assert.ok(commands.includes(flag), `mirror arguments: ${l.html}`);
      assert.ok(!commands.includes('https://raw.githubusercontent.com/woodfishhhh/obituary/main/'), `GitHub installer in mirror: ${l.html}`);
    }
  }
}
console.log(`Checked ${pages.length} pages × ${locales.length} languages, alternate links, sitemap and installers.`);
