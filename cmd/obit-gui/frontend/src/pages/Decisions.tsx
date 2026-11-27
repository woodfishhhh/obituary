import { Archive, Check, CheckCheck, EyeOff, FileCode2, Flame, HardDrive, Inbox, RotateCcw, Settings as SettingsIcon, ShieldCheck, ShieldOff, TimerReset, X } from "lucide-react";
import { useState } from "react";
import { errorText, useApi } from "../api";
import { approvalReceipt } from "../approval";
import { t } from "../i18n";
import { bytes, extendedTo, lifespanText, ruleOrigin, rulePattern, underProject, when } from "../format";
import { useLoad, useRefresh } from "../hooks";
import { askDecisions, setAskDecisions } from "../prefs";
import type { AutoHandled, Item, Proposal, ProposalMode, RuleFile } from "../types";
import { Button, Empty, Head, LifeLine, NameCell, Outcome, Seg, Skeleton, isContainer, useConfirm, useNavigate, useToast, type OutcomeNote } from "../ui";
import { extendSpans, spanLabel } from "../variants/shared";

const kinds = ["onboard", "large", "leftovers", "watch-policy", "watch-burial", "graveyard-quota", "container"] as const;
const kindText = (kind: string) => ((kinds as readonly string[]).includes(kind) ? t(`kind.${kind as (typeof kinds)[number]}`) : kind);
const ruleTone: Record<string, string> = { untrusted: "warn", changed: "danger", trusted: "ok" };
export const modes: ProposalMode[] = ["off", "tiered", "auto", "ask"];
const classes = ["created", "regenerable", "container", "auto"] as const;
const classText = (c: string) => ((classes as readonly string[]).includes(c) ? t(`dec.class.${c as (typeof classes)[number]}`) : c);
const ruleText = (status: string): [string, string] => (ruleTone[status] ? [t(`rule.${status as "untrusted" | "changed" | "trusted"}`), ruleTone[status]] : [status, ""]);

export default function Decisions() {
  const api = useApi();
  const { version, bump } = useRefresh();
  const confirm = useConfirm();
  const toast = useToast();
  const { data, error, loading } = useLoad(() => Promise.all([api.Proposals(), api.Rules(), api.Upcoming("24h")]), [api, version]);
  const [note, setNote] = useState<OutcomeNote | null>(null);
  if (loading && !data) return <><Head title={t("page.decisions")} /><div className="card"><Skeleton /></div></>;
  const proposals = data?.[0].proposals ?? [];
  const handled = data?.[0].handled ?? [];
  const mode = data?.[0].mode;
  const graveMaxBytes = data?.[0].graveMaxBytes;
  const rules = data?.[1].rules ?? [];
  const pending = rules.filter((r) => r.status !== "trusted");
  const trusted = rules.filter((r) => r.status === "trusted");
  // A failed burial is due by definition, so the next 24 hours hold them all.
  const failed = (data?.[2].items ?? []).filter((it) => it.lastError && (stuck(it) || !waitingWatch(it.lastError)));
  const waiting = proposals.length + pending.length + failed.filter(stuck).length;
  // An expired folder or oversized quota item is never part of "approve all".
  const batchable = proposals.filter((p) => p.kind !== "watch-burial" && p.kind !== "graveyard-quota");

  // One proposal at a time, each with the fingerprint shown on its card, so a
  // proposal that changed meanwhile is refused instead of half-applied.
  async function approveAll() {
    const ok = !askDecisions() || await confirm({
      title: t("dec.askApproveAll", batchable.length),
      body: <><p>{t("dec.approveAllBody")}</p><ul>{batchable.map((p) => <li key={p.id}>{t("dec.allLine", p.id, p.project, p.candidates.length)}</li>)}</ul></>,
      ok: t("dec.approveAllOk"),
      remember: { label: t("dec.noAsk"), onRemember: () => { setAskDecisions(false); toast(t("dec.noAskToast")); } },
    });
    if (!ok) return;
    const details: [string, string][] = [];
    let failed = 0;
    let done = 0;
    for (const p of batchable) {
      try {
        const reply = await api.Approve([`#${p.id}`], [p.fingerprint]);
        const receipt = approvalReceipt(reply, t("dec.noReply"));
        if (receipt.partial) failed++; else done++;
        details.push(...candidateDetails(receipt).map(([path, why]): [string,string] => [`#${p.id}: ${path}`, why]));
        if (receipt.error || !receipt.rows.length) details.push([`#${p.id}`, receipt.error || t("dec.appliedItem")]);
      } catch (error) {
        failed++;
        details.push([`#${p.id}`, errorText(error)]);
      }
    }
    bump();
    if (failed) {
      setNote({ title: t("dec.allPartial", done, failed), items: details });
      toast(t("dec.allPartial", done, failed), "error");
    } else {
      setNote(null);
      toast(t("dec.allDone", done));
    }
  }
  return (
    <>
      <Head title={t("page.decisions")}>
        {waiting ? t("dec.deck", waiting) : t("dec.deckEmpty")}
      </Head>
      {error ? <div className="notice error">{t("dec.readError", error)}</div> : null}
      {note ? <Outcome note={note} onClose={() => setNote(null)} /> : null}
      {mode && api.PlanProposalMode ? <ModeCard mode={mode} /> : null}
      {!waiting && !failed.length ? <div className="card"><Empty icon={Inbox} title={t("dec.none")} text={t("dec.noneText")} /></div> : null}
      {failed.length ? <FailedList items={failed} /> : null}
      {proposals.length ? (
        <div className="section-title section-row">
          <span>{t("dec.proposals")}</span>
          {batchable.length > 1 ? <Button kind="primary" icon={CheckCheck} onClick={approveAll}>{t("dec.approveAll", batchable.length)}</Button> : null}
        </div>
      ) : null}
      {proposals.map((p) => p.kind === "graveyard-quota"
        ? <QuotaDecisionCard key={`${p.id}-${p.fingerprint}`} p={p} graveMaxBytes={graveMaxBytes} onOutcome={setNote} />
        : <ProposalCard key={`${p.id}-${p.fingerprint}`} p={p} onOutcome={setNote} />)}
      {pending.length ? <div className="section-title">{t("dec.rules")}</div> : null}
      {pending.map((r) => <RuleCard key={r.path} r={r} />)}
      {trusted.length ? <details><summary className="hint">{t("dec.trusted", trusted.length)}</summary>{trusted.map((r) => <RuleCard key={r.path} r={r} />)}</details> : null}
      {handled.length ? <HandledList handled={handled} /> : null}
    </>
  );
}

// The proposal mode, switched where proposals are decided. Going fully
// automatic widens what agents may do, so it asks first; the rest apply at once.
export function ModeCard({ mode }: { mode: string }) {
  const api = useApi();
  const { bump } = useRefresh();
  const confirm = useConfirm();
  const toast = useToast();
  const current = (modes as string[]).includes(mode) ? (mode as ProposalMode) : "off";
  async function choose(next: ProposalMode) {
    if (next === current || !api.PlanProposalMode) return;
    if (next === "auto" && !await confirm({ title: t("dec.askAuto"), body: <p>{t("dec.askAutoBody")}</p>, ok: t("dec.askAutoOk") })) return;
    try {
      const plan = await api.PlanProposalMode(next);
      await api.ApplyPlan(plan.id);
      toast(t("dec.modeDone", t(`dec.mode.${next}`)));
    } catch (error) {
      toast(errorText(error), "error");
    }
    bump();
  }
  return (
    <section className="card panel mode-card">
      <div className="panel-head">
        <span className="panel-title grow">{t("dec.mode")}</span>
        <Seg<ProposalMode> label={t("dec.mode")} value={current} onChange={choose} options={modes.map((m): [ProposalMode, string] => [m, t(`dec.mode.${m}`)])} />
      </div>
      <p className="hint">{t(`dec.modeDesc.${current}`)}</p>
    </section>
  );
}

// Retries stop after repeated failures (the backend's far-future NextAttempt).
const stuck = (it: Item) => !!it.nextAttempt?.startsWith("9999");
// A folder-policy item waiting for its folder to go quiet has not failed
// (the backend's texts, as obit doctor matches them).
const waitingWatch = (e: string) => e.startsWith("folder child is still active") || e.startsWith("download temporary name is still present:");

// Burials that failed, with the reason: try again now, or stop tracking and
// leave the file where it is.
function FailedList({ items }: { items: Item[] }) {
  const api = useApi();
  const { bump } = useRefresh();
  const confirm = useConfirm();
  const toast = useToast();
  async function act(it: Item, retry: boolean) {
    if (!retry && !await confirm({ title: t("ask.cancel", 1), body: <><p>{t("ask.cancelBody")}</p><ul><li>{it.path}</li></ul></>, ok: t("stopTracking"), danger: true })) return;
    const reply = await (retry && api.Retry ? api.Retry : api.Cancel)([`#${it.id}`]);
    const failure = reply.error || reply.results.find((r) => r.error)?.error;
    if (failure) toast(failure, "error");
    else toast(retry ? t("dec.retried", it.path) : t("dec.undone", it.path));
    bump();
  }
  return (
    <>
      <div className="section-title">{t("dec.burials")}</div>
      <p className="hint">{t("dec.burialsHint")}</p>
      <div className="card list">
        {items.map((it) => (
          <div key={it.id} className="row cols-failed">
            <NameCell path={it.path} isDir={it.isDir} kind={it.kind} />
            <span className="reason">{it.lastError}</span>
            <span><span className={`tag ${stuck(it) ? "danger" : "warn"}`}>{stuck(it) ? t("dec.stuck") : t("dec.retrying", when(it.nextAttempt))}</span></span>
            <span className="acts">
              {api.Retry ? <Button icon={RotateCcw} onClick={() => act(it, true)}>{t("up.retry")}</Button> : null}
              <Button kind="danger" icon={EyeOff} onClick={() => act(it, false)}>{t("stopTracking")}</Button>
            </span>
          </div>
        ))}
      </div>
    </>
  );
}

// What went in without asking, while it is still tracked: each can be undone
// (spared; the file stays) or kept longer, like any tracked item.
function HandledList({ handled }: { handled: AutoHandled[] }) {
  const api = useApi();
  const { bump } = useRefresh();
  const confirm = useConfirm();
  const toast = useToast();
  const [span, setSpan] = useState("1d");
  async function undo(h: AutoHandled) {
    if (askDecisions() && !await confirm({ title: t("dec.askUndo", h.item.path), body: <p>{t("dec.undoBody")}</p>, ok: t("dec.undo"), danger: true })) return;
    const reply = await api.Cancel([`#${h.item.id}`]);
    const failure = reply.error || reply.results.find((r) => r.error)?.error;
    if (failure) toast(failure, "error");
    else toast(t("dec.undone", h.item.path));
    bump();
  }
  async function keep(h: AutoHandled) {
    const reply = await api.Extend([`#${h.item.id}`], span);
    const failure = reply.error || reply.results.find((r) => r.error)?.error;
    if (failure) toast(failure, "error");
    else toast(t("dec.kept", spanLabel(span)));
    bump();
  }
  return (
    <>
      <div className="section-title section-row">
        <span>{t("dec.handled")}</span>
        <select className="select" aria-label={t("howLong")} value={span} onChange={(e) => setSpan(e.target.value)}>
          {extendSpans().map(([v, label]) => <option key={v} value={v}>{t("extendBy", label)}</option>)}
        </select>
      </div>
      <p className="hint">{t("dec.handledHint")}</p>
      <div className="card list">
        {handled.map((h) => (
          <div key={h.item.id} className="row cols-handled">
            <NameCell path={h.item.path} isDir={h.item.isDir} kind={h.item.kind} />
            <span><span className="tag">{classText(h.class)}</span></span>
            <LifeLine start={h.item.createdAt} end={h.item.expiresAt} left={(s) => t("life.left", s)} over={t("life.expired")} />
            <span className="acts">
              <Button icon={TimerReset} onClick={() => keep(h)}>{t("dec.keep")}</Button>
              <Button kind="danger" icon={EyeOff} onClick={() => undo(h)}>{t("dec.undo")}</Button>
            </span>
          </div>
        ))}
      </div>
    </>
  );
}

// The fingerprint sent back is the version shown on this card: a proposal
// that gained or lost candidates since is refused, never half-applied.
const PROPOSAL_CHANGED = "proposal_changed";

function candidateDetails(receipt: ReturnType<typeof approvalReceipt>): [string,string][] {
  return receipt.rows.map(r => [r.ref, r.pending ? t("dec.pendingItem", r.error ?? "") : r.error ? t("dec.failedItem", r.error) : t("dec.appliedItem")]);
}

function QuotaDecisionCard({ p, graveMaxBytes, onOutcome }: { p: Proposal; graveMaxBytes?: number; onOutcome: (note: OutcomeNote | null) => void }) {
  const api = useApi();
  const { bump } = useRefresh();
  const confirm = useConfirm();
  const toast = useToast();
  const navigate = useNavigate();
  const [span, setSpan] = useState("1d");
  const c = p.candidates[0];
  const itemBytes = p.quotaReview?.itemBytes ?? c?.size ?? 0;
  const recordedMax = p.quotaReview?.maxBytes ?? 0;
  const effectiveMax = graveMaxBytes !== undefined ? graveMaxBytes : recordedMax;
  const fitsNow = graveMaxBytes !== undefined && (graveMaxBytes === 0 || itemBytes <= graveMaxBytes);
  const path = c?.path ?? "";

  async function approveItem() {
    try {
      const reply = await api.Approve([`#${p.id}`], [p.fingerprint]);
      const receipt = approvalReceipt(reply, t("dec.noReply"));
      if (receipt.error) {
        const changed = receipt.code === PROPOSAL_CHANGED;
        onOutcome(changed
          ? { title: t("dec.changed", p.id, true), text: t("dec.changedText") }
          : { title: t("dec.failed", p.id, true), text: receipt.error });
        toast(changed ? t("dec.changedToast", p.id) : receipt.error, "error");
      } else {
        onOutcome(null);
        const ev = receipt.quotaEviction;
        if (ev && ev.purgedCount > 0) {
          toast(t("quota.dec.buriedEvicted", ev.purgedCount, bytes(ev.freedBytes), bytes(ev.afterBytes)));
        } else if (ev && (ev.failedCount ?? 0) > 0) {
          toast(t("quota.dec.buriedFailed", ev.failedCount!), "error");
        } else {
          toast(t("dec.buried"));
        }
      }
    } catch (error) {
      toast(errorText(error), "error");
    }
    bump();
  }

  async function extendItem() {
    if (!api.ExtendQuotaProposal) return;
    try {
      const reply = await api.ExtendQuotaProposal(`#${p.id}`, p.fingerprint, span);
      const failure = reply.error || reply.results[0]?.error;
      if (failure) {
        onOutcome({ title: t("dec.failed", p.id, true), text: failure });
        toast(failure, "error");
      } else {
        onOutcome(null);
        toast(t("quota.dec.extended", spanLabel(span)));
      }
    } catch (error) {
      toast(errorText(error), "error");
    }
    bump();
  }

  async function spareItem() {
    const ok = await confirm({
      title: t("ask.cancel", 1),
      body: <><p>{t("ask.cancelBody")}</p><ul><li>{path}</li></ul></>,
      ok: t("stopTracking"),
    });
    if (!ok) return;
    try {
      const reply = await api.Reject([`#${p.id}`], [p.fingerprint]);
      const failure = reply.error || reply.results[0]?.error;
      if (failure) {
        onOutcome({ title: t("dec.failed", p.id, false), text: failure });
        toast(failure, "error");
      } else {
        onOutcome(null);
        toast(t("quota.dec.spared", path));
      }
    } catch (error) {
      toast(errorText(error), "error");
    }
    bump();
  }

  async function purgeItem() {
    if (!api.PurgeQuotaProposal) return;
    const ok = await confirm({
      title: t("quota.dec.askPurge"),
      body: <><p>{t("quota.dec.purgeBody", bytes(itemBytes))}</p><ul><li>{path}</li></ul></>,
      ok: t("quota.dec.purge"),
      danger: true,
    });
    if (!ok) return;
    try {
      const reply = await api.PurgeQuotaProposal(`#${p.id}`, p.fingerprint);
      const failure = reply.error || reply.results[0]?.error;
      if (failure) {
        onOutcome({ title: t("dec.failed", p.id, true), text: failure });
        toast(failure, "error");
      } else {
        onOutcome(null);
        toast(t("quota.dec.purged", path));
      }
    } catch (error) {
      toast(errorText(error), "error");
    }
    bump();
  }

  return (
    <section className="card panel quota-dec-card">
      <div className="panel-head">
        <HardDrive size={16} aria-hidden />
        <span className="panel-title">{kindText(p.kind)}</span>
        {fitsNow
          ? <span className="tag ok">{effectiveMax > 0 ? `${bytes(itemBytes)} / ${bytes(effectiveMax)}` : t("quota.unlimited")}</span>
          : effectiveMax > 0 ? <span className="tag warn">{bytes(itemBytes)} / {bytes(effectiveMax)}</span> : null}
        <span className="grow" />
        <span className="hint">#{p.id}，{when(p.createdAt)}</span>
      </div>
      <p className="quota-dec-reason">
        {fitsNow
          ? t("quota.dec.fitsReason", bytes(itemBytes), effectiveMax > 0 ? bytes(effectiveMax) : t("quota.unlimited"))
          : effectiveMax > 0 ? t("quota.dec.reason", bytes(itemBytes), bytes(effectiveMax)) : c?.rule}
      </p>
      {c ? (
        <div className="quota-dec-item">
          <NameCell path={path} isDir={p.quotaReview?.isDir} />
          <span className="num">{bytes(itemBytes)}</span>
        </div>
      ) : null}
      <div className="quota-dec-foot">
        {fitsNow ? <Button kind="primary" icon={Archive} onClick={approveItem}>{t("quota.dec.buryNow")}</Button> : null}
        {api.ExtendQuotaProposal ? (
          <>
            <select className="select" aria-label={t("howLong")} value={span} onChange={(e) => setSpan(e.target.value)}>
              {extendSpans().map(([v, label]) => <option key={v} value={v}>{t("extendBy", label)}</option>)}
            </select>
            <Button icon={TimerReset} onClick={extendItem}>{t("extend")}</Button>
          </>
        ) : null}
        <Button icon={EyeOff} onClick={spareItem}>{t("stopTracking")}</Button>
        {!fitsNow && navigate ? <Button icon={SettingsIcon} onClick={() => navigate("settings")}>{t("quota.dec.raiseLimit")}</Button> : null}
        <span className="grow" />
        {api.PurgeQuotaProposal ? <Button kind="danger" icon={Flame} onClick={purgeItem}>{t("quota.dec.purge")}</Button> : null}
      </div>
    </section>
  );
}

function ProposalCard({ p, onOutcome }: { p: Proposal; onOutcome: (note: OutcomeNote | null) => void }) {
  const api = useApi();
  const { bump } = useRefresh();
  const confirm = useConfirm();
  const toast = useToast();
  const total = p.candidates.reduce((n, c) => n + c.size, 0);
  const burial = p.kind === "watch-burial";
  const policy = p.kind === "watch-policy";
  // A policy is about the folder, a container has no file size: neither shows bytes.
  const sized = !policy && p.candidates.some((c) => !isContainer(c.kind));
  const n = p.candidates.length;
  // The confirmation names what changes and when, not the proposal number.
  const items = <ul>{p.candidates.map((c) => {
    const due = !burial && !policy ? extendedTo(new Date().toISOString(), c.lifespan) : null;
    return <li key={c.path}>{underProject(c.path, p.project)}{due ? <b className="moves">{t("dec.dueAt", lifespanText(c.lifespan), when(due))}</b> : null}</li>;
  })}</ul>;
  async function decide(approve: boolean) {
    // Once the owner says a decision is a decision, the buttons act at once.
    const ok = !askDecisions() || await confirm({
      title: policy ? (approve ? t("dec.askPolicy", p.project) : t("dec.askNoPolicy", p.project))
        : burial ? (approve ? t("dec.askApprove", p.id) : t("dec.askReject", p.id))
        : approve ? t("dec.askStart", n) : t("dec.askSkip", n),
      body: <><p>{policy ? t(approve ? "dec.policyBody" : "dec.noPolicyBody") : burial ? t(approve ? "dec.buryBody" : "dec.spareBody") : p.kind === "onboard" ? t(approve ? "dec.onboardBody" : "dec.onboardRejectBody") : approve ? t("dec.approveBody", n) : t("dec.rejectBody")}</p>{policy ? null : items}</>,
      ok: approve ? t("approve") : t("reject"),
      danger: burial ? approve : !approve,
      remember: { label: t("dec.noAsk"), onRemember: () => { setAskDecisions(false); toast(t("dec.noAskToast")); } },
    });
    if (!ok) return;
    const ref = `#${p.id}`;
    let receipt: ReturnType<typeof approvalReceipt> | undefined;
    let failure: string | undefined;
    let failureCode: string | undefined;
    try {
      const reply = await (approve ? api.Approve : api.Reject)([ref], [p.fingerprint]);
      receipt = approvalReceipt(reply, t("dec.noReply"));
      failure = receipt.error;
      failureCode = receipt.code;
    } catch (error) {
      failure = errorText(error);
    }
    bump();
    if (approve && receipt?.partial && receipt.rows.length) {
      const title = t("dec.partial", p.id);
      onOutcome({ title, text: failure, items: candidateDetails(receipt) });
      toast(title, "error");
      return;
    }
    if (failure) {
      const changed = failureCode === PROPOSAL_CHANGED;
      onOutcome(changed
        ? { title: t("dec.changed", p.id, approve), text: t("dec.changedText") }
        : { title: t("dec.failed", p.id, approve), text: failure });
      toast(changed ? t("dec.changedToast", p.id) : t("dec.failedToast", approve, failure), "error");
      return;
    }
    const done = receipt?.rows.length || p.candidates.length;
    const ev = receipt?.quotaEviction;
    onOutcome(null);
    if (approve && ev && ev.purgedCount > 0) {
      toast(t("quota.dec.buriedEvicted", ev.purgedCount, bytes(ev.freedBytes), bytes(ev.afterBytes)));
    } else if (approve && ev && (ev.failedCount ?? 0) > 0) {
      toast(t("quota.dec.buriedFailed", ev.failedCount!), "error");
    } else {
      toast(approve ? burial ? t("dec.buried") : t("dec.approved", done) : t("dec.rejected", p.id));
    }
  }
  return (
    <section className="card panel">
      <div className="panel-head">
        <span className="panel-title">{kindText(p.kind)}</span>
        <span className="name-sub grow">{p.project}</span>
        <span className="hint">#{p.id}，{when(p.createdAt)}</span>
      </div>
      <div className="card list">
        {p.candidates.map((c) => (
          <div key={c.path} className="row cols-watch">
            <span className="name-sub" title={c.path}>{underProject(c.path, p.project)}</span>
            <span className="tag">{burial ? t("dec.expired") : lifespanText(c.lifespan)}</span>
            <span className="hint" title={c.rule}>{c.rule ? <><code>{rulePattern(c.rule)}</code> {ruleOrigin(c.rule)}</> : null}</span>
            <span className="num">{policy || isContainer(c.kind) ? t("container.size") : bytes(c.size)}</span>
            <span />
          </div>
        ))}
      </div>
      <div className="panel-foot">
        <span className="hint grow">{sized ? t("dec.total", n, bytes(total)) : t("dec.count", n)}</span>
        <Button icon={X} onClick={() => decide(false)}>{t("reject")}</Button>
        <Button kind="primary" icon={Check} disabled={p.blocked === "docker-unavailable"} onClick={() => decide(true)}>{t("approve")}</Button>
      </div>
      {p.blocked === "docker-unavailable" ? <div className="notice warn">{t("dec.dockerDown")}</div> : null}
    </section>
  );
}

function RuleCard({ r }: { r: RuleFile }) {
  const api = useApi();
  const { bump } = useRefresh();
  const confirm = useConfirm();
  const toast = useToast();
  const [label, tone] = ruleText(r.status);
  const rules = ruleLines(r.content);
  async function trust() {
    const ok = await confirm({ title: t("dec.askTrust"), body: <><p>{t("dec.trustBody")}</p><ul><li>{r.path}</li>{rules.map((x, i) => <li key={i}>{x.pattern}<b className="moves">{x.lifespan ? t("dec.ruleBury", lifespanText(x.lifespan)) : t("dec.ruleKeep")}</b></li>)}</ul></>, ok: t("trust") });
    if (!ok) return;
    // The reviewed hash pins exactly the content shown; a later edit is refused.
    await api.Trust(r.path, r.sha256 ?? "");
    toast(t("dec.trustedToast"));
    bump();
  }
  async function untrust() {
    if (!await confirm({ title: t("dec.askUntrust"), body: <ul><li>{r.path}</li></ul>, ok: t("untrust"), danger: true })) return;
    await api.Untrust(r.path);
    bump();
  }
  return (
    <section className="card panel">
      <div className="panel-head">
        <FileCode2 size={16} aria-hidden />
        <span className="name-sub grow">{r.path}</span>
        <span className={`tag ${tone}`}>{label}</span>
      </div>
      {r.warning ? <div className="notice warn">{r.warning}</div> : null}
      {rules.length ? (
        <ul className="rule-summary" aria-label={t("dec.ruleEffect")}>
          {rules.map((x, i) => <li key={i}><code>{x.pattern}</code><span>{x.lifespan ? t("dec.ruleBury", lifespanText(x.lifespan)) : t("dec.ruleKeep")}</span></li>)}
        </ul>
      ) : null}
      <details className="rule-raw">
        <summary className="hint">{t("dec.ruleRaw")}{r.sha256 ? ` · ${t("dec.sha", r.sha256.slice(0, 12))}` : ""}</summary>
        <pre className="well">{r.content || t("dec.emptyFile")}</pre>
      </details>
      <div className="panel-foot">
        <span className="hint grow">{rules.length ? t("dec.ruleCount", rules.length) : ""}</span>
        {r.status === "trusted"
          ? <Button kind="danger" icon={ShieldOff} onClick={untrust}>{t("untrust")}</Button>
          : <Button kind="primary" icon={ShieldCheck} disabled={!r.sha256} onClick={trust}>{t("trust")}</Button>}
      </div>
    </section>
  );
}

// Each line of a rule file as what it does: `pattern lifespan` buries what
// matches after that long, `!pattern` keeps it out. Comments are skipped.
function ruleLines(content: string): { pattern: string; lifespan?: string }[] {
  return content.replace(/^\uFEFF/, "").split(/\r?\n/).flatMap((raw) => {
    const line = raw.replace(/\s#.*$/, "").trim();
    if (!line || line.startsWith("#")) return [];
    const [pattern, lifespan] = line.split(/\s+/);
    return [{ pattern, lifespan: pattern.startsWith("!") ? undefined : lifespan }];
  });
}
