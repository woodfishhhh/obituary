// K 隧道: a tunnel of time in real 3D (CSS perspective, no WebGL). Its gates
// are the logo's tombstone outline, receding into the dark; every file
// floats at the depth of the time it has left, so the nearest are the most
// urgent. The wheel or the scrubber flies through time. Extending a file
// pushes its stone deeper into the tunnel; resurrecting lifts it out.
import { Check, EyeOff, Flame, RefreshCw, Search, TimerReset } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { bytes, graveSize } from "../../format";
import Decisions from "../../pages/Decisions";
import Logs from "../../pages/Logs";
import Settings from "../../pages/Settings";
import { t } from "../../i18n";
import Notices from "../../notices";
import { pageTitle, type ShellProps } from "../../shell";
import { calm, DAY, extendSpans, folderOf, HOUR, nameOf, parts, pause, remaining, span, Star, STAR, useGraves, useItems } from "../shared";
import "./tunnel.css";

const DEPTH = 2600;
const FAR = 8 * DAY;
// Time left to depth: logarithmic, so an hour and a week both get room.
const depthOf = (ms: number) => Math.min(DEPTH * 1.04, DEPTH * Math.log1p(Math.max(0, ms) / HOUR) / Math.log1p(FAR / HOUR));
const gates = [HOUR, 6 * HOUR, DAY, 2 * DAY, 3 * DAY, 5 * DAY, 7 * DAY];
const CAM_MAX = DEPTH - 420;

// Gate colour by depth: gold close by, through pale light, to sky far away.
// The pale middle keeps the mix from passing through green.
const stops: [number, number, number][] = [[248, 224, 0], [226, 238, 255], [0, 120, 248]];
function gateColour(depth: number) {
  const t = depth * depth; // the first day stays warm
  const [a, b, f] = t < 0.5 ? [stops[0], stops[1], t * 2] : [stops[1], stops[2], t * 2 - 1];
  const c = a.map((v, i) => Math.round(v + (b[i] - v) * f));
  return `rgba(${c[0]}, ${c[1]}, ${c[2]}, ${(0.95 - depth * 0.5).toFixed(2)})`;
}

export default function TunnelShell({ page, nav, setPage, summary: s, refresh }: ShellProps) {
  const list = page === "graveyard" || page === "upcoming";
  return (
    <div className="tn" data-list={list}>
      <header className="tn-top">
        <div className="tn-brand"><img src="icon.png" alt="" />{t("brand")}</div>
        <nav className="tn-nav" aria-label={t("pagesLabel")}>
          {nav.map((id) => (
            <button key={id} type="button" aria-current={page === id ? "page" : undefined} onClick={() => setPage(id)}>
              {pageTitle(id)}{id === "decisions" && s?.decisionCount ? <span className="tn-count">{s.decisionCount}</span> : null}
            </button>
          ))}
        </nav>
        <span className="tn-grow" />
        <Notices page={page} setPage={setPage} summary={s} refresh={refresh} />
        {s ? <span className="tn-sum">{t("tn.sum", s.trackedCount, s.graveCount)}</span> : null}
        <button type="button" className="tn-icon" aria-label={t("refresh")} title={t("refresh")} onClick={refresh}><RefreshCw size={15} aria-hidden /></button>
      </header>
      {page === "graveyard" ? <Graves key="g" /> : page === "upcoming" ? <Upcoming key="u" /> : (
        <>
          <Scene cam={DEPTH * 0.35} still />
          <div className="tn-panel"><div className="tn-panel-scroll">{page === "decisions" ? <Decisions /> : page === "settings" ? <Settings /> : <Logs />}</div></div>
        </>
      )}
    </div>
  );
}

// ── The tunnel ─────────────────────────────────────────────────────────

interface Stone { id: number; ms: number; name: string; folder: string; caption: ReactNode; note?: string }

function Scene({ cam, still, children }: { cam: number; still?: boolean; children?: ReactNode }) {
  return (
    <div className="tn-scene" data-still={still} aria-hidden={still}>
      <div className="tn-world" style={{ transform: `translateZ(${cam.toFixed(0)}px)` }}>
        {gates.map((ms) => {
          const label = span(ms);
          const z = -depthOf(ms);
          const depth = depthOf(ms) / DEPTH;
          const e = z + cam;
          return (
            <div key={label} className="tn-gate" style={{ "--gate": gateColour(depth), transform: `translate3d(-50%, -50%, ${z.toFixed(0)}px)`, opacity: e > 40 ? 0 : 1 } as CSSProperties}>
              <span>{label}</span>
            </div>
          );
        })}
        {children}
      </div>
    </div>
  );
}

// The camera: wheel and scrubber move it; a stone that takes focus is
// brought near enough to read.
function useCamera() {
  const [cam, setCam] = useState(calm() ? 0 : -500);
  const box = useRef<HTMLDivElement>(null);
  const aimed = useRef(false);
  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => { e.preventDefault(); setCam((c) => Math.max(0, Math.min(CAM_MAX, c + e.deltaY * 1.4))); };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, []);
  const bring = (z: number) => setCam((c) => (z + c < -700 || z + c > -60 ? Math.max(0, Math.min(CAM_MAX, -z - 260)) : c));
  // On arrival, fly in to the most urgent stone so the first view is legible.
  const aim = (z: number | undefined) => {
    if (aimed.current || z === undefined) return;
    aimed.current = true;
    setTimeout(() => setCam(Math.max(0, Math.min(CAM_MAX, -z - 480))), calm() ? 0 : 40);
  };
  return { cam, setCam, box, bring, aim };
}

// Stones wind around the tunnel wall on a golden-angle spiral, so no two
// neighbours in time sit in the same place.
const GOLDEN = 2.39996;

function Tunnel({ stones, has, toggle, fate, cam, bring, aim }: {
  stones: Stone[]; has: (id: number) => boolean; toggle: (id: number) => void; fate: { ids: number[]; kind: string } | null; cam: number;
  bring: (z: number) => void; aim: (z: number | undefined) => void;
}) {
  const placed = useMemo(() => [...stones].sort((a, b) => a.ms - b.ms).map((st, i) => {
    const angle = 2.6 + i * GOLDEN;
    return { st, x: Math.cos(angle) * 330, y: Math.sin(angle) * 175 + 20, z: -Math.max(30, depthOf(st.ms)) };
  }), [stones]);
  useEffect(() => { aim(placed[0]?.z); }, [placed, aim]);
  return (
    <Scene cam={cam}>
      {placed.map(({ st, x, y, z }) => {
        const e = z + cam;
        const gone = e > 140;
        const p = parts(st.ms);
        // Far stones still read: undo part of the perspective's shrink (880px
        // in tunnel.css), so depth shows without names turning into specks.
        const grow = e < 0 ? Math.min(2.4, Math.pow((880 - e) / 880, 0.7)) : 1;
        return (
          <button key={st.id} type="button" className="tn-stone" data-on={has(st.id)} data-fate={fate?.ids.includes(st.id) ? fate.kind : undefined}
            aria-pressed={has(st.id)} aria-label={t("tn.stone", st.name, st.ms > 0 ? t("span", p.n, p.unit) : null)} tabIndex={gone ? -1 : 0}
            style={{ transform: `translate3d(calc(-50% + ${(x * grow).toFixed(0)}px), calc(-50% + ${(y * grow).toFixed(0)}px), ${z.toFixed(0)}px) scale(${grow.toFixed(3)})`, opacity: gone ? 0 : e > 20 ? 0.4 : 1, pointerEvents: gone ? "none" : undefined }}
            onFocus={() => bring(z)} onClick={() => { bring(z); toggle(st.id); }}>
            <span className="tn-stone-body">
              <span className="tn-stone-top">
                <svg className="tn-star" width="14" height="14" viewBox="0 0 24 24" aria-hidden><path d={STAR} /></svg>
                <b>{st.ms > 0 ? p.n : t("tn.dueBig")}</b><i>{st.ms > 0 ? p.unit : t("tn.dueSmall")}</i>
                <span className="tn-tick" aria-hidden><Check size={12} strokeWidth={3} /></span>
              </span>
              <span className="tn-name">{st.name}</span>
              <span className="tn-folder">{st.folder}</span>
              <span className="tn-caption">{st.caption}</span>
            </span>
          </button>
        );
      })}
    </Scene>
  );
}

function Scrubber({ cam, setCam }: { cam: number; setCam: (n: number) => void }) {
  const pos = cam / CAM_MAX;
  return (
    <div className="tn-scrub">
      <span>{t("tn.far")}</span>
      <input type="range" min={0} max={CAM_MAX} step={10} value={cam} aria-label={t("tn.travel")} aria-valuetext={pos < 0.05 ? t("tn.now") : t("tn.ahead", Math.round(pos * 100))} onChange={(e) => setCam(Number(e.target.value))} />
      <span>{t("tn.now")}</span>
    </div>
  );
}

function Hud({ title, deck, tools }: { title: string; deck: ReactNode; tools?: ReactNode }) {
  return (
    <div className="tn-hud">
      <h1>{title}</h1>
      <p>{deck}</p>
      {tools}
    </div>
  );
}

function Dock({ open, children }: { open: boolean; children: ReactNode }) {
  return <div className="tn-dock" data-open={open} aria-hidden={!open} inert={!open}>{children}</div>;
}

function Quiet({ title, text }: { title: string; text: string }) {
  return <div className="tn-quiet"><Star size={20} /><b>{title}</b><span>{text}</span></div>;
}

// ── 墓地 ───────────────────────────────────────────────────────────────

function Graves() {
  const [filter, setFilter] = useState("");
  const g = useGraves(filter);
  const { cam, setCam, box, bring, aim } = useCamera();
  const [fate, setFate] = useState<{ ids: number[]; kind: string } | null>(null);
  const play = (kind: string) => async () => { setFate({ ids: g.sel.chosen, kind }); await pause(820); setFate(null); };
  const stones: Stone[] = g.graves.map((b) => {
    const { ms } = remaining(b.buriedAt, b.purgeAt);
    return { id: b.id, ms, name: nameOf(b.path), folder: folderOf(b.path), note: b.note, caption: t("tn.graveCaption", ms > 0, graveSize(b)) };
  });
  const k = g.sel.chosen.length;
  return (
    <div className="tn-stage" ref={box}>
      {!g.loading || g.data ? <Tunnel stones={stones} has={g.sel.has} toggle={g.sel.toggle} fate={fate} cam={cam} bring={bring} aim={aim} /> : <Scene cam={cam} />}
      <Hud title={t("head.graveyard")} deck={g.graves.length ? t("tn.graveDeck", g.graves.length, bytes(g.bytes)) : t("tn.graveDeckEmpty")}
        tools={<label className="tn-search"><Search size={14} aria-hidden /><input type="search" placeholder={t("filterByPath")} aria-label={t("filterByPath")} value={filter} onChange={(e) => setFilter(e.target.value)} /></label>} />
      {g.error ? <p className="tn-alert">{t("grave.readError", g.error)}</p> : null}
      {g.data && !g.graves.length ? <Quiet title={filter ? t("tn.noStone") : t("tn.empty")} text={filter ? t("notInPath", filter) : t("grave.emptyText")} /> : null}
      <Scrubber cam={cam} setCam={setCam} />
      <Dock open={k > 0}>
        <span className="tn-dock-count">{t("tn.pickedGraves", k)}</span>
        <button type="button" className="tn-btn gold" onClick={() => g.resurrect(undefined, play("rise"))}><Star size={13} />{t("resurrect")}</button>
        <button type="button" className="tn-btn ink" onClick={() => g.purge(undefined, play("ash"))}><Flame size={14} aria-hidden />{t("purge")}</button>
        <button type="button" className="tn-btn quiet" onClick={g.sel.clear}>{t("clearSelection")}</button>
      </Dock>
    </div>
  );
}

// ── 即将下葬 ───────────────────────────────────────────────────────────

function Upcoming() {
  const it = useItems("");
  const { cam, setCam, box, bring, aim } = useCamera();
  const [lifespan, setLifespan] = useState("1d");
  const stones: Stone[] = it.items.map((x) => {
    const ms = +new Date(x.expiresAt) - Date.now();
    return { id: x.id, ms, name: nameOf(x.path), folder: folderOf(x.path), note: x.note, caption: t("tn.itemCaption", ms > 0) };
  });
  const k = it.sel.chosen.length;
  return (
    <div className="tn-stage" ref={box}>
      {!it.loading || it.data ? <Tunnel stones={stones} has={it.sel.has} toggle={it.sel.toggle} fate={null} cam={cam} bring={bring} aim={aim} /> : <Scene cam={cam} />}
      <Hud title={t("head.upcoming")} deck={it.items.length ? t("tn.upDeck", it.items.length) : t("tn.upDeckEmpty")} />
      {it.error ? <p className="tn-alert">{t("up.readError", it.error)}</p> : null}
      {it.data && !it.items.length ? <Quiet title={t("tn.empty")} text={t("up.quietAll")} /> : null}
      <Scrubber cam={cam} setCam={setCam} />
      <Dock open={k > 0}>
        <span className="tn-dock-count">{t("tn.pickedItems", k)}</span>
        <select className="tn-select" aria-label={t("howLong")} value={lifespan} onChange={(e) => setLifespan(e.target.value)}>
          {extendSpans().map(([v, label]) => <option key={v} value={v}>{t("giveMore", label)}</option>)}
        </select>
        <button type="button" className="tn-btn cobalt" onClick={() => it.extend(lifespan)}><TimerReset size={14} aria-hidden />{t("extend")}</button>
        <button type="button" className="tn-btn ink" onClick={() => it.cancel()}><EyeOff size={14} aria-hidden />{t("stopTracking")}</button>
        <button type="button" className="tn-btn quiet" onClick={it.sel.clear}>{t("clearSelection")}</button>
      </Dock>
    </div>
  );
}

