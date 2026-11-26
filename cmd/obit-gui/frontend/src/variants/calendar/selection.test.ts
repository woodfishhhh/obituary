import assert from "node:assert/strict";
import { test } from "node:test";
import { bytes } from "../../bytes.ts";
import { en } from "../../i18n/en.ts";
import { zh } from "../../i18n/zh-CN.ts";
import {
  BURY_CONFIRM_THRESHOLD,
  columnSelectionState,
  countRenamedResurrections,
  evaluateBuryPolicy,
  extractBuriedIds,
  needsBuryConfirm,
  selectedGraveTotals,
  summarizeBuryReply,
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

test("extractBuriedIds keeps committed graveyard burials even when post-commit cleanup attached an error", () => {
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

  // Fallback when an older/mock payload omits buriedIds: excludes recycled/refused,
  // but keeps action === "buried" with a burial ID (e.g. manifest cleanup failed after commit).
  assert.deepEqual(
    extractBuriedIds({
      results: [
        { ref: "#31", action: "buried", burial: { id: 31, path: "a", isDir: false, size: 10, buriedAt: "", purgeAt: "" } },
        { ref: "#32", action: "recycled" },
        { ref: "#33", action: "buried", error: "manifest cleanup locked", burial: { id: 33, path: "c", isDir: true, size: 10, buriedAt: "", purgeAt: "" } },
        { ref: "#34", action: "refused", error: "refused" },
      ],
    }),
    [31, 33],
  );
});

test("recycle mode forces bury confirmation whenever plain files are selected and summarizes outcomes honestly", () => {
  const items = [
    { id: 1, isDir: false },
    { id: 2, isDir: true },
    { id: 3, isDir: false, kind: "container" },
  ];

  // In graveyard mode, <10 without whole Later runs directly without confirm.
  assert.deepEqual(evaluateBuryPolicy(items, [1], [], false), { askConfirm: false, recycleCount: 0, graveCount: 1 });

  // In recycle mode, even 1 plain file forces confirmation; folder/container-only stays direct when <10.
  assert.deepEqual(evaluateBuryPolicy(items, [1], [], true), { askConfirm: true, recycleCount: 1, graveCount: 0 });
  assert.deepEqual(evaluateBuryPolicy(items, [1, 2, 3], [], true), { askConfirm: true, recycleCount: 1, graveCount: 2 });
  assert.deepEqual(evaluateBuryPolicy(items, [2, 3], [], true), { askConfirm: false, recycleCount: 0, graveCount: 2 });

  // Mixed bury + recycle + cleanup warning preserves buriedIds, recycledCount and hasErrors.
  const summary = summarizeBuryReply({
    results: [
      { ref: "#1", action: "recycled", path: "a.txt" },
      { ref: "#2", action: "buried", error: "manifest locked", burial: { id: 2, path: "dir", isDir: true, size: 10, buriedAt: "", purgeAt: "" } },
    ],
  });
  assert.deepEqual(summary, { buriedIds: [2], recycledCount: 1, failedCount: 1, hasErrors: true });

  // Pure recycle reply has no undoable buriedIds.
  const pureRecycle = summarizeBuryReply({
    results: [{ ref: "#1", action: "recycled", path: "a.txt" }],
  });
  assert.deepEqual(pureRecycle, { buriedIds: [], recycledCount: 1, failedCount: 0, hasErrors: false });

  // Honest zh/en recycle copy.
  assert.match(zh["ask.buryNowRecycleBody"], /回收站/);
  assert.match(en["ask.buryNowRecycleBody"], /Recycle Bin/);
  assert.match(zh["ask.buryNowMixedBody"](1, 2), /1 个普通文件.*2 项/);
  assert.match(en["ask.buryNowMixedBody"](1, 2), /1 plain file.*2 items/);
  assert.match(zh["cal.buriedAndRecycledToast"](2, 1), /2 项移入墓地.*1 项移入系统回收站/);
  assert.match(en["cal.buriedAndRecycledToast"](2, 1), /2 items to the graveyard and 1 item to the Recycle Bin/);
});

test("countRenamedResurrections detects collision renames and undo copy avoids promising in-place restore", () => {
  const originalPaths = new Map<number, string>([
    [10, "D:\\work\\a.txt"],
    [11, "D:\\work\\b.txt"],
  ]);
  const renamed = countRenamedResurrections(
    [
      { ref: "#10", action: "resurrected", path: "D:\\work\\a.txt" },
      { ref: "#11", action: "resurrected", path: "D:\\work\\b (resurrected).txt" },
    ],
    originalPaths,
  );
  assert.equal(renamed, 1);
  assert.equal(zh["cal.undoneBury"](2, 0), "已撤销，已复活 2 项");
  assert.match(zh["cal.undoneBury"](2, 1), /已改名恢复/);
  assert.equal(en["cal.undoneBury"](2, 0), "Undone; resurrected 2 items");
  assert.match(en["cal.undoneBury"](2, 1), /1 renamed to avoid overwriting/);
});

test("purge confirmation totals compute exact count, container count and formatted total size for 1 or many graves", () => {
  const graves = [
    { id: 1, size: 100 * 1024 * 1024 },
    { id: 2, size: 84 * 1024 * 1024 },
    { id: 3, size: 0, kind: "container" },
  ];

  // Even 1 item shows count and total size in danger copy
  const one = selectedGraveTotals(graves, [1]);
  assert.deepEqual(one, { count: 1, totalBytes: 100 * 1024 * 1024, containerCount: 0, allContainers: false });
  assert.equal(zh["ask.purgeDangerBody"](one.count, bytes(one.totalBytes)), "将永久火化选中的 1 项（共 100 MB），删除后无法恢复。");
  assert.equal(en["ask.purgeDangerBody"](one.count, bytes(one.totalBytes)), "This will permanently cremate 1 selected item (100 MB total). Deleted items cannot be recovered.");

  // Mixed files + Docker container calls out unknown container size explicitly
  const mixed = selectedGraveTotals(graves, [1, 2, 3]);
  assert.deepEqual(mixed, { count: 3, totalBytes: 184 * 1024 * 1024, containerCount: 1, allContainers: false });
  assert.equal(
    zh["ask.purgeDangerMixed"](mixed.count - mixed.containerCount, bytes(mixed.totalBytes), mixed.containerCount),
    "将永久火化选中的 2 项文件/目录（共 184 MB），并移除 1 个大小未知的 Docker 容器，删除后无法恢复。",
  );
  assert.equal(
    en["ask.purgeDangerMixed"](mixed.count - mixed.containerCount, bytes(mixed.totalBytes), mixed.containerCount),
    "This will permanently cremate 2 selected items (184 MB total) and remove 1 Docker container (size unknown). Deleted items cannot be recovered.",
  );

  // Container-only selection
  const onlyContainer = selectedGraveTotals(graves, [3]);
  assert.deepEqual(onlyContainer, { count: 1, totalBytes: 0, containerCount: 1, allContainers: true });
});
