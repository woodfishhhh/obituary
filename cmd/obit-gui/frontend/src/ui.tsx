import * as Dialog from "@radix-ui/react-dialog";
import { Container, File, Folder, TriangleAlert, X, type LucideIcon } from "lucide-react";
import { Component, createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react";
import { errorText, useApi } from "./api";
import { duration, lifeLeft, noteText, splitPath, when } from "./format";
import { t } from "./i18n";

// ── Buttons and small parts ─────────────────────────────────────────────

type Kind = "primary" | "gold" | "danger" | "ghost" | "danger solid" | "";
export function Button({ kind = "", icon: Icon, children, ...rest }: { kind?: Kind; icon?: LucideIcon; children?: ReactNode } & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button type="button" className={`btn ${kind}`.trim()} {...rest}>
      {Icon ? <Icon aria-hidden /> : null}
      {children}
    </button>
  );
}

export function Seg<T extends string>({ value, options, onChange, label }: { value: T; options: [T, string][]; onChange: (v: T) => void; label: string }) {
  return (
    <div className="seg" role="group" aria-label={label}>
      {options.map(([v, text]) => (
        <button key={v} type="button" aria-pressed={v === value} onClick={() => onChange(v)}>{text}</button>
      ))}
    </div>
  );
}

export function LifeLine({ start, end, left: leftWord, over, soonMs = 86400e3 }: { start: string; end: string; left: (span: string) => string; over: string; soonMs?: number }) {
  const { left, ms } = lifeLeft(start, end);
  const soon = ms > 0 && ms < soonMs;
  return (
    <div className={`life${soon ? " soon" : ""}`} title={t("date.range", when(start), when(end))}>
      <span className="life-track"><span className="life-fill" style={{ transform: `scaleX(${left.toFixed(3)})` }} /></span>
      <span className="life-label">{ms <= 0 ? over : leftWord(duration(ms))}</span>
    </div>
  );
}

export const isContainer = (kind?: string) => kind === "container";

export function NameCell({ path, isDir, note, kind }: { path: string; isDir?: boolean; note?: string; kind?: string }) {
  const container = isContainer(kind);
  const { name, folder } = container ? { name: path, folder: t("container.sub") } : splitPath(path);
  const Icon = container ? Container : isDir ? Folder : File;
  return (
    <div className="name">
      <span className="name-icon"><Icon aria-hidden /></span>
      <div className="name-text">
        <div className="name-main" title={path}>{name}</div>
        {folder ? <div className="name-sub">{folder}</div> : null}
        {note ? <div className="name-note">{noteText(note)}</div> : null}
      </div>
    </div>
  );
}

export function Empty({ icon: Icon, title, text }: { icon: LucideIcon; title: string; text?: string }) {
  return (
    <div className="empty">
      <Icon aria-hidden />
      <b>{title}</b>
      {text ? <span>{text}</span> : null}
    </div>
  );
}

export const Skeleton = () => (
  <div className="skeleton" aria-busy="true" aria-label={t("loading")}>{Array.from({ length: 6 }, (_, i) => <i key={i} />)}</div>
);

export function Head({ title, children, actions }: { title?: string; children?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="head">
      <div className="head-text">
        {title ? <h1>{title}</h1> : null}
        {children ? <p>{children}</p> : null}
      </div>
      {actions}
    </div>
  );
}

// ── Outcome: what did not go through, kept on the page until dismissed ──
// A toast is too brief for a list of failed paths and their reasons.

export interface OutcomeNote { title: string; text?: string; items?: [string, string][] }
export function Outcome({ note, onClose }: { note: OutcomeNote; onClose: () => void }) {
  return (
    <div className="notice warn outcome" role="alert">
      <div className="outcome-body">
        <b>{note.title}</b>
        {note.text ? <p>{note.text}</p> : null}
        {note.items?.length ? <ul>{note.items.map(([what, why]) => <li key={what}><span className="name-sub">{what}</span>{why ? <span>{why}</span> : null}</li>)}</ul> : null}
      </div>
      <Button kind="ghost" icon={X} aria-label={t("dismiss")} title={t("dismiss")} onClick={onClose} />
    </div>
  );
}

// ── Frame: the element dialogs and toasts render into ──────────────────
// In the playground several apps sit side by side; each keeps its own
// dialogs inside its own frame so they inherit that variant's tokens.

const FrameContext = createContext<HTMLElement | null>(null);
export const FrameProvider = FrameContext.Provider;

// ── Confirm ────────────────────────────────────────────────────────────

// remember adds a "don't ask again" checkbox; it counts only with confirm.
export interface Ask { title: string; body?: ReactNode; ok: string; danger?: boolean; remember?: { label: string; onRemember: () => void } }
const ConfirmContext = createContext<(a: Ask) => Promise<boolean>>(async () => false);
export const useConfirm = () => useContext(ConfirmContext);

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const frame = useContext(FrameContext);
  const [ask, setAsk] = useState<Ask | null>(null);
  const [quiet, setQuiet] = useState(false);
  const resolver = useRef<(v: boolean) => void>(() => {});
  const open = useCallback((a: Ask) => new Promise<boolean>((resolve) => { resolver.current = resolve; setQuiet(false); setAsk(a); }), []);
  const done = (v: boolean) => { if (v && quiet) ask?.remember?.onRemember(); resolver.current(v); setAsk(null); };
  return (
    <ConfirmContext.Provider value={open}>
      {children}
      <Dialog.Root open={!!ask} onOpenChange={(o) => { if (!o) done(false); }}>
        <Dialog.Portal container={frame ?? undefined}>
          <Dialog.Overlay className="overlay" />
          <Dialog.Content className="dialog" aria-describedby={undefined}>
            <div className="dialog-body">
              <Dialog.Title asChild><h3>{ask?.title}</h3></Dialog.Title>
              {ask?.body}
            </div>
            <div className="dialog-actions">
              {ask?.remember ? <label className="dialog-quiet"><input type="checkbox" checked={quiet} onChange={(e) => setQuiet(e.target.checked)} />{ask.remember.label}</label> : null}
              <Button onClick={() => done(false)}>{t("cancel")}</Button>
              <Button kind={ask?.danger ? "danger solid" : "primary"} onClick={() => done(true)} autoFocus>{ask?.ok}</Button>
            </div>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </ConfirmContext.Provider>
  );
}

// ── Toasts ─────────────────────────────────────────────────────────────

// An action keeps the toast up longer, so there is time to reach it.
export interface ToastAction { label: string; run: () => void }
type Toast = { id: number; text: string; kind: "" | "error"; action?: ToastAction };
const ToastContext = createContext<(text: string, kind?: "" | "error", action?: ToastAction) => void>(() => {});
export const useToast = () => useContext(ToastContext);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const seq = useRef(0);
  const push = useCallback((text: string, kind: "" | "error" = "", action?: ToastAction) => {
    const id = ++seq.current;
    setToasts((t) => [...t, { id, text, kind, action }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), action ? 12000 : kind === "error" ? 8000 : 3200);
  }, []);
  return (
    <ToastContext.Provider value={push}>
      {children}
      <div className="toasts" aria-live="polite">
        {toasts.map((t) => (
          <div key={t.id} className={`toast ${t.kind}`} role={t.kind ? "alert" : "status"}>
            <span>{t.text}</span>
            {t.action ? <button type="button" className="toast-act" onClick={t.action.run}>{t.action.label}</button> : null}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

// Batch replies keep completed receipts even when a later item failed.
export function useBatch() {
  const toast = useToast();
  return (reply: { results: { ref: string; error?: string }[]; error?: string }, verb: string) => {
    const results = reply.results ?? [];
    const failed = results.filter((r) => r.error);
    const done = results.length - failed.length;
    if (reply.error || failed.length) toast(t("batch.partial", verb, done, failed.length, reply.error ?? t("pair", failed[0].ref, failed[0].error ?? "")), "error");
    else toast(t("batch.done", verb, done));
  };
}

// One resurrected item: say where it went, with a way to open its folder.
// Several are reported like any batch.
export function useResurrectReport() {
  const api = useApi();
  const toast = useToast();
  const report = useBatch();
  return (reply: { results: { ref: string; path?: string; error?: string }[]; error?: string }) => {
    const one = !reply.error && reply.results?.length === 1 && !reply.results[0].error ? reply.results[0].path : undefined;
    if (!one) return report(reply, t("resurrect"));
    const show = api.ShowInFolder;
    toast(t("grave.backAt", one), "", show ? { label: t("showInFolder"), run: () => show(one).catch((e) => toast(errorText(e), "error")) } : undefined);
  };
}

// ── Page errors ─────────────────────────────────────────────────────────

// A page that throws shows what broke and a way back, instead of unmounting
// the whole window to a blank sheet. App keys it by page, so another tab
// starts clean.
export class PageBoundary extends Component<{ children: ReactNode; onHome: () => void }, { error: Error | null }> {
  state: { error: Error | null } = { error: null };
  static getDerivedStateFromError(error: Error) { return { error }; }
  render() {
    const { error } = this.state;
    if (!error) return this.props.children;
    return (
      <div className="page crash" role="alert">
        <Empty icon={TriangleAlert} title={t("crash.title")} text={t("crash.text")} />
        <div className="well">{error.message || String(error)}</div>
        <div className="crash-actions">
          <Button onClick={this.props.onHome}>{t("crash.home")}</Button>
          <Button kind="primary" onClick={() => location.reload()}>{t("crash.reload")}</Button>
        </div>
      </div>
    );
  }
}

// ── Selection ───────────────────────────────────────────────────────────

// Stays at the bottom of the scrolling page while something is selected, so
// the actions are in reach after picking at the end of a long list.
export function SelectionBar({ count, onClear, children }: { count: number; onClear: () => void; children: ReactNode }) {
  if (!count) return null;
  return (
    <div className="selbar" role="region" aria-label={t("sel.label")}>
      <b>{t("sel.count", count)}</b>
      <Button kind="ghost" onClick={onClear}>{t("sel.clear")}</Button>
      <span className="grow" />
      {children}
    </div>
  );
}
