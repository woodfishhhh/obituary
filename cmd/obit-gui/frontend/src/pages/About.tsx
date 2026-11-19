import { BellRing, Copy, Download, RefreshCw } from "lucide-react";
import { useState, type ReactNode } from "react";
import { errorText, useApi } from "../api";
import { when } from "../format";
import { useLoad } from "../hooks";
import { t } from "../i18n";
import type { Plan, UpdateInfo } from "../types";
import { Button, useConfirm, useToast } from "../ui";

const REPO = "https://github.com/woodfishhhh/obituary";
const channels = ["scoop", "winget", "script", "other"] as const;
const channelText = (c: string) => t(`upd.channel.${(channels as readonly string[]).includes(c) ? (c as (typeof channels)[number]) : "other"}`);

export function openURL(url: string) {
  if (window.runtime?.BrowserOpenURL) window.runtime.BrowserOpenURL(url);
  else window.open(url, "_blank", "noopener");
}

async function copyText(text: string) {
  if (window.runtime?.ClipboardSetText) await window.runtime.ClipboardSetText(text);
  else await navigator.clipboard.writeText(text);
}

// Version, how this copy was installed, the release check and its switch.
// With a newer release the card offers to update in place where the install
// channel allows it, and always shows the command to run by hand.
export default function About({ apply, switchFor }: {
  apply: (plan: Plan, title: string) => Promise<void>;
  switchFor: (on: boolean, label: string, onToggle: () => void) => ReactNode;
}) {
  const api = useApi();
  const toast = useToast();
  const [fresh, setFresh] = useState<UpdateInfo | null>(null);
  const [checking, setChecking] = useState(false);
  const { data } = useLoad(() => (api.Update ? api.Update() : Promise.resolve(null)), [api]);
  const info = fresh ?? data;
  if (!info) return null;
  const current = info;

  async function check() {
    setChecking(true);
    try {
      setFresh(await api.CheckUpdate!());
    } catch (error) {
      toast(t("upd.failed", errorText(error)), "error");
    } finally {
      setChecking(false);
    }
  }

  async function toggle(on: boolean) {
    await apply(await api.PlanUpdateCheck!(on), on ? t("upd.autoOn") : t("upd.autoOff"));
    setFresh({ ...current, enabled: on });
  }

  return (
    <section className="card about" id="about" aria-label={t("upd.title")}>
      <div className="about-head">
        <img className="about-icon" src="icon.png" alt="" />
        <div className="about-id">
          <b>{t("brand")}</b>
          <span className="about-meta"><span>{t("upd.version", info.current)}</span><span className="tag">{channelText(info.channel)}</span></span>
        </div>
        {api.CheckUpdate ? <Button icon={RefreshCw} onClick={check} disabled={checking} aria-busy={checking}>{checking ? t("upd.checking") : t("upd.check")}</Button> : null}
      </div>
      <LastApplyNote info={info} />
      {info.newer && info.latest ? <NewRelease info={info} /> : (
        <p className="hint about-status">
          {info.error ? t("upd.failed", info.error) : info.latest ? t("upd.latest") : t("upd.never")}
          {info.checkedAt ? <><br />{t("upd.checkedAt", when(info.checkedAt))}</> : null}
        </p>
      )}
      {api.PlanUpdateCheck ? (
        <div className="setting">
          <span className="setting-icon"><BellRing aria-hidden /></span>
          <span className="setting-label">{t("upd.auto")}</span>
          <span className="setting-desc">{t("upd.autoDesc")}</span>
          {switchFor(info.enabled, t("upd.auto"), () => toggle(!info.enabled))}
        </div>
      ) : null}
    </section>
  );
}

function Link({ href, children }: { href: string; children: ReactNode }) {
  return <a role="link" tabIndex={0} onClick={() => openURL(href)} onKeyDown={(e) => { if (e.key === "Enter") openURL(href); }}>{children}</a>;
}

// The outcome of the last in-app update: a failure stays until a later one
// succeeds; a success shows only while it is the version running now.
function LastApplyNote({ info }: { info: UpdateInfo }) {
  const last = info.lastApply;
  if (!last) return null;
  if (last.success) return last.version === info.current ? <p className="hint about-status">{t("upd.applied", last.version)}</p> : null;
  return (
    <p className="hint about-status about-error" role="alert">
      {t("upd.lastFailed", last.targetVersion, last.error ?? "")}
      <br />{t("upd.lastLog", last.logPath)}
    </p>
  );
}

function NewRelease({ info }: { info: UpdateInfo }) {
  const api = useApi();
  const toast = useToast();
  const confirm = useConfirm();
  const [starting, setStarting] = useState(false);
  const canApply = !!(info.canApply && api.ApplyUpdate);
  async function update() {
    if (!(await confirm({ title: t("upd.applyTitle", info.latest ?? ""), body: <p>{t("upd.applyBody")}</p>, ok: t("upd.apply") }))) return;
    setStarting(true);
    try {
      await api.ApplyUpdate!();
      // The helper waits for this process to exit before it replaces any file.
      window.runtime?.Quit?.();
    } catch (error) {
      toast(t("upd.applyFailed", errorText(error)), "error");
      setStarting(false);
    }
  }
  async function copy() {
    try {
      await copyText(info.command ?? "");
      toast(t("upd.copied"));
    } catch (error) {
      toast(errorText(error), "error");
    }
  }
  return (
    <div className="about-new" role="status">
      <div className="about-new-head">
        <b>{t("upd.newer", info.latest ?? "")}</b>
        {info.published ? <span className="hint">{t("upd.published", when(info.published))}</span> : null}
      </div>
      {canApply ? (
        <div><Button kind="primary" icon={Download} onClick={update} disabled={starting} aria-busy={starting}>{starting ? t("upd.applying") : t("upd.apply")}</Button></div>
      ) : null}
      {info.command ? (
        <>
          <p>{canApply ? t("upd.orTerminal") : t("upd.howTo")}</p>
          <div className="about-cmd"><code>{info.command}</code><Button kind="ghost" icon={Copy} onClick={copy}>{t("upd.copy")}</Button></div>
        </>
      ) : <p>{t("upd.howToOther")}</p>}
      {info.notes ? (
        <details className="about-notes">
          <summary>{t("upd.notes")}</summary>
          <div className="about-notes-body"><Notes text={info.notes} /></div>
        </details>
      ) : null}
      <div className="about-links">
        {info.url ? <Link href={info.url}>{t("upd.open")}</Link> : null}
        <Link href={REPO}>{t("upd.repo")}</Link>
      </div>
    </div>
  );
}

// Release notes are Markdown. Headings, bullets and paragraphs are enough
// here; links keep their text and code marks are dropped. Nothing becomes HTML.
function Notes({ text }: { text: string }) {
  const plain = (s: string) => s.replace(/\[([^\]]+)\]\([^)]*\)/g, "$1").replace(/`([^`]+)`/g, "$1").replace(/\*\*([^*]+)\*\*/g, "$1");
  const blocks: ReactNode[] = [];
  let bullets: string[] = [];
  const flush = () => {
    if (bullets.length) blocks.push(<ul key={blocks.length}>{bullets.map((b, i) => <li key={i}>{plain(b)}</li>)}</ul>);
    bullets = [];
  };
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    const bullet = /^[-*+]\s+(.*)$/.exec(line);
    if (bullet) { bullets.push(bullet[1]); continue; }
    flush();
    if (!line) continue;
    const heading = /^#{1,6}\s+(.*)$/.exec(line);
    blocks.push(heading ? <b key={blocks.length}>{plain(heading[1])}</b> : <p key={blocks.length}>{plain(line)}</p>);
  }
  flush();
  return <>{blocks}</>;
}
