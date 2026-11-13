import { Archive, Flame, Search, Sparkles } from "lucide-react";
import { useState } from "react";
import { useApi } from "../api";
import { bytes, when } from "../format";
import { useLoad, useRefresh } from "../hooks";
import { t } from "../i18n";
import { Button, Empty, Head, LifeLine, NameCell, SelectionBar, Skeleton, isContainer, useBatch, useConfirm, useResurrectReport } from "../ui";

const pendingTone: Record<string, string> = { pending: "warn", interrupted: "warn", purging: "warn", empty: "warn", unverifiable: "danger" };
const pendingText = (status: string): [string, string] | undefined =>
  pendingTone[status] ? [t(`grave.status.${status as "pending" | "interrupted" | "purging" | "empty" | "unverifiable"}`), pendingTone[status]] : undefined;

export default function Graveyard({ embedded = false }: { embedded?: boolean }) {
  const api = useApi();
  const { version, bump } = useRefresh();
  const confirm = useConfirm();
  const report = useBatch();
  const resurrected = useResurrectReport();
  const [filter, setFilter] = useState("");
  const [chosen, setChosen] = useState<Set<number>>(new Set());
  const { data, error, loading } = useLoad(() => api.Graveyard({ filter, limit: 200, after: "" }), [api, filter, version]);

  const rows = data?.rows ?? [];
  const buried = rows.filter((r) => r.status === "buried");
  const size = rows.reduce((n, r) => n + (r.burial?.size ?? 0), 0);
  const toggle = (id: number) => setChosen((s) => { const n = new Set(s); n.has(id) ? n.delete(id) : n.add(id); return n; });
  const allOn = buried.length > 0 && buried.every((r) => chosen.has(r.id));

  async function act(method: "Resurrect" | "Purge") {
    const refs = [...chosen].map((id) => `#${id}`);
    const paths = rows.filter((r) => chosen.has(r.id)).map((r) => r.burial?.path ?? `#${r.id}`);
    const purging = method === "Purge";
    const ok = await confirm({
      title: purging ? t("ask.purge", refs.length) : t("ask.resurrect", refs.length),
      body: <><p>{purging ? t("ask.purgeBody") : t("ask.resurrectBody")}</p><ul>{paths.map((p) => <li key={p}>{p}</li>)}</ul></>,
      ok: purging ? t("purge") : t("resurrect"),
      danger: purging,
    });
    if (!ok) return;
    const reply = await api[method](refs);
    if (purging) report(reply, t("purge"));
    else resurrected(reply);
    setChosen(new Set());
    bump();
  }

  return (
    <>
      <Head
        title={embedded ? undefined : t("head.graveyard")}
        actions={
          <div className="toolbar" style={{ margin: 0 }}>
            <label className="search"><Search aria-hidden /><input className="input" type="search" placeholder={t("filterByPath")} aria-label={t("filterByPath")} value={filter} onChange={(e) => setFilter(e.target.value)} /></label>
            <Button kind="gold" icon={Sparkles} disabled={!chosen.size} onClick={() => act("Resurrect")}>{t("resurrect")}</Button>
            <Button kind="danger" icon={Flame} disabled={!chosen.size} onClick={() => act("Purge")}>{t("purge")}</Button>
          </div>
        }
      >
        {data?.total ? t("grave.deck", data.total, bytes(size)) : t("grave.deckEmpty")}
      </Head>
      {error ? <div className="notice error">{t("grave.readError", error)}</div> : null}
      {data?.pendingCount ? <div className="notice warn">{t("grave.pending", data.pendingCount)}</div> : null}
      <div className="card list">
        {loading && !data ? <Skeleton /> : rows.length === 0 ? (
          <Empty icon={Archive} title={filter ? t("grave.noMatch") : t("grave.empty")} text={filter ? t("notInPath", filter) : t("grave.emptyText")} />
        ) : (
          <>
            <div className="row head-row cols-grave">
              <input type="checkbox" aria-label={t("selectAll")} checked={allOn} disabled={!buried.length} onChange={() => setChosen(allOn ? new Set() : new Set(buried.map((r) => r.id)))} />
              <span>{t("col.original")}</span><span>{t("col.untilPurge")}</span><span className="num">{t("col.size")}</span><span>{t("col.buried")}</span>
            </div>
            {rows.map((r) => {
              const b = r.burial;
              const tag = pendingText(r.status);
              return (
                <div key={r.id} className={`row cols-grave${chosen.has(r.id) ? " chosen" : ""}`}>
                  {r.status === "buried" ? <input type="checkbox" aria-label={t("select", `#${r.id}`)} checked={chosen.has(r.id)} onChange={() => toggle(r.id)} /> : <span />}
                  <div>
                    <NameCell path={b?.path ?? r.pending?.path ?? r.pending?.manifest ?? ""} isDir={b?.isDir} note={b?.note} kind={b?.kind} />
                    {tag ? <span className={`tag ${tag[1]}`}>{tag[0]}</span> : null}
                  </div>
                  {b ? <LifeLine start={b.buriedAt} end={b.purgeAt} left={(s) => t("life.canResurrect", s)} over={t("life.awaitPurge")} /> : <span />}
                  <span className="num">{b ? (isContainer(b.kind) ? t("container.size") : bytes(b.size)) : ""}</span>
                  <span className="when">{when(b?.buriedAt)}</span>
                </div>
              );
            })}
          </>
        )}
      </div>
      <SelectionBar count={chosen.size} onClear={() => setChosen(new Set())}>
        <Button kind="gold" icon={Sparkles} onClick={() => act("Resurrect")}>{t("resurrect")}</Button>
        <Button kind="danger" icon={Flame} onClick={() => act("Purge")}>{t("purge")}</Button>
      </SelectionBar>
    </>
  );
}
