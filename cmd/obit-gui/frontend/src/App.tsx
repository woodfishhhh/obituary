import { useCallback, useMemo, useState, type CSSProperties } from "react";
import { useApi } from "./api";
import { RefreshContext, useLoad } from "./hooks";
import { lang, t } from "./i18n";
import { skippedUpdate, skipUpdate } from "./prefs";
import type { PageId } from "./shell";
import type { Variant } from "./theme";
import { Button, ConfirmProvider, FrameProvider, PageBoundary, ToastProvider, useToast } from "./ui";

export type { PageId } from "./shell";

export const navFor = (v: Variant): PageId[] =>
  // In the order things happen: due for burial, then the graveyard.
  v.home === "horizon" ? ["horizon", "decisions", "settings", "logs"] : ["upcoming", "graveyard", "decisions", "settings", "logs"];

// The window: shared providers around the variant's own shell. The page
// chosen survives a change of variant when the new one has it too.
export default function App({ variant, page: forced, style }: { variant: Variant; page?: PageId; style?: CSSProperties }) {
  const api = useApi();
  const nav = navFor(variant);
  const [chosen, setPage] = useState<PageId | null>(null);
  const [version, setVersion] = useState(0);
  const bump = useCallback(() => setVersion((n) => n + 1), []);
  const refresh = useMemo(() => ({ version, bump }), [version, bump]);
  const [frame, setFrame] = useState<HTMLElement | null>(null);
  const { data: s } = useLoad(() => api.Summary(), [api, version]);
  const page = chosen && nav.includes(chosen) ? chosen : forced && nav.includes(forced) ? forced : nav[0];
  const { Shell } = variant;
  const language = lang();
  const tokens = language === "en" && variant.tokensEn ? { ...variant.tokens, ...variant.tokensEn } : variant.tokens;

  return (
    <div className="obit" ref={setFrame} data-nav={variant.nav} data-variant={variant.id} data-dark={variant.dark} data-lang={language} lang={language === "zh" ? "zh-CN" : "en"} style={{ ...(tokens as CSSProperties), ...style, position: "relative" }}>
      <FrameProvider value={frame}>
        <RefreshContext.Provider value={refresh}>
          <ToastProvider>
            <ConfirmProvider>
              {/* Keyed by language: the shell's memoised labels are rebuilt on a switch. */}
              <PageBoundary key={`${variant.id}:${page}:${language}`} onHome={() => setPage(nav[0])}>
                <Shell key={language} page={page} nav={nav} setPage={setPage} summary={s} refresh={bump} />
              </PageBoundary>
              <div className="banners">
                {page !== "settings" ? <UpdateBanner onView={() => setPage("settings")} /> : null}
                {s?.paused ? <PausedBanner onResumed={bump} /> : null}
              </div>
            </ConfirmProvider>
          </ToastProvider>
        </RefreshContext.Provider>
      </FrameProvider>
    </div>
  );
}

// A newer release, until the user opens Settings or skips that version. The
// tray item for it opens this window, so the window says it too.
function UpdateBanner({ onView }: { onView: () => void }) {
  const api = useApi();
  const [skipped, setSkipped] = useState(skippedUpdate);
  const { data } = useLoad(() => (api.Update ? api.Update() : Promise.resolve(null)), [api]);
  if (!data?.newer || !data.latest || data.latest === skipped) return null;
  const latest = data.latest;
  return (
    <div className="update-banner" role="status">
      <span>{t("upd.banner", latest)}</span>
      <Button kind="ghost" onClick={() => { skipUpdate(latest); setSkipped(latest); }}>{t("upd.bannerSkip")}</Button>
      <Button kind="primary" onClick={onView}>{t("upd.bannerView")}</Button>
    </div>
  );
}

// While clean-up is paused, every page says so: countdowns alone would
// suggest things are still being buried on time.
function PausedBanner({ onResumed }: { onResumed: () => void }) {
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
    <div className="paused-banner" role="status">
      <span>{t("pause.banner")}</span>
      <Button kind="primary" onClick={resume}>{t("pause.resume")}</Button>
    </div>
  );
}
