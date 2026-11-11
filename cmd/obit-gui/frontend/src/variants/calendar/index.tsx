// I 日历: a calendar of fates. Each column is a day; every file sits on the
// day something happens to it: tracked files on the day they are buried,
// graves on the day they are cremated. A band across the day heads runs
// cobalt to gold, time passing left to right. Drag a tracked file to a later
// day to extend it to that day.
import { CalendarDays, EyeOff, Flame, RefreshCw, Search, TimerReset } from "lucide-react";
import { useState, type DragEvent, type ReactNode } from "react";
import { bytes, graveSize } from "../../format";
import Decisions from "../../pages/Decisions";
import Logs from "../../pages/Logs";
import Settings from "../../pages/Settings";
import { t } from "../../i18n";
import { pageTitle, type ShellProps } from "../../shell";
import { DAY, extendSpans, hm, md, nameOf, remaining, sourceLabel, span, Star, useGraves, useItems, weekday } from "../shared";
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

function Board({ entries, has, toggle, onMove, empty }: {
  entries: Entry[]; has: (id: number) => boolean; toggle: (id: number) => void; empty: ReactNode;
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

  // Empty days stay as narrow drop targets; days with files get the room.
  const filled = (i: number) => i === 0 || i === DAYS || entries.some((e) => Math.min(DAYS, dayIndex(e.at)) === i);
  const widths = entries.length ? Array.from({ length: cols }, (_, i) => (filled(i) ? "minmax(0, 1fr)" : "minmax(0, 0.5fr)")).join(" ") : `repeat(${cols}, minmax(0, 1fr))`;
  return (
    <div className="cal-board" style={{ gridTemplateColumns: widths }}>
      {Array.from({ length: cols }, (_, i) => {
        const date = new Date(startOfDay(now) + i * DAY);
        const isLater = i === DAYS;
        const list = isLater ? later : entries.filter((e) => dayIndex(e.at) === i).sort((a, b) => +new Date(a.at) - +new Date(b.at));
        const weekend = !isLater && (date.getDay() === 0 || date.getDay() === 6);
        return (
          <section key={i} className="cal-day" data-today={i === 0} data-weekend={weekend} data-over={over === i}
            data-target={drag !== null && i > drag.from && i < DAYS} style={{ "--i": i, "--n": cols } as React.CSSProperties} {...dropProps(i)}
            aria-label={isLater ? t("cal.later") : t("date.md", date.getMonth() + 1, date.getDate())}>
            <header className="cal-day-head">
              {isLater ? <b className="later">{t("cal.later")}</b> : <b>{date.getDate()}</b>}
              <span>{isLater ? t("cal.count", later.length) : i === 0 ? t("cal.today") : i === 1 ? t("cal.tomorrow") : weekday(date)}</span>
            </header>
            <div className="cal-day-body">
              {list.map((e) => (
                <button key={e.id} type="button" className="cal-chip" data-on={has(e.id)} data-soon={e.soon} aria-pressed={has(e.id)}
                  title={e.path}
                  draggable={!!onMove} onClick={() => toggle(e.id)}
                  onDragStart={(ev) => { ev.dataTransfer.effectAllowed = "move"; ev.dataTransfer.setData("text/plain", String(e.id)); setDrag({ id: e.id, from: Math.min(DAYS - 1, dayIndex(e.at)) }); }}
                  onDragEnd={() => { setDrag(null); setOver(null); }}>
                  <span className="cal-time">{isLater ? md(e.at) : e.soon ? t("cal.in", span(+new Date(e.at) - now)) : hm(e.at)}</span>
                  <span className="cal-name">{e.name}</span>
                  <span className="cal-meta">{e.meta}</span>
                  {e.note ? <span className="cal-note">{e.note}</span> : null}
                </button>
              ))}
              {over === i && drag ? <div className="cal-drop">{t("cal.dropTo", t("date.md", date.getMonth() + 1, date.getDate()))}</div> : null}
            </div>
          </section>
        );
      })}
      {entries.length === 0 ? <div className="cal-empty">{empty}</div> : null}
    </div>
  );
}

function Head({ title, deck, children }: { title: string; deck: ReactNode; children?: ReactNode }) {
  return (
    <div className="cal-head">
      <div className="cal-head-text"><h1>{title}</h1><p>{deck}</p></div>
      <div className="cal-actions">{children}</div>
    </div>
  );
}

const Loading = () => <div className="cal-board loading" aria-busy="true" aria-label={t("loading")}>{Array.from({ length: DAYS }, (_, i) => <section key={i} className="cal-day"><header className="cal-day-head"><b>&nbsp;</b><span>&nbsp;</span></header><div className="cal-day-body"><i /><i /></div></section>)}</div>;

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
  const g = useGraves(filter);
  const n = g.graves.length;
  const entries: Entry[] = g.graves.map((b) => {
    const { ms } = remaining(b.buriedAt, b.purgeAt);
    return { id: b.id, at: b.purgeAt, path: b.path, name: nameOf(b.path), meta: t("cal.graveMeta", buriedDay(b.buriedAt), `${b.isDir ? t("cal.folder") : ""}${graveSize(b)}`), soon: ms > 0 && ms < 6 * 3600e3, note: b.note };
  });
  const k = g.sel.chosen.length;
  return (
    <div className="cal-page">
      <Head title={t("head.graveyard")} deck={n ? t("cal.graveDeck", n, bytes(g.bytes)) : t("cal.graveDeckEmpty")}>
        <label className="cal-search"><Search size={14} aria-hidden /><input type="search" placeholder={t("filterByPath")} aria-label={t("filterByPath")} value={filter} onChange={(e) => setFilter(e.target.value)} /></label>
        <span className="cal-picked" aria-live="polite">{k ? t("cal.pickedGraves", k) : t("cal.pickGraves")}</span>
        <button type="button" className="cal-btn gold" disabled={!k} onClick={() => g.resurrect()}><Star size={13} />{t("resurrect")}</button>
        <button type="button" className="cal-btn ink" disabled={!k} onClick={() => g.purge()}><Flame size={14} aria-hidden />{t("purge")}</button>
      </Head>
      {g.error ? <p className="cal-alert">{t("grave.readError", g.error)}</p> : null}
      {g.pending.length ? <p className="cal-alert">{t("grave.pending", g.pending.length)}</p> : null}
      {g.loading && !g.data ? <Loading /> : (
        <Board entries={entries} has={g.sel.has} toggle={g.sel.toggle}
          empty={filter ? <><b>{t("grave.noMatch")}</b><span>{t("notInPath", filter)}</span></> : <><b>{t("cal.graveQuiet")}</b><span>{t("cal.graveQuietText")}</span></>} />
      )}
    </div>
  );
}

// ── 即将下葬: the burial calendar ─────────────────────────────────────

function Upcoming() {
  const it = useItems("");
  const [lifespan, setLifespan] = useState("1d");
  const now = Date.now();
  const entries: Entry[] = it.items.map((x) => {
    const ms = +new Date(x.expiresAt) - now;
    return { id: x.id, at: x.expiresAt, path: x.path, name: nameOf(x.path), meta: ms <= 0 ? t("cal.expired") : t("cal.meta", sourceLabel(x.source), span(+new Date(x.expiresAt) - +new Date(x.createdAt))), soon: ms > 0 && ms < 3600e3, note: x.note };
  });
  const k = it.sel.chosen.length;
  const n = it.items.length;
  const today = it.items.filter((x) => dayIndex(x.expiresAt) === 0).length;

  // Dropping on a later day extends by whole days, keeping the time of day.
  function move(id: number, from: number, to: number) {
    const days = to - from;
    const ids = it.sel.has(id) ? it.sel.chosen : [id];
    const one = it.items.find((x) => x.id === id);
    const target = one ? new Date(Math.max(+new Date(one.expiresAt), Date.now()) + days * DAY) : null;
    it.extend(`${days}d`, ids, t("span.days", days), <p>{t("cal.moveBody", ids.length, days, target && ids.length === 1 ? `${md(target.toISOString())} ${hm(target.toISOString())}` : null)}</p>);
  }

  return (
    <div className="cal-page">
      <Head title={t("head.upcoming")} deck={n ? t("cal.upDeck", n, today) : t("cal.upDeckEmpty")}>
        <span className="cal-picked" aria-live="polite">{k ? t("cal.pickedItems", k) : t("cal.pickItems")}</span>
        <select className="cal-select" aria-label={t("howLong")} value={lifespan} disabled={!k} onChange={(e) => setLifespan(e.target.value)}>
          {extendSpans().map(([v, label]) => <option key={v} value={v}>{t("giveMore", label)}</option>)}
        </select>
        <button type="button" className="cal-btn cobalt" disabled={!k} onClick={() => it.extend(lifespan)}><TimerReset size={14} aria-hidden />{t("extend")}</button>
        <button type="button" className="cal-btn" disabled={!k} onClick={() => it.cancel()}><EyeOff size={14} aria-hidden />{t("stopTracking")}</button>
      </Head>
      {it.error ? <p className="cal-alert">{t("up.readError", it.error)}</p> : null}
      {it.loading && !it.data ? <Loading /> : (
        <Board entries={entries} has={it.sel.has} toggle={it.sel.toggle} onMove={move}
          empty={<><b>{t("cal.upQuiet")}</b><span>{t("cal.upQuietText")}</span></>} />
      )}
    </div>
  );
}
