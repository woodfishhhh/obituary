import assert from "node:assert/strict";
import { test } from "node:test";
import { en } from "./i18n/en.ts";
import { zh } from "./i18n/zh-CN.ts";

test("calendar metadata keeps a visible separator after the source", () => {
  assert.equal(en["cal.meta"]("Manual", "24 hours"), "Manual,\u00a0lifespan 24 hours");
  assert.equal(zh["cal.meta"]("手动", "24 小时"), "手动，寿命 24 小时");
});
