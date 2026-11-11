import { useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { BotIcon, CheckIcon, CopyIcon, DownloadIcon, SparkleIcon } from "lucide-react";
import { BorderBeam } from "@/components/ui/border-beam";
import { isMirror, mirrorFiles, version, waitlist } from "@/config";
import { agentInstall, installCode, repo, type Copy, type Method } from "@/copy";
import { brand } from "@/lib/brand";
import { cn } from "@/lib/utils";

type Os = "windows" | "macos" | "linux";
const osNames: Record<Os, string> = { windows: "Windows", macos: "macOS", linux: "Linux" };

const rawAgentInstall = "https://raw.githubusercontent.com/woodfishhhh/obituary/main/docs/agent-install.md";

// The mirror build points every download at obituary.woodfish.site, so a
// visitor in mainland China never waits on GitHub.
const sentenceFor = (lang: Copy["lang"]) => (isMirror ? agentInstall[lang].replace(rawAgentInstall, mirrorFiles.agentInstall) : agentInstall[lang]);
const mirrorScript = [`irm ${mirrorFiles.installScript} -OutFile install.ps1`, `.\\install.ps1 -Version ${version} -Mirror cn`];

// The visitor's system, when the browser says it; Windows otherwise.
function detectOs(): Os {
  const ua = typeof navigator === "undefined" ? "" : navigator.userAgent;
  if (/Windows/i.test(ua)) return "windows";
  if (/Mac OS X|Macintosh/i.test(ua) && !/iPhone|iPad/i.test(ua)) return "macos";
  if (/Linux|X11/i.test(ua) && !/Android/i.test(ua)) return "linux";
  return "windows";
}

const focusRing = "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cobalt";

function CopyButton({ text, c }: { text: string; c: Copy["install"] }) {
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle");
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setState("copied");
    } catch {
      setState("failed");
    }
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setState("idle"), 1800);
  }
  const label = state === "copied" ? c.copied : state === "failed" ? c.copyFailed : c.copy;
  return (
    <>
      <button
        type="button"
        onClick={copy}
        className={cn(
          "inline-flex min-h-11 shrink-0 items-center gap-2 rounded-full border border-line-2 bg-paper px-4 text-sm font-medium whitespace-nowrap transition-colors hover:bg-paper-3 active:translate-y-px",
          focusRing,
          state === "copied" ? "text-ok" : state === "failed" ? "text-bad" : "text-ink-2",
        )}
      >
        {state === "copied" ? <CheckIcon className="size-4" aria-hidden="true" /> : <CopyIcon className="size-4" aria-hidden="true" />}
        <span>{label}</span>
      </button>
      <span className="sr-only" aria-live="polite">
        {state === "idle" ? "" : label}
      </span>
    </>
  );
}

function CodeBox({ lines, label, c }: { lines: readonly string[]; label: string; c: Copy["install"] }) {
  return (
    <div className="flex items-start gap-3 rounded-2xl border border-line bg-paper p-3 pl-5 shadow-sm">
      <pre aria-label={label} className="min-w-0 flex-1 py-2 font-mono text-[13px] leading-relaxed whitespace-pre-wrap text-ink">
        {lines.map((line, i) => (
          <span key={i} className="block">
            {line.split(" ").map((tok, j) => (
              <span key={j} className={tok.length < 32 ? "whitespace-nowrap" : "break-all"}>
                {j > 0 ? " " : ""}
                {tok}
              </span>
            ))}
          </span>
        ))}
      </pre>
      <CopyButton text={lines.join("\n")} c={c} />
    </div>
  );
}

function Row({ head, children }: { head: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-1 gap-3 py-5 md:grid-cols-[10rem_minmax(0,1fr)] md:gap-8">
      <h4 className="pt-3 font-semibold">{head}</h4>
      <div className="flex min-w-0 flex-col gap-2">{children}</div>
    </div>
  );
}

function ByHand({ c }: { c: Copy }) {
  const i = c.install;
  if (isMirror) {
    return (
      <>
        <Row head={i.methods.script}>
          <CodeBox lines={mirrorScript} label={i.methods.script} c={i} />
          <p className="text-sm text-muted">{i.scriptNote}</p>
        </Row>
        <Row head={i.downloadHead}>
          <ul className="flex flex-wrap gap-2 pt-1.5">
            {i.downloads.map((d) => (
              <li key={d.file}>
                <a
                  href={mirrorFiles.download(d.file)}
                  className={cn("inline-flex min-h-11 items-center gap-2 rounded-full border border-line-2 bg-paper px-4 text-sm font-medium text-ink transition-colors hover:bg-paper-3", focusRing)}
                >
                  <DownloadIcon className="size-4 text-cobalt" aria-hidden="true" />
                  {d.label}
                </a>
              </li>
            ))}
          </ul>
        </Row>
      </>
    );
  }
  return (Object.keys(installCode) as Method[]).map((m) => (
    <Row key={m} head={i.methods[m]}>
      <CodeBox lines={installCode[m]} label={i.methods[m]} c={i} />
      {m === "script" ? <p className="text-sm text-muted">{i.scriptNote}</p> : null}
    </Row>
  ));
}

type Route = "ai" | "hand";

function Windows({ c }: { c: Copy }) {
  const i = c.install;
  const sentence = sentenceFor(c.lang);
  const [route, setRoute] = useState<Route>("ai");
  const routes: { id: Route; label: string }[] = [
    { id: "ai", label: i.agentHead },
    { id: "hand", label: i.handHead },
  ];
  return (
    <div className="flex flex-col gap-6">
      <div role="tablist" aria-label={i.routeLabel} className="inline-flex self-start rounded-2xl bg-paper-3 p-1">
        {routes.map((r) => (
          <button
            key={r.id}
            id={`route-${r.id}`}
            type="button"
            role="tab"
            aria-selected={route === r.id}
            aria-controls={`route-${r.id}-panel`}
            onClick={() => setRoute(r.id)}
            className={cn(
              "flex min-h-11 items-center gap-2 rounded-xl px-5 text-sm font-medium whitespace-nowrap transition-colors",
              focusRing,
              route === r.id ? "bg-paper text-ink shadow-sm" : "text-ink-2 hover:text-ink",
            )}
          >
            {r.id === "ai" ? <BotIcon className="size-4 text-cobalt" aria-hidden="true" /> : null}
            {r.label}
          </button>
        ))}
      </div>

      {route === "ai" ? (
        <div id="route-ai-panel" role="tabpanel" aria-labelledby="route-ai" className="relative overflow-hidden rounded-3xl border border-glint bg-paper p-6 shadow-glow-soft md:p-10">
          <div aria-hidden="true" className="absolute -top-24 -right-16 size-72 rounded-full bg-cream/50 blur-3xl" />
          <div className="relative flex flex-col gap-5">
            <p className="max-w-[60ch] text-ink-2">{i.agentLine}</p>
            <div className="flex flex-col gap-3 rounded-2xl border border-line bg-paper-2 p-4 md:flex-row md:items-center md:gap-5 md:p-5">
              <p aria-label={i.agentLabel} className="min-w-0 flex-1 text-base leading-relaxed [overflow-wrap:anywhere] text-ink md:text-lg">
                {sentence}
              </p>
              <CopyButton text={sentence} c={i} />
            </div>
            <p className="flex max-w-[64ch] items-start gap-2.5 text-ink-2">
              <CheckIcon className="mt-1 size-4 shrink-0 text-ok" aria-hidden="true" />
              {i.payoff}
            </p>
          </div>
          <BorderBeam size={180} duration={8} colorFrom={brand.sky} colorTo={brand.gold} borderWidth={2} transition={{ repeat: 1 }} />
        </div>
      ) : (
        <div id="route-hand-panel" role="tabpanel" aria-labelledby="route-hand">
          <p className="mb-2 text-ink-2">{isMirror ? i.mirrorHandLine : i.handLine}</p>
          <div className="flex flex-col divide-y divide-line border-y border-line">
            <ByHand c={c} />
            <Row head={i.then}>
              <div className="grid min-w-0 grid-cols-1 gap-3 md:grid-cols-2">
                {i.init.map((x) => (
                  <div key={x.code} className="flex min-w-0 flex-col gap-2">
                    <span className="text-sm text-muted">{x.label}</span>
                    <CodeBox lines={[x.code]} label={x.label} c={i} />
                  </div>
                ))}
              </div>
            </Row>
            <Row head={i.updateHead}>
              <p className="pt-2.5 text-ink-2">{i.update}</p>
              <p className="text-ink-2">{i.uninstall}</p>
            </Row>
          </div>
          <p className="mt-4 text-sm text-muted">{isMirror ? i.mirrorFact : i.fact}</p>
        </div>
      )}

      <p className="text-sm text-muted">{i.network}</p>
    </div>
  );
}

type Sending = "idle" | "sending" | "sent" | "invalid" | "busy" | "failed";

// Without the server's CORS answer, `pnpm dev` cannot reach the list; it
// answers locally instead: an address starting with 400@ or 429@ gets that
// status, offline@ a network failure, anything else success.
async function devPost(email: string): Promise<Response> {
  await new Promise((r) => setTimeout(r, 600));
  if (email.startsWith("offline@")) throw new TypeError("Failed to fetch");
  const status = email.startsWith("400@") ? 400 : email.startsWith("429@") ? 429 : 200;
  return new Response("{}", { status });
}

async function subscribe(email: string, list: string): Promise<Sending> {
  try {
    const res = import.meta.env.DEV
      ? await devPost(email)
      : await fetch(waitlist.endpoint, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email, name: "", list_uuids: [list] }),
          signal: AbortSignal.timeout(15000),
        });
    if (res.ok) return "sent";
    if (res.status === 400) return "invalid";
    if (res.status === 429) return "busy";
    return "failed";
  } catch {
    return "failed";
  }
}

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function Waitlist({ c, os }: { c: Copy; os: Exclude<Os, "windows"> }) {
  const i = c.install;
  const [state, setState] = useState<Sending>("idle");
  useEffect(() => setState("idle"), [os]);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (state === "sending") return;
    const form = new FormData(e.currentTarget);
    // Bots fill every field; people never see this one.
    if (String(form.get("website") ?? "") !== "") return setState("sent");
    const email = String(form.get("email") ?? "").trim();
    if (!emailPattern.test(email)) return setState("invalid");
    setState("sending");
    setState(await subscribe(email, waitlist.lists[os]));
  }

  if (state === "sent") {
    return (
      <p role="status" className="flex items-center gap-2.5 rounded-2xl border border-glint bg-soft px-4 py-3 text-ink">
        <CheckIcon className="size-4 shrink-0 text-ok" aria-hidden="true" />
        {i.sent}
      </p>
    );
  }
  const message = { invalid: i.invalid, busy: i.busy, failed: i.failed, sending: i.sending, idle: "" }[state];
  const error = state === "invalid" || state === "busy" || state === "failed";
  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <label className="flex min-w-0 flex-1 flex-col gap-1.5 text-sm font-medium text-ink-2">
          {i.emailLabel}
          <input
            type="email"
            name="email"
            required
            autoComplete="email"
            inputMode="email"
            placeholder="you@example.com"
            aria-invalid={state === "invalid" || undefined}
            aria-describedby="waitlist-status"
            className={cn(
              "h-12 rounded-full border bg-paper px-5 text-base text-ink placeholder:text-muted focus-visible:border-cobalt",
              focusRing,
              state === "invalid" ? "border-bad" : "border-line-2",
            )}
          />
        </label>
        <div aria-hidden="true" className="absolute -left-[9999px] size-px overflow-hidden">
          <label>
            Website
            <input type="text" name="website" tabIndex={-1} autoComplete="off" defaultValue="" />
          </label>
        </div>
        <button
          type="submit"
          disabled={state === "sending"}
          className={cn(
            "h-12 rounded-full bg-cobalt px-6 text-base font-medium whitespace-nowrap text-white transition-colors hover:bg-cobalt-deep active:translate-y-px disabled:opacity-60",
            focusRing,
          )}
        >
          {state === "sending" ? i.sending : i.submit}
        </button>
      </div>
      <p id="waitlist-status" role="status" className={cn("min-h-6 text-sm", error ? "text-bad" : "text-muted")}>
        {state === "sending" ? "" : message}
      </p>
      <p className="text-sm text-muted">{i.privacy}</p>
    </form>
  );
}

function ComingSoon({ c, os }: { c: Copy; os: Exclude<Os, "windows"> }) {
  const i = c.install;
  const name = osNames[os];
  const open = Boolean(waitlist.endpoint && waitlist.lists[os]);
  return (
    <div className="relative overflow-hidden rounded-3xl border border-line bg-paper p-6 md:p-10">
      <div aria-hidden="true" className="dot-field fade-edges absolute inset-0 opacity-70" />
      <div className="relative flex max-w-[44rem] flex-col gap-5">
        <span className="inline-flex items-center gap-2 self-start rounded-full bg-cream/70 px-3 py-1 text-sm font-medium text-on-gold">
          <SparkleIcon className="size-3.5 fill-gold text-gold-deep" aria-hidden="true" />
          {name}
        </span>
        <h3 className="text-2xl font-semibold tracking-[-0.01em] md:text-3xl">{i.soonHead}</h3>
        <p className="text-ink-2">{i.soonLine(name)}</p>
        {open ? (
          <Waitlist c={c} os={os} />
        ) : (
          <p role="status" className="rounded-2xl border border-glint bg-soft px-4 py-3 text-ink">
            {i.closed}
          </p>
        )}
        <p className="text-sm text-ink-2">
          {i.watch}{" "}
          <a href={repo} className="font-medium text-cobalt underline-offset-4 hover:underline">
            {i.watchLink}
          </a>
        </p>
      </div>
    </div>
  );
}

export function Install({ c }: { c: Copy }) {
  const i = c.install;
  const [os, setOs] = useState<Os>("windows");
  useEffect(() => setOs(detectOs()), []);
  return (
    <section id="install" aria-labelledby="install-head" className="relative pt-28 pb-24 md:pt-40 md:pb-32">
      <div className="mx-auto max-w-[1240px] px-6 md:px-10">
        <div className="mb-10 grid gap-6 lg:grid-cols-2 lg:items-end">
          <h2 id="install-head" className="text-[clamp(2rem,3.4vw,3.25rem)] leading-[1.08] font-semibold tracking-[-0.02em]">{i.head}</h2>
          <p className="max-w-[48ch] text-lg text-ink-2">{i.lead}</p>
        </div>
        <div role="radiogroup" aria-label={i.platformLabel} className="mb-8 inline-flex rounded-full border border-line bg-paper p-1 shadow-sm">
          {(Object.keys(osNames) as Os[]).map((id) => (
            <button
              key={id}
              type="button"
              role="radio"
              aria-checked={os === id}
              onClick={() => setOs(id)}
              className={cn(
                "min-h-11 rounded-full px-5 text-sm font-medium whitespace-nowrap transition-colors",
                focusRing,
                os === id ? "bg-cobalt text-white shadow-sm" : "text-ink-2 hover:bg-paper-3 hover:text-ink",
              )}
            >
              {osNames[id]}
            </button>
          ))}
        </div>
        {os === "windows" ? <Windows c={c} /> : <ComingSoon c={c} os={os} />}
      </div>
    </section>
  );
}
