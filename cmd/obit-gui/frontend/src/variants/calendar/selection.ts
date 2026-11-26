// Pure state logic for Calendar column multi-selection and anti-mistake batch
// policies (Scheme 1A + 2A). Kept DOM-free so node --test exercises every edge.
import type { Burial, BuryReply } from "../../types.ts";

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

export function extractBuriedIds(reply: BuryReply): number[] {
  if (reply.buriedIds && reply.buriedIds.length > 0) return [...reply.buriedIds];
  const ids: number[] = [];
  for (const r of reply.results ?? []) {
    if (!r.error && r.action === "buried" && r.burial?.id != null) ids.push(r.burial.id);
  }
  return ids;
}

export function selectedGraveTotals(
  graves: readonly Pick<Burial, "id" | "size" | "kind">[],
  selectedIds: readonly number[],
): { count: number; totalBytes: number; allContainers: boolean } {
  const picked = new Set(selectedIds);
  const chosen = graves.filter((g) => picked.has(g.id));
  const totalBytes = chosen.reduce((sum, g) => sum + (g.kind === "container" ? 0 : g.size || 0), 0);
  const allContainers = chosen.length > 0 && chosen.every((g) => g.kind === "container");
  return { count: chosen.length, totalBytes, allContainers };
}
