// H 长明: a vigil. Night, hairlines, one light: every file keeps a small gold
// lamp whose glow is the life it has left. The list is the row of lamps; the
// right side holds one of them up close, its time set large in a light serif.
// Extending a life brightens the lamp; resurrecting flares it.
import { RefreshCw, Search } from "lucide-react";
import { useEffect, useRef, useState, type CSSProperties, type KeyboardEvent, type ReactNode } from "react";
import { bytes, graveSize } from "../../format";
import Decisions from "../../pages/Decisions";
import Logs from "../../pages/Logs";
import Settings from "../../pages/Settings";
import { t } from "../../i18n";
import { pageTitle, type ShellProps } from "../../shell";
import { extendSpans, folderOf, hm, md, nameOf, parts, pause, remaining, sourceLabel, span, Star, useGraves, useItems, type Burial, type Item } from "../shared";
import "./vigil.css";

export default function VigilShell({ page, nav, setPage, summary: s, refresh }: ShellProps) {
  return (
    <div className="vg">
      <header className="vg-top">
        <div className="vg-mark"><Star size={14} />{t("brand")}</div>
        <nav className="vg-nav" aria-label={t("pagesLabel")}>
          {nav.map((id) => (
            <button key={id} type="button" aria-current={page === id ? "page" : undefined} onClick={() => setPage(id)}>
              {pageTitle(id)}{id === "decisions" && s?.decisionCount ? <span className="vg-count">{s.decisionCount}</span> : null}
            </button>
          ))}
        </nav>
        <span className="vg-grow" />
        {s ? <div className="vg-sum"><span>{t("vg.lit", s.trackedCount)}</span><span>{t("vg.graves", s.graveCount, bytes(s.graveBytes))}</span>{s.paused ? <span>{t("paused")}</span> : null}</div> : null}
        <button type="button" className="vg-icon" aria-label={t("refresh")} title={t("refresh")} onClick={refresh}><RefreshCw size={15} aria-hidden /></button>
      </header>
      <main className="vg-main" key={page}>
        {page === "graveyard" ? <Graves /> : page === "upcoming" ? <Upcoming /> : (
          <div className="vg-shared">{page === "decisions" ? <Decisions /> : page === "settings" ? <Settings /> : <Logs />}</div>
        )}
      </main>
    </div>
  );
}

// ── Pieces ─────────────────────────────────────────────────────────────

const lit = (life: number) => ({ "--life": life.toFixed(3) }) as CSSProperties;

function Lamp({ life, soon }: { life: number; soon?: boolean }) {
  return <span className="vg-lamp" data-soon={soon} style={lit(life)} aria-hidden />;
}

// Arrow keys walk the row of lamps.
function walk(e: KeyboardEvent<HTMLElement>) {
  if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
  const rows = [...e.currentTarget.querySelectorAll<HTMLButtonElement>(".vg-row-main")];
  const i = rows.indexOf(document.activeElement as HTMLButtonElement);
  const next = rows[Math.min(rows.length - 1, Math.max(0, i + (e.key === "ArrowDown" ? 1 : -1)))];
  if (next) { e.preventDefault(); next.focus(); next.click(); }
}

function Row({ name, folder, life, soon, active, on, onPick, onToggle, time }: {
  name: string; folder: string; life: number; soon: boolean; active: boolean; on: boolean; onPick: () => void; onToggle: () => void; time: ReactNode;
}) {
  return (
    <li className="vg-row" data-active={active} data-on={on}>
      <input type="checkbox" className="vg-check" aria-label={t("select", name)} checked={on} onChange={onToggle} />
      <button type="button" className="vg-row-main" aria-current={active ? "true" : undefined} onClick={onPick}>
        <Lamp life={life} soon={soon} />
        <span className="vg-row-text"><b title={name}>{name}</b><small>{folder}</small></span>
        <span className="vg-row-time">{time}</span>
      </button>
    </li>
  );
}

function ListHead({ title, deck, children }: { title: string; deck: ReactNode; children?: ReactNode }) {
  return (
    <div className="vg-list-head">
      <h1>{title}</h1>
      <p>{deck}</p>
      {children}
    </div>
  );
}

function Quiet({ title, text }: { title: string; text: string }) {
  return (
    <div className="vg-quiet">
      <Lamp life={0} />
      <h2>{title}</h2>
      <p>{text}</p>
    </div>
  );
}

const Loading = () => <ul className="vg-rows" aria-busy="true" aria-label={t("loading")}>{Array.from({ length: 5 }, (_, i) => <li key={i} className="vg-ghost" />)}</ul>;

function Figure({ ms, life, flare, children }: { ms: number; life: number; flare?: boolean; children?: ReactNode }) {
  const p = parts(ms);
  return (
    <div className="vg-figure-wrap" data-flare={flare} style={lit(life)}>
      <span className="vg-halo" aria-hidden />
      <div className="vg-figure">{ms > 0 ? <><b>{p.n}</b><span>{p.unit}</span></> : <b className="out">{t("vg.out")}</b>}</div>
      {children}
    </div>
  );
}

// Keep the active item valid as the list changes.
function useActive<T extends { id: number }>(list: T[]) {
  const [id, setId] = useState<number | null>(null);
  const current = list.find((x) => x.id === id) ?? list[0];
  return [current, setId] as const;
}

// ── 墓地 ───────────────────────────────────────────────────────────────

function Graves() {
  const [filter, setFilter] = useState("");
  const g = useGraves(filter);
  const graves = [...g.graves].sort((a, b) => +new Date(a.purgeAt) - +new Date(b.purgeAt));
  const [active, setActive] = useActive(graves);
  const [flare, setFlare] = useState(false);
  const chosen = graves.filter((b) => g.sel.has(b.id));
  const glow = async () => { setFlare(true); await pause(700); setFlare(false); };

  return (
    <div className="vg-split">
      <section className="vg-list" aria-label={t("page.graveyard")}>
        <ListHead title={t("head.graveyard")} deck={g.graves.length ? t("vg.deck", g.graves.length, bytes(g.bytes)) : t("vg.deckEmpty")}>
          <label className="vg-search"><Search size={14} aria-hidden /><input type="search" placeholder={t("filterByPath")} aria-label={t("filterByPath")} value={filter} onChange={(e) => setFilter(e.target.value)} /></label>
        </ListHead>
        {g.error ? <p className="vg-alert">{t("grave.readError", g.error)}</p> : null}
        {g.pending.length ? <p className="vg-alert">{t("grave.pending", g.pending.length)}</p> : null}
        {g.loading && !g.data ? <Loading /> : (
          <ul className="vg-rows" onKeyDown={walk}>
            {graves.map((b) => {
              const { ms, left } = remaining(b.buriedAt, b.purgeAt);
              const p = parts(ms);
              return (
                <Row key={b.id} name={nameOf(b.path)} folder={folderOf(b.path)} life={left} soon={ms < 86400e3}
                  active={active?.id === b.id && !chosen.length} on={g.sel.has(b.id)} onPick={() => setActive(b.id)} onToggle={() => g.sel.toggle(b.id)}
                  time={ms > 0 ? <><b>{p.n}</b>{p.unit}</> : t("vg.due")} />
              );
            })}
          </ul>
        )}
        {graves.length > 1 ? <button type="button" className="vg-all" onClick={g.sel.toggleAll}>{g.sel.all ? t("selectNone") : t("selectAll")}</button> : null}
      </section>
      <section className="vg-detail" aria-live="polite">
        {chosen.length > 1 ? (
          <Batch count={chosen.length} unit={t("vg.unitGraves", chosen.length)} names={chosen.map((b) => b.path)} note={t("vg.graveBatch")} onClear={g.sel.clear}>
            <button type="button" className="vg-btn gold" onClick={() => g.resurrect(undefined, glow)}><Star size={13} />{t("vg.resurrectN", chosen.length)}</button>
            <button type="button" className="vg-btn ash" onClick={() => g.purge()}>{t("purge")}</button>
          </Batch>
        ) : (chosen[0] ?? active) ? (
          <GraveDetail b={chosen[0] ?? active!} flare={flare} onResurrect={() => g.resurrect([(chosen[0] ?? active!).id], glow)} onPurge={() => g.purge([(chosen[0] ?? active!).id])} />
        ) : g.loading ? null : filter ? <Quiet title={t("vg.noLamp")} text={t("notInPath", filter)} /> : <Quiet title={t("vg.noLamps")} text={t("vg.noLampsText")} />}
      </section>
    </div>
  );
}

function GraveDetail({ b, flare, onResurrect, onPurge }: { b: Burial; flare: boolean; onResurrect: () => void; onPurge: () => void }) {
  const { ms, left } = remaining(b.buriedAt, b.purgeAt);
  return (
    <div className="vg-card" key={b.id}>
      <Figure ms={ms} life={left} flare={flare}>
        <p className="vg-sub">{ms > 0 ? t("vg.graveSub", `${md(b.purgeAt)} ${hm(b.purgeAt)}`) : t("vg.graveSubOut")}</p>
      </Figure>
      <h2 className="vg-name" title={b.path}>{nameOf(b.path)}</h2>
      <p className="vg-path">{folderOf(b.path)}</p>
      <dl className="vg-facts">
        <dt>{t("vg.fact.buried")}</dt><dd>{md(b.buriedAt)} {hm(b.buriedAt)}</dd>
        <dt>{t("vg.fact.size")}</dt><dd>{b.isDir ? t("vg.folder") : ""}{graveSize(b)}</dd>
        {b.note ? <><dt>{t("vg.fact.note")}</dt><dd>{b.note}</dd></> : null}
      </dl>
      <div className="vg-actions">
        <button type="button" className="vg-btn gold" onClick={onResurrect}><Star size={13} />{t("resurrect")}</button>
        <button type="button" className="vg-btn ash" onClick={onPurge}>{t("purge")}</button>
      </div>
    </div>
  );
}

function Batch({ count, unit, names, note, onClear, children }: { count: number; unit: string; names: string[]; note: string; onClear: () => void; children: ReactNode }) {
  return (
    <div className="vg-card">
      <div className="vg-figure-wrap" style={lit(1)}>
        <span className="vg-halo" aria-hidden />
        <div className="vg-figure"><b>{count}</b><span>{unit}</span></div>
        <p className="vg-sub">{t("vg.selected", note)}</p>
      </div>
      <ul className="vg-names">{names.slice(0, 6).map((p) => <li key={p}>{nameOf(p)}<small>{folderOf(p)}</small></li>)}</ul>
      {names.length > 6 ? <p className="vg-more">{t("vg.more", names.length - 6)}</p> : null}
      <div className="vg-actions">{children}<button type="button" className="vg-btn quiet" onClick={onClear}>{t("clearSelection")}</button></div>
    </div>
  );
}

// ── 即将下葬 ───────────────────────────────────────────────────────────

type Range = "" | "24h" | "7d";

function Upcoming() {
  const [range, setRange] = useState<Range>("");
  const it = useItems(range);
  const items = it.items;
  const [active, setActive] = useActive(items);
  const chosen = items.filter((x) => it.sel.has(x.id));
  const n = items.length;
  return (
    <div className="vg-split">
      <section className="vg-list" aria-label={t("page.upcoming")}>
        <ListHead title={t("head.upcoming")} deck={n ? t("vg.upDeck", n) : t("vg.upDeckEmpty")}>
          <div className="vg-tabs" role="group" aria-label={t("range.label")}>
            {([["24h", t("range.24h")], ["7d", t("range.7d")], ["", t("range.all")]] as [Range, string][]).map(([v, label]) => (
              <button key={v} type="button" aria-pressed={range === v} onClick={() => { setRange(v); it.sel.clear(); }}>{label}</button>
            ))}
          </div>
        </ListHead>
        {it.error ? <p className="vg-alert">{t("up.readError", it.error)}</p> : null}
        {it.loading && !it.data ? <Loading /> : (
          <ul className="vg-rows" onKeyDown={walk}>
            {items.map((x) => {
              const { ms, left } = remaining(x.createdAt, x.expiresAt);
              const p = parts(ms);
              return (
                <Row key={x.id} name={nameOf(x.path)} folder={folderOf(x.path)} life={left} soon={ms > 0 && ms < 3600e3}
                  active={active?.id === x.id && !chosen.length} on={it.sel.has(x.id)} onPick={() => setActive(x.id)} onToggle={() => it.sel.toggle(x.id)}
                  time={ms > 0 ? <><b>{p.n}</b>{p.unit}</> : t("vg.expired")} />
              );
            })}
          </ul>
        )}
        {n > 1 ? <button type="button" className="vg-all" onClick={it.sel.toggleAll}>{it.sel.all ? t("selectNone") : t("selectAll")}</button> : null}
      </section>
      <section className="vg-detail" aria-live="polite">
        {chosen.length > 1 ? (
          <Batch count={chosen.length} unit={t("vg.unitLamps", chosen.length)} names={chosen.map((x) => x.path)} note={t("vg.itemBatch")} onClear={it.sel.clear}>
            <Oil onPick={(v) => it.extend(v)} />
            <button type="button" className="vg-btn ash" onClick={() => it.cancel()}>{t("stopTracking")}</button>
          </Batch>
        ) : (chosen[0] ?? active) ? (
          <ItemDetail x={chosen[0] ?? active!} onExtend={(v) => it.extend(v, [(chosen[0] ?? active!).id])} onCancel={() => it.cancel([(chosen[0] ?? active!).id])} />
        ) : it.loading ? null : <Quiet title={t("vg.noLit")} text={range ? t("up.quietRange") : t("vg.noLitAll")} />}
      </section>
    </div>
  );
}

// Brighten the lamp when its life grows, so the change is felt, not read.
function useBrighten(life: number) {
  const last = useRef(life);
  const [up, setUp] = useState(false);
  useEffect(() => {
    if (life > last.current + 0.001) { setUp(true); const t = setTimeout(() => setUp(false), 900); last.current = life; return () => clearTimeout(t); }
    last.current = life;
  }, [life]);
  return up;
}

function ItemDetail({ x, onExtend, onCancel }: { x: Item; onExtend: (v: string) => void; onCancel: () => void }) {
  const { ms, left } = remaining(x.createdAt, x.expiresAt);
  const up = useBrighten(left);
  return (
    <div className="vg-card" key={x.id}>
      <Figure ms={ms} life={left} flare={up}>
        <p className="vg-sub">{ms > 0 ? t("vg.itemSub", `${md(x.expiresAt)} ${hm(x.expiresAt)}`) : t("life.expired")}</p>
      </Figure>
      <h2 className="vg-name" title={x.path}>{nameOf(x.path)}</h2>
      <p className="vg-path">{folderOf(x.path)}</p>
      <dl className="vg-facts">
        <dt>{t("vg.fact.age")}</dt><dd>{span(+new Date(x.expiresAt) - +new Date(x.createdAt))}</dd>
        <dt>{t("vg.fact.registered")}</dt><dd>{t("vg.registeredAt", `${md(x.createdAt)} ${hm(x.createdAt)}`, sourceLabel(x.source))}</dd>
        {x.note ? <><dt>{t("vg.fact.note")}</dt><dd>{x.note}</dd></> : null}
        {x.lastError ? <><dt>{t("vg.fact.error")}</dt><dd>{x.lastError}</dd></> : null}
      </dl>
      <div className="vg-actions">
        <Oil onPick={onExtend} />
        <button type="button" className="vg-btn ash" onClick={onCancel}>{t("stopTracking")}</button>
      </div>
    </div>
  );
}

// Extending is adding oil to the lamp: one press per amount.
function Oil({ onPick }: { onPick: (v: string) => void }) {
  return (
    <div className="vg-oil" role="group" aria-label={t("extend")}>
      <span>{t("extend")}</span>
      {extendSpans().map(([v, label]) => <button key={v} type="button" onClick={() => onPick(v)}>{label}</button>)}
    </div>
  );
}
