import { Copy, ScrollText, Search } from "lucide-react";
import { useMemo, useState } from "react";
import { useApi } from "../api";
import { splitPath, when } from "../format";
import { useLoad, useRefresh } from "../hooks";
import { t } from "../i18n";
import type { LogLine } from "../types";
import { Button, Empty, Head, Skeleton, useToast } from "../ui";

const sources = (): [string, string][] => [["audit.jsonl", t("log.src.audit")], ["obitd.log", t("log.src.obitd")], ["hook-audit.log", t("log.src.hookAudit")], ["hook.log", t("log.src.hook")]];
const levels = (): [string, string][] => [["", t("log.lv.all")], ["error", t("log.lv.error")], ["warn", t("log.lv.warn")], ["info", t("log.lv.info")], ["debug", t("log.lv.debug")]];

export default function Logs() {
  const api = useApi();
  const { version } = useRefresh();
  const toast = useToast();
  const [source, setSource] = useState("audit.jsonl");
  const [level, setLevel] = useState("");
  const [text, setText] = useState("");
  const [lines, setLines] = useState(2000);
  const { data, error, loading } = useLoad(() => api.Logs({ source, lines, text, from: "", to: "", level }), [api, source, level, text, lines, version]);
  const shown = useMemo(() => data?.lines ?? [], [data]);
  // Newest first on screen; copy keeps the file's order.
  const newest = useMemo(() => [...shown].reverse(), [shown]);

  async function copy() {
    const out = shown.map((l) => l.text).join("\n");
    try {
      if (window.runtime?.ClipboardSetText) await window.runtime.ClipboardSetText(out);
      else await navigator.clipboard.writeText(out);
      toast(t("log.copied", shown.length));
    } catch (e) {
      toast(t("log.copyFailed", String(e)), "error");
    }
  }

  return (
    <>
      <Head title={t("page.logs")}>{data ? t("log.deck", shown.length, data.scannedLines, data.truncated) : t("loading")}</Head>
      <div className="toolbar">
        <select className="select" aria-label={t("log.source")} value={source} onChange={(e) => setSource(e.target.value)}>{sources().map(([v, label]) => <option key={v} value={v}>{label}</option>)}</select>
        <select className="select" aria-label={t("log.level")} value={level} onChange={(e) => setLevel(e.target.value)}>{levels().map(([v, label]) => <option key={v} value={v}>{label}</option>)}</select>
        <label className="search"><Search aria-hidden /><input className="input" type="search" placeholder={t("log.text")} aria-label={t("log.textLabel")} value={text} onChange={(e) => setText(e.target.value)} /></label>
        <select className="select" aria-label={t("log.lines")} value={lines} onChange={(e) => setLines(Number(e.target.value))}>{[500, 2000, 10000].map((n) => <option key={n} value={n}>{t("log.last", n)}</option>)}</select>
        <span className="grow" />
        <Button icon={Copy} onClick={copy} disabled={!shown.length}>{t("copy")}</Button>
      </div>
      {error ? <div className="notice error">{t("log.readError", error)}</div> : null}
      <div className="card">
        {loading && !data ? <Skeleton /> : shown.length === 0 ? (text ? (
          <div className="empty"><ScrollText aria-hidden /><b>{t("log.noMatch", text)}</b><Button kind="ghost" onClick={() => setText("")}>{t("log.clearText")}</Button></div>
        ) : <Empty icon={ScrollText} title={t("log.none")} text={t("log.noneText")} />) : (
          <div className="log" role="log">
            {newest.map((l, i) => (
              <div key={i} className={`log-line ${l.level}`}>
                <time>{l.time ? when(l.time) : ""}</time>
                <span className="lv">{levelText(l.level)}</span>
                {l.action ? <Entry line={l} /> : <span className="msg">{l.text}</span>}
              </div>
            ))}
          </div>
        )}
      </div>
    </>
  );
}

// One audit entry as a sentence: what happened, to which item, and why.
// The raw line stays in the tooltip.
function Entry({ line }: { line: LogLine }) {
  const { name, folder } = splitPath(line.path ?? "");
  return (
    <span className="msg entry" title={line.text}>
      <b>{t("log.act", line.action ?? "")}</b>
      {line.id ? <span className="ref">#{line.id}</span> : null}
      {name ? <span className="what">{name}</span> : null}
      {folder ? <span className="where">{folder}</span> : null}
      {line.detail ? <span className="why">{detailText(line.detail)}</span> : null}
    </span>
  );
}

const levels4 = ["error", "warn", "info", "debug"] as const;
const levelText = (level: string) => ((levels4 as readonly string[]).includes(level) ? t(`log.level.${level as (typeof levels4)[number]}`) : level);

// The backend writes details in English for the record; the common ones are
// said in the window's language, anything else stays as written.
function detailText(detail: string): string {
  const expires = /^(?:container )?expires (\S+)$/.exec(detail);
  if (expires) return t("log.d.expires", when(expires[1]));
  const moved = /^moved from (.+)$/.exec(detail);
  if (moved) return t("log.d.movedFrom", moved[1]);
  const known: Record<string, Parameters<typeof t>[0]> = {
    "a newer lifespan superseded this claim": "log.d.superseded",
    "could not verify the Recycle Bin move": "log.d.unverified",
    "candidate no longer exists or is Git-tracked": "log.d.pruned",
    "automatically withdrawn: no candidates remain": "log.d.withdrawn",
    "docker stop confirmed; writable layer retained": "log.d.stopped",
    "container removed externally on original Docker daemon": "log.d.containerGone",
  };
  const key = known[detail];
  return key ? t(key as "log.d.superseded") : detail;
}
