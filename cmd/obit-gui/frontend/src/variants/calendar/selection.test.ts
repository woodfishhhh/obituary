import assert from "node:assert/strict";
import { test } from "node:test";
import { bytes } from "../../bytes.ts";
import { en } from "../../i18n/en.ts";
import { zh } from "../../i18n/zh-CN.ts";
import {
  BURY_CONFIRM_THRESHOLD,
  columnSelectionState,
  extractBuriedIds,
  needsBuryConfirm,
  selectedGraveTotals,
  toggleColumnSelection,
  UNDO_TOAST_MS,
} from "./selection.ts";

test("empty column reports empty state and clicking it keeps selection unchanged", () => {
  const initial = new Set<number>([10, 11]);
  assert.deepEqual(columnSelectionState([], initial), { state: "empty", chosen: 0, total: 0 });
  const next = toggleColumnSelection(initial, []);
  assert.deepEqual([...next], [10, 11]);
});

test("day column click cycles none -> all -> none, and partial -> all", () => {
  const day1 = [1, 2, 3];
  const empty = new Set<number>();
  assert.deepEqual(columnSelectionState(day1, empty), { state: "none", chosen: 0, total: 3 });

  // None -> all
  const all = toggleColumnSelection(empty, day1);
  assert.deepEqual([...all], [1, 2, 3]);
  assert.deepEqual(columnSelectionState(day1, all), { state: "all", chosen: 3, total: 3 });

  // Partial (after deselecting one card) -> clicking header promotes to all
  const partial = new Set<number>([1]);
  assert.deepEqual(columnSelectionState(day1, partial), { state: "partial", chosen: 1, total: 3 });
  const promoted = toggleColumnSelection(partial, day1);
  assert.deepEqual([...promoted].sort((a, b) => a - b), [1, 2, 3]);

  // All -> clicking header deselects the whole column
  const cleared = toggleColumnSelection(promoted, day1);
  assert.equal(cleared.size, 0);
});

test("multi-day column clicks accumulate across columns and deselect only the clicked day", () => {
  const today = [1, 2];
  const tomorrow = [3, 4, 5];
  const later = [6, 7];

  let sel = toggleColumnSelection(new Set(), today);
  sel = toggleColumnSelection(sel, tomorrow);
  sel = toggleColumnSelection(sel, later);
  assert.deepEqual([...sel].sort((a, b) => a - b), [1, 2, 3, 4, 5, 6, 7]);

  // Deselect tomorrow while keeping today and later selected
  sel = toggleColumnSelection(sel, tomorrow);
  assert.deepEqual([...sel].sort((a, b) => a - b), [1, 2, 6, 7]);
  assert.equal(columnSelectionState(today, sel).state, "all");
  assert.equal(columnSelectionState(tomorrow, sel).state, "none");
  assert.equal(columnSelectionState(later, sel).state, "all");
});

test("bury confirmation threshold: <10 without whole Later runs directly; >=10 or whole Later asks confirmation", () => {
  assert.equal(BURY_CONFIRM_THRESHOLD, 10);
  assert.equal(UNDO_TOAST_MS, 5000);

  // Small selection, no Later items -> direct bury + 5s undo toast
  assert.equal(needsBuryConfirm([1, 2, 3], []), false);
  assert.equal(needsBuryConfirm([1, 2, 3, 4, 5, 6, 7, 8, 9], [100, 101]), false);

  // Partial Later selection with <10 total -> direct bury
  assert.equal(needsBuryConfirm([1, 100], [100, 101]), false);

  // Whole non-empty Later column selected -> requires light confirmation even if <10
  assert.equal(needsBuryConfirm([100, 101], [100, 101]), true);

  // 10 or more items -> always requires light confirmation
  assert.equal(needsBuryConfirm([1, 2, 3, 4, 5, 6, 7, 8, 9, 10], []), true);
});

test("extractBuriedIds returns only successfully buried graveyard IDs for undo resurrection", () => {
  assert.deepEqual(
    extractBuriedIds({
      buriedIds: [21, 22],
      results: [
        { ref: "#21", action: "buried" },
        { ref: "#22", action: "buried" },
        { ref: "#23", error: "refused" },
      ],
    }),
    [21, 22],
  );

  // Fallback when an older/mock payload omits buriedIds: excludes failed and recycled items
  assert.deepEqual(
    extractBuriedIds({
      results: [
        { ref: "#31", action: "buried", burial: { id: 31, path: "a", isDir: false, size: 10, buriedAt: "", purgeAt: "" } },
        { ref: "#32", action: "recycled" },
        { ref: "#33", action: "buried", error: "failed", burial: { id: 33, path: "c", isDir: false, size: 10, buriedAt: "", purgeAt: "" } },
      ],
    }),
    [31],
  );
});

test("purge confirmation totals compute exact count and formatted total size for 1 or many graves", () => {
  const graves = [
    { id: 1, size: 100 * 1024 * 1024 },
    { id: 2, size: 84 * 1024 * 1024 },
    { id: 3, size: 0, kind: "container" },
  ];

  // Even 1 item shows count and total size in danger copy
  const one = selectedGraveTotals(graves, [1]);
  assert.deepEqual(one, { count: 1, totalBytes: 100 * 1024 * 1024, allContainers: false });
  assert.equal(zh["ask.purgeDangerBody"](one.count, bytes(one.totalBytes)), "将永久火化选中的 1 项（共 100 MB），删除后无法恢复。");
  assert.equal(en["ask.purgeDangerBody"](one.count, bytes(one.totalBytes)), "This will permanently cremate 1 selected item (100 MB total). Deleted items cannot be recovered.");

  // Multi-item total size
  const many = selectedGraveTotals(graves, [1, 2, 3]);
  assert.deepEqual(many, { count: 3, totalBytes: 184 * 1024 * 1024, allContainers: false });
  assert.equal(zh["ask.purgeDangerBody"](many.count, bytes(many.totalBytes)), "将永久火化选中的 3 项（共 184 MB），删除后无法恢复。");

  // Container-only selection
  const onlyContainer = selectedGraveTotals(graves, [3]);
  assert.deepEqual(onlyContainer, { count: 1, totalBytes: 0, allContainers: true });
});
