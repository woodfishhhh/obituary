import assert from "node:assert/strict";
import { test } from "node:test";
import { en } from "./i18n/en.ts";
import { zh } from "./i18n/zh-CN.ts";

test("calendar metadata keeps a visible separator after the source", () => {
  assert.equal(en["cal.meta"]("Manual", "24 hours"), "Manual,\u00a0lifespan 24 hours");
  assert.equal(zh["cal.meta"]("手动", "24 小时"), "手动，寿命 24 小时");
});

test("zh-CN and en message tables share the exact same key set and calendar batch copy", () => {
  assert.deepEqual(Object.keys(en).sort(), Object.keys(zh).sort());
  assert.equal(zh["cal.buriedToast"](3), "已将 3 项移入墓地");
  assert.equal(en["cal.buriedToast"](1), "Moved 1 item to the graveyard");
  assert.equal(en["cal.buriedToast"](3), "Moved 3 items to the graveyard");
  assert.equal(zh["ask.buryNow"](12), "确定将选中的 12 项立即移入墓地吗？");
  assert.equal(en["ask.buryNow"](12), "Move these 12 items to the graveyard now?");
});
