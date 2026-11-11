// The looks the app ships, in the order the 外观 setting lists them. Each
// one is its own chunk: the app loads the chosen look at start and the others
// only when picked. Variants not listed here (C, D, G) live only in the
// playground and never reach dist.
import { t } from "./i18n";
import type { Variant } from "./theme";

export interface Look {
  id: string;
  readonly name: string;
  readonly note: string;
  load: () => Promise<Variant>;
}

const load = {
  calendar: () => import("./variants/calendar/calendar"),
  palette: () => import("./variants/palette/palette"),
  tunnel: () => import("./variants/tunnel/tunnel"),
  vigil: () => import("./variants/vigil/vigil"),
  notice: () => import("./variants/notice/notice"),
  horizon: () => import("./variants/horizon/horizon"),
  mica: () => import("./variants/mica/mica"),
  night: () => import("./variants/night/night"),
};

export const looks: Look[] = (Object.keys(load) as (keyof typeof load)[]).map((id) => ({
  id,
  get name() { return t(`look.${id}`); },
  get note() { return t(`look.${id}Note`); },
  load: () => load[id]().then((m) => m.default),
}));
