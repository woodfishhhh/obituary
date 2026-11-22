import { Fragment, type CSSProperties } from "react";
import mark from "@/assets/mark-full.webp";
import { agents, LogoTile, num, usePg } from "../common";
import { logoColor, range, toggle, type Knob, type Values } from "../knobs";

export const knobs: Knob[] = [
  range("speed", "转一圈要", 30, 240, 5, 110, " 秒"),
  range("tilt", "俯视倾斜", 0, 70, 1, 56, "°"),
  range("size", "图标大小", 34, 76, 2, 54, " px"),
  range("spread", "轨道张开", 0.7, 1.1, 0.01, 1),
  toggle("lines", "画出轨道线", true),
  toggle("alternate", "相邻轨道反向", true),
  toggle("center", "中心放讣告", true),
  logoColor,
];

const RINGS = [6, 9, 12]; // 27 agents, inner to outer
const SHARE = [0.36, 0.66, 1];

// Three rings around the logo. Each ring spins; the icons turn back against it (and against the tilt), so they
// stay upright and face the reader. Hover or focus stops the sky.
export function Wall({ v }: { v: Values }) {
  const { width, reduce } = usePg();
  const side = Math.min(Math.max(width - 24, 280), 720);
  const tilt = num(v.tilt);
  const k = Math.min(1, side / 640);
  const size = Math.round(num(v.size) * Math.max(0.62, k));
  const reach = (side / 2 - size / 2 - 6) * num(v.spread);
  const height = Math.round(side * Math.cos((tilt * Math.PI) / 180) + size * 2.2);
  let at = 0;
  return (
    <div className="pg-orbit" style={{ width: side, height, perspective: side * 2.4, "--tilt": `${tilt}deg` } as CSSProperties}>
      <div className="pg-orbit-plane" style={{ width: side, height: side, translate: "-50% -50%", transform: `rotateX(${tilt}deg)` }}>
        {RINGS.map((n, r) => {
          const radius = reach * SHARE[r];
          const ring = agents.slice(at, (at += n));
          const back = Boolean(v.alternate) && r % 2 === 1;
          return (
            <Fragment key={r}>
              {v.lines && <div className="pg-ring-line" style={{ width: radius * 2, height: radius * 2 }} />}
              <div className="pg-ring" data-rev={back || undefined} style={{ "--dur": `${num(v.speed) * (0.7 + r * 0.3)}s`, animationPlayState: reduce ? "paused" : undefined } as CSSProperties}>
                {ring.map((a, j) => {
                  const deg = (j / n) * 360 + r * 17;
                  return (
                    <div key={a.id} className="pg-slot" style={{ transform: `rotate(${deg}deg) translateX(${radius}px)` }}>
                      <div className="pg-up1" style={{ transform: `rotate(${-deg}deg)` }}>
                        <div className="pg-up2" style={{ "--dur": `${num(v.speed) * (0.7 + r * 0.3)}s` } as CSSProperties}>
                          <div className="pg-up3" style={{ transform: `rotateX(${-tilt}deg)` }}>
                            <Orb a={a} size={size} mode={String(v.logo)} />
                          </div>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </Fragment>
          );
        })}
        {v.center && (
          <div className="pg-center" style={{ transform: `rotateX(${-tilt}deg)` }}>
            <img src={mark} alt="" className="pg-mark" style={{ width: size * 2.4 }} />
          </div>
        )}
      </div>
    </div>
  );
}

function Orb({ a, size, mode }: { a: (typeof agents)[number]; size: number; mode: string }) {
  const { selected, select } = usePg();
  return (
    <button type="button" className="pg-orb pg-item" data-name={a.name} data-on={selected === a.id || undefined} aria-pressed={selected === a.id} aria-label={a.name} onClick={() => select(a.id)}>
      <LogoTile a={a} size={size} mode={mode} />
    </button>
  );
}
