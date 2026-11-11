import { AppWindow, Archive, CalendarClock, ClipboardList, FolderInput, FolderOpen, LayoutGrid, ListChecks, Monitor, MousePointerClick, PanelBottom, PauseCircle, Plus, Power, Trash2 } from "lucide-react";
import { Fragment, useState, type ReactNode } from "react";
import { errorText, useApi } from "../api";
import { bytes, lifespanText, when } from "../format";
import { useLoad, useRefresh } from "../hooks";
import { askDecisions, setAskDecisions } from "../prefs";
import { t, useLangChoice } from "../i18n";
import type { Lifetime, Plan, PlanReceipt, WatchPolicy } from "../types";
import { Button, Head, Outcome, Seg, Skeleton, useConfirm, useToast, type OutcomeNote } from "../ui";
import About from "./About";
import Appearance from "./Appearance";

const watchActive = (p: WatchPolicy) => p.consent === undefined || (p.consent === "approved" && (p.approvedRev ?? 0) === (p.rev ?? 0));
const ages = (): [string, string][] => [["first-seen", t("set.age.firstSeen")], ["modified", t("set.age.modified")]];
const actions = ["settings", "contextMenu", "autostart", "pause", "language", "sessionSummary", "updateCheck", "trayIcon", "taskbar", "startMenu", "desktopShortcut"] as const;
const actionText = (action: string) => ((actions as readonly string[]).includes(action) ? t(`set.action.${action as (typeof actions)[number]}`) : action);

export default function Settings() {
  const api = useApi();
  const { version, bump } = useRefresh();
  const confirm = useConfirm();
  const toast = useToast();
  const { data } = useLoad(() => Promise.all([api.Settings(), api.Watches()]), [api, version]);
  const [days, setDays] = useState<string | null>(null);
  const [note, setNote] = useState<OutcomeNote | null>(null);
  const [ask, setAsk] = useState(askDecisions);
  const langChoice = useLangChoice();
  const { data: release } = useLoad(() => (api.Update ? api.Update() : Promise.resolve(null)), [api]);
  if (!data) return <><Head title={t("page.settings")} /><div className="card"><Skeleton /></div></>;
  const [s, watches] = data;
  const policies = watches.policies ?? [];

  // Only changes with consequences ask first: new Explorer menu entries and
  // the graveyard mode. The rest apply at once and say what changed.
  async function apply(plan: Plan, title: string, opts: { after?: () => Promise<void>; ask?: boolean; body?: ReactNode } = {}) {
    const { after, ask = false, body } = opts;
    if (!plan.changes.length) { toast(t("set.already")); return; }
    const ok = !ask || await confirm({
      title,
      body: body ?? <><p>{actionText(plan.action)}</p><div className="diff">{plan.changes.map((c, i) => <Fragment key={i}><span className="diff-target">{c.target}</span><span>{c.before ? <s>{c.before}</s> : t("set.none")} → <b>{c.after || t("set.deleted")}</b></span></Fragment>)}</div></>,
      ok: t("set.apply"),
    });
    if (!ok) return;
    let receipt: PlanReceipt;
    try {
      receipt = await api.ApplyPlan(plan.id);
    } catch (error) {
      setNote({ title: t("set.notDone", title), text: errorText(error) });
      toast(t("set.notDoneToast", errorText(error)), "error");
      bump();
      return;
    }
    setDays(null);
    bump();
    await after?.();
    // changed only means at least one step landed; remaining or an error
    // means it stopped part way.
    const left = receipt.remaining ?? [];
    if (left.length || receipt.error) {
      setNote({ title: t("set.partial", title), text: t("set.partialText", receipt.applied?.length ?? 0, left.length, receipt.error ?? ""), items: left.map((k) => [plan.changes.find((c) => c.key === k)?.target ?? k, ""]) });
      toast(t("set.partial", title), "error");
      return;
    }
    setNote(null);
    toast(ask ? t("set.applied") : title);
  }

  // A newer release moves this card to the top, where the banner points.
  const about = <About apply={(plan, title) => apply(plan, title)} switchFor={(on, label, onToggle) => <Switch on={on} label={label} onToggle={onToggle} />} />;

  return (
    <>
      <Head title={t("page.settings")} />
      <Life lifetime={s.lifetime} days={s.retentionDays} />
      {release?.newer ? about : null}
      <div className="section-title">{t("set.general")}</div>
      {note ? <Outcome note={note} onClose={() => setNote(null)} /> : null}
      <div className="card">
        <Setting icon={Archive} label={t("set.mode")} desc={s.mode === "recycle" ? t("set.modeRecycle") : t("set.modeGraveyard")}>
          <select className="select" value={s.mode} aria-label={t("set.mode")} onChange={async (e) => { const recycle = e.target.value === "recycle"; await apply(await api.PlanSettings(e.target.value, s.retentionDays), recycle ? t("set.toRecycle") : t("set.toGraveyard"), { ask: true, body: <p>{recycle ? t("set.toRecycleBody") : t("set.toGraveyardBody", s.retentionDays)}</p> }); }}>
            <option value="graveyard">{t("set.graveyardOption")}</option>
            <option value="recycle">{t("set.recycle")}</option>
          </select>
        </Setting>
        <Setting icon={CalendarClock} label={t("set.retention")} desc={t("set.retentionDesc")}>
          <span className="toolbar" style={{ margin: 0 }}>
            <input className="input" type="number" min={1} max={365} style={{ width: 72 }} aria-label={t("set.retentionDays")} value={days ?? String(s.retentionDays)} onChange={(e) => setDays(e.target.value)} />
            <span className="hint">{t("set.daysUnit")}</span>
            {days !== null && days !== String(s.retentionDays) ? <Button kind="primary" onClick={async () => apply(await api.PlanSettings(s.mode, Number(days)), t("set.retentionDone", Number(days)))}>{t("set.save")}</Button> : null}
          </span>
        </Setting>
        <Setting icon={PauseCircle} label={t("set.auto")} desc={s.paused ? t("set.autoPaused") : t("set.autoOn")}>
          <Switch on={!s.paused} label={t("set.auto")} onToggle={async () => apply(await api.PlanPause(!s.paused), s.paused ? t("set.resumed") : t("set.paused"))} />
        </Setting>
        <Setting icon={Power} label={t("set.autostart")} desc={t("set.autostartDesc")}>
          <Switch on={s.autostart} label={t("set.autostart")} onToggle={async () => apply(await api.PlanAutostart(!s.autostart), s.autostart ? t("set.autostartOff") : t("set.autostartOn"))} />
        </Setting>
        {s.sessionSummary !== undefined && api.PlanSessionSummary ? (
          <Setting icon={ClipboardList} label={t("set.summary")} desc={t("set.summaryDesc")}>
            <Switch on={s.sessionSummary} label={t("set.summary")} onToggle={async () => apply(await api.PlanSessionSummary!(!s.sessionSummary), s.sessionSummary ? t("set.summaryOff") : t("set.summaryOn"))} />
          </Setting>
        ) : null}
        <Setting icon={ListChecks} label={t("set.askDecisions")} desc={ask ? t("set.askDecisionsOn") : t("set.askDecisionsOff")}>
          <Switch on={ask} label={t("set.askDecisions")} onToggle={() => { setAskDecisions(!ask); setAsk(!ask); toast(!ask ? t("set.askDecisionsOnDone") : t("set.askDecisionsOffDone")); }} />
        </Setting>
      </div>

      <div className="section-title">{t("set.entry")}</div>
      <div className="card">
        {s.trayIcon !== undefined && api.PlanTrayIcon ? (
          <Setting icon={PanelBottom} label={t("set.tray")} desc={t("set.trayDesc")}>
            <Switch on={s.trayIcon} label={t("set.tray")} onToggle={async () => apply(await api.PlanTrayIcon!(!s.trayIcon), s.trayIcon ? t("set.trayOff") : t("set.trayOn"))} />
          </Setting>
        ) : null}
        {s.taskbar !== undefined && api.PlanTaskbar ? (
          <Setting icon={AppWindow} label={t("set.taskbar")} desc={t("set.taskbarDesc")}>
            <Switch on={s.taskbar} label={t("set.taskbar")} onToggle={async () => apply(await api.PlanTaskbar!(!s.taskbar), s.taskbar ? t("set.taskbarOff") : t("set.taskbarOn"))} />
          </Setting>
        ) : null}
        {s.startMenu !== undefined && api.PlanStartMenu ? (
          <Setting icon={LayoutGrid} label={t("set.startMenu")} desc={t("set.startMenuDesc")}>
            <Switch on={s.startMenu} label={t("set.startMenu")} onToggle={async () => apply(await api.PlanStartMenu!(!s.startMenu), s.startMenu ? t("set.startMenuOff") : t("set.startMenuOn"))} />
          </Setting>
        ) : null}
        {s.desktopShortcut !== undefined && api.PlanDesktopShortcut ? (
          <Setting icon={Monitor} label={t("set.desktop")} desc={t("set.desktopDesc")}>
            <Switch on={s.desktopShortcut} label={t("set.desktop")} onToggle={async () => apply(await api.PlanDesktopShortcut!(!s.desktopShortcut), s.desktopShortcut ? t("set.desktopOff") : t("set.desktopOn"))} />
          </Setting>
        ) : null}
        <Setting icon={MousePointerClick} label={t("set.menu")} desc={s.contextMenuInstalled && !s.contextMenuCurrent ? s.contextMenuDetail || t("set.menuStale") : t("set.menuDesc")}>
          <Switch on={s.contextMenuInstalled} label={t("set.menu")} onToggle={async () => apply(await api.PlanContextMenu(!s.contextMenuInstalled), s.contextMenuInstalled ? t("set.menuOff") : t("set.menuOn"), { ask: true })} />
        </Setting>
      </div>

      <div className="section-title">{t("set.watches")}</div>
      <p className="hint section-deck">{t("set.watchesDeck")}</p>
      <div className="card list">
        {policies.length === 0 ? <div className="row watch-none"><FolderInput aria-hidden /><span className="hint">{t("set.noWatchesText")}</span></div> : policies.map((p) => (
          <div key={p.id} className="row cols-policy">
            <span className="name-sub" title={p.path}>{p.path}</span>
            <span className="tag">{lifespanText(p.lifespan)}</span>
            <span className={`tag ${watchActive(p) ? "ok" : "warn"}`}>{watchActive(p) ? t("set.watchOn") : t("set.watchPending")}</span>
            <span className="hint">{ages().find(([v]) => v === p.age)?.[1] ?? p.age}</span>
            <span className="when">{when(p.createdAt)}</span>
            <Button kind="ghost" icon={Trash2} aria-label={t("set.removeWatch")} onClick={async () => {
              if (!await confirm({ title: t("set.askRemoveWatch"), body: <><p>{t("set.removeWatchBody")}</p><ul><li>{p.path}</li></ul></>, ok: t("set.remove"), danger: true })) return;
              await api.RemoveWatch(p.path);
              bump();
            }} />
          </div>
        ))}
      </div>
      <AddWatch downloads={s.downloads} />

      <Appearance />
      {s.language !== undefined && api.PlanLanguage ? (
        <div className="lang-row">
          <span className="lang-label">{t("lang.title")}</span>
          <Seg<string> label={t("lang.title")} value={s.language} onChange={async (v) => {
            // Same flow as the switches: preview, confirm, apply. Then the
            // window takes the language the backend now resolves to.
            const plan = await api.PlanLanguage!(v);
            await apply(plan, t("lang.changed"), { after: async () => langChoice?.adopt((await api.Settings()).effectiveLanguage) });
          }} options={[["auto", t("lang.system")], ["zh-CN", t("lang.zh")], ["en", t("lang.en")]]} />
          {s.language === "auto" && s.effectiveLanguage ? <span className="hint">{t("lang.systemIs", s.effectiveLanguage === "en" ? t("lang.en") : t("lang.zh"))}</span> : null}
        </div>
      ) : null}
      {release?.newer ? null : <><div className="section-title">{t("upd.title")}</div>{about}</>}
      <p className="hint config-path">{t("set.deck", s.configPath)}</p>
    </>
  );
}

function Setting({ icon: Icon, label, desc, children }: { icon: typeof Archive; label: string; desc: string; children: ReactNode }) {
  return (
    <div className="setting">
      <span className="setting-icon"><Icon aria-hidden /></span>
      <span className="setting-label">{label}</span>
      <span className="setting-desc">{desc}</span>
      {children}
    </div>
  );
}

function Switch({ on, label, onToggle }: { on: boolean; label: string; onToggle: () => void }) {
  return <button type="button" role="switch" className="switch" aria-checked={on} aria-label={label} onClick={onToggle} />;
}

// A folder, a number and a unit: no paths or "2w" to type unless wanted.
function AddWatch({ downloads }: { downloads?: string }) {
  const api = useApi();
  const { bump } = useRefresh();
  const toast = useToast();
  const [path, setPath] = useState("");
  const [count, setCount] = useState("2");
  const [unit, setUnit] = useState<"h" | "d" | "w">("w");
  const [age, setAge] = useState("first-seen");
  const n = Math.max(1, Math.floor(Number(count) || 0));
  const lifespan = `${n}${unit}`;
  async function pick() {
    try {
      const chosen = await api.PickFolder!();
      if (chosen) setPath(chosen);
    } catch (error) {
      toast(errorText(error), "error");
    }
  }
  return (
    <div className="card watch-add">
      <div className="toolbar" style={{ margin: 0 }}>
        <input className="input grow" placeholder={t("set.watchPath")} aria-label={t("set.watchPathLabel")} value={path} onChange={(e) => setPath(e.target.value)} />
        {api.PickFolder ? <Button icon={FolderOpen} onClick={pick}>{t("set.pickFolder")}</Button> : null}
        {downloads ? <Button kind="ghost" onClick={() => setPath(downloads)}>{t("set.useDownloads")}</Button> : null}
      </div>
      <div className="toolbar watch-rule">
        <span>{t("set.watchEach")}</span>
        <input className="input" type="number" min={1} max={999} style={{ width: 64 }} aria-label={t("col.lifespan")} value={count} onChange={(e) => setCount(e.target.value)} />
        <select className="select" aria-label={t("set.unitLabel")} value={unit} onChange={(e) => setUnit(e.target.value as "h" | "d" | "w")}>
          <option value="h">{t("unit.hour", n)}</option>
          <option value="d">{t("unit.day", n)}</option>
          <option value="w">{t("unit.week", n)}</option>
        </select>
        <span>{t("set.watchAfter")}</span>
        <select className="select" aria-label={t("set.ageLabel")} value={age} onChange={(e) => setAge(e.target.value)}>
          {ages().map(([v, label]) => <option key={v} value={v}>{label}</option>)}
        </select>
        <span className="grow" />
        <Button kind="primary" icon={Plus} onClick={async () => {
          if (!path.trim()) { toast(t("set.needPath"), "error"); return; }
          const reply = await api.SetWatch(path.trim(), lifespan, age);
          if (reply.error) { toast(reply.error, "error"); return; }
          toast(t("set.watchAdded"));
          setPath("");
          bump();
        }}>{t("set.addWatch")}</Button>
      </div>
      <p className="hint watch-preview">{path.trim() ? t("set.watchPreview", path.trim(), lifespanText(lifespan), age === "modified") : t("set.watchHint")}</p>
    </div>
  );
}

// What Obituary has cleared so far, and the three steps every item goes
// through, so the words on the other pages mean something.
function Life({ lifetime, days }: { lifetime?: Lifetime; days: number }) {
  const steps: [string, string][] = [[t("life.step1"), t("life.step1Text")], [t("life.step2"), t("life.step2Text")], [t("life.step3"), t("life.step3Text", days)]];
  return (
    <section className="card life" aria-label={t("life.cleared")}>
      {lifetime ? (
        <div className="life-head">
          <span className="life-label">{t("life.cleared")}</span>
          <b className="life-num">{bytes(lifetime.buriedBytes)}</b>
          <span className="hint life-text">{lifetime.buried ? t("life.text", lifetime.buried, bytes(lifetime.purgedBytes), lifetime.resurrected) : t("life.none")}</span>
        </div>
      ) : null}
      <ol className="life-steps">
        {steps.map(([name, text], i) => <li key={name}><b><span className="life-step-n">{i + 1}</span>{name}</b><span>{text}</span></li>)}
      </ol>
    </section>
  );
}
