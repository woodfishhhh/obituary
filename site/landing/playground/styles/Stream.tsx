import { useRef, type CSSProperties } from "react";
import { agents, deal, LogoTile, num, usePg } from "../common";
import { logoColor, range, toggle, type Knob, type Values } from "../knobs";

export const knobs: Knob[] = [
  range("rows", "行数", 1, 4, 1, 3),
  range("speed", "速度", 15, 140, 1, 48, " px/s"),
  range("size", "图标大小", 36, 76, 2, 50, " px"),
  range("gap", "间距", 6, 32, 1, 14, " px"),
  range("tilt", "倾斜", 0, 8, 0.5, 3, "°"),
  range("fade", "两端淡出", 0, 30, 1, 14, "%"),
  range("hover", "悬停放大", 0, 1, 0.05, 0.6),
  toggle("names", "显示名字", true),
  toggle("pause", "悬停时暂停", true),
  logoColor,
];

// Each row is three copies of its logos, sliding by one copy: a seamless loop that is longer than any screen.
export function Wall({ v }: { v: Values }) {
  const rows = deal(agents, num(v.rows));
  const style = { "--tilt": `${v.tilt}deg`, "--fade": `${v.fade}%`, "--gap": `${v.gap}px`, "--hover": v.hover } as CSSProperties;
  return (
    <div className="pg-stream" style={style} data-pause={v.pause || undefined}>
      {rows.map((row, i) => (
        <Row key={i} row={row} v={v} reverse={i % 2 === 1} />
      ))}
    </div>
  );
}

function Row({ row, v, reverse }: { row: typeof agents; v: Values; reverse: boolean }) {
  const { selected, select } = usePg();
  const box = useRef<HTMLDivElement>(null);
  const size = num(v.size);
  const one = row.length * (size + num(v.gap) + (v.names ? 130 : 0));
  return (
    <div
      ref={box}
      className="pg-stream-row"
      data-reverse={reverse || undefined}
      style={{ "--dur": `${(one / num(v.speed)).toFixed(1)}s` } as CSSProperties}
      // With keyboard focus inside, the row stops looping and becomes scrollable, so the focused logo is in view.
      onFocusCapture={() => { if (box.current) box.current.dataset.focus = "true"; }}
      onBlurCapture={(e) => { if (box.current && !box.current.contains(e.relatedTarget as Node | null)) delete box.current.dataset.focus; }}
    >
      <ul className="pg-stream-track">
        {[0, 1, 2].flatMap((copy) =>
          row.map((a) => (
            <li key={`${copy}-${a.id}`} aria-hidden={copy > 0 || undefined}>
              <button
                type="button"
                className="pg-chip pg-item"
                data-on={selected === a.id || undefined}
                data-nameless={v.names ? undefined : true}
                tabIndex={copy > 0 ? -1 : undefined}
                aria-pressed={copy > 0 ? undefined : selected === a.id}
                aria-label={v.names ? undefined : a.name}
                onClick={() => select(a.id)}
              >
                <LogoTile a={a} size={size} mode={String(v.logo)} />
                {v.names && <span className="pg-chip-name">{a.name}</span>}
              </button>
            </li>
          )),
        )}
      </ul>
    </div>
  );
}
