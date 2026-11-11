import { t } from "./i18n";
import type { Summary } from "./types";

export type PageId = "horizon" | "graveyard" | "upcoming" | "decisions" | "settings" | "logs";

export const pageTitle = (id: PageId) => t(`page.${id}`);

// A variant with its own shell gets the page state and the summary; the data
// layer (useApi, the refresh context, confirm and toasts) stays shared.
export interface ShellProps {
  page: PageId;
  nav: PageId[];
  setPage: (page: PageId) => void;
  summary?: Summary;
  refresh: () => void;
}
