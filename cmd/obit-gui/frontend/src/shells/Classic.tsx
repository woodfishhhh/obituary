// The classic window of variants A to E: a sidebar or a top bar (the
// variant's `nav`) over the shared pages. E puts the horizon scene first.
import { Archive, Hourglass, Inbox, RefreshCw, ScrollText, Settings2, Sunrise, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { bytes } from "../format";
import Decisions from "../pages/Decisions";
import Graveyard from "../pages/Graveyard";
import Horizon from "../pages/Horizon";
import Logs from "../pages/Logs";
import Settings from "../pages/Settings";
import Upcoming from "../pages/Upcoming";
import { t } from "../i18n";
import { pageTitle, type PageId, type ShellProps } from "../shell";

const pages: Record<PageId, { icon: LucideIcon; view: () => ReactNode }> = {
  horizon: { icon: Sunrise, view: () => <Horizon /> },
  graveyard: { icon: Archive, view: () => <div className="page"><Graveyard /></div> },
  upcoming: { icon: Hourglass, view: () => <div className="page"><Upcoming /></div> },
  decisions: { icon: Inbox, view: () => <div className="page"><Decisions /></div> },
  settings: { icon: Settings2, view: () => <div className="page"><Settings /></div> },
  logs: { icon: ScrollText, view: () => <div className="page"><Logs /></div> },
};

export default function ClassicShell({ page, nav, setPage, summary: s, refresh }: ShellProps) {
  return (
    <>
      <nav className="nav" aria-label={t("pagesLabel")}>
        <div className="brandmark"><img src="icon.png" alt="" />{t("brand")}</div>
        {nav.map((id) => {
          const { icon: Icon } = pages[id];
          const title = pageTitle(id);
          const count = id === "decisions" ? s?.decisionCount : 0;
          return (
            <button key={id} type="button" className="nav-item" aria-current={page === id ? "page" : undefined} onClick={() => setPage(id)}>
              <Icon size={17} aria-hidden />
              <span className="label">{title}</span>
              {count ? <span className="count">{count}</span> : null}
            </button>
          );
        })}
        <span className="nav-spacer" />
        {s ? (
          <div className="nav-summary">
            <span>{t("sum.tracked", s.trackedCount)}</span>
            <span>{t("sum.graves", s.graveCount, bytes(s.graveBytes))}</span>
            {s.paused ? <span>{t("paused")}</span> : null}
          </div>
        ) : null}
        <button type="button" className="nav-item" aria-label={t("refresh")} title={t("refresh")} onClick={refresh}><RefreshCw size={16} aria-hidden /></button>
      </nav>
      <main className="main" key={page}>{pages[page].view()}</main>
    </>
  );
}
