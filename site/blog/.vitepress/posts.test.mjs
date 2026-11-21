import assert from 'node:assert/strict';
import test from 'node:test';
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { postIndex } from './posts.mjs';

const pair = (key, developmentDate, sequence) => ['', '/en'].map(dir => ({url:`${dir}/posts/${key}.html`, src:'example', frontmatter:{title:key, description:'Summary', draft:true, translationKey:key, developmentDate, ...(sequence === undefined ? {} : {sequence})}}));
test('both lists use development date, then slug, regardless of file order', () => {
  const posts = postIndex([...pair('first','2026-01-01'), ...pair('z','2026-11-01'), ...pair('a','2026-11-01')]);
  for (const en of [true,false]) assert.deepEqual(posts.filter(p => p.en === en).map(p => p.key), ['a','z','first']);
});
test('a translation cannot silently disappear or change the development date', () => {
  const [zh,en] = pair('example','2026-11-01');
  assert.throws(() => postIndex([zh]), /Missing translation/);
  en.frontmatter.developmentDate = '2026-11-02';
  assert.throws(() => postIndex([zh,en]), /matching development date/);
});
test('within one development day a later sequence is listed first, in both languages', () => {
  const posts = postIndex([...pair('morning','2026-11-08',1), ...pair('evening','2026-11-08',3), ...pair('noon','2026-11-08',2), ...pair('day-before','2026-11-07',9)]);
  for (const en of [true,false]) assert.deepEqual(posts.filter(p => p.en === en).map(p => p.key), ['evening','noon','morning','day-before']);
});
test('both translations must agree on the sequence, and it must be a whole number', () => {
  const [zh,en] = pair('example','2026-11-01',2);
  en.frontmatter.sequence = 3;
  assert.throws(() => postIndex([zh,en]), /matching development date and sequence/);
  for (const bad of [-1, 1.5, '2']) {
    const [a,b] = pair('example','2026-11-01',bad);
    assert.throws(() => postIndex([a,b]), /Invalid post frontmatter/);
  }
});

// Images: every post carries real pictures that are cheap to load and described for readers.
const blogRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
function webpSize(buf) {
  assert.equal(buf.toString('ascii', 0, 4), 'RIFF'); assert.equal(buf.toString('ascii', 8, 12), 'WEBP');
  const kind = buf.toString('ascii', 12, 16);
  if (kind === 'VP8 ') return [buf.readUInt16LE(26) & 0x3fff, buf.readUInt16LE(28) & 0x3fff];
  if (kind === 'VP8L') { const b = buf.readUInt32LE(21); return [(b & 0x3fff) + 1, ((b >> 14) & 0x3fff) + 1]; }
  assert.equal(kind, 'VP8X'); return [buf.readUIntLE(24, 3) + 1, buf.readUIntLE(27, 3) + 1];
}
test('every post has figures with alt text, size attributes, lazy loading and a light WebP file', () => {
  for (const dir of ['posts', 'en/posts']) for (const f of readdirSync(join(blogRoot, dir)).filter(n => n.endsWith('.md'))) {
    const where = `${dir}/${f}`, text = readFileSync(join(blogRoot, dir, f), 'utf8');
    const tags = [...text.matchAll(/<img\b[^>]*>/g)].map(m => m[0]);
    assert.ok(tags.length >= 1, `${where}: needs at least one image`);
    assert.equal([...text.matchAll(/<figure>[\s\S]*?<img\b[\s\S]*?<figcaption>[^<]{12,}<\/figcaption>\s*<\/figure>/g)].length, tags.length, `${where}: each image needs a figure with a caption`);
    for (const tag of tags) {
      const attr = n => tag.match(new RegExp(`\\s${n}="([^"]*)"`))?.[1];
      const [w, h] = [Number(attr('width')), Number(attr('height'))], file = join(blogRoot, dir, attr('src') ?? '');
      assert.ok((attr('alt') ?? '').length >= 12 && !/woodfish|C:\\Users/i.test(attr('alt')), `${where}: alt text`);
      assert.ok(['lazy', 'eager'].includes(attr('loading')), `${where}: loading attribute`);
      assert.ok(file.endsWith('.webp') && existsSync(file), `${where}: image file ${attr('src')}`);
      const buf = readFileSync(file), [rw, rh] = webpSize(buf);
      assert.ok(buf.length <= 300 * 1024, `${where}: ${attr('src')} is ${Math.round(buf.length / 1024)} KB`);
      assert.ok(rw <= 1600 && w === rw && h === rh, `${where}: ${attr('src')} is ${rw}x${rh}, attributes say ${w}x${h}`);
    }
  }
});
