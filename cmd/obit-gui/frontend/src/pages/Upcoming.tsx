import { EyeOff, Hourglass, TimerReset } from "lucide-react";
import { useState } from "react";
import { useApi } from "../api";
import { when } from "../format";
import { useLoad, useRefresh } from "../hooks";
import { t } from "../i18n";
import { Button, Empty, Head, LifeLine, NameCell, Seg, SelectionBar, Skeleton, useBatch, useConfirm } from "../ui";
import { extendList, extendSpans, sourceLabel, spanLabel } from "../variants/shared";

type Range = "" | "24h" | "7d";

export default function Upcoming({ embedded = false }: { embedded?: boolean }) {
  const api = useApi();
  const { version, bump } = useRefresh();
  const confirm = useConfirm();
  const report = useBatch();
  const [range, setRange] = useState<Range>(embedded ? "" : "24h");
  const [span, setSpan] = useState("1d");
  const [chosen, setChosen] = useState<Set<number>>(new Set());
  const { data, error, loading } = useLoad(() => api.Upcoming(range), [api, range, version]);
  const items = data?.items ?? [];
  const allOn = items.length > 0 && items.every((it) => chosen.has(it.id));
  const toggle = (id: number) => setChosen((s) => { const n = new Set(s); n.has(id) ? n.delete(id) : n.add(id); return n; });

  async function act(extending: boolean) {
    const refs = [...chosen].map((id) => `#${id}`);
    const paths = items.filter((it) => chosen.has(it.id)).map((it) => it.path);
    const label = spanLabel(span);
    const ok = await confirm({
      title: extending ? t("ask.extend", refs.length, label) : t("ask.cancel", refs.length),
      body: <><p>{extending ? t("ask.extendBody") : t("ask.cancelBody")}</p>{extending ? extendList(items.filter((it) => chosen.has(it.id)), span) : <ul>{paths.map((p) => <li key={p}>{p}</li>)}</ul>}</>,
      ok: extending ? t("extend") : t("stopTracking"),
      danger: !extending,
    });
    if (!ok) return;
    report(extending ? await api.Extend(refs, span) : await api.Cancel(refs), extending ? t("extend") : t("stopTracking"));
    setChosen(new Set());
    bump();
  }

  return (
    <>
      <Head title={embedded ? undefined : t("head.upcoming")}>
        {items.length ? t("up.deck", range, items.length) : t("up.deckEmpty", range)}
      </Head>
      <div className="toolbar">
        <Seg<Range> label={t("range.label")} value={range} onChange={(v) => { setRange(v); setChosen(new Set()); }} options={[["24h", t("range.24h")], ["7d", t("range.7d")], ["", t("range.all")]]} />
        <span className="grow" />
        <select className="select" aria-label={t("howLong")} value={span} onChange={(e) => setSpan(e.target.value)} disabled={!chosen.size}>
          {extendSpans().map(([v, label]) => <option key={v} value={v}>{t("extendBy", label)}</option>)}
        </select>
        <Button kind="primary" icon={TimerReset} disabled={!chosen.size} onClick={() => act(true)}>{t("extend")}</Button>
        <Button kind="danger" icon={EyeOff} disabled={!chosen.size} onClick={() => act(false)}>{t("stopTracking")}</Button>
      </div>
      {error ? <div className="notice error">{t("up.readError", error)}</div> : null}
      <div className="card list">
        {loading && !data ? <Skeleton /> : items.length === 0 ? (
          <Empty icon={Hourglass} title={t("up.quiet")} text={range ? t("up.quietRange") : t("up.quietAll")} />
        ) : (
          <>
            <div className="row head-row cols-item">
              <input type="checkbox" aria-label={t("selectAll")} checked={allOn} onChange={() => setChosen(allOn ? new Set() : new Set(items.map((it) => it.id)))} />
              <span>{t("col.path")}</span><span>{t("col.lifespan")}</span><span>{t("col.expires")}</span><span>{t("col.source")}</span>
            </div>
            {items.map((it) => (
              <div key={it.id} className={`row cols-item${chosen.has(it.id) ? " chosen" : ""}`}>
                <input type="checkbox" aria-label={t("select", `#${it.id}`)} checked={chosen.has(it.id)} onChange={() => toggle(it.id)} />
                <div>
                  <NameCell path={it.path} isDir={it.isDir} note={it.note} kind={it.kind} />
                  {it.lastError ? <span className="tag danger">{it.lastError}</span> : null}
                </div>
                <LifeLine start={it.createdAt} end={it.expiresAt} left={(s) => t("life.left", s)} over={t("life.expired")} soonMs={3600e3} />
                <span className="when">{when(it.expiresAt)}</span>
                <span><span className="tag">{sourceLabel(it.source)}</span></span>
              </div>
            ))}
          </>
        )}
      </div>
      <SelectionBar count={chosen.size} onClear={() => setChosen(new Set())}>
        <select className="select" aria-label={t("howLong")} value={span} onChange={(e) => setSpan(e.target.value)}>
          {extendSpans().map(([v, label]) => <option key={v} value={v}>{t("extendBy", label)}</option>)}
        </select>
        <Button kind="primary" icon={TimerReset} onClick={() => act(true)}>{t("extend")}</Button>
        <Button kind="danger" icon={EyeOff} onClick={() => act(false)}>{t("stopTracking")}</Button>
      </SelectionBar>
    </>
  );
}
