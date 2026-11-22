import { AgentStream, streamDefaults } from "@/components/agent-stream";
import { agents, docUrl, num, usePg } from "../common";
import { logoColor, range, toggle, type Knob, type Values } from "../knobs";

// The dials start at the values the home page uses (streamDefaults), so what is previewed is what is live.
const d = streamDefaults;
export const knobs: Knob[] = [
  range("rows", "行数", 1, 4, 1, d.rows),
  range("speed", "速度", 15, 140, 1, d.speed, " px/s"),
  range("size", "图标大小", 36, 76, 2, d.size, " px"),
  range("gap", "间距", 6, 32, 1, d.gap, " px"),
  range("tilt", "倾斜", 0, 8, 0.5, d.tilt, "°"),
  range("fade", "两端淡出", 0, 30, 1, d.fade, "%"),
  range("hover", "悬停放大", 0, 1, 0.05, d.hover),
  toggle("names", "显示名字", d.names),
  toggle("pause", "悬停时暂停", d.pause),
  logoColor,
];

// The wall itself is the page's own component (src/components/agent-stream.tsx).
export function Wall({ v }: { v: Values }) {
  const { selected, select, lang } = usePg();
  const options = { rows: num(v.rows), speed: num(v.speed), size: num(v.size), gap: num(v.gap), tilt: num(v.tilt), fade: num(v.fade), hover: num(v.hover), names: Boolean(v.names), pause: Boolean(v.pause), logo: String(v.logo) as typeof d.logo };
  return <AgentStream agents={agents} options={options} selected={selected} onSelect={select} href={(a) => docUrl(a.id, lang)} label="agents" />;
}
