import { EyeOff, Flame, Sparkles, TimerReset, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useApi } from "../api";
import { bytes, duration, splitPath, when } from "../format";
import { useLoad, useRefresh } from "../hooks";
import type { Burial, Item } from "../types";
import { t } from "../i18n";
import { Button, Seg, useBatch, useConfirm } from "../ui";
import { extendSpans } from "../variants/shared";
import Graveyard from "./Graveyard";
import Upcoming from "./Upcoming";

// A cross-section of every file's fate: tracked items hang above the
// horizon, lower as their time runs out; graves sink below it toward the
// cremation line. Click any of them to act on it.

const MIN = 10 * 60e3;
const MAX = 30 * 86400e3;
const GUTTER = 96;
const short = (s: string, n = 20) => (s.length > n ? `${s.slice(0, n - 1)}…` : s);
const spread = (seed: number) => { const t = Math.sin(seed * 12.9898) * 43758.5453; return t - Math.floor(t); };

type Focus = { kind: "soul"; it: Item; x: number; y: number } | { kind: "grave"; b: Burial; x: number; y: number };

export default function Horizon() {
  const api = useApi();
  const { version } = useRefresh();
  const box = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ w: 900, h: 380 });
  const [focus, setFocus] = useState<Focus | null>(null);
  const [view, setView] = useState<"sky" | "ground">("sky");
  const { data } = useLoad(() => Promise.all([api.Upcoming(""), api.Graveyard({ filter: "", limit: 200, after: "" })]), [api, version]);

  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setSize({ w: el.clientWidth, h: el.clientHeight }));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const items = data?.[0].items ?? [];
  const graves = (data?.[1].rows ?? []).filter((r) => r.status === "buried" && r.burial).map((r) => r.burial!) ;
  const { w: W, h: H } = size;
  const ground = Math.round(H * 0.56);

  const marks = useMemo(() => {
    const now = Date.now();
    const placed: { x: number; y: number; w: number }[] = [];
    const span = Math.max(40, W - GUTTER - 170);
    const hits = (x: number, y: number, w: number) => placed.some((p) => Math.abs(p.y - y) < 18 && x < p.x + p.w && p.x < x + w);
    // A label that finds no free spot is not drawn over another one: the
    // mark stays as a dot and its name shows on hover and in its card.
    const place = (seed: number, y: number, label: string) => {
      const w = 26 + [...label].reduce((n, c) => n + (c.charCodeAt(0) > 0x2e7f ? 13 : 7.5), 0);
      const start = GUTTER + spread(seed) * span;
      for (let i = 0; i < 24; i++) {
        const x = GUTTER + ((start - GUTTER + i * 137) % span);
        if (!hits(x, y, w)) { placed.push({ x, y, w }); return { x, labeled: true }; }
      }
      let x = start;
      for (let i = 0; i < 24 && hits(x, y, 18); i++) x = GUTTER + ((x - GUTTER + 23) % span);
      placed.push({ x, y, w: 18 });
      return { x, labeled: false };
    };
    const skyY = (left: number) => left <= 0 ? ground - 8 : ground - 16 - Math.min(1, Math.max(0, Math.log(left / MIN) / Math.log(MAX / MIN))) * (ground - 50);
    const deepY = (b: Burial) => {
      const p = Math.min(1, Math.max(0, (now - +new Date(b.buriedAt)) / Math.max(1, +new Date(b.purgeAt) - +new Date(b.buriedAt))));
      return ground + 30 + p * (H - 64 - ground);
    };
    const souls = [...items].sort((a, b) => +new Date(a.expiresAt) - +new Date(b.expiresAt)).slice(0, 60).map((it) => {
      const left = +new Date(it.expiresAt) - now;
      const y = skyY(left);
      const label = short(splitPath(it.path).name);
      return { it, y, ...place(it.id, y, label), label, soon: left < 3600e3 };
    });
    const stones = [...graves].sort((a, b) => +new Date(a.purgeAt) - +new Date(b.purgeAt)).slice(0, 60).map((b) => {
      const y = deepY(b);
      const label = short(splitPath(b.path).name);
      return { b, y, ...place(b.id * 7 + 3, y, label), label, hope: 1 - 0.7 * (y - ground) / Math.max(1, H - ground) };
    });
    const ticks = ([[t("hz.week"), 7 * 86400e3], [t("hz.day"), 86400e3], [t("hz.hour"), 3600e3]] as const).map(([label, ms]) => ({ t: label, y: skyY(ms) }));
    return { souls, stones, ticks };
  }, [items, graves, W, H, ground]);

  const isOn = (kind: string, id: number) => focus?.kind === kind && (focus.kind === "soul" ? focus.it.id : focus.b.id) === id;

  return (
    <>
      <div className="scene" ref={box} onClick={(e) => { if (e.target === e.currentTarget || (e.target as Element).tagName === "svg") setFocus(null); }}
        onKeyDown={(e) => { if (e.key === "Escape") setFocus(null); }}>
        <div className="sky" style={{ height: ground }} />
        <div className="earth" style={{ top: ground }} />
        <div className="horizon" style={{ top: ground - 1 }} />
        <div className="cremation" />
        <svg width={W} height={H} role="group" aria-label={t("hz.scene")}>
          {marks.ticks.map((tick) => (
            <g key={tick.t} className="tick" transform={`translate(0 ${tick.y})`}><line x1={16} x2={26} /><text className="zone" x={32} y={4}>{t("hz.left", tick.t)}</text></g>
          ))}
          <text className="zone strong" x={16} y={ground - 9}>{t("hz.horizon")}</text>
          <text className="zone" x={16} y={ground + 20}>{t("hz.justBuried")}</text>
          <text className="zone strong" x={16} y={H - 34}>{t("hz.cremation")}</text>
          {marks.souls.length === 0 ? <text className="zone" x={W / 2} y={ground / 2} textAnchor="middle">{t("hz.skyEmpty")}</text> : null}
          {marks.stones.length === 0 ? <text className="zone" x={W / 2} y={ground + (H - ground) / 2} textAnchor="middle">{t("hz.groundEmpty")}</text> : null}
          {marks.souls.map(({ it, x, y, label, labeled, soon }) => (
            <g key={`s${it.id}`} className={`soul${soon ? " soon" : ""}${isOn("soul", it.id) ? " on" : ""}`} transform={`translate(${x} ${y})`}
              tabIndex={0} role="button" aria-label={t("hz.soulLabel", label, +new Date(it.expiresAt) > Date.now() ? duration(+new Date(it.expiresAt) - Date.now()) : null)}
              onClick={() => setFocus({ kind: "soul", it, x, y })} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); setFocus({ kind: "soul", it, x, y }); } }}>
              <title>{label}</title><circle className="halo" r={9} /><circle className="core" r={4} />{labeled ? <text x={13} y={4}>{label}</text> : null}
            </g>
          ))}
          {marks.stones.map(({ b, x, y, label, labeled, hope }) => (
            <g key={`g${b.id}`} className={`grave${isOn("grave", b.id) ? " on" : ""}`} transform={`translate(${x} ${y})`}
              tabIndex={0} role="button" aria-label={t("hz.graveLabel", label, duration(+new Date(b.purgeAt) - Date.now()))}
              onClick={() => setFocus({ kind: "grave", b, x, y })} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); setFocus({ kind: "grave", b, x, y }); } }}>
              <path className="star" style={{ opacity: hope }} d="M0 -21 L1.6 -16.6 L6 -15 L1.6 -13.4 L0 -9 L-1.6 -13.4 L-6 -15 L-1.6 -16.6 Z" />
              <path className="stone" d="M-6 7 V-2 L-2.5 -6 H2.5 L6 -2 V7 Z" />
              <title>{label}</title>{labeled ? <text x={13} y={5}>{label}</text> : null}
            </g>
          ))}
        </svg>
        {focus ? <FateCard focus={focus} w={W} h={H} onClose={() => setFocus(null)} /> : null}
      </div>
      <div className="page">
        <div className="below">
          <Seg label={t("hz.list")} value={view} onChange={setView} options={[["sky", t("hz.above", items.length)], ["ground", t("hz.below", graves.length)]]} />
          <span className="hint">{t("hz.hint")}</span>
        </div>
        {view === "sky" ? <Upcoming embedded /> : <Graveyard embedded />}
      </div>
    </>
  );
}

function FateCard({ focus, w, h, onClose }: { focus: Focus; w: number; h: number; onClose: () => void }) {
  const api = useApi();
  const { bump } = useRefresh();
  const confirm = useConfirm();
  const report = useBatch();
  const [span, setSpan] = useState("1d");
  const path = focus.kind === "soul" ? focus.it.path : focus.b.path;
  const { name, folder } = splitPath(path);
  const left = Math.min(Math.max(8, focus.x + 20), w - 308);
  const top = Math.min(Math.max(8, focus.y - 70), h - 190);
  const finish = () => { onClose(); bump(); };

  return (
    <div className="fate" style={{ left, top }} role="dialog" aria-label={name}>
      <h4>{name}</h4>
      {folder ? <div className="name-sub">{folder}</div> : null}
      {focus.kind === "soul" ? (
        <>
          <p>{t("hz.soulText", +new Date(focus.it.expiresAt) > Date.now() ? duration(+new Date(focus.it.expiresAt) - Date.now()) : null, when(focus.it.expiresAt), focus.it.note ?? "")}</p>
          <div className="fate-actions">
            <select className="select" aria-label={t("howLong")} value={span} onChange={(e) => setSpan(e.target.value)}>
              {extendSpans().map(([v, label]) => <option key={v} value={v}>{t("giveMore", label)}</option>)}
            </select>
            <Button kind="primary" icon={TimerReset} onClick={async () => { report(await api.Extend([`#${focus.it.id}`], span), t("extend")); finish(); }}>{t("extend")}</Button>
            <Button kind="danger" icon={EyeOff} onClick={async () => {
              if (!await confirm({ title: t("ask.cancel", 1), body: <ul><li>{path}</li></ul>, ok: t("stopTracking"), danger: true })) return;
              report(await api.Cancel([`#${focus.it.id}`]), t("stopTracking")); finish();
            }}>{t("stopTracking")}</Button>
            <Button kind="ghost" icon={X} aria-label={t("close")} onClick={onClose} />
          </div>
        </>
      ) : (
        <>
          <p>{t("hz.graveText", when(focus.b.buriedAt), focus.b.isDir, bytes(focus.b.size), duration(+new Date(focus.b.purgeAt) - Date.now()), when(focus.b.purgeAt))}</p>
          <div className="fate-actions">
            <Button kind="gold" icon={Sparkles} onClick={async () => { report(await api.Resurrect([`#${focus.b.id}`]), t("resurrect")); finish(); }}>{t("resurrect")}</Button>
            <Button kind="danger" icon={Flame} onClick={async () => {
              if (!await confirm({ title: t("ask.purge", 1), body: <><p>{t("ask.purgeBody")}</p><ul><li>{path}</li></ul></>, ok: t("purge"), danger: true })) return;
              report(await api.Purge([`#${focus.b.id}`]), t("purge")); finish();
            }}>{t("purge")}</Button>
            <Button kind="ghost" icon={X} aria-label={t("close")} onClick={onClose} />
          </div>
        </>
      )}
    </div>
  );
}
