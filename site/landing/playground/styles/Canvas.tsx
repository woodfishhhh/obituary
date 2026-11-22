import { useEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent, type PointerEvent } from "react";
import { agents, LogoTile, num, usePg, words } from "../common";
import { logoColor, range, toggle, type Knob, type Values } from "../knobs";

export const knobs: Knob[] = [
  range("height", "画布高度", 260, 560, 10, 400, " px"),
  range("cell", "格子大小", 84, 180, 2, 120, " px"),
  range("size", "图标大小", 36, 80, 2, 56, " px"),
  range("jitter", "散乱程度", 0, 1, 0.05, 0.4),
  range("drift", "自己漂移", 0, 60, 1, 14, " px/s"),
  range("inertia", "惯性", 0.8, 0.98, 0.01, 0.93),
  toggle("hex", "错位排布", true),
  toggle("names", "显示名字", true),
  logoColor,
];

const COLS = 9;
const ROWS = 3; // 27 agents: one tile of the canvas, repeated in every direction
const hash = (i: number, salt: number) => {
  const x = Math.sin(i * 127.1 + salt * 311.7) * 43758.5453;
  return x - Math.floor(x) - 0.5;
};

// A canvas with no edge: drag it, flick it, or let it drift. One tile of 27 logos repeats, and the layer
// wraps by whole tiles, so the only thing that moves every frame is one transform.
export function Wall({ v }: { v: Values }) {
  const { width, reduce, selected, select, lang } = usePg();
  const cell = num(v.cell), height = num(v.height);
  const tileW = COLS * cell, tileH = ROWS * cell;
  const stageW = Math.min(width, 1240);
  const nx = Math.ceil(stageW / tileW) + 1, ny = Math.ceil(height / tileH) + 1;
  const stage = useRef<HTMLDivElement>(null);
  const layer = useRef<HTMLDivElement>(null);
  const live = useRef({ x: 0, y: 0, vx: 0, vy: 0, drag: false, moved: 0, hold: false, target: null as null | { x: number; y: number }, last: { x: 0, y: 0, t: 0 } });
  const knob = useRef({ tileW, tileH, drift: num(v.drift), inertia: num(v.inertia), reduce });
  knob.current = { tileW, tileH, drift: num(v.drift), inertia: num(v.inertia), reduce };
  const [touched, setTouched] = useState(false);

  const base = useMemo(
    () =>
      agents.map((_, i) => {
        const col = i % COLS, row = Math.floor(i / COLS);
        const shift = v.hex && row % 2 === 1 ? 0.5 : 0;
        return { x: (col + 0.5 + shift) * cell + hash(i, 1) * cell * num(v.jitter), y: (row + 0.5) * cell + hash(i, 2) * cell * num(v.jitter) * 0.8 };
      }),
    [cell, v.hex, v.jitter],
  );

  useEffect(() => {
    const el = stage.current;
    if (!el) return;
    let raf = 0, prev = performance.now(), visible = true;
    const wrap = (n: number, m: number) => ((n % m) + m) % m - m;
    const tick = (now: number) => {
      raf = 0;
      const s = live.current, k = knob.current;
      const dt = Math.min(0.05, (now - prev) / 1000);
      prev = now;
      if (s.target) {
        s.x += (s.target.x - s.x) * 0.16;
        s.y += (s.target.y - s.y) * 0.16;
        if (Math.hypot(s.target.x - s.x, s.target.y - s.y) < 0.5) s.target = null;
      } else if (!s.drag) {
        s.x += s.vx * dt;
        s.y += s.vy * dt;
        const f = Math.pow(k.inertia, dt * 60);
        s.vx *= f;
        s.vy *= f;
        if (!s.hold && !k.reduce && Math.hypot(s.vx, s.vy) < 20) s.x -= k.drift * dt;
      }
      if (layer.current) layer.current.style.transform = `translate3d(${wrap(s.x, k.tileW)}px, ${wrap(s.y, k.tileH)}px, 0)`;
      if (visible) raf = requestAnimationFrame(tick);
    };
    const start = () => { if (!raf) { prev = performance.now(); raf = requestAnimationFrame(tick); } };
    const io = new IntersectionObserver(([e]) => { visible = e.isIntersecting; if (visible) start(); });
    io.observe(el);
    start();
    return () => { io.disconnect(); cancelAnimationFrame(raf); };
  }, []);

  const down = (e: PointerEvent) => {
    if (e.button !== 0) return;
    const s = live.current;
    s.drag = true; s.moved = 0; s.target = null; s.vx = s.vy = 0;
    s.last = { x: e.clientX, y: e.clientY, t: performance.now() };
  };
  const move = (e: PointerEvent) => {
    const s = live.current;
    if (!s.drag) return;
    const dx = e.clientX - s.last.x, dy = e.clientY - s.last.y, now = performance.now(), dt = Math.max(1, now - s.last.t) / 1000;
    s.moved += Math.abs(dx) + Math.abs(dy);
    // Capture only once it is a drag, so a plain click still lands on the logo under the pointer.
    if (s.moved > 5 && !stage.current?.hasPointerCapture(e.pointerId)) { stage.current?.setPointerCapture(e.pointerId); stage.current!.dataset.drag = "true"; setTouched(true); }
    s.x += dx; s.y += dy;
    s.vx = s.vx * 0.6 + (dx / dt) * 0.4;
    s.vy = s.vy * 0.6 + (dy / dt) * 0.4;
    s.last = { x: e.clientX, y: e.clientY, t: now };
  };
  const up = () => { live.current.drag = false; delete stage.current?.dataset.drag; };
  const keys = (e: KeyboardEvent) => {
    if (e.target !== e.currentTarget) return;
    const step = { ArrowLeft: [90, 0], ArrowRight: [-90, 0], ArrowUp: [0, 90], ArrowDown: [0, -90] }[e.key];
    if (!step) return;
    e.preventDefault();
    const s = live.current;
    s.target = { x: s.x + step[0], y: s.y + step[1] };
  };
  // Focusing a logo with the keyboard slides the canvas so that logo is in the middle.
  const bring = (i: number) => {
    const s = live.current, w = stage.current?.clientWidth ?? stageW;
    const wrapTo = (want: number, have: number, m: number) => have + ((((want - have) % m) + 1.5 * m) % m) - m / 2;
    s.target = { x: wrapTo(w / 2 - base[i].x, s.x, tileW), y: wrapTo(height / 2 - base[i].y, s.y, tileH) };
  };

  return (
    <div
      ref={stage}
      className="pg-canvas"
      tabIndex={0}
      role="group"
      aria-label={lang === "zh" ? "可以拖动的画布，方向键平移" : "A canvas you can drag; arrow keys pan it"}
      style={{ height, maxWidth: 1240 } as CSSProperties}
      onPointerDown={down}
      onPointerMove={move}
      onPointerUp={up}
      onPointerCancel={up}
      onPointerEnter={() => { live.current.hold = true; }}
      onPointerLeave={() => { live.current.hold = false; }}
      onFocus={() => { live.current.hold = true; }}
      onBlur={() => { live.current.hold = false; }}
      onKeyDown={keys}
      onClickCapture={(e) => { if (live.current.moved > 6) { e.stopPropagation(); e.preventDefault(); } }}
    >
      <div ref={layer} className="pg-canvas-layer" style={{ width: nx * tileW, height: ny * tileH }}>
        {Array.from({ length: nx * ny }, (_, c) => ({ cx: c % nx, cy: Math.floor(c / nx) })).flatMap(({ cx, cy }) =>
          agents.map((a, i) => {
            const primary = cx === 0 && cy === 0;
            return (
              <button
                key={`${cx}-${cy}-${a.id}`}
                type="button"
                className="pg-canvas-item pg-item"
                data-on={selected === a.id || undefined}
                style={{ left: cx * tileW + base[i].x, top: cy * tileH + base[i].y }}
                tabIndex={primary ? undefined : -1}
                aria-hidden={primary ? undefined : true}
                aria-pressed={primary ? selected === a.id : undefined}
                aria-label={v.names ? undefined : a.name}
                onFocus={primary ? () => bring(i) : undefined}
                onClick={() => select(a.id)}
              >
                <LogoTile a={a} size={num(v.size)} mode={String(v.logo)} />
                {v.names && <span className="pg-canvas-name">{a.name}</span>}
              </button>
            );
          }),
        )}
      </div>
      {!touched && (
        <span className="pointer-events-none absolute bottom-3 left-1/2 -translate-x-1/2 rounded-full bg-ink px-3 py-1 text-xs text-paper">{words[lang].drag}</span>
      )}
    </div>
  );
}
