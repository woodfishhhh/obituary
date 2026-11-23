import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { locales, siteOrigin } from '../landing/src/locales.ts';
import { agentData, agentDocLink } from '../landing/src/agents-data.ts';
import { findBrokenLinks } from './check-links.mjs';

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
const repo = fileURLToPath(new URL('../..', import.meta.url));
execFileSync(process.execPath, [fileURLToPath(new URL('./generate-command-reference.mjs', import.meta.url)), '--check'], { cwd: repo, stdio: 'inherit' });
const sitemap = await read(new URL('sitemap.xml', output));
// User pages and generated agent guides retain their ten-language contract below.
const devPages = await pageNames(new URL('dev/', root));
assert.ok(devPages.includes('index.md'));
assert.deepEqual(await pageNames(new URL('en/dev/', root)), devPages);
for (const l of locales) {
  if (l.id !== 'zh' && l.id !== 'en') {
    assert.ok(!(await readdir(new URL(l.path, root))).includes('dev'), `unexpected development tree: ${l.path}`);
    continue;
  }
  for (const page of devPages) {
    const path = `${l.path}dev/${page}`;
    const src = await read(new URL(path, root));
    assert.equal((src.match(/^ *```/gm) ?? []).length, blocks(src).length * 2, `unclosed code fence: ${path}`);
    const html = await read(new URL(path.replace(/\.md$/, '.html'), output));
    assert.equal((html.match(/<link rel="alternate" hreflang=/g) ?? []).length, 2, `dev alternates: ${path}`);
    assert.ok(sitemap.includes(`/docs/${path.replace(/index\.md$/, '').replace(/\.md$/, '.html')}</loc>`), `dev sitemap: ${path}`);
  }
}
const configSource = await read(new URL('../../internal/config/config.go', import.meta.url));
const structs = new Map([...configSource.matchAll(/type (\w+) struct \{([\s\S]*?)\n\}/g)].map(([, name, body]) => [name, [...body.matchAll(/^\s*\w+\s+(\S+)\s+`toml:"([^"]+)"`/gm)].map(([, type, tag]) => [type, tag.replace(/,.*$/, '')])]));
const configKeys = [];
const expandConfigKeys = (type, prefix = '') => {
  for (const [fieldType, tag] of structs.get(type) ?? []) {
    const key = prefix ? `${prefix}.${tag}` : tag;
    if (structs.has(fieldType)) expandConfigKeys(fieldType, key);
    else configKeys.push(key);
  }
};
expandConfigKeys('Config');
for (const l of locales) {
  assert.deepEqual(await pageNames(new URL(l.path || '.', root)), pages, l.html);
  const agentPages = agentData.map(a => `agents/${a.id}.md`);
  const landing = await read(new URL(`../${l.path}index.html`, output));
  const directory = await read(new URL(`${l.path}agents.html`, output));
  const article = directory.match(/<div[^>]+class="vp-doc[^>]*>([\s\S]*?)<\/main>/)?.[1];
  for (const { id } of agentData) {
    const route = `docs/${l.path}agents/${id}`;
    const target = `${route}.html`;
    assert.equal(agentDocLink(id, l.id), route, `landing route: ${id} (${l.id})`);
    assert.ok(landing.includes(`href="${process.env.SITE_BASE ?? '/'}${target}"`), `missing landing link: ${target}`);
    assert.ok(article?.includes(`href="${process.env.SITE_BASE ?? '/'}${target}"`), `missing directory entry: ${target}`);
  }
  for (const page of pages) {
    const source = await read(new URL(l.path + page, root));
    if (page === 'config.md') for (const key of configKeys) assert.ok(source.includes(String.fromCharCode(96) + key + String.fromCharCode(96)), 'config key missing: ' + l.path + key);
    assert.ok(!/<[0-9]+>/.test(withoutCodeBlocks(source)), 'placeholder marker in ' + l.path + page);
    if (page === 'index.md') {
      const text = withoutCodeBlocks(source);
      if (!['zh', 'zh-TW', 'ja'].includes(l.id)) assert.ok(!han.test(text), 'Han character in ' + l.path + page);
      if (l.id === 'ja' || l.id === 'zh-TW') assert.ok(!simplified.test(text), 'Simplified character in ' + l.path + page);
    }
    if (!['en','zh'].includes(l.id)) {
      const origin = await read(new URL((page === 'index.md' ? 'en/' : l.id === 'zh-TW' ? '' : 'en/') + page, root));
      if (page === 'index.md') assert.deepEqual(blocks(source), blocks(origin), 'changed code: ' + l.path + page);
      const originals = new Set(prose(origin));
      for (const line of prose(source)) assert.ok(!originals.has(line), 'untranslated: ' + l.path + page + ': ' + line);
    }
  }
  for (const page of [...pages, ...agentPages]) {
    const file = page.replace(/\.md$/, '.html');
    const href = l.path + (page === 'index.md' ? '' : file);
    const html = await read(new URL(l.path + file, output));
    if (agentPages.includes(page)) {
      const name = agentData.find(a => page === `agents/${a.id}.md`).name;
      assert.ok(html.match(/<h1\b[^>]*>([^<]*)/u)?.[1].trim() === name, `wrong agent: ${href}`);
    }
    assert.ok(sitemap.includes(`/docs/${href}</loc>`), `sitemap: ${href}`);
    for (const other of locales) {
      const dest = other.path + (page === 'index.md' ? '' : file);
      assert.ok(html.includes(`hreflang="${other.html}" href="${process.env.SITE_URL ?? 'https://obituary.woodfish.site'}${process.env.SITE_BASE ?? '/'}docs/${dest}"`), `alternate ${other.html} on ${href}`);
    }
    if (process.env.VITE_SITE === 'cn' && (page === 'index.md' || agentPages.includes(page))) {
      const commands = [...html.matchAll(/<pre\b[^>]*>([\s\S]*?)<\/pre>/g)].map(m => m[1].replace(/<[^>]+>/g,'')).join('\n');
      for (const script of ['install.sh','install.ps1']) assert.ok(commands.includes(`https://cn.obituary.woodfish.site/${script}`), `mirror ${script}: ${l.html}`);
      for (const flag of ['bash install.sh --mirror cn', '.\\install.ps1 -Mirror cn']) assert.ok(commands.includes(flag), `mirror arguments: ${l.html}`);
      assert.ok(!commands.includes('https://raw.githubusercontent.com/woodfishhhh/obituary/main/'), `GitHub installer in mirror: ${l.html}`);
    }
  }
}
console.log(`Checked ${pages.length} docs and ${agentData.length} agent guides × ${locales.length} languages, landing links, directories, alternate links, sitemap and installers.`);
const site = new URL(process.env.SITE_BASE ?? '/', process.env.SITE_URL ?? siteOrigin);
const broken = await findBrokenLinks(new URL('../', output), site);
console.log(`Checked site links: ${broken.length} broken (distinct source page + href).`);
assert.ok(!broken.length, `Broken site links:\n${broken.join('\n')}`);
