import { t } from "./i18n";

const units = ["B", "KB", "MB", "GB", "TB"];

export function bytes(n: number): string {
  if (!n) return "0 B";
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`;
}

const pad = (n: number) => String(n).padStart(2, "0");

export function when(value?: string): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime()) || d.getFullYear() < 2000) return "—";
  const sameYear = d.getFullYear() === new Date().getFullYear();
  return t("date.when", sameYear ? null : d.getFullYear(), d.getMonth() + 1, d.getDate(), `${pad(d.getHours())}:${pad(d.getMinutes())}`);
}

export function duration(ms: number): string {
  const abs = Math.abs(ms);
  const [n, unit] = abs < 3600e3 ? [Math.max(1, Math.round(abs / 60e3)), "unit.minute" as const]
    : abs < 86400e3 ? [Math.round(abs / 3600e3), "unit.hour" as const] : [Math.round(abs / 86400e3), "unit.day" as const];
  return t("span", String(n), t(unit, n));
}

export function splitPath(path: string): { name: string; folder: string } {
  const cut = Math.max(path.lastIndexOf("\\"), path.lastIndexOf("/"));
  return cut >= 0 ? { name: path.slice(cut + 1) || path, folder: path.slice(0, cut) } : { name: path, folder: "" };
}

export const lifeLeft = (start: string, end: string) => {
  const from = +new Date(start);
  const to = +new Date(end);
  const now = Date.now();
  return { left: Math.min(1, Math.max(0, (to - now) / Math.max(1, to - from))), ms: to - now };
};

// A container has no file size; say what it is instead of "0 B".
export const graveSize = (g: { kind?: string; size: number }) => (g.kind === "container" ? t("container.label") : bytes(g.size));

// A lifespan as people say it ("3d" → "3 天"); anything else as written.
export function lifespanText(lifespan: string): string {
  const m = /^(\d+)([mhdw])$/.exec(lifespan.trim());
  if (!m) return lifespan;
  const n = Number(m[1]);
  const unit = m[2] === "m" ? "unit.minute" : m[2] === "h" ? "unit.hour" : m[2] === "d" ? "unit.day" : "unit.week";
  return t("span", String(n), t(unit, n));
}

// A path inside the proposal's project, shown from the project down.
export const underProject = (path: string, project: string) =>
  project && path.toLowerCase().startsWith(project.toLowerCase().replace(/[\/]+$/, "") + "\\") ? path.slice(project.replace(/[\/]+$/, "").length + 1) : path;

const unitMs = { m: 60e3, h: 3600e3, d: 86400e3, w: 7 * 86400e3 } as const;

// When an extension lands: added to the current expiry, or to now if that
// has already passed (the backend's rule).
export function extendedTo(expiresAt: string, lifespan: string): string | null {
  const m = /^(\d+)([mhdw])$/.exec(lifespan.trim());
  if (!m) return null;
  return new Date(Math.max(+new Date(expiresAt), Date.now()) + Number(m[1]) * unitMs[m[2] as keyof typeof unitMs]).toISOString();
}
