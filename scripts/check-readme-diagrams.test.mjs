// Proves scripts/check-readme-diagrams.mjs still catches the defects it exists for:
// copies the generated diagrams, breaks nine of them on purpose, and requires the
// checker to report the matching rule for each broken file and nothing for the rest.
// Run from docs/assets/readme/i18n: pnpm test
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, "..");
const source = path.join(repo, "docs/assets/readme/i18n");
const LANGS = ["en", "zh-CN", "zh-TW", "ja", "ko", "es", "fr", "de", "pt-BR", "ru"];

const CARD = /(<rect data-role="card" x="32" y="[0-9.]+" width=")896/;
const FIRST_CARD_HEIGHT = /(<rect data-role="card" x="32" y="[0-9.]+" width="896" height=")[0-9.]+/;
const withHead = (text) => (m, head) => head + text;

// [file, what is broken, rule the checker must report, how to break it]
const MUTANTS = [
  ["es/lifecycle-light.svg", "caption pill dragged onto the arrow", "shape-overlap", (t) => t.replace(/(<rect data-role="pill" x=")[0-9.]+/, withHead("60"))],
  ["es/lifecycle-dark.svg", "card edge cuts through its command chip", "chip-outside-card", (t) => t.replace(CARD, withHead("850"))],
  ["ko/lifecycle-dark.svg", "card narrowed until its command chip lies outside it", "chip-outside-card", (t) => t.replace(CARD, withHead("520"))],
  ["pt-BR/delete-guard-light.svg", "card too short for its text", "card-padding", (t) => t.replace(FIRST_CARD_HEIGHT, withHead("66"))],
  // x=130 lies inside the title and note of the same row, whatever the fonts are
  ["de/delete-guard-dark.svg", "command chip slid over the title and note", "text-crosses-chip", (t) => t.replace(/(<rect data-role="chip" x=")[0-9.]+/, withHead("130"))],
  ["en/lifecycle-light.svg", "note colour washed out", "contrast", (t) => { let n = 0; return t.replace(/fill="#53637D"/g, (m) => (n++ < 3 ? 'fill="#B6C1D2"' : m)); }],
  ["ru/lifecycle-dark.svg", "a word dropped from a note", "copy-differs", (t) => t.replace("что осталось", "что")],
  ["fr/lifecycle-light.svg", "title pushed down onto its command chip", "text-overlap", (t) => t.replace(/(<text x="[0-9.]+" y=")([0-9.]+)(" font-family="[^"]+" font-size="24" fill="[^"]+" font-weight="700" data-t="title" data-k="purge")/, (m, a, y, b) => a + (parseFloat(y) + 26) + b)],
  ["ja/delete-guard-light.svg", "text squeezed with textLength", "text-compression", (t) => t.replace('data-t="title"', 'textLength="60" data-t="title"')],
];

test("the checker reports every deliberately broken diagram and passes the untouched ones", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "obit-diagram-mutants-"));
  try {
    for (const lang of LANGS) fs.cpSync(path.join(source, lang), path.join(root, lang), { recursive: true });
    for (const [file, what, , mutate] of MUTANTS) {
      const target = path.join(root, file);
      const original = fs.readFileSync(target, "utf8");
      const broken = mutate(original);
      assert.notEqual(broken, original, `mutation did not change ${file}: ${what}`);
      fs.writeFileSync(target, broken);
    }
    const run = spawnSync(process.execPath, [path.join(here, "check-readme-diagrams.mjs"), "--root", root, "--json"], { encoding: "utf8", maxBuffer: 64 << 20 });
    const results = JSON.parse(run.stdout);
    const byFile = new Map(results.map((r) => [r.file, new Set(r.issues.map((i) => i.rule))]));
    for (const [file, what, rule] of MUTANTS) {
      const rules = byFile.get(file.replace(/\.svg$/, ""));
      assert.ok(rules?.has(rule), `${file} (${what}): expected rule ${rule}, got ${[...(rules ?? [])].join(", ") || "nothing"}`);
    }
    const touched = new Set(MUTANTS.map(([file]) => file.replace(/\.svg$/, "")));
    for (const [file, rules] of byFile) {
      if (!touched.has(file)) assert.equal(rules.size, 0, `${file} should pass but reports ${[...rules].join(", ")}`);
    }
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
