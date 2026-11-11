// JSON shapes of internal/gui (Go), field names as the backend sends them.

export interface Summary {
  trackedCount: number;
  graveCount: number;
  graveBytes: number;
  pendingGraveCount: number;
  decisionCount: number;
  paused: boolean;
  error?: string;
}

// A Docker container tracked by full ID (core.KindContainer); path is
// "docker:<name> (<short ID>)" and size is always 0.
export interface ContainerRef {
  id: string;
  created: string;
  name: string;
  shortId: string;
}

export interface Burial {
  id: number;
  path: string;
  isDir: boolean;
  kind?: string;
  container?: ContainerRef;
  size: number;
  buriedAt: string;
  purgeAt: string;
  note?: string;
}

export interface PendingGrave {
  manifest: string;
  status: string;
  error?: string;
  path?: string;
}

export interface GraveRow {
  id: number;
  status: string;
  burial?: Burial;
  pending?: PendingGrave;
}

export interface GraveyardPage {
  rows: GraveRow[];
  total: number;
  pendingCount: number;
  hasMore: boolean;
  nextAfter: string;
  restored: GraveResult[];
  error?: string;
}

export interface Item {
  id: number;
  path: string;
  isDir: boolean;
  kind?: string;
  container?: ContainerRef;
  createdAt: string;
  expiresAt: string;
  note?: string;
  source?: string;
  lastError?: string;
}

export interface Candidate {
  path: string;
  kind?: string;
  container?: ContainerRef;
  lifespan: string;
  rule?: string;
  size: number;
}

export interface Proposal {
  id: number;
  kind: string;
  project: string;
  createdAt: string;
  candidates: Candidate[];
  // The version the user is looking at (internal/gui ReviewedProposal).
  // Approve and Reject send it back; a proposal that changed since is refused.
  fingerprint: string;
}

// One proposal's outcome (core.ProposalResult). On approval, results holds
// one entry per candidate, ref being the candidate's path; any of them can
// fail while the proposal itself succeeds.
export interface ProposalResult {
  ref: string;
  proposal?: Proposal;
  results?: Result[];
  error?: string;
}

export interface RuleFile {
  path: string;
  status: "untrusted" | "trusted" | "changed" | string;
  sha256?: string;
  warning?: string;
  content: string;
}

export interface WatchPolicy {
  id: number;
  path: string;
  lifespan: string;
  age: string;
  createdAt: string;
  // Active once approved at its current revision; missing on older backends.
  consent?: string;
  rev?: number;
  approvedRev?: number;
}

export interface Settings {
  mode: "graveyard" | "recycle" | string;
  retentionDays: number;
  configPath: string;
  contextMenuInstalled: boolean;
  contextMenuCurrent: boolean;
  contextMenuDetail: string;
  autostart: boolean;
  paused: boolean;
  // The interface language, kept in config.toml so the tray and the context
  // menu follow it too. language is the user's choice; effectiveLanguage is
  // what "auto" resolves to from the Windows display language. Both are
  // missing on an older backend.
  language?: "auto" | "zh-CN" | "en" | string;
  effectiveLanguage?: "zh-CN" | "en" | string;
  // The user's Downloads folder, offered for folder policies.
  downloads?: string;
  // [hooks] session_summary; missing on an older backend.
  sessionSummary?: boolean;
  // What Obituary has buried so far; missing on an older backend.
  lifetime?: Lifetime;
  // Where Obituary shows up: the tray icon, the window on the taskbar, and
  // the shortcuts Obituary made. Missing on an older backend.
  trayIcon?: boolean;
  taskbar?: boolean;
  startMenu?: boolean;
  desktopShortcut?: boolean;
}

export interface Plan {
  id: string;
  action: string;
  changes: { key?: string; target: string; before: string; after: string }[];
}

// What ApplyPlan did (internal/gui PlanReceipt). For the context menu,
// applied and remaining are registry keys; changed is true once any key was
// written, so remaining or error mean it stopped part way.
export interface PlanReceipt {
  action: string;
  path?: string;
  changed: boolean;
  applied?: string[] | null;
  remaining?: string[] | null;
  error?: string;
}

export interface Lifetime {
  buried: number;
  buriedBytes: number;
  purgedBytes: number;
  resurrected: number;
}

// The release check (internal/gui UpdateInfo). latest is empty until a check
// has succeeded; channel says how this copy was installed, and command is the
// one to run for it ("" when there is none, e.g. a hand-copied zip).
export interface UpdateInfo {
  current: string;
  latest?: string;
  newer: boolean;
  name?: string;
  notes?: string;
  url?: string;
  published?: string;
  checkedAt?: string;
  error?: string;
  enabled: boolean;
  channel: "scoop" | "winget" | "script" | "other" | string;
  command?: string;
}

export interface LogLine {
  text: string;
  time?: string;
  level: string;
  // Audit entries only; missing on an older backend.
  action?: string;
  id?: number;
  path?: string;
  detail?: string;
}

export interface LogPage {
  lines: LogLine[];
  truncated: boolean;
  scannedLines: number;
}

export interface GraveResult { ref: string; action?: string; path?: string; error?: string }
export interface Result { ref: string; action?: string; error?: string }
export interface BatchReply<T> { results: T[]; error?: string }

export interface Api {
  Summary(): Promise<Summary>;
  Graveyard(q: { filter: string; limit: number; after: string }): Promise<GraveyardPage>;
  Resurrect(refs: string[]): Promise<BatchReply<GraveResult>>;
  Purge(refs: string[]): Promise<BatchReply<GraveResult>>;
  Upcoming(within: "" | "24h" | "7d"): Promise<{ items: Item[] }>;
  Extend(refs: string[], lifespan: string): Promise<BatchReply<Result>>;
  Cancel(refs: string[]): Promise<BatchReply<Result>>;
  Proposals(): Promise<{ proposals: Proposal[] }>;
  Approve(refs: string[], fingerprints: string[]): Promise<BatchReply<ProposalResult>>;
  Reject(refs: string[], fingerprints: string[]): Promise<BatchReply<ProposalResult>>;
  Rules(): Promise<{ rules: RuleFile[] }>;
  Trust(path: string, reviewedSHA256: string): Promise<RuleFile>;
  Untrust(path: string): Promise<{ action: string; path?: string; changed: boolean }>;
  Watches(): Promise<{ policies: WatchPolicy[] }>;
  SetWatch(path: string, lifespan: string, age: string): Promise<{ policy: WatchPolicy; error?: string }>;
  // The native folder dialog; "" when cancelled. Missing on older backends.
  PickFolder?(): Promise<string>;
  RemoveWatch(path: string): Promise<{ action: string; changed: boolean }>;
  Settings(): Promise<Settings>;
  PlanSettings(mode: string, retentionDays: number): Promise<Plan>;
  PlanContextMenu(enabled: boolean): Promise<Plan>;
  PlanAutostart(enabled: boolean): Promise<Plan>;
  PlanPause(paused: boolean): Promise<Plan>;
  // Missing on an older backend.
  PlanLanguage?(language: string): Promise<Plan>;
  PlanSessionSummary?(enabled: boolean): Promise<Plan>;
  PlanTrayIcon?(enabled: boolean): Promise<Plan>;
  PlanTaskbar?(enabled: boolean): Promise<Plan>;
  PlanStartMenu?(enabled: boolean): Promise<Plan>;
  PlanDesktopShortcut?(enabled: boolean): Promise<Plan>;
  // Release checks; missing on an older backend.
  Update?(): Promise<UpdateInfo>;
  CheckUpdate?(): Promise<UpdateInfo>;
  PlanUpdateCheck?(enabled: boolean): Promise<Plan>;
  ApplyPlan(id: string): Promise<PlanReceipt>;
  Logs(q: { source: string; lines: number; text: string; from: string; to: string; level: string }): Promise<LogPage>;
}
