import assert from "node:assert/strict";
import { test } from "node:test";
import { columnTemplate, MIN_COLUMN } from "./columns.ts";

// The template is a function of the column count alone, so a day with 40 long
// names and a day with none can't get different widths.
for (const cols of [8, 9]) {
  test(`${cols} columns: equal tracks, none under the minimum`, () => {
    const m = columnTemplate(cols).match(/^repeat\((\d+), minmax\((\d+)px, 1fr\)\)$/);
    assert.ok(m, "one repeat() of identical minmax() tracks, each growing at the same rate");
    assert.equal(Number(m[1]), cols);
    assert.equal(Number(m[2]), MIN_COLUMN);
    assert.ok(MIN_COLUMN >= 100, "a card stays readable");
    assert.ok(9 * MIN_COLUMN <= 1040 - 42, "the default window shows every column without scrolling");
  });
}
