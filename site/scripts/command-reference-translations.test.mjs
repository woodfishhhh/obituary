import assert from 'node:assert/strict';
import test from 'node:test';
import { checkCatalog, translationTemplate } from './command-reference-translations.mjs';

const help = [{ name: 'obit demo', summary: 'summary-v1', flags: [{ long: '--demo', description: 'flag-v1' }],
  examples: [{ code: 'obit demo --demo', comment: 'comment-v1' }, { code: 'obit demo' }] }];
const expected = translationTemplate(help, ['ja']);

function completeFixture() {
  const catalog = structuredClone(expected);
  for (const field of Object.values(catalog['obit demo'])) field.translations.ja = 'test-value';
  return catalog;
}

test('one missing translation prevents publication of an otherwise complete catalog', () => {
  const catalog = completeFixture();
  checkCatalog(expected, catalog, ['ja']);
  delete catalog['obit demo']['--demo'].translations.ja;
  assert.throws(() => checkCatalog(expected, catalog, ['ja']), /1 missing translations; first: obit demo \/ --demo \(ja\)/);
});

test('a change in live help invalidates the reviewed English even when checking English alone', () => {
  const changed = structuredClone(help);
  changed[0].flags[0].description = 'flag-v2';
  assert.throws(() => checkCatalog(translationTemplate(changed, ['ja']), completeFixture(), []), /English help changed: obit demo \/ --demo/);
});

test('example comments and summaries require translations but bare shell commands do not', () => {
  assert.deepEqual(Object.keys(expected['obit demo']), ['summary', '--demo', 'example: obit demo --demo']);
  for (const key of ['summary', 'example: obit demo --demo']) {
    const catalog = completeFixture();
    catalog['obit demo'][key].translations.ja = ' ';
    assert.throws(() => checkCatalog(expected, catalog, ['ja']), { message: `1 missing translations; first: obit demo / ${key} (ja)` });
  }
});
