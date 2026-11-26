// Pure state logic for Calendar column multi-selection and anti-mistake batch
// policies (Scheme 1A + 2A). Kept DOM-free so node --test exercises every edge.
import type { Burial, BuryReply, GraveResult, Item } from "../../types.ts";

export const BURY_CONFIRM_THRESHOLD = 10;
export const UNDO_TOAST_MS = 5000;

export type ColumnSelectionState = "empty" | "none" | "partial" | "all";

export interface ColumnSelectionInfo {
  state: ColumnSelectionState;
  chosen: number;
  total: number;
}

export function columnSelectionState(colIds: readonly number[], selected: ReadonlySet<number>): ColumnSelectionInfo {
  const total = colIds.length;
  if (total === 0) return { state: "empty", chosen: 0, total: 0 };
  let chosen = 0;
  for (const id of colIds) {
    if (selected.has(id)) chosen++;
  }
  const state: ColumnSelectionState = chosen === 0 ? "none" : chosen === total ? "all" : "partial";
  return { state, chosen, total };
}

// Empty column: no-op.
// Non-empty column with 0 or partial selection: select every card in the column, keeping other columns.
// Non-empty column fully selected: deselect every card in the column, keeping other columns.
export function toggleColumnSelection(selected: ReadonlySet<number>, colIds: readonly number[]): Set<number> {
  const next = new Set(selected);
  if (colIds.length === 0) return next;
  const allSelected = colIds.every((id) => next.has(id));
  if (allSelected) {
    for (const id of colIds) next.delete(id);
  } else {
    for (const id of colIds) next.add(id);
  }
  return next;
}

// Direct bury (with 5s undo toast) is allowed only when selectedCount < 10
// AND the selection does not encompass the entire non-empty "Later" column.
export function needsBuryConfirm(selectedIds: readonly number[], laterIds: readonly number[]): boolean {
  if (selectedIds.length >= BURY_CONFIRM_THRESHOLD) return true;
  if (laterIds.length === 0) return false;
  const picked = new Set(selectedIds);
  return laterIds.every((id) => picked.has(id));
}

export interface BuryPolicy {
  askConfirm: boolean;
  recycleCount: number;
  graveCount: number;
}

// In recycle mode, plain files go straight to the system Recycle Bin and cannot
// be undone inside Obituary, so any selection containing a plain file always asks.
export function evaluateBuryPolicy(
  items: readonly Pick<Item, "id" | "isDir" | "kind">[],
  selectedIds: readonly number[],
  laterIds: readonly number[],
  recycleMode: boolean,
): BuryPolicy {
  let recycleCount = 0;
  let graveCount = 0;
  if (recycleMode) {
    const picked = new Set(selectedIds);
    for (const item of items) {
      if (!picked.has(item.id)) continue;
      if (!item.isDir && !item.kind) recycleCount++;
      else graveCount++;
    }
  } else {
    graveCount = selectedIds.length;
  }
  const askConfirm = recycleCount > 0 || needsBuryConfirm(selectedIds, laterIds);
  return { askConfirm, recycleCount, graveCount };
}

// ActionBuried with a burial record is only emitted after a committed burial,
// so keep its ID even if post-commit cleanup attached a warning error.
export function extractBuriedIds(reply: BuryReply): number[] {
  if (reply.buriedIds && reply.buriedIds.length > 0) return [...reply.buriedIds];
  const ids: number[] = [];
  for (const r of reply.results ?? []) {
    if (r.action === "buried" && r.burial?.id != null) ids.push(r.burial.id);
  }
  return ids;
}

export interface BuryOutcomeSummary {
  buriedIds: number[];
  recycledCount: number;
  failedCount: number;
  hasErrors: boolean;
}

export function summarizeBuryReply(reply: BuryReply): BuryOutcomeSummary {
  const results = reply.results ?? [];
  const buriedIds = extractBuriedIds(reply);
  let recycledCount = 0;
  let failedCount = 0;
  for (const r of results) {
    if (r.error) failedCount++;
    else if (r.action === "recycled") recycledCount++;
  }
  return { buriedIds, recycledCount, failedCount, hasErrors: !!reply.error || failedCount > 0 };
}

export function countRenamedResurrections(
  results: readonly GraveResult[],
  originalPaths?: ReadonlyMap<number, string>,
): number {
  let renamed = 0;
  for (const r of results) {
    if (r.error || !r.path) continue;
    const id = r.burial?.id ?? Number(r.ref.replace(/^#/, ""));
    const orig = r.burial?.path ?? (Number.isFinite(id) ? originalPaths?.get(id) : undefined);
    if (orig && r.path.toLowerCase() !== orig.toLowerCase()) renamed++;
  }
  return renamed;
}

export function selectedGraveTotals(
  graves: readonly Pick<Burial, "id" | "size" | "kind">[],
  selectedIds: readonly number[],
): { count: number; totalBytes: number; containerCount: number; allContainers: boolean } {
  const picked = new Set(selectedIds);
  const chosen = graves.filter((g) => picked.has(g.id));
  let totalBytes = 0;
  let containerCount = 0;
  for (const g of chosen) {
    if (g.kind === "container") containerCount++;
    else totalBytes += g.size || 0;
  }
  const allContainers = chosen.length > 0 && containerCount === chosen.length;
  return { count: chosen.length, totalBytes, containerCount, allContainers };
}
