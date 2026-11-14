// An in-memory stand-in for the Go backend, used by `pnpm dev` and the
// playground. Actions change its state, so a preview behaves like the app.
import type { Api, AutoHandled, Burial, Candidate, Item, Plan, PlanReceipt, Proposal, ProposalMode, ProposalResult, RuleFile, Settings, UpdateInfo, WatchPolicy } from "./types";

export type Scenario = "normal" | "busy" | "empty";

const hour = 3600e3;
const at = (hours: number) => new Date(Date.now() + hours * hour).toISOString();
const DL = "C:\\Users\\me\\Downloads";
const SHOP = "D:\\work\\shop";

// A proposal as the mock keeps it: `fail` makes named candidates fail on
// approval (a partial approval), `drift` is a candidate an agent adds just
// before the first decision (so the fingerprint the user saw is stale).
type MockProposal = Omit<Proposal, "fingerprint"> & { fail?: Record<string, string>; drift?: Candidate };

// Same forms as core.ParseDuration: 30m, 2h, 3d, 1w, 1d12h.
const unitHours: Record<string, number> = { s: 1 / 3600, m: 1 / 60, h: 1, d: 24, w: 168 };
const hoursOf = (lifespan: string) => [...lifespan.toLowerCase().matchAll(/(\d+)([smhdw])/g)].reduce((n, [, count, u]) => n + Number(count) * unitHours[u], 0);

function fingerprint(p: MockProposal) {
  let h = 5381;
  for (const ch of `${p.id}|${p.candidates.map((c) => `${c.path}@${c.lifespan}`).join("|")}`) h = ((h * 33) ^ ch.charCodeAt(0)) >>> 0;
  return h.toString(16).padStart(8, "0");
}
const view = ({ fail: _fail, drift: _drift, ...p }: MockProposal): Proposal => ({ ...p, fingerprint: fingerprint(p) });

function seed(scenario: Scenario) {
  if (scenario === "empty") return { burials: [] as Burial[], items: [] as Item[], proposals: [] as MockProposal[], rules: [] as RuleFile[], receipts: [] as Receipt[] };
  const burials: Burial[] = [
    { id: 12, path: `${DL}\\setup-node-22.11.0-x64.msi`, isDir: false, size: 31457280, buriedAt: at(-30), purgeAt: at(138) },
    { id: 11, path: `${SHOP}\\.venv`, isDir: true, size: 412090368, buriedAt: at(-96), purgeAt: at(72), note: "agent 会话留下的虚拟环境" },
    { id: 9, path: `${SHOP}\\debug-2026-11-08.log`, isDir: false, size: 1843200, buriedAt: at(-150), purgeAt: at(18) },
    { id: 8, path: `${DL}\\IMG_20261102_0931.HEIC`, isDir: false, size: 3355443, buriedAt: at(-60), purgeAt: at(108) },
    { id: 7, path: "docker:shop-api-dev (5e1b07c9d2aa)", isDir: false, kind: "container", size: 0, buriedAt: at(-20), purgeAt: at(148) },
  ];
  const items: Item[] = [
    { id: 21, path: `${SHOP}\\screenshot-checkout.png`, isDir: false, createdAt: at(-23), expiresAt: at(0.4), source: "rule" },
    { id: 22, path: `${SHOP}\\notes-draft.md`, isDir: false, createdAt: at(-4), expiresAt: at(20), source: "cli", note: "周五前整理进文档" },
    { id: 23, path: `${DL}\\report-q3.csv`, isDir: false, createdAt: at(-200), expiresAt: at(136), source: "watch" },
    { id: 24, path: `${SHOP}\\coverage`, isDir: true, createdAt: at(-2), expiresAt: at(70), source: "rule", note: "rule coverage" },
    { id: 25, path: `${DL}\\obs-recording-1102.mkv`, isDir: false, createdAt: at(-10), expiresAt: at(3), source: "watch" },
    { id: 26, path: "docker:shop-db-test (3f9c2a1b7e04)", isDir: false, kind: "container", createdAt: at(-20), expiresAt: at(12), source: "cli" },
    { id: 27, path: `${SHOP}\\dist`, isDir: true, createdAt: at(-30), expiresAt: at(42), source: "proposal", note: "auto-approved #29 (regenerable)" },
    { id: 28, path: `${SHOP}\\scratch-repro.py`, isDir: false, createdAt: at(-1), expiresAt: at(47), source: "rule", note: "rule *.py (.obitrules:3)" },
    { id: 29, path: "docker:shop-redis-1 (a81d93c0f2b6)", isDir: false, kind: "container", createdAt: at(-0.5), expiresAt: at(71.5), source: "proposal", note: "auto-approved #30 (container)" },
  ];
  // Registrations made without asking (tiered mode), shown under "Handled automatically".
  const receipts: Receipt[] = [
    { id: 28, class: "created", at: at(-1) },
    { id: 29, class: "container", at: at(-0.5) },
    { id: 27, class: "regenerable", at: at(-30) },
  ];
  if (scenario === "busy") {
    for (let i = 0; i < 26; i++) {
      items.push({ id: 100 + i, path: `${SHOP}\\tmp\\render-${String(i).padStart(3, "0")}.png`, isDir: false, createdAt: at(-5), expiresAt: at(1 + i * 6), source: "rule" });
      burials.push({ id: 200 + i, path: `${SHOP}\\.cache\\chunk-${i}.bin`, isDir: false, size: 1048576 * (i + 1), buriedAt: at(-i * 5), purgeAt: at(168 - i * 5) });
    }
  }
  const proposals: MockProposal[] = [
    { id: 31, kind: "leftovers", project: SHOP, createdAt: at(-1), candidates: [
      { path: `${SHOP}\\tmp\\fixtures.json`, lifespan: "3d", rule: "会话里新建、没被 Git 跟踪", size: 20480 },
      { path: `${SHOP}\\coverage-old`, lifespan: "1w", rule: "*/coverage", size: 5242880 },
    ], fail: { [`${SHOP}\\coverage-old`]: "目录里有被 Git 跟踪的文件，没有登记" } },
    { id: 33, kind: "large", project: SHOP, createdAt: at(-0.3), candidates: [
      { path: `${SHOP}\\data\\dump-2026-11.sql`, lifespan: "1w", rule: "大于 1 GB", size: 1288490188 },
    ], drift: { path: `${SHOP}\\data\\dump-2026-11-b.sql`, lifespan: "1w", rule: "大于 1 GB", size: 1395864371 } },
    { id: 32, kind: "watch-policy", project: DL, createdAt: at(-3), candidates: [
      { path: DL, lifespan: "2w", rule: "文件夹策略：放进来的东西两周后进墓地", size: 0 },
    ] },
  ];
  const rules: RuleFile[] = [
    { path: `${SHOP}\\.obitrules`, status: "untrusted", sha256: "9f2c4e1a7b3d5f60a1c2d3e4", content: "# 构建产物一周后清理\ndist/ 1w\n*.log 3d\ncoverage/ 1w\n" },
  ];
  return { burials, items, proposals, rules, receipts };
}

type Receipt = { id: number; class: AutoHandled["class"]; at: string };

export function createMockApi(scenario: Scenario = "normal"): Api {
  const s = seed(scenario);
  // "auto" resolves the way the backend does from the Windows display
  // language; the browser's language stands in for it here.
  const windowsLanguage = () => (navigator.language.toLowerCase().startsWith("zh") ? "zh-CN" : "en");
  const settings: Settings = { mode: "graveyard", retentionDays: 7, configPath: "C:\\Users\\me\\AppData\\Roaming\\obituary\\config.toml", contextMenuInstalled: true, contextMenuCurrent: true, contextMenuDetail: "", autostart: true, paused: false, language: "auto", effectiveLanguage: windowsLanguage(), sessionSummary: false, sessionCleanup: true, downloads: DL, trayIcon: true, taskbar: true, startMenu: true, desktopShortcut: true,
    lifetime: scenario === "empty" ? { buried: 0, buriedBytes: 0, purgedBytes: 0, resurrected: 0 } : { buried: 216, buriedBytes: 1_395_864_371, purgedBytes: 29_884_416, resurrected: 2 } };
  const watches: WatchPolicy[] = scenario === "empty" ? [] : [{ id: 3, path: DL, lifespan: "2w", age: "first-seen", createdAt: at(-500), consent: "approved", rev: 1, approvedRev: 1 }];
  const plans = new Map<string, () => PlanReceipt>();
  const update: UpdateInfo = scenario === "empty"
    ? { current: "1.0.0", newer: false, enabled: true, channel: "scoop", checkedAt: at(-3), latest: "1.0.0", url: "https://github.com/woodfishhhh/obituary/releases/tag/v1.0.0" }
    : { current: "1.0.0", latest: "1.0.1", newer: true, name: "1.0.1", published: at(-20), checkedAt: at(-2), enabled: true, channel: "scoop", command: "scoop update obituary",
        url: "https://github.com/woodfishhhh/obituary/releases/tag/v1.0.1",
        notes: ["## 修复", "", "- 托盘菜单在高 DPI 屏幕上不再模糊", "- 文件夹策略遇到只读文件时会跳过并记日志，不再整轮停下", "", "## 改进", "", "- 墓地页搜索支持按扩展名过滤"].join("\n") };
  let proposalMode: ProposalMode = "tiered";
  // Receipts follow their items, as core.State.AutoHandled does.
  const handled = (): AutoHandled[] => s.receipts.flatMap((r) => {
    const item = s.items.find((it) => it.id === r.id);
    return item ? [{ item, class: r.class, at: r.at }] : [];
  });
  // The first context-menu change stops part way, as a locked key would.
  let menuStalls = scenario !== "empty";
  let nextItem = 900;
  let planSeq = 0;
  const idOf = (ref: string) => Number(ref.replace("#", ""));
  const wait = <T,>(v: T) => new Promise<T>((r) => setTimeout(() => r(v), 120));
  const plan = (action: string, changes: Plan["changes"], apply: () => void | PlanReceipt): Promise<Plan> => {
    const id = `p${++planSeq}`;
    plans.set(id, () => apply() ?? { action, changed: true, applied: null, remaining: null });
    return wait({ id, action, changes });
  };

  // Approve or reject one proposal the way core does: a stale fingerprint is
  // refused; an approval reports every candidate, failures included.
  function decide(ref: string, seen: string | undefined, approve: boolean): ProposalResult {
    const i = s.proposals.findIndex((p) => p.id === idOf(ref));
    if (i < 0) return { ref, error: "没有这个提议" };
    const p = s.proposals[i];
    if (p.drift) { p.candidates.push(p.drift); p.drift = undefined; }
    if (seen !== fingerprint(p)) return { ref, error: "提议已变化，请重新查看" };
    s.proposals.splice(i, 1);
    if (!approve) return { ref, proposal: view(p) };
    const results = p.candidates.map((c) => {
      const error = p.fail?.[c.path];
      if (!error && p.kind !== "watch-policy") {
        s.items.push({ id: ++nextItem, path: c.path, isDir: !/\.\w+$/.test(c.path), createdAt: new Date().toISOString(), expiresAt: at(hoursOf(c.lifespan)), source: "proposal" });
      }
      return error ? { ref: c.path, error } : { ref: c.path, action: "registered" };
    });
    return { ref, proposal: view(p), results };
  }

  return {
    Summary: () => wait({
      trackedCount: s.items.length, graveCount: s.burials.length, graveBytes: s.burials.reduce((n, b) => n + b.size, 0),
      pendingGraveCount: 0, decisionCount: s.proposals.length + s.rules.filter((r) => r.status !== "trusted").length, paused: settings.paused,
    }),
    Graveyard: ({ filter }) => {
      const rows = s.burials.filter((b) => !filter || b.path.toLowerCase().includes(filter.toLowerCase()))
        .sort((a, b) => +new Date(b.buriedAt) - +new Date(a.buriedAt)).map((b) => ({ id: b.id, status: "buried", burial: b }));
      return wait({ rows, total: rows.length, pendingCount: 0, hasMore: false, nextAfter: "", restored: [] });
    },
    Resurrect: (refs) => {
      const results = refs.map((ref) => {
        const i = s.burials.findIndex((b) => b.id === idOf(ref));
        if (i < 0) return { ref, error: "not found" };
        const [b] = s.burials.splice(i, 1);
        return { ref, action: "resurrected", path: b.path };
      });
      return wait({ results });
    },
    Purge: (refs) => {
      const results = refs.map((ref) => {
        const i = s.burials.findIndex((b) => b.id === idOf(ref));
        if (i >= 0) s.burials.splice(i, 1);
        return { ref, action: "purged" };
      });
      return wait({ results });
    },
    Upcoming: (within) => {
      const limit = within === "24h" ? 24 * hour : within === "7d" ? 168 * hour : Infinity;
      return wait({ items: s.items.filter((it) => +new Date(it.expiresAt) - Date.now() < limit).sort((a, b) => +new Date(a.expiresAt) - +new Date(b.expiresAt)) });
    },
    Extend: (refs, lifespan) => {
      const add = hoursOf(lifespan) || 24;
      const results = refs.map((ref) => {
        const it = s.items.find((x) => x.id === idOf(ref));
        if (it) it.expiresAt = new Date(Math.max(+new Date(it.expiresAt), Date.now()) + add * hour).toISOString();
        return { ref, action: "extended" };
      });
      return wait({ results });
    },
    Cancel: (refs) => {
      refs.forEach((ref) => { const i = s.items.findIndex((x) => x.id === idOf(ref)); if (i >= 0) s.items.splice(i, 1); });
      return wait({ results: refs.map((ref) => ({ ref, action: "canceled" })) });
    },
    Proposals: () => wait({ proposals: s.proposals.map(view), handled: handled(), mode: proposalMode }),
    PlanProposalMode: (mode) => plan("proposalMode", mode === proposalMode ? [] : [{ target: "[proposals] mode", before: proposalMode, after: mode }], () => { proposalMode = mode; }),
    Approve: (refs, fingerprints) => wait({ results: refs.map((ref, i) => decide(ref, fingerprints[i], true)) }),
    Reject: (refs, fingerprints) => wait({ results: refs.map((ref, i) => decide(ref, fingerprints[i], false)) }),
    Rules: () => wait({ rules: s.rules.map((r) => ({ ...r })) }),
    Trust: (path) => { const r = s.rules.find((x) => x.path === path)!; r.status = "trusted"; return wait({ ...r }); },
    Untrust: (path) => { const r = s.rules.find((x) => x.path === path)!; r.status = "untrusted"; return wait({ action: "untrusted", path, changed: true }); },
    Watches: () => wait({ policies: [...watches] }),
    SetWatch: (path, lifespan, age) => {
      // Like the backend: one policy per folder; a change needs approval again.
      const old = watches.find((w) => w.path.toLowerCase() === path.toLowerCase());
      const policy = { id: old?.id ?? watches.length + 10, path, lifespan, age, createdAt: old?.createdAt ?? new Date().toISOString(), consent: "pending", rev: (old?.rev ?? 0) + 1, approvedRev: old?.approvedRev };
      if (old) Object.assign(old, policy); else watches.push(policy);
      return wait({ policy });
    },
    PickFolder: () => wait(DL.replace("Downloads", "Videos\\Captures")),
    ShowInFolder: () => wait(undefined),
    RemoveWatch: (path) => { const i = watches.findIndex((w) => w.path === path); if (i >= 0) watches.splice(i, 1); return wait({ action: "removed", changed: i >= 0 }); },
    Settings: () => wait({ ...settings }),
    PlanSettings: (mode, days) => plan("settings", [
      ...(mode !== settings.mode ? [{ target: "[graveyard] mode", before: settings.mode, after: mode }] : []),
      ...(days !== settings.retentionDays ? [{ target: "[graveyard] retention_days", before: String(settings.retentionDays), after: String(days) }] : []),
    ], () => { settings.mode = mode; settings.retentionDays = days; }),
    PlanContextMenu: (on) => {
      const entries: [string, string, string][] = [
        ["*\\shell\\Obituary.Mark", "文件的右键菜单", "写讣告"],
        ["*\\shell\\Obituary.Resurrect", "文件的右键菜单", "复活最近下葬的"],
        ["Directory\\shell\\Obituary.Mark", "文件夹的右键菜单", "写讣告"],
        ["Directory\\Background\\shell\\Obituary.Policy", "文件夹空白处的右键菜单", "设置文件夹策略"],
        ["Obituary.ContextMenu", "「写讣告」的子菜单", "1 小时、1 天、7 天、30 天"],
      ];
      const keys = entries.map(([key]) => key);
      const changes = on === settings.contextMenuInstalled && settings.contextMenuCurrent ? [] : entries.map(([key, target, label]) => ({ key, target, before: on ? "" : label, after: on ? label : "" }));
      return plan("contextMenu", changes, () => {
        if (menuStalls) {
          menuStalls = false;
          settings.contextMenuCurrent = false;
          settings.contextMenuDetail = "上次只改了一部分，再切换一次就能补齐";
          return { action: "contextMenu", changed: true, applied: keys.slice(0, 2), remaining: keys.slice(2), error: `拒绝访问：${keys[2]} 正被另一个程序占用` };
        }
        settings.contextMenuInstalled = on;
        settings.contextMenuCurrent = true;
        settings.contextMenuDetail = "";
        return { action: "contextMenu", changed: true, applied: keys, remaining: [] };
      });
    },
    PlanAutostart: (on) => plan("autostart", on === settings.autostart ? [] : [{ target: "HKCU\\…\\Run\\Obituary", before: on ? "" : "obitd.exe -tray", after: on ? "obitd.exe -tray" : "" }], () => { settings.autostart = on; }),
    PlanLanguage: (language) => {
      const next = language === "zh-CN" || language === "en" ? language : "auto";
      const effective = next === "auto" ? windowsLanguage() : next;
      const menuName = (l: string) => (l === "en" ? "Obituary" : "讣告");
      return plan("language", next === settings.language ? [] : [
        { target: "[ui] language", before: settings.language ?? "auto", after: next },
        ...(settings.contextMenuInstalled && effective !== settings.effectiveLanguage
          ? [{ target: "HKCU\\Software\\Classes\\*\\shell\\Obituary (MUIVerb)", before: menuName(settings.effectiveLanguage ?? "zh-CN"), after: menuName(effective) }]
          : []),
      ], () => { settings.language = next; settings.effectiveLanguage = effective; });
    },
    PlanSessionCleanup: (on) => plan("sessionCleanup", on === settings.sessionCleanup ? [] : [{ target: "[hooks] session_cleanup", before: String(settings.sessionCleanup), after: String(on) }], () => { settings.sessionCleanup = on; }),
    PlanSessionSummary: (on) => plan("sessionSummary", on === settings.sessionSummary ? [] : [{ target: "[hooks] session_summary", before: String(settings.sessionSummary), after: String(on) }], () => { settings.sessionSummary = on; }),
    Update: () => wait({ ...update }),
    CheckUpdate: () => new Promise((r) => setTimeout(() => { update.checkedAt = new Date().toISOString(); r({ ...update }); }, 900)),
    PlanUpdateCheck: (on) => plan("updateCheck", on === update.enabled ? [] : [{ target: "[update] check", before: String(update.enabled), after: String(on) }], () => { update.enabled = on; }),
    PlanTrayIcon: (on) => plan("trayIcon", on === settings.trayIcon ? [] : [{ target: "[ui] tray", before: String(settings.trayIcon), after: String(on) }], () => { settings.trayIcon = on; }),
    PlanTaskbar: (on) => plan("taskbar", on === settings.taskbar ? [] : [{ target: "[ui] taskbar", before: String(settings.taskbar), after: String(on) }], () => { settings.taskbar = on; }),
    PlanStartMenu: (on) => plan("startMenu", on === settings.startMenu ? [] : [{ target: "开始菜单：讣告 Obituary", before: on ? "" : "讣告 Obituary.lnk", after: on ? "讣告 Obituary.lnk" : "" }], () => { settings.startMenu = on; }),
    PlanDesktopShortcut: (on) => plan("desktopShortcut", on === settings.desktopShortcut ? [] : [{ target: "桌面：讣告 Obituary", before: on ? "" : "讣告 Obituary.lnk", after: on ? "讣告 Obituary.lnk" : "" }], () => { settings.desktopShortcut = on; }),
    PlanPause: (paused) => plan("pause", paused === settings.paused ? [] : [{ target: "paused", before: String(settings.paused), after: String(paused) }], () => { settings.paused = paused; }),
    ApplyPlan: (id) => {
      const apply = plans.get(id);
      plans.delete(id);
      if (!apply) return Promise.reject(new Error("plan is no longer current; preview again"));
      return wait(apply());
    },
    Logs: ({ level, text }) => {
      const entry = (hours: number, level: string, action: string, id: number, path: string, detail = "") => {
        const time = at(hours);
        return { time, level, action, id, path, detail, text: JSON.stringify({ time, action, id, path, ...(detail ? { detail } : {}) }) };
      };
      const all = [
        entry(-2, "info", "buried", 12, "C:\\Users\\me\\Downloads\\setup-node-22.11.0-x64.msi"),
        entry(-1.6, "info", "extended", 22, "D:\\work\\shop\\debug-2026-11-08.log", "expires 2026-11-12T09:00:00+08:00"),
        entry(-1.2, "warn", "skipped", 19, "D:\\work\\shop\\.venv", "a newer lifespan superseded this claim"),
        entry(-0.7, "info", "resurrected", 7, "D:\\work\\shop\\schema.sql"),
        entry(-0.2, "error", "retry", 20, "C:\\Users\\me\\Downloads\\IMG_20261102_0931.HEIC", "could not verify the Recycle Bin move"),
      ];
      const rank: Record<string, number> = { debug: 0, info: 1, warn: 2, error: 3 };
      const lines = all.filter((l) => (!level || rank[l.level] >= rank[level]) && (!text || l.text.includes(text)));
      return wait({ lines, truncated: false, scannedLines: all.length });
    },
  };
}
