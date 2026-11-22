import type { CSSProperties } from "react";
import SpotlightCard from "@/components/reactbits/SpotlightCard";
import { agents, LogoTile, num, usePg } from "../common";
import { choice, logoColor, range, toggle, type Knob, type Values } from "../knobs";

export const knobs: Knob[] = [
  range("card", "卡片宽度", 110, 240, 2, 150, " px"),
  range("gap", "间距", 6, 28, 1, 12, " px"),
  range("radius", "圆角", 10, 40, 1, 22, " px"),
  range("logoSize", "图标大小", 32, 80, 2, 52, " px"),
  choice("layout", "排布", [["stack", "竖排"], ["row", "横排"]], "stack"),
  range("size", "聚光半径", 80, 420, 10, 220, " px"),
  range("intensity", "光的强度", 0, 1, 0.05, 0.55),
  range("glow", "描边光", 0, 1, 0.05, 0.9),
  range("soft", "光的柔和度", 0, 1, 0.05, 0.7),
  range("proximity", "感应距离", 0, 320, 10, 120, " px"),
  choice("shape", "光的形状", [["circle", "圆形"], ["beam", "垂直光束"]], "circle"),
  choice("hue", "光的颜色", [["cobalt", "钴蓝"], ["gold", "金黄"], ["ink", "墨色"]], "cobalt"),
  toggle("ambient", "没有指针时自己游走", false),
  logoColor,
];

const hues = { light: { cobalt: "#0048e8", gold: "#e0b800", ink: "#111522" }, dark: { cobalt: "#6aa0ff", gold: "#f8e000", ink: "#ffffff" } };

// React Bits' Spotlight Card, one per agent: the light follows the pointer, or the keyboard focus.
export function Wall({ v }: { v: Values }) {
  const { theme, selected, select } = usePg();
  const stack = v.layout === "stack";
  // The card's own light palette dims the fill, so ask for more of it there.
  const intensity = Math.min(1, num(v.intensity) * (theme === "light" ? 2.4 : 1));
  const style = { gridTemplateColumns: `repeat(auto-fill, minmax(${v.card}px, 1fr))`, gap: num(v.gap) } as CSSProperties;
  return (
    <div className="pg-spot mx-auto max-w-[1240px] px-5 @3xl:px-10" style={style}>
      {agents.map((a) => (
        <SpotlightCard
          key={a.id}
          theme={theme}
          spotlightColor={hues[theme][v.hue as "cobalt"]}
          spotlightSize={num(v.size)}
          intensity={intensity}
          borderGlow={num(v.glow)}
          softness={num(v.soft)}
          proximity={num(v.proximity)}
          shape={v.shape as "circle"}
          ambient={Boolean(v.ambient)}
          flare={false}
          data-on={selected === a.id || undefined}
          className="pg-spot-card p-0! rounded-(--pg-r)!"
          style={{ "--pg-r": `${v.radius}px`, "--spotlight-card-surface": "var(--color-paper)", "--spotlight-card-border": "var(--color-line)" } as CSSProperties}
        >
          <button
            type="button"
            className="pg-spot-btn pg-item"
            data-layout={v.layout}
            style={{ "--pad": stack ? "1.1rem 0.6rem 1rem" : "0.8rem 0.9rem" } as CSSProperties}
            aria-pressed={selected === a.id}
            onClick={() => select(a.id)}
          >
            <LogoTile a={a} size={num(v.logoSize)} mode={String(v.logo)} />
            <span className="min-w-0">
              <span className="pg-spot-name">{a.name}</span>
              <span className="pg-spot-id">{a.id}</span>
            </span>
          </button>
        </SpotlightCard>
      ))}
    </div>
  );
}
