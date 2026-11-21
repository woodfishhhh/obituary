import assert from "node:assert/strict";
import { test } from "node:test";
import postcss from "postcss";
import squircleCorners from "./squircle-corners.mjs";

const run = (css, options) => postcss([squircleCorners(options)]).process(css, { from: undefined }).css.replace(/\s+/g, " ").trim();
const K = "var(--squircle-k, 1)";
const SHAPE = "corner-shape:var(--squircle-shape, round)";

test("a literal radius is scaled and asks for the squircle", () => {
  assert.equal(run(".a{border-radius:8px}"), `.a{border-radius:calc(8px * ${K});${SHAPE}}`);
  assert.equal(run(".a{border-top-left-radius:1rem}"), `.a{border-top-left-radius:calc(1rem * ${K});${SHAPE}}`);
});

test("a radius token, with or without arithmetic, is scaled whole", () => {
  assert.equal(run(".a{border-radius:var(--radius)}"), `.a{border-radius:calc(var(--radius) * ${K});${SHAPE}}`);
  assert.equal(run(".a{border-radius:calc(var(--radius) + 2px)}"), `.a{border-radius:calc((var(--radius) + 2px) * ${K});${SHAPE}}`);
});

test("only the rounded corners of a shorthand are scaled, and an elliptical slash survives", () => {
  assert.equal(run(".a{border-radius:8px 8px 0 0}"), `.a{border-radius:calc(8px * ${K}) calc(8px * ${K}) 0 0;${SHAPE}}`);
  assert.equal(run(".a{border-radius:10px/20px}"), `.a{border-radius:calc(10px * ${K}) / calc(20px * ${K});${SHAPE}}`);
});

test("circles and pills keep their value and a circular corner", () => {
  for (const value of ["50%", "999px", "100px", "calc(infinity * 1px)"]) {
    assert.equal(run(`.a{border-radius:${value}}`), `.a{border-radius:${value};corner-shape:round}`, value);
  }
});

test("zero and tiny radii are left as they are", () => {
  for (const value of ["0", "0px 0", "3px", "4.9px", "0 3px 3px 0"]) {
    assert.equal(run(`.a{border-radius:${value}}`), `.a{border-radius:${value}}`, value);
  }
});

test("inherit inherits the shape too", () => {
  assert.equal(run(".a::before{border-radius:inherit}"), ".a::before{border-radius:inherit;corner-shape:inherit}");
});

test("a radius composed by hand is not scaled again", () => {
  const value = `calc(var(--radius-sm) * ${K} + 3px)`;
  assert.equal(run(`.a{border-radius:${value}}`), `.a{border-radius:${value};${SHAPE}}`);
});

test("a rule that sets its own corner-shape, and keyframes, are left alone", () => {
  assert.equal(run(".a{border-radius:8px;corner-shape:round}"), ".a{border-radius:8px;corner-shape:round}");
  assert.equal(run("@keyframes k{to{border-radius:8px}}"), "@keyframes k{to{border-radius:8px}}");
});

test("important carries over, and running twice changes nothing more", () => {
  const once = run(".a{border-radius:8px!important}");
  assert.equal(once, `.a{border-radius:calc(8px * ${K})!important;${SHAPE}!important}`);
  assert.equal(run(once), once);
});

test("roundSelectors keep a pill written with a small radius circular", () => {
  assert.equal(run(".VPButton.medium[data-v-1]{border-radius:20px}", { roundSelectors: /\.VPButton/ }), ".VPButton.medium[data-v-1]{border-radius:20px;corner-shape:round}");
  assert.equal(run(".card{border-radius:20px}", { roundSelectors: /\.VPButton/ }), `.card{border-radius:calc(20px * ${K});${SHAPE}}`);
});

test("rules inside media queries are rewritten like any other", () => {
  assert.equal(run("@media (min-width:1px){.a{border-radius:12px}}"), `@media (min-width:1px){.a{border-radius:calc(12px * ${K});${SHAPE}}}`);
});
