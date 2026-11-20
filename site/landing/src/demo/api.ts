// The backend the window demo talks to. The desktop front end ships an in-memory stand-in
// (@gui/mock) for Settings, rules, logs and proposals; the tracked files, the graveyard and
// the lifetime totals come from ./data, so the calendar is full of believable files.
import { createMockApi } from "@gui/mock";
import type { Api, Burial, Item, Settings } from "@gui/types";
import { demoGraves, demoItems, lifetime, type DemoItem, type DemoLang } from "./data";

const HOUR = 3600e3;
const hoursOf = (lifespan: string) => [...lifespan.toLowerCase().matchAll(/(\d+)([smhdw])/g)].reduce((n, [, c, u]) => n + Number(c) * ({ s: 1 / 3600, m: 1 / 60, h: 1, d: 24, w: 168 } as Record<string, number>)[u], 0);
const idOf = (ref: string) => Number(ref.replace("#", ""));
const wait = <T,>(v: T) => new Promise<T>((r) => setTimeout(() => r(v), 60));

export function createDemoApi(lang: DemoLang): { api: Api; items: () => DemoItem[]; graves: () => Burial[] } {
  const base = createMockApi("normal");
  const items: DemoItem[] = demoItems(lang);
  const burials: Burial[] = demoGraves();

  const api: Api = {
    ...base,
    // The base data counts one failed burial among the decisions; the demo has none.
    Summary: async () => ({ ...(await base.Summary()), decisionCount: (await base.Summary()).decisionCount - 1, trackedCount: items.length, graveCount: burials.length, graveBytes: burials.reduce((n, b) => n + b.size, 0), autoMode: undefined }),
    // A page that sells the app has no "new version" banner in its window.
    Update: async () => ({ ...(await base.Update!()), newer: false, latest: "1.0.0" }),
    Settings: async (): Promise<Settings> => ({ ...(await base.Settings()), lifetime }),
    Upcoming: (within) => {
      const limit = within === "24h" ? 24 * HOUR : within === "7d" ? 168 * HOUR : Infinity;
      return wait({ items: items.filter((it) => +new Date(it.expiresAt) - Date.now() < limit).sort((a, b) => +new Date(a.expiresAt) - +new Date(b.expiresAt)) as Item[] });
    },
    Extend: (refs, lifespan) => {
      const add = (hoursOf(lifespan) || 24) * HOUR;
      return wait({
        results: refs.map((ref) => {
          const it = items.find((x) => x.id === idOf(ref));
          if (it) it.expiresAt = new Date(Math.max(+new Date(it.expiresAt), Date.now()) + add).toISOString();
          return { ref, action: "extended" };
        }),
      });
    },
    Cancel: (refs) => {
      refs.forEach((ref) => { const i = items.findIndex((x) => x.id === idOf(ref)); if (i >= 0) items.splice(i, 1); });
      return wait({ results: refs.map((ref) => ({ ref, action: "canceled" })) });
    },
    Graveyard: ({ filter }) => {
      const rows = burials.filter((b) => !filter || b.path.toLowerCase().includes(filter.toLowerCase()))
        .sort((a, b) => +new Date(b.buriedAt) - +new Date(a.buriedAt)).map((b) => ({ id: b.id, status: "buried", burial: b }));
      return wait({ rows, total: rows.length, pendingCount: 0, hasMore: false, nextAfter: "", restored: [] });
    },
    Resurrect: (refs) => wait({
      results: refs.map((ref) => {
        const i = burials.findIndex((b) => b.id === idOf(ref));
        if (i < 0) return { ref, error: "not found" };
        const [b] = burials.splice(i, 1);
        return { ref, action: "resurrected", path: b.path };
      }),
    }),
    Purge: (refs) => wait({
      results: refs.map((ref) => {
        const i = burials.findIndex((b) => b.id === idOf(ref));
        if (i >= 0) burials.splice(i, 1);
        return { ref, action: "purged" };
      }),
    }),
  };
  return { api, items: () => items, graves: () => burials };
}
