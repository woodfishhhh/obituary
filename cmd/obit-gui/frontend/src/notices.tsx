// The paused, new-version and automatic-registration notices: small chips each shell puts in its own
// top bar (or sidebar), so they never cover the page. Placement B in
// docs/playground/notice-placement.html, chosen by the owner on 2026-11-14.
import { ArrowUpCircle, Inbox, PauseCircle, X } from "lucide-react";
import { useState } from "react";
import { useApi } from "./api";
import { useLoad } from "./hooks";
import { t } from "./i18n";
import { skippedUpdate, skipUpdate } from "./prefs";
import type { ShellProps } from "./shell";
import { useToast } from "./ui";

export default function Notices({ page, setPage, summary: s, refresh }: Pick<ShellProps, "page" | "setPage" | "summary" | "refresh">) {
  return (
    <div className="notices">
      {s?.paused ? <PausedChip onResumed={refresh} /> : null}
      {s?.autoMode && (page === "upcoming" || page === "horizon") ? <AutoChip mode={s.autoMode} onView={() => setPage("settings")} /> : null}
      {page !== "settings" ? <UpdateChip onView={() => setPage("settings")} /> : null}
    </div>
  );
}

// A newer release, until the user opens Settings or skips that version. The
// tray item for it opens this window, so the window says it too.
function UpdateChip({ onView }: { onView: () => void }) {
  const api = useApi();
  const [skipped, setSkipped] = useState(skippedUpdate);
  const { data } = useLoad(() => (api.Update ? api.Update() : Promise.resolve(null)), [api]);
  if (!data?.newer || !data.latest || data.latest === skipped) return null;
  const latest = data.latest;
  return (
    <span className="notice-chip update" role="status" title={t("upd.banner", latest)}>
      <ArrowUpCircle className="notice-icon" aria-hidden />
      <span className="notice-text">{t("upd.chip", latest)}</span>
      <button type="button" className="notice-act" onClick={onView}>{t("upd.bannerView")}</button>
      <button type="button" className="notice-x" aria-label={t("upd.bannerSkip")} title={t("upd.bannerSkip")} onClick={() => { skipUpdate(latest); setSkipped(latest); }}><X aria-hidden /></button>
    </span>
  );
}

// The home page says whether anything goes in by itself, so a file that
// shows up there is never a surprise; Settings switches it.
function AutoChip({ mode, onView }: { mode: string; onView: () => void }) {
  const off = mode === "off";
  const title = off ? t("auto.off") : t("auto.on", t(`dec.mode.${mode as "tiered" | "auto" | "ask"}`));
  return (
    <button type="button" className={`notice-chip auto${off ? "" : " on"}`} title={title} aria-label={title} onClick={onView}>
      <Inbox className="notice-icon" aria-hidden />
      <span className="notice-text">{off ? t("auto.chipOff") : t("auto.chipOn")}</span>
    </button>
  );
}

// While clean-up is paused, every page says so: countdowns alone would
// suggest things are still being buried on time.
function PausedChip({ onResumed }: { onResumed: () => void }) {
  const api = useApi();
  const toast = useToast();
  async function resume() {
    try {
      const plan = await api.PlanPause(false);
      if (plan.changes.length) await api.ApplyPlan(plan.id);
      onResumed();
    } catch (error) {
      toast(String(error), "error");
    }
  }
  return (
    <span className="notice-chip paused" role="status" title={t("pause.banner")}>
      <PauseCircle className="notice-icon" aria-hidden />
      <span className="notice-text">{t("pause.chip")}</span>
      <button type="button" className="notice-act" onClick={resume}>{t("pause.resume")}</button>
    </span>
  );
}
