// F 讣闻: the window is a newspaper's obituary page. The masthead is the
// wordmark, every grave is a notice set in columns, tracked files are a
// table of lifespans. Ink, paper, cobalt for the masthead and selection,
// gold only for the resurrection stamp.
import { RefreshCw, Search } from "lucide-react";
import { useState, type ReactNode } from "react";
import { bytes, graveSize, noteText } from "../../format";
import Decisions from "../../pages/Decisions";
import Logs from "../../pages/Logs";
import Settings from "../../pages/Settings";
import { t } from "../../i18n";
import Notices from "../../notices";
import { pageTitle, type ShellProps } from "../../shell";
import { extendSpans, folderOf, hm, md, nameOf, parts, remaining, sourceLabel, span, Star, pause, useGraves, useItems } from "../shared";
import "./notice.css";

// The dateline as a paper prints it: 二〇二六年十一月十日 / November 10, 2026.
function dateline(d = new Date()) {
  return { date: t("date.paper", d.getFullYear(), d.getMonth() + 1, d.getDate()), day: t("date.paperDay", d.getDay()) };
}

export default function NoticeShell({ page, nav, setPage, summary: s, refresh }: ShellProps) {
  const { date, day } = dateline();
  return (
    <div className="nt">
      <header className="nt-mast">
        <div className="nt-ear">{date}<br />{day}</div>
        <div className="nt-title">
          <svg className="nt-swoosh" viewBox="0 0 170 56" aria-hidden>
            <path d="M2 52C44 28 100 12 152 10C102 16 48 32 2 52Z" />
            <path transform="translate(146 0) scale(0.75)" d="M12 0.5C12.7 7.8 14.2 10.6 20 12C14.2 13.4 12.7 16.2 12 23.5C11.3 16.2 9.8 13.4 4 12C9.8 10.6 11.3 7.8 12 0.5Z" />
          </svg>
          <h1>{t("brand")}</h1>
        </div>
        <div className="nt-ear right">
          {s ? <>{t("nt.living", s.trackedCount)}<br />{t("nt.buried", s.graveCount, bytes(s.graveBytes))}</> : <>&nbsp;<br />&nbsp;</>}
        </div>
      </header>
      <nav className="nt-sections" aria-label={t("pagesLabel")}>
        {nav.map((id) => (
          <button key={id} type="button" aria-current={page === id ? "page" : undefined} onClick={() => setPage(id)}>
            {pageTitle(id)}
            {id === "decisions" && s?.decisionCount ? <span className="nt-count">{s.decisionCount}</span> : null}
          </button>
        ))}
        <span className="nt-grow" />
        <Notices page={page} setPage={setPage} summary={s} refresh={refresh} />
        <button type="button" className="nt-refresh" onClick={refresh}><RefreshCw aria-hidden />{t("refresh")}</button>
      </nav>
      <div className="nt-stage">
        <main className="nt-main" key={page}>
          {page === "graveyard" ? <Graves /> : page === "upcoming" ? <Upcoming /> : (
            <div className="nt-shared">{page === "decisions" ? <Decisions /> : page === "settings" ? <Settings /> : <Logs />}</div>
          )}
        </main>
      </div>
    </div>
  );
}

function PageHead({ title, deck, tools }: { title: string; deck: ReactNode; tools?: ReactNode }) {
  return (
    <div className="nt-head">
      <div className="nt-head-text">
        <h2>{title}</h2>
        <p className="nt-deck">{deck}</p>
      </div>
      {tools ? <div className="nt-tools">{tools}</div> : null}
    </div>
  );
}

function PressBar({ open, children }: { open: boolean; children: ReactNode }) {
  return <div className="nt-press" data-open={open} aria-hidden={!open} inert={!open}>{children}</div>;
}

function Empty({ title, text }: { title: string; text: string }) {
  return <div className="nt-empty"><Star size={22} /><h3>{title}</h3><p>{text}</p></div>;
}

const Loading = () => <div className="nt-loading" aria-busy="true" aria-label={t("loading")}>{Array.from({ length: 6 }, (_, i) => <i key={i} />)}</div>;

// ── 墓地: a page of notices ───────────────────────────────────────────

function Graves() {
  const [filter, setFilter] = useState("");
  const g = useGraves(filter);
  const [stamp, setStamp] = useState<{ ids: number[]; kind: "rise" | "ash" } | null>(null);
  const mark = (kind: "rise" | "ash") => async () => { setStamp({ ids: g.sel.chosen, kind }); await pause(560); setStamp(null); };
  const n = g.graves.length;
  // The lead notice is the grave closest to cremation, set across the page.
  const ordered = [...g.graves].sort((a, b) => +new Date(a.purgeAt) - +new Date(b.purgeAt));

  return (
    <>
      <PageHead
        title={t("head.graveyard")}
        deck={n ? t("nt.deck", n, bytes(g.bytes)) : t("nt.deckEmpty")}
        tools={<>
          <label className="nt-search"><Search aria-hidden /><input type="search" placeholder={t("nt.search")} aria-label={t("nt.search")} value={filter} onChange={(e) => setFilter(e.target.value)} /></label>
          <button type="button" className="nt-link" disabled={!n} onClick={g.sel.toggleAll}>{g.sel.all ? t("selectNone") : t("selectAll")}</button>
        </>}
      />
      {g.error ? <p className="nt-error">{t("grave.readError", g.error)}</p> : null}
      {g.pending.length ? <p className="nt-error">{t("grave.pending", g.pending.length)}</p> : null}
      {g.loading && !g.data ? <Loading /> : n === 0 ? (
        filter ? <Empty title={t("nt.notFound")} text={t("nt.notFoundText", filter)} /> : <Empty title={t("nt.none")} text={t("nt.noneText")} />
      ) : (
        <div className="nt-columns">
          {ordered.map((b, i) => {
            const { ms, left } = remaining(b.buriedAt, b.purgeAt);
            const p = parts(ms);
            const on = g.sel.has(b.id);
            const stamped = stamp?.ids.includes(b.id) ? stamp.kind : undefined;
            const lead = i === 0 && ordered.length > 2;
            return (
              <article key={b.id} className={`nt-notice${lead ? " lead" : ""}`} data-on={on} data-stamp={stamped} onClick={() => g.sel.toggle(b.id)}>
                <input type="checkbox" className="nt-check" aria-label={t("select", nameOf(b.path))} checked={on} onChange={() => g.sel.toggle(b.id)} onClick={(e) => e.stopPropagation()} />
                <span className="nt-box" aria-hidden />
                <div className="nt-copy">
                  <h3 title={b.path}>{nameOf(b.path)}</h3>
                  <p className="nt-home">{t("nt.home")}<span>{folderOf(b.path) || t("nt.root")}</span></p>
                  <p className="nt-body">{t("nt.body", md(b.buriedAt), hm(b.buriedAt), b.isDir, graveSize(b), md(b.purgeAt), hm(b.purgeAt))}</p>
                  {b.note ? <p className="nt-note">{t("nt.note", noteText(b.note))}</p> : null}
                </div>
                <div className="nt-left" data-soon={ms < 86400e3}>
                  {ms > 0 ? <><span>{lead ? t("nt.leadLeft") : t("nt.left")}</span><b>{p.n}</b>{p.unit}</> : <>{t("life.awaitPurge")}</>}
                  <span className="nt-band" aria-hidden><i style={{ transform: `scaleX(${left.toFixed(3)})` }} /></span>
                </div>
                {stamped ? <span className="nt-stamp" aria-hidden>{stamped === "rise" ? t("nt.stampRise") : t("nt.stampAsh")}</span> : null}
              </article>
            );
          })}
        </div>
      )}
      <PressBar open={g.sel.chosen.length > 0}>
        <span className="nt-press-count">{t("nt.pickedNotices", g.sel.chosen.length)}</span>
        <span className="nt-grow" />
        <button type="button" className="nt-btn quiet" onClick={g.sel.clear}>{t("clearSelection")}</button>
        <button type="button" className="nt-btn ink" onClick={() => g.purge(undefined, mark("ash"))}>{t("purge")}</button>
        <button type="button" className="nt-btn gold" onClick={() => g.resurrect(undefined, mark("rise"))}><Star size={13} />{t("resurrect")}</button>
      </PressBar>
    </>
  );
}

// ── 即将下葬: a table of lifespans ────────────────────────────────────

type Range = "" | "24h" | "7d";
const ranges = (): [Range, string][] => [["24h", t("range.24h")], ["7d", t("range.7d")], ["", t("range.all")]];

function Upcoming() {
  const [range, setRange] = useState<Range>("24h");
  const [lifespan, setLifespan] = useState("1d");
  const it = useItems(range);
  const n = it.items.length;

  return (
    <>
      <PageHead
        title={t("head.upcoming")}
        deck={n ? t("nt.upDeck", range, n) : t("nt.upDeckEmpty", range)}
        tools={<div className="nt-ranges" role="group" aria-label={t("range.label")}>
          {ranges().map(([v, label]) => <button key={v} type="button" aria-pressed={range === v} onClick={() => { setRange(v); it.sel.clear(); }}>{label}</button>)}
        </div>}
      />
      {it.error ? <p className="nt-error">{t("up.readError", it.error)}</p> : null}
      {it.loading && !it.data ? <Loading /> : n === 0 ? (
        <Empty title={t("nt.quiet")} text={range ? t("nt.quietRange") : t("nt.quietAll")} />
      ) : (
        <table className="nt-table">
          <thead>
            <tr>
              <th className="c"><input type="checkbox" className="nt-tick" aria-label={t("selectAll")} checked={it.sel.all} onChange={it.sel.toggleAll} /></th>
              <th>{t("nt.col.item")}</th><th>{t("col.source")}</th><th className="r">{t("nt.col.age")}</th><th className="r">{t("nt.col.due")}</th><th className="r">{t("nt.col.left")}</th>
            </tr>
          </thead>
          <tbody>
            {it.items.map((x) => {
              const { ms, left } = remaining(x.createdAt, x.expiresAt);
              const p = parts(ms);
              const on = it.sel.has(x.id);
              return (
                <tr key={x.id} data-on={on} data-soon={ms > 0 && ms < 3600e3} onClick={() => it.sel.toggle(x.id)}>
                  <td className="c"><input type="checkbox" className="nt-tick" aria-label={t("select", nameOf(x.path))} checked={on} onChange={() => it.sel.toggle(x.id)} onClick={(e) => e.stopPropagation()} /></td>
                  <td>
                    <div className="nt-cell-name" title={x.path}>{nameOf(x.path)}</div>
                    <div className="nt-cell-path">{folderOf(x.path)}</div>
                    {x.note ? <div className="nt-cell-note">{t("nt.note", noteText(x.note))}</div> : null}
                    {x.lastError ? <div className="nt-cell-note">{t("nt.lastError", x.lastError)}</div> : null}
                  </td>
                  <td className="nt-src">{sourceLabel(x.source)}</td>
                  <td className="r nt-age">{span(+new Date(x.expiresAt) - +new Date(x.createdAt))}</td>
                  <td className="r nt-date">{md(x.expiresAt)}<br />{hm(x.expiresAt)}</td>
                  <td className="r nt-rest">
                    {ms > 0 ? <><b>{p.n}</b>{p.unit}</> : <span className="nt-due">{t("nt.due")}</span>}
                    <span className="nt-band" aria-hidden><i style={{ transform: `scaleX(${left.toFixed(3)})` }} /></span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
      <PressBar open={it.sel.chosen.length > 0}>
        <span className="nt-press-count">{t("nt.picked", it.sel.chosen.length)}</span>
        <span className="nt-grow" />
        <button type="button" className="nt-btn quiet" onClick={it.sel.clear}>{t("clearSelection")}</button>
        <button type="button" className="nt-btn ink" onClick={() => it.cancel()}>{t("stopTracking")}</button>
        <select className="nt-select" aria-label={t("howLong")} value={lifespan} onChange={(e) => setLifespan(e.target.value)}>
          {extendSpans().map(([v, label]) => <option key={v} value={v}>{t("giveMore", label)}</option>)}
        </select>
        <button type="button" className="nt-btn cobalt" onClick={() => it.extend(lifespan)}>{t("extend")}</button>
      </PressBar>
    </>
  );
}
