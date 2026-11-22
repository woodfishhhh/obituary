import type { CSSProperties } from "react";
import { agentById, commandOf, LogoTile, num, usePg, type Agent } from "../common";
import { logoColor, range, toggle, type Knob, type Values } from "../knobs";

export const knobs: Knob[] = [
  range("tile", "图标格宽度", 76, 140, 2, 96, " px"),
  range("logoSize", "图标大小", 30, 64, 2, 44, " px"),
  range("gap", "舱与舱的间距", 8, 32, 1, 16, " px"),
  range("radius", "圆角", 12, 40, 1, 26, " px"),
  range("lift", "悬停上浮", 0, 1.6, 0.05, 1),
  toggle("featured", "最常用的两家放大", true),
  toggle("counts", "舱头显示数量", true),
  logoColor,
];

// How the agents are grouped is a guess for the playground: it is not in the site's data.
const groups = [
  { key: "cli", accent: "var(--color-cobalt)", zh: "终端里用", en: "In the terminal", ids: ["claude", "codex", "gemini", "kimi", "vibe", "opencode", "openclaw", "pi", "omp", "hermes", "droid", "minimax-code", "deepseek-harness"] },
  { key: "ide", accent: "var(--color-sky)", zh: "编辑器里用", en: "In the editor", ids: ["cursor", "trae", "windsurf", "antigravity", "codebuddy", "qoder", "lingma", "zcode", "copilot"] },
  { key: "more", accent: "var(--color-gold-deep)", zh: "插件与桌面助手", en: "Extensions & desktop apps", ids: ["cline", "kilocode", "roo", "workbuddy", "qoderwork"] },
];

export function Wall({ v }: { v: Values }) {
  const { lang } = usePg();
  const style = { "--gap": `${v.gap}px`, "--r": `${v.radius}px`, "--tile": `${v.tile}px`, "--lift": v.lift } as CSSProperties;
  return (
    <div className="mx-auto max-w-[1240px] px-5 @3xl:px-10">
      <div className="pg-bento @4xl:grid-cols-[1.25fr_1fr]" style={style}>
        {groups.map((g, i) => (
          <section key={g.key} className={`pg-bento-panel${i === 0 ? " @4xl:row-span-2" : ""}`} style={{ "--accent": g.accent } as CSSProperties} aria-label={lang === "zh" ? g.zh : g.en}>
            <header className="relative flex items-baseline gap-2">
              <span aria-hidden="true" className="size-2 rounded-full" style={{ background: g.accent }} />
              <h3 className="m-0 text-base font-semibold">{lang === "zh" ? g.zh : g.en}</h3>
              {v.counts && <span className="text-sm text-muted">{g.ids.length}</span>}
            </header>
            <div className="pg-bento-grid">
              {g.ids.map((id, j) => {
                const a = agentById(id)!;
                return <Tile key={id} a={a} big={Boolean(v.featured) && i === 0 && j < 2} v={v} />;
              })}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}

function Tile({ a, big, v }: { a: Agent; big: boolean; v: Values }) {
  const { selected, select } = usePg();
  return (
    <button type="button" className="pg-bento-tile pg-item" data-big={big || undefined} data-on={selected === a.id || undefined} aria-pressed={selected === a.id} onClick={() => select(a.id)}>
      <LogoTile a={a} size={Math.round(num(v.logoSize) * (big ? 1.5 : 1))} mode={String(v.logo)} />
      <span className="pg-bento-name">{a.name}</span>
      {big && <code className="font-mono text-[0.6875rem] text-muted">{commandOf(a.id)}</code>}
    </button>
  );
}
