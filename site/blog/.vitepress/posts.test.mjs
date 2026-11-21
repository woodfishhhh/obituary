import assert from 'node:assert/strict';
import test from 'node:test';
import { postIndex } from './posts.mjs';

const pair = (key, developmentDate) => ['', '/en'].map(dir => ({url:`${dir}/posts/${key}.html`, src:'example', frontmatter:{title:key, description:'Summary', draft:true, translationKey:key, developmentDate}}));
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
