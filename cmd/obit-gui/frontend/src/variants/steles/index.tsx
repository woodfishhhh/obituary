// G 碑林: the logo's tombstone is a file with a folded corner, so every file
// here is a stele in that shape. In the graveyard, grass grows over a stone
// as its cremation nears; for tracked files the stone is not built yet and
// its outline is the lifeline, running out toward burial.
import { Check, Hourglass, Inbox, RefreshCw, ScrollText, Search, Settings2, X, type LucideIcon } from "lucide-react";
import { createContext, useContext, useId, useMemo, useState, type ReactNode } from "react";
import { bytes } from "../../format";
import Decisions from "../../pages/Decisions";
import Logs from "../../pages/Logs";
import Settings from "../../pages/Settings";
import { pageTitle, type PageId, type ShellProps } from "../../shell";
import { extendSpans, folderOf, hm, md, nameOf, parts, pause, remaining, sourceLabel, Star, STAR, useGraves, useItems } from "../shared";
import "./steles.css";

// The stele, 160 x 212: chamfered top corners, the fold at the top right.
const OUTER = "M0 30Q0 26 3 23L23 3Q26 0 30 0H126Q130 0 133 3L157 27Q160 30 160 34V212H0Z";
const FACE = "M6 32Q6 29 8 27L27 8Q29 6 32 6H124Q127 6 129 8L154 33V212H6Z";
const FOLD = "M125 7L153 35H133Q125 35 125 27Z";
const LIFE = "M1.5 212V30Q1.5 27 4 24.5L24.5 4Q27 1.5 30 1.5H126Q129 1.5 132 4.5L156 28.5Q158.5 31 158.5 34V212";
const RING = "M-5 29Q-5 23 -1 19L19 -1Q23 -5 29 -5H127Q133 -5 137 -1L161 23Q165 27 165 33V214";

const Ids = createContext({ rim: "", face: "", grass: "", life: "" });

function StoneIcon(props: { size?: number }) {
  const s = props.size ?? 20;
  return <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinejoin="round" aria-hidden><path d="M5 21V8L9 4H15L19 8V21Z" /><path d="M3 21H21" /><path d="M12 9V15M9.5 11.5H14.5" /></svg>;
}

const icons: Partial<Record<PageId, LucideIcon | typeof StoneIcon>> = { graveyard: StoneIcon, upcoming: Hourglass, decisions: Inbox, settings: Settings2, logs: ScrollText };

export default function StelesShell({ page, nav, setPage, summary: s, refresh }: ShellProps) {
  const id = useId();
  const ids = useMemo(() => ({ rim: `${id}rim`, face: `${id}face`, grass: `${id}grass`, life: `${id}life` }), [id]);
  return (
    <Ids.Provider value={ids}>
      <div className="st">
        <svg className="st-defs" aria-hidden>
          <defs>
            <linearGradient id={ids.rim} x1="0" y1="0" x2="1" y2="0.25">
              <stop offset="0" stopColor="var(--cobalt)" /><stop offset="0.42" stopColor="var(--sky)" /><stop offset="0.66" stopColor="var(--glint)" /><stop offset="1" stopColor="var(--gold)" />
            </linearGradient>
            <linearGradient id={ids.face} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stopColor="var(--face-top)" /><stop offset="1" stopColor="var(--face-low)" />
            </linearGradient>
            <linearGradient id={ids.grass} x1="0" y1="1" x2="0" y2="0">
              <stop offset="0" stopColor="var(--sky)" stopOpacity="0.15" /><stop offset="0.45" stopColor="var(--sky)" stopOpacity="0.75" /><stop offset="1" stopColor="var(--gold)" />
            </linearGradient>
            <linearGradient id={ids.life} x1="0" y1="1" x2="1" y2="0">
              <stop offset="0" stopColor="var(--cobalt)" /><stop offset="0.5" stopColor="var(--sky)" /><stop offset="1" stopColor="var(--gold)" />
            </linearGradient>
          </defs>
        </svg>
        <nav className="st-rail" aria-label="页面">
          <img className="st-logo" src="icon.png" alt="讣告" />
          {nav.map((p) => {
            const Icon = icons[p] ?? Inbox;
            return (
              <button key={p} type="button" className="st-nav" aria-current={page === p ? "page" : undefined} onClick={() => setPage(p)}>
                <span className="st-nav-icon"><Icon size={20} aria-hidden />{p === "decisions" && s?.decisionCount ? <span className="st-badge">{s.decisionCount}</span> : null}</span>
                <span className="st-nav-label">{pageTitle(p)}</span>
              </button>
            );
          })}
          <span className="st-grow" />
          <button type="button" className="st-nav small" onClick={refresh} title="刷新"><RefreshCw size={17} aria-hidden /><span className="st-nav-label">刷新</span></button>
        </nav>
        <div className="st-stage">
          <main className="st-main" key={page}>
            {page === "graveyard" ? <Graves /> : page === "upcoming" ? <Upcoming /> : (
              <div className="st-shared">{page === "decisions" ? <Decisions /> : page === "settings" ? <Settings /> : <Logs />}</div>
            )}
          </main>
        </div>
      </div>
    </Ids.Provider>
  );
}

// ── Pieces ─────────────────────────────────────────────────────────────

function Head({ title, deck, tools }: { title: string; deck: ReactNode; tools?: ReactNode }) {
  return (
    <header className="st-head">
      <div className="st-head-text"><h1>{title}</h1><p>{deck}</p></div>
      {tools ? <div className="st-tools">{tools}</div> : null}
    </header>
  );
}

function Dock({ open, children, onClose }: { open: boolean; children: ReactNode; onClose: () => void }) {
  return (
    <div className="st-dock" data-open={open} aria-hidden={!open} inert={!open}>
      {children}
      <button type="button" className="st-dock-close" aria-label="取消选择" onClick={onClose}><X size={16} aria-hidden /></button>
    </div>
  );
}

function Empty({ title, text }: { title: string; text: string }) {
  const ids = useContext(Ids);
  return (
    <div className="st-empty">
      <svg viewBox="-8 -8 176 228" width="64" height="84" aria-hidden><path d={OUTER} fill="none" stroke={`url(#${ids.rim})`} strokeWidth="5" /><path d={STAR} transform="translate(68 70) scale(1)" fill="var(--gold)" /></svg>
      <h2>{title}</h2><p>{text}</p>
    </div>
  );
}

const Loading = () => <div className="st-field" aria-busy="true" aria-label="正在读取">{Array.from({ length: 5 }, (_, i) => <div key={i} className="st-plot ghost" />)}</div>;

// Grass: deterministic blades per grave, so a stone keeps its own tuft.
function blades(seed: number) {
  let r = (seed * 7919) % 233280;
  const rand = () => (r = (r * 9301 + 49297) % 233280) / 233280;
  return Array.from({ length: 24 }, (_, i) => {
    const x = (i / 23) * 172 - 6 + (rand() - 0.5) * 8;
    const h = 34 + rand() * 66;
    const lean = (rand() - 0.5) * 34;
    const w = 1.3 + rand() * 1.7;
    return { back: i % 3 === 1, d: `M${x - w} 100Q${x - w * 0.2 + lean * 0.35} ${100 - h * 0.55} ${x + lean} ${100 - h}Q${x + w * 0.2 + lean * 0.35} ${100 - h * 0.5} ${x + w} 100Z` };
  });
}

function Plot({ id, on, onToggle, label, fate, kind, children, caption, grass, life, soon }: {
  id: number; on: boolean; onToggle: () => void; label: string; fate?: "rise" | "ash"; kind: "grave" | "plan";
  children: ReactNode; caption: ReactNode; grass?: number; life?: number; soon?: boolean;
}) {
  const ids = useContext(Ids);
  const tuft = useMemo(() => blades(id), [id]);
  return (
    <div className={`st-plot ${kind}`} data-on={on} data-fate={fate} data-soon={soon} onClick={onToggle}>
      <input type="checkbox" className="st-check" aria-label={label} checked={on} onChange={onToggle} onClick={(e) => e.stopPropagation()} />
      <span className="st-ground" aria-hidden />
      <div className="st-lift">
        <svg className="st-stone" viewBox="-8 -8 176 222" aria-hidden>
          <path className="st-ring" d={RING} />
          {kind === "grave" ? (
            <>
              <path d={OUTER} fill={`url(#${ids.rim})`} />
              <path d={FACE} fill={`url(#${ids.face})`} />
              <path className="st-fold" d={FOLD} />
            </>
          ) : (
            <>
              <path className="st-glass" d={OUTER} />
              <path className="st-track" d={LIFE} pathLength={100} />
              <path className="st-life" d={LIFE} pathLength={100} stroke={`url(#${ids.life})`} strokeDasharray={`${((life ?? 0) * 100).toFixed(2)} 100`} />
            </>
          )}
        </svg>
        <div className="st-body">
          <div className="st-face">{children}</div>
        </div>
        <span className="st-tick" aria-hidden><Check size={13} strokeWidth={3} /></span>
      </div>
      {grass !== undefined ? (
        <svg className="st-grass" viewBox="0 0 160 100" preserveAspectRatio="none" style={{ transform: `scaleY(${((18 + grass * 58) / 76).toFixed(3)})` }} aria-hidden>
          {tuft.map((b, i) => <path key={i} d={b.d} className={b.back ? "back" : undefined} fill={b.back ? undefined : `url(#${ids.grass})`} />)}
        </svg>
      ) : null}
      <div className="st-label">{caption}</div>
    </div>
  );
}

// ── 墓地 ───────────────────────────────────────────────────────────────

function Graves() {
  const [filter, setFilter] = useState("");
  const g = useGraves(filter);
  const [fate, setFate] = useState<{ ids: number[]; kind: "rise" | "ash" } | null>(null);
  const play = (kind: "rise" | "ash") => async () => { setFate({ ids: g.sel.chosen, kind }); await pause(720); setFate(null); };
  const n = g.graves.length;
  return (
    <>
      <Head
        title="墓地"
        deck={n ? <>{n} 座碑，{bytes(g.bytes)}。草长到碑顶的那天火化，在那之前都能复活。</> : "下葬的文件在这里立碑，火化之前都能复活。"}
        tools={<>
          <label className="st-search"><Search size={15} aria-hidden /><input type="search" placeholder="按路径筛选" aria-label="按路径筛选" value={filter} onChange={(e) => setFilter(e.target.value)} /></label>
          <button type="button" className="st-text" disabled={!n} onClick={g.sel.toggleAll}>{g.sel.all ? "全不选" : "全选"}</button>
        </>}
      />
      {g.error ? <p className="st-note">读不到墓地：{g.error}</p> : null}
      {g.pending.length ? <p className="st-note">{g.pending.length} 座碑还在核实，后台下一轮会处理。</p> : null}
      {g.loading && !g.data ? <Loading /> : n === 0 ? (
        filter ? <Empty title="没有这样的碑" text={`路径里没有「${filter}」`} /> : <Empty title="碑林空着" text="下葬的文件和文件夹会在这里立碑，火化之前随时可以复活。" />
      ) : (
        <div className="st-field">
          {g.graves.map((b) => {
            const { ms, left } = remaining(b.buriedAt, b.purgeAt);
            const p = parts(ms);
            return (
              <Plot key={b.id} id={b.id} kind="grave" on={g.sel.has(b.id)} onToggle={() => g.sel.toggle(b.id)} label={`选择 ${nameOf(b.path)}`}
                fate={fate?.ids.includes(b.id) ? fate.kind : undefined} grass={1 - left} soon={ms < 86400e3}
                caption={<>{md(b.buriedAt)}下葬，{b.isDir ? "文件夹 " : ""}{bytes(b.size)}</>}>
                <svg className="st-star" width="18" height="18" viewBox="0 0 24 24" style={{ opacity: 0.3 + 0.7 * left }} aria-hidden><path d={STAR} /></svg>
                <div className="st-count">{ms > 0 ? <><b>{p.n}</b>{p.unit}</> : <b>到期</b>}</div>
                <div className="st-caption">{ms > 0 ? "还能复活" : "等待火化"}</div>
                <div className="st-name" title={b.path}>{nameOf(b.path)}</div>
                <div className="st-home">{folderOf(b.path).split(/[\\/]/).pop()}</div>
              </Plot>
            );
          })}
        </div>
      )}
      <Dock open={g.sel.chosen.length > 0} onClose={g.sel.clear}>
        <span className="st-dock-count">已选 {g.sel.chosen.length} 座</span>
        <button type="button" className="st-btn gold" onClick={() => g.resurrect(undefined, play("rise"))}><Star size={14} />复活</button>
        <button type="button" className="st-btn ink" onClick={() => g.purge(undefined, play("ash"))}>火化</button>
      </Dock>
    </>
  );
}

// ── 即将下葬 ───────────────────────────────────────────────────────────

type Range = "" | "24h" | "7d";

function Upcoming() {
  const [range, setRange] = useState<Range>("24h");
  const [lifespan, setLifespan] = useState("1d");
  const it = useItems(range);
  const n = it.items.length;
  const when = { "": "目前", "24h": "24 小时内", "7d": "7 天内" }[range];
  return (
    <>
      <Head
        title="即将下葬"
        deck={n ? <>{when}有 {n} 块碑要立。轮廓上的光走完，文件就入土。</> : `${when}没有要立的碑。`}
        tools={<div className="st-seg" role="group" aria-label="时间范围">
          {([["24h", "24 小时内"], ["7d", "7 天内"], ["", "全部"]] as [Range, string][]).map(([v, t]) => (
            <button key={v} type="button" aria-pressed={range === v} onClick={() => { setRange(v); it.sel.clear(); }}>{t}</button>
          ))}
        </div>}
      />
      {it.error ? <p className="st-note">读不到列表：{it.error}</p> : null}
      {it.loading && !it.data ? <Loading /> : n === 0 ? (
        <Empty title="这段时间很安静" text={range ? "换到「全部」看看所有在跟踪的东西" : "用 obit mark 或文件夹策略给文件设寿命后，它们会在这里等着立碑"} />
      ) : (
        <div className="st-field">
          {it.items.map((x) => {
            const { ms, left } = remaining(x.createdAt, x.expiresAt);
            const p = parts(ms);
            const soon = ms > 0 && ms < 3600e3;
            return (
              <Plot key={x.id} id={x.id} kind="plan" on={it.sel.has(x.id)} onToggle={() => it.sel.toggle(x.id)} label={`选择 ${nameOf(x.path)}`} life={left} soon={soon}
                caption={<>{md(x.expiresAt)} {hm(x.expiresAt)}，{sourceLabel(x.source)}</>}>
                <div className="st-count top">{ms > 0 ? <><b>{p.n}</b>{p.unit}</> : <b>到期</b>}</div>
                <div className="st-caption">{ms > 0 ? "后入土" : "等下一轮清理"}</div>
                <div className="st-name" title={x.path}>{nameOf(x.path)}</div>
                <div className="st-home">{folderOf(x.path).split(/[\\/]/).pop()}</div>
                {x.note ? <div className="st-plan-note">{x.note}</div> : null}
              </Plot>
            );
          })}
        </div>
      )}
      <Dock open={it.sel.chosen.length > 0} onClose={it.sel.clear}>
        <span className="st-dock-count">已选 {it.sel.chosen.length} 块</span>
        <select className="st-select" aria-label="延长多久" value={lifespan} onChange={(e) => setLifespan(e.target.value)}>
          {extendSpans().map(([v, t]) => <option key={v} value={v}>再给 {t}</option>)}
        </select>
        <button type="button" className="st-btn cobalt" onClick={() => it.extend(lifespan)}>延长</button>
        <button type="button" className="st-btn ink" onClick={() => it.cancel()}>取消跟踪</button>
      </Dock>
    </>
  );
}
