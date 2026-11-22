import Dock from "@/components/reactbits/Dock";
import { agents, LogoTile, num, runs, usePg } from "../common";
import { choice, range, toggle, type Knob, type Values } from "../knobs";

export const knobs: Knob[] = [
  range("size", "图标大小", 32, 72, 2, 48, " px"),
  range("zoom", "放大倍数", 1, 2.6, 0.05, 1.7, "×"),
  range("distance", "影响范围", 80, 360, 10, 200, " px"),
  range("gap", "间距", 2, 20, 1, 8, " px"),
  range("rows", "排数（窄屏会自动加）", 1, 4, 1, 2),
  range("round", "圆角", 0.1, 0.9, 0.05, 0.5),
  toggle("bounce", "点击弹跳", true),
  toggle("labels", "悬停显示名字", true),
  choice("tiles", "图标底板", [["on", "有底板"], ["off", "无底板"]], "on"),
];

// React Bits' Dock, one panel per row; a narrow stage gets more rows so a panel is never squeezed small.
export function Wall({ v }: { v: Values }) {
  const { width, theme, selected, select } = usePg();
  const size = num(v.size), gap = num(v.gap), peak = size * num(v.zoom);
  const room = Math.max(200, Math.min(width, 1240) - 56);
  const perRow = Math.max(3, Math.floor(room / ((size + gap) * 0.7)));
  const rows = runs(agents, Math.max(num(v.rows), Math.ceil(agents.length / perRow)));
  return (
    <div className="mx-auto flex max-w-[1240px] flex-col px-5 @3xl:px-10">
      {rows.map((row, i) => (
        <div key={i} className="relative" style={{ height: size + 16 + (peak - size) + (v.labels ? 34 : 6) }}>
          <Dock
            theme={theme}
            baseItemSize={size}
            magnification={peak}
            distance={num(v.distance)}
            gap={gap}
            panelHeight={size + 16}
            roundness={num(v.round)}
            bounce={Boolean(v.bounce)}
            tiles={v.tiles === "on"}
            showLabels={Boolean(v.labels)}
            showIndicators
            accentColor={theme === "dark" ? "#4f8bff" : "#0048e8"}
            items={row.map((a) => ({
              label: a.name,
              active: selected === a.id,
              onClick: () => select(a.id),
              icon: <LogoTile a={a} size={size} mode="color" className="!size-full !rounded-[inherit] !shadow-none" />,
            }))}
          />
        </div>
      ))}
    </div>
  );
}
