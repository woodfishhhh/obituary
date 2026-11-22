import { useEffect, type CSSProperties, type PointerEvent } from "react";
import { agents, commandOf, docUrl, LogoTile, num, useCopied, usePg, words, type Agent } from "../common";
import { logoColor, range, type Knob, type Values } from "../knobs";

export const knobs: Knob[] = [
  range("card", "卡片宽度", 110, 240, 2, 150, " px"),
  range("ratio", "卡片高宽比", 0.8, 1.5, 0.05, 1.1),
  range("gap", "间距", 6, 28, 1, 12, " px"),
  range("radius", "圆角", 10, 36, 1, 22, " px"),
  range("logoSize", "图标大小", 32, 76, 2, 52, " px"),
  range("ms", "翻转用时", 150, 1000, 10, 560, " ms"),
  range("tilt", "悬停倾斜", 0, 20, 1, 8, "°"),
  logoColor,
];

// Choosing an agent turns its card over: the back holds the command, copy, and its page. One card is open at a time.
export function Wall({ v }: { v: Values }) {
  const { select, width } = usePg();
  // Escape turns the open card back.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") select(null); };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [select]);
  const style = { gridTemplateColumns: `repeat(auto-fill, minmax(${v.card}px, 1fr))`, gap: num(v.gap), "--r": `${v.radius}px`, "--ratio": 1 / (width < 560 ? Math.min(num(v.ratio), 0.9) : num(v.ratio)), "--ms": `${v.ms}ms` } as CSSProperties;
  return (
    <div className="pg-flip-grid mx-auto max-w-[1240px] px-5 @3xl:px-10" style={style}>
      {agents.map((a) => (
        <Card key={a.id} a={a} v={v} />
      ))}
    </div>
  );
}

function Card({ a, v }: { a: Agent; v: Values }) {
  const { selected, select, lang, reduce } = usePg();
  const [copied, copy] = useCopied();
  const t = words[lang];
  const open = selected === a.id;
  const tilt = (e: PointerEvent<HTMLDivElement>) => {
    if (open || reduce || e.pointerType !== "mouse") return;
    const r = e.currentTarget.getBoundingClientRect();
    const px = (e.clientX - r.left) / r.width - 0.5, py = (e.clientY - r.top) / r.height - 0.5;
    e.currentTarget.style.setProperty("--ry", `${px * num(v.tilt) * 2}deg`);
    e.currentTarget.style.setProperty("--rx", `${-py * num(v.tilt) * 2}deg`);
  };
  const rest = (e: PointerEvent<HTMLDivElement>) => { e.currentTarget.style.removeProperty("--rx"); e.currentTarget.style.removeProperty("--ry"); };
  return (
    <div className="pg-flip" data-on={open || undefined} onPointerMove={tilt} onPointerLeave={rest}>
      <div className="pg-flip-in">
        <button type="button" className="pg-face pg-face-front pg-item" inert={open} aria-pressed={open} onClick={() => select(a.id)}>
          <LogoTile a={a} size={num(v.logoSize)} mode={String(v.logo)} />
          <span className="text-center text-[0.9375rem] leading-tight font-semibold">{a.name}</span>
        </button>
        <div className="pg-face pg-face-back" inert={!open} aria-hidden={!open}>
          <span className="text-center text-sm font-semibold">{a.name}</span>
          <code>{commandOf(a.id)}</code>
          <div className="flex flex-wrap justify-center gap-1.5">
            <button type="button" className="pg-btn" onClick={() => copy(a.id, commandOf(a.id))}>{copied === a.id ? t.copied : t.copy}</button>
            <a className="pg-btn pg-btn-solid" href={docUrl(a.id, lang)} target="_blank" rel="noreferrer">{t.docs} ↗</a>
          </div>
          <button type="button" className="text-xs text-muted underline underline-offset-2" onClick={() => select(null)}>{t.flipBack}</button>
        </div>
      </div>
    </div>
  );
}
