// Shared by the variants that bring their own pages: the data hooks with the
// confirmed actions, selection, time wording and the logo's four-point star.
// Each variant draws these however it likes; the behaviour stays identical.
import { useCallback, useMemo, useState, type ReactNode } from "react";
import { useApi } from "../api";
import { extendedTo, splitPath, when } from "../format";
import { useLoad, useRefresh } from "../hooks";
import { t } from "../i18n";
import type { Burial, Item } from "../types";
import { useBatch, useConfirm, useToast } from "../ui";

// ── Time ────────────────────────────────────────────────────────────────

export const HOUR = 3600e3;
export const DAY = 86400e3;
const pad = (n: number) => String(n).padStart(2, "0");

// The leading unit of a span, for setting the number large and the unit small.
export function parts(ms: number): { n: string; unit: string } {
  const abs = Math.abs(ms);
  const n = abs < HOUR ? Math.max(1, Math.round(abs / 60e3)) : abs < DAY ? Math.round(abs / HOUR) : Math.round(abs / DAY);
  const unit = abs < HOUR ? t("unit.minute", n) : abs < DAY ? t("unit.hour", n) : t("unit.day", n);
  return { n: String(n), unit };
}
export const span = (ms: number) => { const p = parts(ms); return t("span", p.n, p.unit); };

export const md = (iso: string) => { const d = new Date(iso); return t("date.md", d.getMonth() + 1, d.getDate()); };
export const hm = (iso: string) => { const d = new Date(iso); return `${pad(d.getHours())}:${pad(d.getMinutes())}`; };
export const weekday = (d: Date) => t("date.weekday", d.getDay());

// Fraction of a life still left, 1 at the start and 0 at the end.
export function remaining(start: string, end: string) {
  const from = +new Date(start);
  const to = +new Date(end);
  const ms = to - Date.now();
  return { ms, left: Math.min(1, Math.max(0, ms / Math.max(1, to - from))) };
}

export const nameOf = (path: string) => splitPath(path).name;
export const folderOf = (path: string) => splitPath(path).folder;

const spanKeys = ["2h", "1d", "3d", "1w", "4w"] as const;
// [lifespan, label] pairs, labelled in the current language.
export const extendSpans = (): [string, string][] => spanKeys.map((k) => [k, t(`span.${k}`)]);
export const spanLabel = (v: string) => (spanKeys as readonly string[]).includes(v) ? t(`span.${v as (typeof spanKeys)[number]}`) : v;
const sources = ["cli", "rule", "mktemp", "proposal", "watch"] as const;
export const sourceLabel = (source?: string) => (sources as readonly string[]).includes(source ?? "cli") ? t(`source.${(source ?? "cli") as (typeof sources)[number]}`) : source!;

// ── Selection ───────────────────────────────────────────────────────────

export function useSelection(ids: number[]) {
  const [picked, setPicked] = useState<Set<number>>(new Set());
  // Only ids still on screen count; a refresh drops the rest silently.
  const chosen = useMemo(() => ids.filter((id) => picked.has(id)), [ids, picked]);
  const has = useCallback((id: number) => picked.has(id), [picked]);
  const toggle = useCallback((id: number) => setPicked((s) => { const n = new Set(s); if (n.has(id)) n.delete(id); else n.add(id); return n; }), []);
  const set = useCallback((next: number[]) => setPicked(new Set(next)), []);
  const all = ids.length > 0 && chosen.length === ids.length;
  return { chosen, has, toggle, set, clear: () => setPicked(new Set()), all, toggleAll: () => setPicked(all ? new Set() : new Set(ids)) };
}

const list = (paths: string[]) => <ul>{paths.map((p) => <li key={p}>{p}</li>)}</ul>;

// Each item with its current and new expiry, so nobody has to add dates up.
export const extendList = (items: Item[], lifespan: string) => (
  <ul>{items.map((it) => { const to = extendedTo(it.expiresAt, lifespan); return <li key={it.id}>{it.path}{to ? <b className="moves">{when(it.expiresAt)} → {when(to)}</b> : null}</li>; })}</ul>
);

// ── Graves ──────────────────────────────────────────────────────────────

export function useGraves(filter: string) {
  const api = useApi();
  const { version, bump } = useRefresh();
  const confirm = useConfirm();
  const report = useBatch();
  const toast = useToast();
  const q = useLoad(() => api.Graveyard({ filter, limit: 200, after: "" }), [api, filter, version]);
  const rows = q.data?.rows ?? [];
  const graves = useMemo(() => rows.filter((r) => r.status === "buried" && r.burial).map((r) => r.burial!), [rows]);
  const pending = rows.filter((r) => r.status !== "buried");
  const sel = useSelection(useMemo(() => graves.map((g) => g.id), [graves]));
  const bytes = graves.reduce((n, g) => n + g.size, 0);

  async function run(method: "Resurrect" | "Purge", ids: number[], before?: () => Promise<void>) {
    if (!ids.length) return false;
    const purging = method === "Purge";
    const paths = graves.filter((g) => ids.includes(g.id)).map((g) => g.path);
    const ok = await confirm({
      title: purging ? t("ask.purge", ids.length) : t("ask.resurrect", ids.length),
      body: <><p>{purging ? t("ask.purgeBody") : t("ask.resurrectBody")}</p>{list(paths)}</>,
      ok: purging ? t("purge") : t("resurrect"),
      danger: purging,
    });
    if (!ok) return false;
    const reply = await api[method](ids.map((id) => `#${id}`));
    await before?.();
    // One resurrected item: say where it is now, so it can be found.
    const one = !purging && !reply.error && reply.results?.length === 1 && !reply.results[0].error ? reply.results[0].path : undefined;
    if (one) toast(t("grave.backAt", one));
    else report(reply, purging ? t("purge") : t("resurrect"));
    sel.clear();
    bump();
    return true;
  }

  return {
    ...q, rows, graves, pending, sel, bytes,
    resurrect: (ids: number[] = sel.chosen, before?: () => Promise<void>) => run("Resurrect", ids, before),
    purge: (ids: number[] = sel.chosen, before?: () => Promise<void>) => run("Purge", ids, before),
  };
}

// ── Tracked items ───────────────────────────────────────────────────────

export function useItems(range: "" | "24h" | "7d" = "") {
  const api = useApi();
  const { version, bump } = useRefresh();
  const confirm = useConfirm();
  const report = useBatch();
  const q = useLoad(() => api.Upcoming(range), [api, range, version]);
  const items = useMemo(() => q.data?.items ?? [], [q.data]);
  const sel = useSelection(useMemo(() => items.map((it) => it.id), [items]));

  async function extend(lifespan: string, ids: number[] = sel.chosen, label = spanLabel(lifespan), body?: ReactNode) {
    if (!ids.length) return false;
    const ok = await confirm({
      title: t("ask.extend", ids.length, label),
      body: <>{body ?? <p>{t("ask.extendBody")}</p>}{extendList(items.filter((it) => ids.includes(it.id)), lifespan)}</>,
      ok: t("extend"),
    });
    if (!ok) return false;
    report(await api.Extend(ids.map((id) => `#${id}`), lifespan), t("extend"));
    sel.clear();
    bump();
    return true;
  }

  async function cancel(ids: number[] = sel.chosen) {
    if (!ids.length) return false;
    const paths = items.filter((it) => ids.includes(it.id)).map((it) => it.path);
    const ok = await confirm({
      title: t("ask.cancel", ids.length),
      body: <><p>{t("ask.cancelBody")}</p>{list(paths)}</>,
      ok: t("stopTracking"),
      danger: true,
    });
    if (!ok) return false;
    report(await api.Cancel(ids.map((id) => `#${id}`)), t("stopTracking"));
    sel.clear();
    bump();
    return true;
  }

  return { ...q, items, sel, extend, cancel };
}

export type { Burial, Item };

// ── Marks ───────────────────────────────────────────────────────────────

// The logo's four-point star: taller than wide, concave sides.
export const STAR = "M12 0.5C12.7 7.8 14.2 10.6 20 12C14.2 13.4 12.7 16.2 12 23.5C11.3 16.2 9.8 13.4 4 12C9.8 10.6 11.3 7.8 12 0.5Z";
export function Star({ size = 16, className }: { size?: number; className?: string }) {
  return <svg className={className} width={size} height={size} viewBox="0 0 24 24" aria-hidden><path d={STAR} fill="currentColor" /></svg>;
}

// Reduced motion skips the pauses that only exist to let an animation play.
export const calm = () => typeof window !== "undefined" && window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
export const pause = (ms: number) => new Promise<void>((r) => setTimeout(r, calm() ? 0 : ms));
