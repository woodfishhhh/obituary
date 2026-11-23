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

test("stylesheet URLs require files relative to the stylesheet", () => {
  const files = new Map([
    ["assets/demo.css", [
      "@font-face { font-family: demo; src: url(/project/fonts/missing.woff2) }",
      ".exists { background: url(../images/image%20one.svg?x=1#shape) }",
      ".missing { background: url(../images/missing.svg) }",
      ".custom { --mask: url(\"../images/missing-mask.svg\") }",
      ".directory { background: url(../docs/) }",
      ".external { background: url(//outside.test/missing.svg) }",
      ".embedded { background: url(data:image/svg+xml,%3Csvg/%3E) }",
      ".fragment { filter: url(#filter) }",
      "/* url(missing-in-comment.svg) */",
    ].join("\n")],
    ["images/image one.svg", null],
    ["docs/index.html", "<h1>Docs</h1>"],
  ]);
  assert.deepEqual(checkLinks(files, new URL("https://example.test/project/")), [
    "/project/assets/demo.css -> ../docs/ (missing target)",
    "/project/assets/demo.css -> ../images/missing-mask.svg (missing target)",
    "/project/assets/demo.css -> ../images/missing.svg (missing target)",
    "/project/assets/demo.css -> /project/fonts/missing.woff2 (missing target)",
  ]);
});
