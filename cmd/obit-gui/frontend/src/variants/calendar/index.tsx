// I 日历: a calendar of fates. Each column is a day; every file sits on the
// day something happens to it: tracked files on the day they are buried,
// graves on the day they are cremated. A band across the day heads runs
// cobalt to gold, time passing left to right. Click any non-empty day header
// to toggle selecting that entire day; drag a tracked file to a later day to
// extend it to that day.
import { Archive, CalendarDays, Check, EyeOff, Flame, Minus, RefreshCw, Search, TimerReset, X } from "lucide-react";
import { useState, type DragEvent, type ReactNode } from "react";
import { bytes, graveSize, noteText } from "../../format";
import Decisions from "../../pages/Decisions";
import Logs from "../../pages/Logs";
import Settings from "../../pages/Settings";
import { t, useTrackingHint } from "../../i18n";
import Notices from "../../notices";
import { pageTitle, type ShellProps } from "../../shell";
import { DAY, extendSpans, hm, md, nameOf, remaining, sourceLabel, span, Star, useGraves, useItems, weekday } from "../shared";
import { columnTemplate } from "./columns";
import { columnSelectionState, needsBuryConfirm, toggleColumnSelection, UNDO_TOAST_MS } from "./selection";
import "./calendar.css";

const startOfDay = (t: number) => { const d = new Date(t); d.setHours(0, 0, 0, 0); return +d; };
const dayIndex = (iso: string) => Math.max(0, Math.round((startOfDay(+new Date(iso)) - startOfDay(Date.now())) / DAY));
const DAYS = 8;

export default function CalendarShell({ page, nav, setPage, summary: s, refresh }: ShellProps) {
  const today = new Date();
  return (
    <div className="cal">
      <header className="cal-top">
        <div className="cal-brand"><img src="icon.png" alt="" />{t("brand")}</div>
        <nav className="cal-tabs" aria-label={t("pagesLabel")}>
          {nav.map((id) => (
            <button key={id} type="button" aria-current={page === id ? "page" : undefined} onClick={() => setPage(id)}>
              {pageTitle(id)}{id === "decisions" && s?.decisionCount ? <span className="cal-count">{s.decisionCount}</span> : null}
            </button>
          ))}
        </nav>
        <span className="cal-grow" />
        <Notices page={page} setPage={setPage} summary={s} refresh={refresh} />
        <div className="cal-today"><CalendarDays size={15} aria-hidden /><span>{t("date.md", today.getMonth() + 1, today.getDate())}</span><span>{weekday(today)}</span></div>
        <button type="button" className="cal-icon" aria-label={t("refresh")} title={t("refresh")} onClick={refresh}><RefreshCw size={15} aria-hidden /></button>
      </header>
      <main className="cal-main" key={page}>
        {page === "graveyard" ? <Graves /> : page === "upcoming" ? <Upcoming /> : (
          <div className="cal-shared">{page === "decisions" ? <Decisions /> : page === "settings" ? <Settings /> : <Logs />}</div>
        )}
      </main>
    </div>
  );
}

// ── The board ──────────────────────────────────────────────────────────

interface Entry { id: number; at: string; path: string; name: string; meta: string; soon: boolean; note?: string }

function Board({ entries, chosenSet, has, toggle, toggleColumn, onMove, empty, capsule }: {
  entries: Entry[];
  chosenSet: ReadonlySet<number>;
  has: (id: number) => boolean;
  toggle: (id: number) => void;
  toggleColumn: (colIds: number[]) => void;
  empty: ReactNode;
  capsule?: ReactNode;
  onMove?: (id: number, from: number, to: number) => void;
}) {
  const [over, setOver] = useState<number | null>(null);
  const [drag, setDrag] = useState<{ id: number; from: number } | null>(null);
  const later = entries.filter((e) => dayIndex(e.at) >= DAYS);
  const cols = DAYS + (later.length ? 1 : 0);
  const now = Date.now();

  const dropProps = (i: number) => (onMove && i < DAYS ? {
    onDragOver: (e: DragEvent) => { if (drag && i > drag.from) { e.preventDefault(); setOver(i); } },
    onDragLeave: () => setOver((o) => (o === i ? null : o)),
    onDrop: (e: DragEvent) => { e.preventDefault(); setOver(null); if (drag && i > drag.from) onMove(drag.id, drag.from, i); setDrag(null); },
  } : {});

  return (
    <div className="cal-board" data-selecting={chosenSet.size > 0}>
      <div className="cal-cols" style={{ gridTemplateColumns: columnTemplate(cols) }}>
        {Array.from({ length: cols }, (_, i) => {
          const date = new Date(startOfDay(now) + i * DAY);
          const isLater = i === DAYS;
          const list = isLater ? later : entries.filter((e) => dayIndex(e.at) === i).sort((a, b) => +new Date(a.at) - +new Date(b.at));
          const colIds = list.map((e) => e.id);
          const colInfo = columnSelectionState(colIds, chosenSet);
          const isEmpty = colInfo.state === "empty";
          const weekend = !isLater && (date.getDay() === 0 || date.getDay() === 6);
          const dayLabel = isLater ? t("cal.later") : i === 0 ? t("cal.today") : i === 1 ? t("cal.tomorrow") : t("date.md", date.getMonth() + 1, date.getDate());
          const headAria = isEmpty
            ? t("cal.colEmpty", dayLabel)
            : colInfo.state === "all"
              ? t("cal.colDeselect", dayLabel, colInfo.total)
              : colInfo.state === "partial"
                ? t("cal.colPartial", dayLabel, colInfo.chosen, colInfo.total)
                : t("cal.colSelect", dayLabel, colInfo.total);
          return (
            <section key={i} className="cal-day" data-today={i === 0} data-weekend={weekend} data-over={over === i}
              data-target={drag !== null && i > drag.from && i < DAYS} style={{ "--i": i, "--n": cols } as React.CSSProperties} {...dropProps(i)}
              aria-label={isLater ? t("cal.later") : t("date.md", date.getMonth() + 1, date.getDate())}>
              <header className="cal-day-head-wrap">
                <button
                  type="button"
                  className="cal-day-head"
                  disabled={isEmpty}
                  data-sel={colInfo.state}
                  aria-pressed={colInfo.state === "all" ? true : colInfo.state === "partial" ? "mixed" : false}
                  aria-label={headAria}
                  title={isEmpty ? undefined : headAria}
                  onClick={() => toggleColumn(colIds)}
                >
                  <span className="cal-day-label">
                    {isLater ? <b className="later">{t("cal.later")}</b> : <b>{date.getDate()}</b>}
                    {isLater && colInfo.chosen > 0 ? null : (
                      <span>{isLater ? t("cal.count", later.length) : i === 0 ? t("cal.today") : i === 1 ? t("cal.tomorrow") : weekday(date)}</span>
                    )}
                  </span>
                  {!isEmpty ? (
                    <span className="cal-day-badge" aria-hidden>
                      {colInfo.state === "all" ? <Check size={11} strokeWidth={3} /> : colInfo.state === "partial" ? <Minus size={11} strokeWidth={3} /> : null}
                      {colInfo.chosen > 0 ? (
                        <em className="cal-day-badge-n">
                          {isLater && colInfo.state === "partial" ? `${colInfo.chosen}/${colInfo.total}` : colInfo.chosen}
                        </em>
                      ) : null}
                    </span>
                  ) : null}
                </button>
              </header>
              <div className="cal-day-body">
                {list.map((e) => (
                  <button key={e.id} type="button" className="cal-chip" data-on={has(e.id)} data-soon={e.soon} aria-pressed={has(e.id)}
                    title={e.path}
                    draggable={!!onMove} onClick={() => toggle(e.id)}
                    onDragStart={(ev) => { ev.dataTransfer.effectAllowed = "move"; ev.dataTransfer.setData("text/plain", String(e.id)); setDrag({ id: e.id, from: Math.min(DAYS - 1, dayIndex(e.at)) }); }}
                    onDragEnd={() => { setDrag(null); setOver(null); }}>
                    <span className="cal-time">{isLater ? md(e.at) : e.soon ? t("cal.in", span(+new Date(e.at) - now)) : hm(e.at)}</span>
                    <Name name={e.name} />
                    <span className="cal-meta">{e.meta.split(/(?<=[，,] ?)/).map((part, n) => <span key={n}>{part}</span>)}</span>
                    {e.note ? <span className="cal-note">{e.note}</span> : null}
                  </button>
                ))}
                {over === i && drag ? <div className="cal-drop">{t("cal.dropTo", t("date.md", date.getMonth() + 1, date.getDate()))}</div> : null}
              </div>
            </section>
          );
        })}
      </div>
      {entries.length === 0 ? <div className="cal-empty">{empty}</div> : null}
      {capsule}
    </div>
  );
}

function CapsuleBar({ count, onClear, children }: { count: number; onClear: () => void; children: ReactNode }) {
  if (!count) return null;
  return (
    <div className="cal-capsule" role="region" aria-label={t("sel.label")}>
      <span className="cal-cap-count" aria-live="polite">{t("sel.count", count)}</span>
      <span className="cal-cap-sep" aria-hidden />
      {children}
      <span className="cal-cap-sep" aria-hidden />
      <button type="button" className="cal-cap-btn ghost" onClick={onClear}><X size={14} aria-hidden />{t("clear")}</button>
    </div>
  );
}

// Cut so the last few characters and the extension survive the ellipsis:
// "screenshot-che…kout.png". The card's tooltip has the full path.
function Name({ name }: { name: string }) {
  const dot = name.lastIndexOf(".");
  const keep = Math.max(8, dot > 0 ? name.length - dot + 4 : 0);
  const cut = name.length > keep + 4 ? name.length - keep : name.length;
  return <span className="cal-name"><span>{name.slice(0, cut)}</span><span>{name.slice(cut)}</span></span>;
}

function Head({ title, deck, children }: { title: string; deck: ReactNode; children?: ReactNode }) {
  return (
    <div className="cal-head">
      <div className="cal-head-text"><h1>{title}</h1><p>{deck}</p></div>
      <div className="cal-actions">{children}</div>
    </div>
  );
}

const Loading = () => <div className="cal-board loading" aria-busy="true" aria-label={t("loading")}><div className="cal-cols" style={{ gridTemplateColumns: columnTemplate(DAYS) }}>{Array.from({ length: DAYS }, (_, i) => <section key={i} className="cal-day"><header className="cal-day-head-wrap"><div className="cal-day-head"><span className="cal-day-label"><b>&nbsp;</b><span>&nbsp;</span></span></div></header><div className="cal-day-body"><i /><i /></div></section>)}</div></div>;

// When it was buried, as people remember it: today, yesterday, or a date.
function buriedDay(iso: string) {
  const days = Math.round((startOfDay(Date.now()) - startOfDay(+new Date(iso))) / DAY);
  if (days <= 0) return t("cal.today");
  if (days === 1) return t("cal.yesterday");
  const d = new Date(iso);
  return t("cal.day", d.getMonth() + 1, d.getDate());
}

// ── 墓地: the cremation calendar ──────────────────────────────────────

function Graves() {
  const [filter, setFilter] = useState("");
  const [lifespan, setLifespan] = useState("1d");
  const g = useGraves(filter);
  const n = g.graves.length;
  const entries: Entry[] = g.graves.map((b) => {
    const { ms } = remaining(b.buriedAt, b.purgeAt);
    return { id: b.id, at: b.purgeAt, path: b.path, name: nameOf(b.path), meta: t("cal.graveMeta", buriedDay(b.buriedAt), `${b.isDir ? t("cal.folder") : ""}${graveSize(b)}`), soon: ms > 0 && ms < 6 * 3600e3, note: noteText(b.note) };
  });
  const k = g.sel.chosen.length;
  const toggleCol = (colIds: number[]) => g.sel.update((cur) => toggleColumnSelection(cur, colIds));

  const capsule = (
    <CapsuleBar count={k} onClear={g.sel.clear}>
      <button type="button" className="cal-cap-btn gold" onClick={() => g.resurrect()}><Star size={13} />{t("resurrect")}</button>
      {g.canExtend ? (
        <span className="cal-cap-group">
          <select className="cal-cap-select" aria-label={t("howLong")} value={lifespan} onChange={(e) => setLifespan(e.target.value)}>
            {extendSpans().map(([v, label]) => <option key={v} value={v}>{t("giveMore", label)}</option>)}
          </select>
          <button type="button" className="cal-cap-btn cobalt" onClick={() => g.extend(lifespan)}><TimerReset size={14} aria-hidden />{t("extend")}</button>
        </span>
      ) : null}
      <button type="button" className="cal-cap-btn danger" onClick={() => g.purge(g.sel.chosen, undefined, { showTotalSize: true })}><Flame size={14} aria-hidden />{t("purgeNow")}</button>
    </CapsuleBar>
  );

  return (
    <div className="cal-page">
      <Head title={t("head.graveyard")} deck={n ? t("cal.graveDeck", n, bytes(g.bytes)) : t("cal.graveDeckEmpty")}>
        <span className="cal-hint" data-hidden={k > 0}>{t("cal.pickHint")}</span>
        <label className="cal-search"><Search size={14} aria-hidden /><input type="search" placeholder={t("filterByPath")} aria-label={t("filterByPath")} value={filter} onChange={(e) => setFilter(e.target.value)} /></label>
      </Head>
      {g.error ? <p className="cal-alert">{t("grave.readError", g.error)}</p> : null}
      {g.pending.length ? <p className="cal-alert">{t("grave.pending", g.pending.length)}</p> : null}
      {g.loading && !g.data ? <Loading /> : (
        <Board entries={entries} chosenSet={g.sel.chosenSet} has={g.sel.has} toggle={g.sel.toggle} toggleColumn={toggleCol} capsule={capsule}
          empty={filter ? <><b>{t("grave.noMatch")}</b><span>{t("notInPath", filter)}</span></> : <><b>{t("cal.graveQuiet")}</b><span>{t("cal.graveQuietText")}</span></>} />
      )}
    </div>
  );
}

// ── 即将下葬: the burial calendar ─────────────────────────────────────

function Upcoming() {
  const emptyHint = useTrackingHint("cal.upQuietText");
  const it = useItems("");
  const [lifespan, setLifespan] = useState("1d");
  const now = Date.now();
  const entries: Entry[] = it.items.map((x) => {
    const ms = +new Date(x.expiresAt) - now;
    return { id: x.id, at: x.expiresAt, path: x.path, name: nameOf(x.path), meta: ms <= 0 ? t("cal.expired") : t("cal.meta", sourceLabel(x.source), span(+new Date(x.expiresAt) - +new Date(x.createdAt))), soon: ms > 0 && ms < 3600e3, note: noteText(x.note) };
  });
  const k = it.sel.chosen.length;
  const n = it.items.length;
  const today = it.items.filter((x) => dayIndex(x.expiresAt) === 0).length;
  const laterIds = it.items.filter((x) => dayIndex(x.expiresAt) >= DAYS).map((x) => x.id);
  const toggleCol = (colIds: number[]) => it.sel.update((cur) => toggleColumnSelection(cur, colIds));

  // Dropping on a later day extends by whole days, keeping the time of day.
  function move(id: number, from: number, to: number) {
    const days = to - from;
    const ids = it.sel.has(id) ? it.sel.chosen : [id];
    const one = it.items.find((x) => x.id === id);
    const target = one ? new Date(Math.max(+new Date(one.expiresAt), Date.now()) + days * DAY) : null;
    it.extend(`${days}d`, ids, t("span.days", days), <p>{t("cal.moveBody", ids.length, days, target && ids.length === 1 ? `${md(target.toISOString())} ${hm(target.toISOString())}` : null)}</p>);
  }

  function burySelected() {
    const askConfirm = needsBuryConfirm(it.sel.chosen, laterIds);
    void it.bury(it.sel.chosen, { askConfirm, undoDurationMs: UNDO_TOAST_MS });
  }

  const capsule = (
    <CapsuleBar count={k} onClear={it.sel.clear}>
      {it.canBury ? (
        <button type="button" className="cal-cap-btn bury" onClick={burySelected}><Archive size={14} aria-hidden />{t("buryNow")}</button>
      ) : null}
      <span className="cal-cap-group">
        <select className="cal-cap-select" aria-label={t("howLong")} value={lifespan} onChange={(e) => setLifespan(e.target.value)}>
          {extendSpans().map(([v, label]) => <option key={v} value={v}>{t("giveMore", label)}</option>)}
        </select>
        <button type="button" className="cal-cap-btn cobalt" onClick={() => it.extend(lifespan)}><TimerReset size={14} aria-hidden />{t("extend")}</button>
      </span>
      <button type="button" className="cal-cap-btn" onClick={() => it.cancel()}><EyeOff size={14} aria-hidden />{t("stopTracking")}</button>
    </CapsuleBar>
  );

  return (
    <div className="cal-page">
      <Head title={t("head.upcoming")} deck={n ? t("cal.upDeck", n, today) : t("cal.upDeckEmpty")}>
        <span className="cal-hint" data-hidden={k > 0}>{t("cal.pickHint")}</span>
      </Head>
      {it.error ? <p className="cal-alert">{t("up.readError", it.error)}</p> : null}
      {it.loading && !it.data ? <Loading /> : (
        <Board entries={entries} chosenSet={it.sel.chosenSet} has={it.sel.has} toggle={it.sel.toggle} toggleColumn={toggleCol} onMove={move} capsule={capsule}
          empty={<><b>{t("cal.upQuiet")}</b><span>{emptyHint}</span></>} />
      )}
    </div>
  );
}
