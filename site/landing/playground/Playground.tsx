import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { copyText, PgCtx, Section, useElementWidth, usePg, type Ctx, type Lang, type Theme } from "./common";
import type { Knob, Values } from "./knobs";
import { initial, styles, type StyleDef } from "./registry";

type Device = "desktop" | "mobile";
interface Page { s: string; theme: Theme; lang: Lang; device: Device; bare: boolean; motion: boolean; aura: boolean; pick: string | null }

// The page's state lives in the hash (#s=dock&theme=dark&lang=en&device=mobile), so a view can be linked, and the
// screenshots script can open each one. `bare` leaves only the section, with no controls around it.
const readHash = (): Page => {
  const p = new URLSearchParams(location.hash.slice(1));
  const pick = <T extends string>(key: string, ok: readonly T[], fallback: T) => (ok.includes(p.get(key) as T) ? (p.get(key) as T) : fallback);
  return {
    s: p.get("s") === "all" || styles.some((x) => x.id === p.get("s")) ? p.get("s")! : styles[0].id,
    theme: pick("theme", ["light", "dark"], "light"),
    lang: pick("lang", ["zh", "en"], "zh"),
    device: pick("device", ["desktop", "mobile"], "desktop"),
    bare: p.has("bare"),
    motion: p.get("motion") === "reduce",
    aura: p.get("aura") !== "off",
    pick: p.get("pick"),
  };
};
const writeHash = (g: Page) => {
  const p = new URLSearchParams({ s: g.s, theme: g.theme, lang: g.lang, device: g.device });
  if (g.motion) p.set("motion", "reduce");
  if (!g.aura) p.set("aura", "off");
  history.replaceState(null, "", `#${p}`);
};

export function Playground() {
  const [g, setG] = useState<Page>(readHash);
  const [selected, select] = useState<string | null>(() => readHash().pick);
  const [values, setValues] = useState<Record<string, Values>>(initial);
  const system = useRef(window.matchMedia("(prefers-reduced-motion: reduce)")).current;
  const [sys, setSys] = useState(system.matches);
  useEffect(() => { const f = () => setSys(system.matches); system.addEventListener("change", f); return () => system.removeEventListener("change", f); }, [system]);

  useEffect(() => {
    const root = document.documentElement;
    root.dataset.theme = g.theme;
    root.lang = g.lang === "zh" ? "zh-CN" : "en";
    if (g.motion) root.dataset.motion = "reduce"; else delete root.dataset.motion;
    if (!g.bare) writeHash(g);
  }, [g]);
  useEffect(() => { const f = () => setG(readHash()); window.addEventListener("hashchange", f); return () => window.removeEventListener("hashchange", f); }, []);

  const ctx: Ctx = useMemo(() => ({ lang: g.lang, theme: g.theme, reduce: g.motion || sys, selected, select, width: 1000 }), [g.lang, g.theme, g.motion, sys, selected]);
  const set = useCallback(<K extends keyof Page>(k: K, v: Page[K]) => setG((o) => ({ ...o, [k]: v })), []);
  const shown = g.s === "all" ? styles : styles.filter((x) => x.id === g.s);
  const change = (id: string, key: string, value: Values[string]) => setValues((o) => ({ ...o, [id]: { ...o[id], [key]: value } }));
  const reset = (id: string) => setValues((o) => ({ ...o, [id]: initial[id] }));

  if (g.bare) {
    return (
      <PgCtx.Provider value={ctx}>
        {shown.map((d) => <Stage key={d.id} def={d} values={values[d.id]} device={g.device} aura={g.aura} bare />)}
      </PgCtx.Provider>
    );
  }
  return (
    <PgCtx.Provider value={ctx}>
      <Bar g={g} set={set} />
      <main className="mx-auto grid max-w-[1560px] gap-14 px-4 pt-6 pb-24 md:px-8">
        {shown.map((d) => (
          <Block key={d.id} def={d} g={g} values={values[d.id]} onChange={(k, v) => change(d.id, k, v)} onReset={() => reset(d.id)} />
        ))}
      </main>
    </PgCtx.Provider>
  );
}

function Seg<T extends string>({ label, value, options, onChange }: { label: string; value: T; options: [T, string][]; onChange: (v: T) => void }) {
  return (
    <div role="group" aria-label={label} className="pg-seg">
      {options.map(([v, text]) => <button key={v} type="button" aria-pressed={value === v} onClick={() => onChange(v)}>{text}</button>)}
    </div>
  );
}

function Bar({ g, set }: { g: Page; set: <K extends keyof Page>(k: K, v: Page[K]) => void }) {
  return (
    <header className="sticky top-3 z-50 px-3">
      <div className="mx-auto grid max-w-[1560px] gap-2 rounded-[1.6rem] border border-line/80 bg-paper/80 p-2 shadow-nav backdrop-blur-md">
        <nav aria-label="样式" className="pg-bar pg-seg flex overflow-x-auto" role="tablist">
          {[...styles.map((s) => [s.id, `${s.no} ${s.name}`] as const), ["all", "全部"] as const].map(([id, text]) => (
            <button key={id} type="button" role="tab" aria-selected={g.s === id} onClick={() => set("s", id)}>{text}</button>
          ))}
        </nav>
        <div className="pg-bar flex items-center gap-2 overflow-x-auto px-1">
          <span className="mr-1 text-sm font-semibold whitespace-nowrap">Agents 墙 playground</span>
          <Seg label="亮暗" value={g.theme} options={[["light", "亮色"], ["dark", "暗色"]]} onChange={(v) => set("theme", v)} />
          <Seg label="语言" value={g.lang} options={[["zh", "中文"], ["en", "English"]]} onChange={(v) => set("lang", v)} />
          <Seg label="宽度" value={g.device} options={[["desktop", "桌面"], ["mobile", "手机"]]} onChange={(v) => set("device", v)} />
          <Seg label="动态效果" value={g.motion ? "reduce" : "normal"} options={[["normal", "动态正常"], ["reduce", "减少动态"]]} onChange={(v) => set("motion", v === "reduce")} />
          <Seg label="光斑" value={g.aura ? "on" : "off"} options={[["on", "光斑开"], ["off", "光斑关"]]} onChange={(v) => set("aura", v === "on")} />
        </div>
      </div>
    </header>
  );
}

function Stage({ def, values, device, aura, bare }: { def: StyleDef; values: Values; device: Device; aura: boolean; bare?: boolean }) {
  const outer = usePg();
  const ref = useRef<HTMLDivElement>(null);
  const width = useElementWidth(ref);
  return (
    <div ref={ref} className="pg-stage" data-device={device} data-bare={bare || undefined} style={bare ? { border: 0, borderRadius: 0 } : undefined}>
      <PgCtx.Provider value={{ ...outer, width }}>
        <Section panel={def.panel} aura={aura}><def.Wall v={values} /></Section>
      </PgCtx.Provider>
    </div>
  );
}

function Block({ def, g, values, onChange, onReset }: { def: StyleDef; g: Page; values: Values; onChange: (k: string, v: Values[string]) => void; onReset: () => void }) {
  return (
    <article id={def.id} className="grid gap-4">
      <header className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <span className="font-mono text-sm text-muted">{def.no}</span>
        <h2 className="m-0 text-2xl font-semibold">{def.name}</h2>
        <span className="text-muted">{def.en}</span>
        <p className="m-0 w-full max-w-[70ch] text-ink-2">{def.blurb}</p>
      </header>
      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_320px]">
        <Stage def={def} values={values} device={g.device} aura={g.aura} />
        <KnobPanel def={def} values={values} onChange={onChange} onReset={onReset} />
      </div>
    </article>
  );
}

function KnobPanel({ def, values, onChange, onReset }: { def: StyleDef; values: Values; onChange: (k: string, v: Values[string]) => void; onReset: () => void }) {
  const [done, setDone] = useState(false);
  const json = JSON.stringify({ style: def.id, name: def.name, params: values }, null, 2);
  return (
    <aside className="pg-panel grid gap-3.5 lg:sticky lg:top-36" aria-label={`${def.name} 的参数`}>
      <h3 className="m-0 text-sm font-semibold">调节 · {def.name}</h3>
      {def.knobs.map((k) => <Dial key={k.key} knob={k} value={values[k.key]} onChange={(v) => onChange(k.key, v)} />)}
      <div className="mt-1 flex gap-2">
        <button type="button" className="pg-btn pg-btn-solid flex-1" onClick={() => void copyText(json).then(() => { setDone(true); setTimeout(() => setDone(false), 1400); })}>{done ? "已复制" : "复制参数"}</button>
        <button type="button" className="pg-btn" onClick={onReset}>重置</button>
      </div>
      <pre className="m-0 max-h-40 overflow-auto rounded-xl bg-paper-3 p-2.5 font-mono text-[0.6875rem] leading-snug text-ink-2" aria-label="当前参数">{json}</pre>
    </aside>
  );
}

function Dial({ knob, value, onChange }: { knob: Knob; value: Values[string]; onChange: (v: Values[string]) => void }): ReactNode {
  const id = `k-${knob.key}`;
  if (knob.type === "range") {
    return (
      <div className="pg-knob">
        <label htmlFor={id}><span>{knob.label}</span><output>{String(value)}{knob.unit}</output></label>
        <input id={id} type="range" min={knob.min} max={knob.max} step={knob.step} value={Number(value)} onChange={(e) => onChange(Number(e.target.value))} />
      </div>
    );
  }
  if (knob.type === "toggle") {
    return (
      <div className="pg-knob">
        <span className="pg-label">{knob.label}</span>
        <Seg label={knob.label} value={value ? "on" : "off"} options={[["on", "开"], ["off", "关"]]} onChange={(v) => onChange(v === "on")} />
      </div>
    );
  }
  return (
    <div className="pg-knob">
      <span className="pg-label">{knob.label}</span>
      <Seg label={knob.label} value={String(value)} options={knob.options} onChange={onChange} />
    </div>
  );
}
