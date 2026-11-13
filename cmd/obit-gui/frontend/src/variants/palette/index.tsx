// J 一行: opened from the tray to do one thing fast, like Everything or
// PowerToys Run. The window is one line of input over Acrylic glass; pages
// are scopes of that line, every file is one row, and the bar at the bottom
// names what Enter will do. Typing a page name or 刷新 offers it as a command.
import { ArrowRight, CornerDownLeft, File, Folder, RefreshCw, Search } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { bytes, graveSize, noteText } from "../../format";
import Decisions from "../../pages/Decisions";
import Logs from "../../pages/Logs";
import Settings from "../../pages/Settings";
import { useAppearance } from "../../appearance";
import { t } from "../../i18n";
import { pageTitle, type PageId, type ShellProps } from "../../shell";
import { extendSpans, folderOf, nameOf, remaining, span, Star, useGraves, useItems, HOUR, DAY } from "../shared";
import "./palette.css";

const aliases: Record<string, string[]> = {
  graveyard: ["墓地", "墓", "坟", "复活", "火化", "graveyard", "grave", "resurrect", "cremate", "mudi"],
  upcoming: ["即将下葬", "下葬", "即将到期", "到期", "延长", "upcoming", "expiring", "expire", "extend", "daoqi", "xiazang"],
  decisions: ["待你决定", "决定", "提议", "规则", "decisions", "for you", "proposal", "rules"],
  settings: ["设置", "选项", "settings", "config", "shezhi"],
  logs: ["日志", "记录", "logs", "log", "rizhi"],
};

interface Cmd { id: string; label: string; run: () => void }

// Typing 外观 (or a look's name) offers every other look as a command; the
// same choice is also on the 设置 scope.
const lookWords = ["外观", "主题", "样式", "皮肤", "换肤", "appearance", "theme", "look"];

export default function PaletteShell({ page, nav, setPage, summary: s, refresh }: ShellProps) {
  const [query, setQuery] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const q = query.trim().toLowerCase();
  const appearance = useAppearance();
  const go = (p: PageId) => { setPage(p); setQuery(""); input.current?.focus(); };
  const commands: Cmd[] = !q ? [] : [
    ...nav.filter((p) => p !== page && aliases[p]?.some((a) => a.toLowerCase().includes(q) || q.includes(a.toLowerCase())))
      .map((p) => ({ id: p, label: t("pl.open", pageTitle(p)), run: () => go(p) })),
    ...("刷新refresh".includes(q) ? [{ id: "refresh", label: t("refresh"), run: () => { refresh(); setQuery(""); } }] : []),
    ...(!appearance ? [] : appearance.looks
      .filter((l) => l.id !== appearance.current && (lookWords.some((w) => w.includes(q) || q.includes(w)) || l.name.toLowerCase().includes(q)))
      .map((l) => ({ id: `look-${l.id}`, label: t("pl.look", l.name, l.note), run: () => { setQuery(""); appearance.choose(l.id); } }))),
  ];
  // Opened from the tray to type at once; no scroll jump inside the playground.
  useEffect(() => { input.current?.focus({ preventScroll: true }); }, [page]);
  const counts: Partial<Record<PageId, number | undefined>> = { graveyard: s?.graveCount, upcoming: s?.trackedCount, decisions: s?.decisionCount };

  function scopeKeys(e: KeyboardEvent) {
    if (e.ctrlKey && /^[1-9]$/.test(e.key)) {
      const p = nav[Number(e.key) - 1];
      if (p) { e.preventDefault(); go(p); }
    }
  }

  return (
    <div className="pl" onKeyDown={scopeKeys}>
      <div className="pl-sheet">
        <label className="pl-search">
          <img src="icon.png" alt="" />
          <input ref={input} value={query} onChange={(e) => setQuery(e.target.value)} aria-label={t("pl.input")}
            placeholder={page === "graveyard" ? t("pl.phGraves") : page === "upcoming" ? t("pl.phItems") : t("pl.phOther")}
            data-scope={page} />
          <Search className="pl-lens" size={18} aria-hidden />
        </label>
        <nav className="pl-scopes" aria-label={t("pagesLabel")}>
          {nav.map((p, i) => (
            <button key={p} type="button" aria-current={page === p ? "page" : undefined} onClick={() => go(p)} title={`Ctrl+${i + 1}`}>
              {pageTitle(p)}{counts[p] ? <span className={p === "decisions" ? "pl-count gold" : "pl-count"}>{counts[p]}</span> : null}
            </button>
          ))}
          <span className="pl-grow" />
          {s?.paused ? <span className="pl-paused">{t("paused")}</span> : null}
          <button type="button" className="pl-refresh" onClick={refresh} aria-label={t("refresh")} title={t("refresh")}><RefreshCw size={14} aria-hidden /></button>
        </nav>
        {page === "graveyard" ? <GraveList key="g" query={q} commands={commands} input={input} />
          : page === "upcoming" ? <ItemList key="u" query={q} commands={commands} input={input} />
            : commands.length ? <Plain commands={commands} input={input} />
              : (
                <>
                  <div className="pl-body pl-shared">{page === "decisions" ? <Decisions /> : page === "settings" ? <Settings /> : <Logs />}</div>
                  <Bar context={pageTitle(page)}><Hint keys={["Ctrl", "1"]}>{t("page.upcoming")}</Hint><Hint keys={["Ctrl", "2"]}>{t("page.graveyard")}</Hint></Bar>
                </>
              )}
      </div>
    </div>
  );
}

// ── Shared pieces ──────────────────────────────────────────────────────

function Keys({ keys }: { keys: string[] }) {
  return <span className="pl-keys">{keys.map((k) => <kbd key={k}>{k === "Enter" ? <CornerDownLeft size={11} aria-label="Enter" /> : k}</kbd>)}</span>;
}
function Hint({ keys, children }: { keys: string[]; children: ReactNode }) {
  return <span className="pl-hint">{children}<Keys keys={keys} /></span>;
}
function Act({ keys, onClick, kind = "", disabled, children }: { keys?: string[]; onClick: () => void; kind?: string; disabled?: boolean; children: ReactNode }) {
  return <button type="button" className={`pl-act ${kind}`} onClick={onClick} disabled={disabled}>{children}{keys ? <Keys keys={keys} /> : null}</button>;
}
function Bar({ context, children }: { context: ReactNode; children?: ReactNode }) {
  return <footer className="pl-bar"><span className="pl-context">{context}</span><span className="pl-grow" />{children}</footer>;
}

interface Row { id: number; path: string; isDir: boolean; group: string; time: string; urgent: boolean; note?: string }

// The list owns the keyboard while the input has focus: arrows move, Space
// (on an empty line) ticks, Enter runs the primary action, Ctrl+Delete the
// destructive one, Ctrl+K opens every action.
function useListKeys({ rows, commands, onPrimary, onDanger, toggle, onMenu, input }: {
  rows: Row[]; commands: Cmd[]; onPrimary: (id?: number) => void; onDanger: (id?: number) => void; toggle: (id: number) => void; onMenu: () => void; input: React.RefObject<HTMLInputElement | null>;
}) {
  const [active, setActive] = useState(0);
  const total = commands.length + rows.length;
  const at = Math.min(active, Math.max(0, total - 1));
  useEffect(() => {
    const el = input.current;
    if (!el) return;
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.key === "ArrowDown" || e.key === "ArrowUp") { e.preventDefault(); setActive((a) => Math.max(0, Math.min(total - 1, Math.min(a, total - 1) + (e.key === "ArrowDown" ? 1 : -1)))); return; }
      const cmd = at < commands.length ? commands[at] : undefined;
      const row = cmd ? undefined : rows[at - commands.length];
      if (e.key === "Enter") { e.preventDefault(); if (cmd) cmd.run(); else onPrimary(row?.id); return; }
      if (e.key === " " && !el.value && row) { e.preventDefault(); toggle(row.id); return; }
      if (e.key === "Delete" && e.ctrlKey && row) { e.preventDefault(); onDanger(row.id); return; }
      if ((e.key === "k" || e.key === "K") && e.ctrlKey) { e.preventDefault(); onMenu(); }
    };
    el.addEventListener("keydown", onKey);
    return () => el.removeEventListener("keydown", onKey);
  }, [input, at, total, commands, rows, onPrimary, onDanger, toggle, onMenu]);
  // Keep the active row in view.
  useEffect(() => { document.getElementById(`pl-opt-${at}`)?.scrollIntoView({ block: "nearest" }); }, [at]);
  return [at, setActive] as const;
}

function List({ rows, commands, at, setActive, has, toggle, onOpen, empty, loading }: {
  rows: Row[]; commands: Cmd[]; at: number; setActive: (i: number) => void; has: (id: number) => boolean; toggle: (id: number) => void; onOpen: (id: number) => void; empty: ReactNode; loading: boolean;
}) {
  let last = "";
  return (
    <div className="pl-body" role="listbox" aria-label={t("pl.results")} aria-multiselectable="true">
      {commands.length ? <div className="pl-group">{t("pl.commands")}</div> : null}
      {commands.map((c, i) => (
        <div key={c.id} id={`pl-opt-${i}`} role="option" aria-selected={at === i} className="pl-row cmd" data-active={at === i} onMouseMove={() => setActive(i)} onClick={c.run}>
          <span className="pl-icon"><ArrowRight size={15} aria-hidden /></span>
          <span className="pl-name">{c.label}</span>
        </div>
      ))}
      {loading ? <div className="pl-skel" aria-busy="true" aria-label={t("loading")}>{Array.from({ length: 6 }, (_, i) => <i key={i} />)}</div> : rows.length === 0 ? (commands.length ? null : <div className="pl-empty">{empty}</div>) : rows.map((r, j) => {
        const i = commands.length + j;
        const head = r.group !== last ? (last = r.group) : null;
        return (
          <div key={r.id}>
            {head ? <div className="pl-group">{head}</div> : null}
            <div id={`pl-opt-${i}`} role="option" aria-selected={has(r.id)} className="pl-row" data-active={at === i} data-on={has(r.id)}
              onMouseMove={() => setActive(i)} onClick={() => setActive(i)} onDoubleClick={() => onOpen(r.id)}>
              <input type="checkbox" className="pl-check" tabIndex={-1} aria-label={t("select", nameOf(r.path))} checked={has(r.id)} onChange={() => toggle(r.id)} onClick={(e) => e.stopPropagation()} />
              <span className="pl-icon">{r.isDir ? <Folder size={15} aria-hidden /> : <File size={15} aria-hidden />}</span>
              <span className="pl-name" title={r.path}>{nameOf(r.path)}</span>
              <span className="pl-folder">{folderOf(r.path)}{r.note ? <em>{noteText(r.note)}</em> : null}</span>
              <span className="pl-time" data-urgent={r.urgent}>{r.time}</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function Menu({ items, onClose }: { items: { label: ReactNode; run: () => void; kind?: string }[]; onClose: () => void }) {
  const [at, setAt] = useState(0);
  const box = useRef<HTMLDivElement>(null);
  useEffect(() => { box.current?.focus(); }, []);
  return (
    <div className="pl-menu" role="menu" tabIndex={-1} ref={box} aria-label={t("pl.allActions")}
      onKeyDown={(e) => {
        if (e.key === "Escape") { e.preventDefault(); onClose(); }
        else if (e.key === "ArrowDown" || e.key === "ArrowUp") { e.preventDefault(); setAt((a) => (a + (e.key === "ArrowDown" ? 1 : items.length - 1)) % items.length); }
        else if (e.key === "Enter") { e.preventDefault(); onClose(); items[at].run(); }
      }}
      onBlur={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node)) onClose(); }}>
      {items.map((it, i) => (
        <button key={i} type="button" role="menuitem" className={`pl-menu-item ${it.kind ?? ""}`} data-active={at === i} tabIndex={-1}
          onMouseMove={() => setAt(i)} onClick={() => { onClose(); it.run(); }}>{it.label}</button>
      ))}
    </div>
  );
}

const matches = (path: string, q: string) => !q || path.toLowerCase().includes(q);

// ── 墓地 ───────────────────────────────────────────────────────────────

function GraveList({ query, commands, input }: { query: string; commands: Cmd[]; input: React.RefObject<HTMLInputElement | null> }) {
  const g = useGraves("");
  const [menu, setMenu] = useState(false);
  const rows: Row[] = useMemo(() => [...g.graves].filter((b) => matches(b.path, query))
    .sort((a, b) => +new Date(a.purgeAt) - +new Date(b.purgeAt))
    .map((b) => {
      const { ms } = remaining(b.buriedAt, b.purgeAt);
      return { id: b.id, path: b.path, isDir: b.isDir, note: b.note, urgent: ms < DAY, group: ms < DAY ? t("pl.g.day") : ms < 3 * DAY ? t("pl.g.three") : t("pl.g.later"), time: t("pl.graveTime", ms > 0 ? span(ms) : null, graveSize(b)) };
    }), [g.graves, query]);
  const targets = (id?: number) => (g.sel.chosen.length ? g.sel.chosen : id !== undefined ? [id] : []);
  const resurrect = (id?: number) => { void g.resurrect(targets(id)).then(() => input.current?.focus()); };
  const purge = (id?: number) => { void g.purge(targets(id)).then(() => input.current?.focus()); };
  const [at, setActive] = useListKeys({ rows, commands, onPrimary: resurrect, onDanger: purge, toggle: g.sel.toggle, onMenu: () => setMenu(true), input });
  const active = rows[at - commands.length];
  const k = g.sel.chosen.length;
  return (
    <>
      <List rows={rows} commands={commands} at={at} setActive={setActive} has={g.sel.has} toggle={g.sel.toggle} onOpen={resurrect} loading={g.loading && !g.data}
        empty={query ? <><b>{t("pl.noGrave", query)}</b><span>{t("pl.noGraveText")}</span></> : <><b>{t("grave.empty")}</b><span>{t("pl.gravesEmptyText")}</span></>} />
      {g.error ? <div className="pl-error">{t("grave.readError", g.error)}</div> : null}
      <Bar context={k ? t("pl.pickedGraves", k) : t("pl.gravesCtx", g.graves.length, bytes(g.bytes))}>
        <Act kind="gold" keys={["Enter"]} disabled={!k && !active} onClick={() => resurrect(active?.id)}><Star size={12} />{k > 1 ? t("pl.resurrectN", k) : t("resurrect")}</Act>
        <Act keys={["Ctrl", "Del"]} disabled={!k && !active} onClick={() => purge(active?.id)}>{t("purge")}</Act>
        <Act keys={["Ctrl", "K"]} onClick={() => setMenu(true)}>{t("more")}</Act>
      </Bar>
      {menu ? <Menu onClose={() => { setMenu(false); input.current?.focus(); }} items={[
        { label: <><Star size={12} />{t("pl.resurrectThese", k)}</>, run: () => resurrect(active?.id), kind: "gold" },
        { label: t("pl.purgeThese", k), run: () => purge(active?.id) },
        { label: g.sel.all ? t("selectNone") : t("selectAll"), run: g.sel.toggleAll },
      ]} /> : null}
    </>
  );
}

// ── 即将下葬 ───────────────────────────────────────────────────────────

function ItemList({ query, commands, input }: { query: string; commands: Cmd[]; input: React.RefObject<HTMLInputElement | null> }) {
  const it = useItems("");
  const [menu, setMenu] = useState(false);
  const rows: Row[] = useMemo(() => it.items.filter((x) => matches(x.path, query)).map((x) => {
    const ms = +new Date(x.expiresAt) - Date.now();
    return { id: x.id, path: x.path, isDir: x.isDir, note: x.note, urgent: ms < HOUR, group: ms < HOUR ? t("pl.i.hour") : ms < DAY ? t("pl.i.today") : ms < 7 * DAY ? t("pl.i.week") : t("pl.i.later"), time: t("pl.itemTime", ms > 0 ? span(ms) : null) };
  }), [it.items, query]);
  const targets = (id?: number) => (it.sel.chosen.length ? it.sel.chosen : id !== undefined ? [id] : []);
  const extend = (v: string, id?: number) => { void it.extend(v, targets(id)).then(() => input.current?.focus()); };
  const cancel = (id?: number) => { void it.cancel(targets(id)).then(() => input.current?.focus()); };
  const [at, setActive] = useListKeys({ rows, commands, onPrimary: (id) => extend("1d", id), onDanger: cancel, toggle: it.sel.toggle, onMenu: () => setMenu(true), input });
  const active = rows[at - commands.length];
  const k = it.sel.chosen.length;
  return (
    <>
      <List rows={rows} commands={commands} at={at} setActive={setActive} has={it.sel.has} toggle={it.sel.toggle} onOpen={(id) => extend("1d", id)} loading={it.loading && !it.data}
        empty={query ? <><b>{t("pl.noItem", query)}</b><span>{t("pl.noItemText")}</span></> : <><b>{t("pl.itemsEmpty")}</b><span>{t("up.quietAll")}</span></>} />
      {it.error ? <div className="pl-error">{t("up.readError", it.error)}</div> : null}
      <Bar context={k ? t("pl.pickedItems", k) : t("pl.itemsCtx", it.items.length)}>
        <Act kind="cobalt" keys={["Enter"]} disabled={!k && !active} onClick={() => extend("1d", active?.id)}>{t("pl.extendDay", k)}</Act>
        <Act keys={["Ctrl", "Del"]} disabled={!k && !active} onClick={() => cancel(active?.id)}>{t("stopTracking")}</Act>
        <Act keys={["Ctrl", "K"]} onClick={() => setMenu(true)}>{t("more")}</Act>
      </Bar>
      {menu ? <Menu onClose={() => { setMenu(false); input.current?.focus(); }} items={[
        ...extendSpans().map(([v, label]) => ({ label: t("extendBy", label), run: () => extend(v, active?.id) })),
        { label: t("stopTracking"), run: () => cancel(active?.id) },
        { label: it.sel.all ? t("selectNone") : t("selectAll"), run: it.sel.toggleAll },
      ]} /> : null}
    </>
  );
}

// Commands only, on the pages that are not lists.
function Plain({ commands, input }: { commands: Cmd[]; input: React.RefObject<HTMLInputElement | null> }) {
  const [at, setActive] = useListKeys({ rows: [], commands, onPrimary: () => {}, onDanger: () => {}, toggle: () => {}, onMenu: () => {}, input });
  return (
    <>
      <List rows={[]} commands={commands} at={at} setActive={setActive} has={() => false} toggle={() => {}} onOpen={() => {}} loading={false} empty={null} />
      <Bar context={t("pl.commands")}><Hint keys={["Enter"]}>{t("run")}</Hint></Bar>
    </>
  );
}
