import assert from 'node:assert/strict';
import test from 'node:test';
import { checkLinks } from './check-links.mjs';

test('assembled links resolve from their source page and require existing anchors', () => {
  const files = new Map([
    ['docs/index.html', `
      <link rel="alternate" hreflang="ja" href="https://example.test/project/docs/ja/">
      <a href="ja/">Same missing page, another spelling</a>
      <a href="#missing-local">Missing local anchor</a>
      <a href="../blog/?lang=en&amp;view=list#missing-remote">Missing cross-page anchor</a>
      <a href="../blog/#%C3%A9">Existing encoded anchor</a>
      <a href="../blog/index.html#old">Existing named anchor</a>
      <a href="../install.sh">Existing download</a>
      <a href="#top">Top of page</a>
      <a href="https://outside.test/missing">External link</a>
      <!-- <a href="/ignored">Commented out</a> -->
    `],
    ['blog/index.html', '<h1 id="é">Blog</h1><a name="old"></a>'],
    ['install.sh', null],
  ]);
  assert.deepEqual(checkLinks(files, new URL('https://example.test/project/')), [
    '/project/docs/ -> #missing-local (missing anchor)',
    '/project/docs/ -> ../blog/?lang=en&view=list#missing-remote (missing anchor)',
    '/project/docs/ -> https://example.test/project/docs/ja/ (missing target)',
    '/project/docs/ -> ja/ (missing target)',
  ]);
});
