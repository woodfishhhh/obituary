import { createContext, useCallback, useContext, useEffect, useRef, useState, type CSSProperties, type RefObject } from "react";
import { agentDocLink, agents, type Agent } from "@/agents";
import { en } from "@/copy/en";
import { zh } from "@/copy/zh";
import { cn } from "@/lib/utils";

export type Lang = "zh" | "en";
export type Theme = "light" | "dark";
export type { Agent };
export { agents };

export interface Ctx {
  lang: Lang;
  theme: Theme;
  reduce: boolean;
  selected: string | null;
  select: (id: string | null) => void;
  /** Width of the stage the wall sits in, in px. */
  width: number;
}
export const PgCtx = createContext<Ctx>(null as unknown as Ctx);
export const usePg = () => useContext(PgCtx);

export const words = {
  zh: { head: zh.agents.head, lead: zh.agents.lead, pick: "点你用的那个", copy: "复制", copied: "已复制", docs: "看文档", flipBack: "翻回去", drag: "拖一拖", filter: "输入名字筛选…", none: "没有这一家", run: "运行这一条" },
  en: { head: en.agents.head, lead: en.agents.lead, pick: "Pick the one you use.", copy: "Copy", copied: "Copied", docs: "Docs", flipBack: "Flip back", drag: "Drag me", filter: "type to filter…", none: "No match", run: "Run this" },
} as const;

export const commandOf = (id: string) => `obit init --agent ${id}`;
// In the playground the docs are the live site's.
export const docUrl = (id: string, lang: Lang) => `https://obituary.woodfish.site/${agentDocLink(id, lang)}.html`;
export const agentById = (id: string | null) => agents.find((a) => a.id === id);

export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // shortcut: execCommand is deprecated, but it is the only way to copy from a file:// page on some browsers.
    const box = document.createElement("textarea");
    box.value = text;
    box.style.cssText = "position:fixed;opacity:0";
    document.body.append(box);
    box.select();
    const ok = document.execCommand("copy");
    box.remove();
    return ok;
  }
}

/** The key last copied, for 1.4 s: show "copied" on that button. */
export function useCopied() {
  const [done, setDone] = useState<string | null>(null);
  const timer = useRef(0);
  const copy = useCallback((key: string, text: string) => {
    void copyText(text).then(() => {
      setDone(key);
      window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => setDone(null), 1400);
    });
  }, []);
  return [done, copy] as const;
}

export function useElementWidth(ref: RefObject<HTMLElement | null>) {
  const [width, setWidth] = useState(1000);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    setWidth(el.clientWidth);
    const ro = new ResizeObserver(() => setWidth(el.clientWidth));
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref]);
  return width;
}

export const px = (n: unknown) => `${n}px`;
export const num = (v: unknown) => Number(v);

/** The logo on its tile; `mode` is the "logo colour" dial: colour, grey, or grey until hovered. */
export function LogoTile({ a, size, mode, className }: { a: Agent; size: number; mode: string; className?: string }) {
  return (
    <span className={cn("pg-tile", className)} data-dark={a.dark || undefined} data-mode={mode} style={{ "--s": `${size}px` } as CSSProperties}>
      {a.logo ? <img src={a.logo} alt="" width={size} height={size} draggable={false} /> : <span aria-hidden="true">{a.name.slice(0, 1)}</span>}
    </span>
  );
}

/** The chosen agent's command, with copy and a link to its page. */
export function CommandPanel() {
  const { selected, lang } = usePg();
  const t = words[lang];
  const [copied, copy] = useCopied();
  const a = agentById(selected);
  return (
    <div className="pg-cmd" data-empty={a ? undefined : true} aria-live="polite">
      {a ? (
        <>
          <LogoTile a={a} size={40} mode="color" />
          <div className="min-w-0 flex-[1_1_15rem]">
            <div className="text-sm text-muted">{a.name}</div>
            <code className="block text-[0.95rem] break-all text-ink"><span className="text-muted">$ </span>{commandOf(a.id)}</code>
          </div>
          <div className="flex gap-2">
            <button type="button" className="pg-btn" onClick={() => copy(a.id, commandOf(a.id))}>{copied === a.id ? t.copied : t.copy}</button>
            <a className="pg-btn pg-btn-solid" href={docUrl(a.id, lang)} target="_blank" rel="noreferrer">{t.docs} ↗</a>
          </div>
        </>
      ) : (
        <p className="m-0 text-[0.95rem]">{t.pick}</p>
      )}
    </div>
  );
}

/** The section as the page has it: dot field, a heading and a lead, the wall, then the command. */
export function Section({ children, panel, aura }: { children: React.ReactNode; panel: boolean; aura: boolean }) {
  const { lang } = usePg();
  const t = words[lang];
  return (
    <section className="relative overflow-hidden border-y border-line bg-paper-2 py-16 @3xl:py-24">
      <div aria-hidden="true" className="pg-aura absolute inset-0" data-off={aura ? undefined : true} />
      <div aria-hidden="true" className="dot-field fade-edges absolute inset-0 opacity-60" />
      <div className="relative mx-auto max-w-[1240px] px-5 @3xl:px-10">
        <div className="mb-10 grid gap-5 @5xl:mb-12 @5xl:grid-cols-2 @5xl:items-end @5xl:gap-6">
          <h2 className="max-w-[16ch] text-[clamp(1.9rem,4.4cqw,3.25rem)] leading-[1.08] font-semibold tracking-[-0.02em]">{t.head}</h2>
          <p className="max-w-[46ch] text-base text-ink-2 @3xl:text-lg">{t.lead}</p>
        </div>
      </div>
      <div className="relative">{children}</div>
      {panel && (
        <div className="relative mx-auto mt-8 max-w-[1240px] px-5 @3xl:px-10">
          <CommandPanel />
        </div>
      )}
    </section>
  );
}

/** Splits into `n` rows, dealing one by one (so each row mixes popular and rare names). */
export function deal<T>(list: T[], n: number): T[][] {
  const rows: T[][] = Array.from({ length: n }, () => []);
  list.forEach((x, i) => rows[i % n].push(x));
  return rows;
}
/** Splits into `n` consecutive runs of about equal length. */
export function runs<T>(list: T[], n: number): T[][] {
  const size = Math.ceil(list.length / n);
  return Array.from({ length: n }, (_, i) => list.slice(i * size, (i + 1) * size)).filter((r) => r.length);
}
