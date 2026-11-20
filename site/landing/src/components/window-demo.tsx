import "@gui/styles.css";
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { useReducedMotion } from "motion/react";
import App, { type PageId } from "@gui/App";
import { ApiContext } from "@gui/api";
import { AppearanceContext } from "@gui/appearance";
import { looks } from "@gui/catalog";
import { setLang, type Lang as GuiLang } from "@gui/i18n";
import calendar from "@gui/variants/calendar/calendar";
import palette from "@gui/variants/palette/palette";

import type { Copy } from "@/copy";
import { createDemoApi } from "@/demo/api";
import { lifetime } from "@/demo/data";
import { cn } from "@/lib/utils";

export type WindowLook = "calendar" | "palette";
export type { PageId };
const variants = { calendar, palette };
// The window is drawn at the size the desktop app opens at, then scaled to the box it sits in.
const W = 1040;
const GB = 1024 ** 3;

type Props = {
  c: Copy;
  look: WindowLook;
  page: PageId;
  label?: string;
  // Lets visitors use it (the demo backend keeps state, so extending and resurrecting work);
  // otherwise it is a picture that assistive technology reads as one labelled image.
  interactive?: boolean;
  // The height the window is drawn at, and the narrowest it is scaled to before the box scrolls sideways.
  height?: number;
  minScale?: number;
  className?: string;
};

// The desktop app's own front end on a demo backend: the same components and styles as the window.
export function WindowDemo({ c, look, page, label, interactive = false, height = 700, minScale = 0.66, className }: Props) {
  const box = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(0);
  useLayoutEffect(() => {
    const el = box.current;
    if (!el) return;
    const fit = () => setWidth(el.clientWidth);
    fit();
    const ro = new ResizeObserver(fit);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  const scale = Math.max(minScale, width / W);

  // The window has Chinese and English; the page's other languages show it in English.
  const guiLang: GuiLang = c.lang === "zh" ? "zh" : "en";
  const demo = useMemo(() => createDemoApi(c.lang), [c.lang]);
  const appearance = useMemo(() => ({ current: look, looks, choose: () => {} }), [look]);
  const [ready, setReady] = useState(false);
  useEffect(() => {
    const html = document.documentElement.lang;
    setLang(guiLang);
    document.documentElement.lang = html; // the page keeps its own language
    setReady(true);
  }, [guiLang]);

  const picture = !interactive;
  const canvas = useRef<HTMLDivElement>(null);
  const card = useHoverCard(box, canvas, demo, c);
  useLifetimeCount(canvas, c.lang, ready);
  useDialogInView(box, ready);

  return (
    <div
      ref={box}
      role={picture ? "img" : "group"}
      aria-label={label ?? c.demo.label}
      style={{ height: width ? height * scale : undefined, aspectRatio: width ? undefined : `${W} / ${height}` }}
      className={cn("relative bg-paper", width && width < W * minScale + 1 ? "overflow-x-auto overflow-y-hidden overscroll-x-contain" : "overflow-hidden", className)}
      {...card.handlers}
    >
      <div style={{ width: W * scale, height: height * scale }} className="relative">
        {ready && width > 0 && (
          <div
            ref={canvas}
            aria-hidden={picture || undefined}
            inert={picture}
            className="absolute top-0 left-0 origin-top-left [&_.life-label]:!text-[15px] [&_.life-num]:!text-[46px]"
            style={{ width: W, height, transform: `scale(${scale})` } as CSSProperties}
          >
            <ApiContext.Provider value={demo.api}>
              <AppearanceContext.Provider value={appearance}>
                <App key={`${guiLang}-${look}-${page}`} variant={variants[look]} page={page} />
              </AppearanceContext.Provider>
            </ApiContext.Provider>
          </div>
        )}
      </div>
      {card.view}
    </div>
  );
}

// What a file is, when it goes and where it came from, next to the pointer or the keyboard focus.
function useHoverCard(box: React.RefObject<HTMLDivElement | null>, canvas: React.RefObject<HTMLDivElement | null>, demo: ReturnType<typeof createDemoApi>, c: Copy) {
  const [shown, setShown] = useState<{ path: string; x: number; y: number; flip: boolean } | null>(null);
  const [saved, setSaved] = useState<{ el: HTMLElement; title: string } | null>(null);
  const restore = () => { if (saved) saved.el.title = saved.title; setSaved(null); };

  const show = (el: HTMLElement) => {
    const outer = box.current;
    if (!outer) return;
    const b = outer.getBoundingClientRect();
    const r = el.getBoundingClientRect();
    const path = el.title || saved?.title || "";
    if (!path) return;
    if (el.title) { setSaved({ el, title: el.title }); el.removeAttribute("title"); } // no second, native tooltip
    setShown({ path, x: Math.min(Math.max(r.left - b.left + outer.scrollLeft, 8), Math.max(8, outer.scrollWidth - 268)), y: r.bottom - b.top + 8, flip: r.bottom - b.top > b.height - 170 });
    if (r.top - b.top < 0) setShown(null);
  };
  const hide = () => { restore(); setShown(null); };
  const chip = (t: EventTarget | null) => (t instanceof Element ? (t.closest(".cal-chip") as HTMLElement | null) : null);

  const handlers = {
    onPointerOver: (e: React.PointerEvent) => { const el = chip(e.target); if (el && canvas.current?.contains(el)) show(el); },
    onPointerOut: (e: React.PointerEvent) => { if (chip(e.target) && !chip(e.relatedTarget)) hide(); },
    onFocus: (e: React.FocusEvent) => { const el = chip(e.target); if (el && canvas.current?.contains(el)) show(el); },
    onBlur: (e: React.FocusEvent) => { if (chip(e.target) && !chip(e.relatedTarget)) hide(); },
    onScroll: hide,
  };

  const view = shown ? <FileCard c={c} path={shown.path} demo={demo} x={shown.x} y={shown.y} flip={shown.flip} /> : null;
  return { handlers, view };
}

function FileCard({ c, path, demo, x, y, flip }: { c: Copy; path: string; demo: ReturnType<typeof createDemoApi>; x: number; y: number; flip: boolean }) {
  const lang = c.lang;
  const item = demo.items().find((i) => i.path === path);
  const grave = demo.graves().find((g) => g.path === path);
  if (!item && !grave) return null;
  const end = new Date(item ? item.expiresAt : grave!.purgeAt);
  const start = new Date(item ? item.createdAt : grave!.buriedAt);
  const hours = (end.getTime() - Date.now()) / 3600e3;
  const unit = (n: number, u: "hour" | "day") => new Intl.NumberFormat(lang, { style: "unit", unit: u, unitDisplay: "long", maximumFractionDigits: 0 }).format(n);
  const left = Math.abs(hours) >= 48 ? unit(Math.round(hours / 24), "day") : unit(Math.max(0, Math.round(hours)), "hour");
  const when = new Intl.DateTimeFormat(lang, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(end);
  const life = item ? unit(Math.round((end.getTime() - start.getTime()) / 3600e3), "hour") : null;
  const name = path.split(/[\\/]/).pop() ?? path;
  const k = c.demo.card;
  const rows: [string, string][] = item
    ? [[k.left, left], [k.expires, when], [k.life, life!], [k.path, path]]
    : [[k.purges, `${when} · ${left}`], [k.size, `${(grave!.size / 1048576).toFixed(grave!.size > 1e8 ? 0 : 1)} MB`], [k.path, path]];
  return (
    <div
      aria-hidden="true"
      style={{ left: x, top: y, transform: flip ? "translateY(calc(-100% - 2.6rem))" : undefined }}
      className="pointer-events-none absolute z-20 w-64 rounded-xl border border-line bg-paper/95 p-3 text-[13px] leading-snug shadow-[0_18px_44px_-12px_rgb(17_21_34/0.3)] ring-1 ring-ink/5 backdrop-blur [animation:card-in_160ms_ease-out]"
    >
      <p className="truncate font-semibold text-ink">{name}</p>
      <dl className="mt-2 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1">
        {rows.map(([a, b]) => (
          <div key={a} className="contents">
            <dt className="text-muted">{a}</dt>
            <dd className="min-w-0 break-all text-ink-2 tabular-nums">{b}</dd>
          </div>
        ))}
      </dl>
      {item ? <p className="mt-2 border-t border-line pt-2 text-muted">{k.hint}</p> : null}
    </div>
  );
}

// Settings opens on what Obituary has cleared; the number counts up to its value once.
function useLifetimeCount(canvas: React.RefObject<HTMLDivElement | null>, lang: string, ready: boolean) {
  const reduce = useReducedMotion();
  useEffect(() => {
    const root = canvas.current;
    if (!root || !ready) return;
    const total = lifetime.buriedBytes / GB;
    const fmt = new Intl.NumberFormat(lang, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    let raf = 0;
    const run = (el: HTMLElement) => {
      if (el.dataset.counted) return;
      el.dataset.counted = "1";
      const t0 = performance.now();
      const step = () => {
        const p = reduce ? 1 : Math.min(1, Math.max(0, (performance.now() - t0) / 1600));
        el.textContent = `${fmt.format(total * (1 - (1 - p) ** 4))} GB`;
        if (p < 1) raf = requestAnimationFrame(step);
      };
      raf = requestAnimationFrame(step);
    };
    const scan = () => root.querySelectorAll<HTMLElement>(".life-num").forEach(run);
    scan();
    const mo = new MutationObserver(scan);
    mo.observe(root, { childList: true, subtree: true });
    return () => { mo.disconnect(); cancelAnimationFrame(raf); };
  }, [canvas, lang, ready, reduce]);
}

// On a narrow screen the window scrolls sideways; a dialog it opens is brought to the middle of the view.
function useDialogInView(box: React.RefObject<HTMLDivElement | null>, ready: boolean) {
  useEffect(() => {
    const outer = box.current;
    if (!outer || !ready) return;
    const mo = new MutationObserver(() => {
      const dialog = outer.querySelector<HTMLElement>("[role=dialog]");
      if (!dialog || outer.scrollWidth <= outer.clientWidth) return;
      const d = dialog.getBoundingClientRect();
      const b = outer.getBoundingClientRect();
      outer.scrollTo({ left: outer.scrollLeft + d.left + d.width / 2 - (b.left + b.width / 2), behavior: "smooth" });
    });
    mo.observe(outer, { childList: true, subtree: true });
    return () => mo.disconnect();
  }, [box, ready]);
}
