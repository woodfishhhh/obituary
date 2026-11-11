import { Check, Copy, Maximize2, Minimize2 } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import App, { type PageId } from "../App";
import { ApiContext } from "../api";
import { AppearanceContext } from "../appearance";
import { looks } from "../catalog";
import { fromTag, LangContext, setLang, type Lang } from "../i18n";
import { createMockApi, type Scenario } from "../mock";
import { variants, type Variant } from "../variants";

// The design playground: every variant as a real, clickable window with its
// own mock data, side by side. Knobs override tokens live; "复制" puts the
// chosen variant and overrides on the clipboard to hand back.

const W = 1040;
const H = 700;
const pageOptions: [PageId, string][] = [["graveyard", "墓地 / 地平线"], ["upcoming", "即将下葬"], ["decisions", "待你决定"], ["settings", "设置"], ["logs", "日志"]];
const fonts: [string, string][] = [
  ["", "跟方案走"],
  ['"Segoe UI Variable Text", "Segoe UI", "Microsoft YaHei UI", sans-serif', "Segoe UI（系统）"],
  ['"Manrope", "Microsoft YaHei UI", sans-serif', "Manrope"],
  ['"Microsoft YaHei UI", sans-serif', "微软雅黑"],
  ['"HarmonyOS Sans SC", "MiSans", "Microsoft YaHei UI", sans-serif', "HarmonyOS / MiSans（若已安装）"],
];
const accents: [string, string][] = [["", "跟方案走"], ["#0048E8", "钴蓝"], ["#0078F8", "天蓝"], ["#111111", "墨黑"]];

export default function Playground() {
  const [page, setPage] = useState<PageId>("graveyard");
  const [scenario, setScenario] = useState<Scenario>("normal");
  // Every frame shows the same language, to compare looks; C, D and G keep
  // their own copy in Chinese.
  const [language, setLanguage] = useState<Lang>("zh");
  setLang(language);
  const langChoice = useMemo(() => ({ lang: language, adopt: (tag?: string) => { const next = fromTag(tag); if (next) setLanguage(next); } }), [language]);
  const [solo, setSolo] = useState<string | null>(null);
  const [accent, setAccent] = useState("");
  const [radius, setRadius] = useState<number | null>(null);
  const [font, setFont] = useState("");
  const [fs, setFs] = useState<number | null>(null);
  const [copied, setCopied] = useState(false);

  const overrides = useMemo(() => {
    const o: Record<string, string> = {};
    if (accent) o["--accent"] = accent;
    if (radius !== null) { o["--radius"] = `${radius}px`; o["--radius-sm"] = `${Math.max(2, Math.round(radius * 0.6))}px`; }
    if (font) o["--font"] = font;
    if (fs !== null) o["--fs"] = `${fs}px`;
    return o;
  }, [accent, radius, font, fs]);

  const shown = solo ? variants.filter((v) => v.id === solo) : variants;

  async function copy() {
    const text = JSON.stringify({ variant: solo ?? "(并排，没选)", page, overrides }, null, 2);
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  return (
    <div className="pg">
      <header className="pg-bar">
        <div className="pg-title"><img src="icon.png" alt="" /> 讣告 · 界面试验场</div>
        <label>页面<select value={page} onChange={(e) => setPage(e.target.value as PageId)}>{pageOptions.map(([v, t]) => <option key={v} value={v}>{t}</option>)}</select></label>
        <label>语言<select value={language} onChange={(e) => setLanguage(e.target.value as Lang)}><option value="zh">中文</option><option value="en">English</option></select></label>
        <label>数据<select value={scenario} onChange={(e) => setScenario(e.target.value as Scenario)}><option value="normal">正常</option><option value="busy">很多</option><option value="empty">空</option></select></label>
        <label>强调色<select value={accent} onChange={(e) => setAccent(e.target.value)}>{accents.map(([v, t]) => <option key={t} value={v}>{t}</option>)}</select></label>
        <label>圆角<input type="range" min={0} max={16} value={radius ?? 8} onChange={(e) => setRadius(Number(e.target.value))} /><span>{radius ?? "默认"}</span></label>
        <label>字体<select value={font} onChange={(e) => setFont(e.target.value)}>{fonts.map(([v, t]) => <option key={t} value={v}>{t}</option>)}</select></label>
        <label>字号<select value={fs ?? ""} onChange={(e) => setFs(e.target.value ? Number(e.target.value) : null)}><option value="">默认</option>{[13, 14, 15].map((n) => <option key={n} value={n}>{n}px</option>)}</select></label>
        <button type="button" onClick={() => { setAccent(""); setRadius(null); setFont(""); setFs(null); }}>重置</button>
        <span className="pg-grow" />
        <button type="button" className="pg-primary" onClick={copy}>{copied ? <Check size={15} /> : <Copy size={15} />} 复制我的选择</button>
      </header>
      <LangContext.Provider value={langChoice}>
      <div className={`pg-grid${solo ? " solo" : ""}`}>
        {shown.map((v) => (
          <Frame key={v.id} variant={v} page={page} scenario={scenario} overrides={overrides} solo={!!solo}
            onToggle={() => setSolo(solo ? null : v.id)} onLook={setSolo} />
        ))}
      </div>
      </LangContext.Provider>
    </div>
  );
}

// 设置 > 外观 inside a frame shows that look on its own here.
function Frame({ variant, page, scenario, overrides, solo, onToggle, onLook }: { variant: Variant; page: PageId; scenario: Scenario; overrides: Record<string, string>; solo: boolean; onToggle: () => void; onLook: (id: string) => void }) {
  const appearance = useMemo(() => ({ current: variant.id, looks, choose: onLook }), [variant.id, onLook]);
  const api = useMemo(() => createMockApi(scenario), [scenario]);
  const outer = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState(0.5);
  useEffect(() => {
    const el = outer.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setScale(Math.min(1, el.clientWidth / W)));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  const forced: PageId = variant.home === "horizon" && (page === "graveyard" || page === "upcoming") ? "horizon" : page;
  return (
    <section className="pg-frame">
      <div className="pg-frame-head">
        <b>{variant.name}</b>
        <span>{variant.note}</span>
        <button type="button" onClick={onToggle} aria-label={solo ? "回到并排" : "放大这一个"}>{solo ? <Minimize2 size={15} /> : <Maximize2 size={15} />}</button>
      </div>
      <div className="pg-window" ref={outer} style={{ height: H * scale }}>
        <div className="pg-canvas" style={{ width: W, height: H, transform: `scale(${scale})` }}>
          <ApiContext.Provider value={api}>
            <AppearanceContext.Provider value={appearance}>
              <App key={`${forced}-${scenario}`} variant={variant} page={forced} style={overrides as CSSProperties} />
            </AppearanceContext.Provider>
          </ApiContext.Provider>
        </div>
      </div>
    </section>
  );
}
