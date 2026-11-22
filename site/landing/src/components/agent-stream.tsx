import { useEffect, useRef, useState, type CSSProperties } from "react";
import type { Agent } from "@/agents";

// The wall of agents on the home page (also drawn by the logo-wall playground, which varies these options): a few
// pill-shaped bands that slide sideways forever. Each pill is a link to the agent's page, so it works (and is
// found) without a script; with one, a plain click picks the agent instead, and a modified click still follows the link. The loop is one CSS
// transform, so it runs on the compositor; it never wakes the main thread. With keyboard focus inside, a band stops
// and becomes an ordinary scrollable row, so the focused pill is in view. With reduced motion every band is a
// still, wrapped grid. The copies of the pills after the first are hidden from assistive technology and from Tab,
// but stay clickable (no `inert`).
export interface StreamOptions {
  rows: number;
  /** Pixels per second. */
  speed: number;
  /** Height of the logo's box, in px. */
  size: number;
  /** Space between pills, in px. */
  gap: number;
  /** Degrees the whole wall leans. */
  tilt: number;
  /** Percent of each end that fades out. */
  fade: number;
  /** 0..1: how far a pill swells under the pointer. */
  hover: number;
  names: boolean;
  /** Stop a band while the pointer is on it. */
  pause: boolean;
  logo: "color" | "mono" | "hover";
}

export const streamDefaults: StreamOptions = { rows: 3, speed: 92, size: 46, gap: 10, tilt: 0, fade: 8, hover: 0.5, names: true, pause: false, logo: "color" };

// Copies of a band's pills: the loop moves by one copy, so a band never shows its end while the screen is narrower
// than the other copies together. Three cover screens up to about 3000 px; wider ones get a fourth after load
// (each copy costs the first paint, which lays out the whole page).
const COPIES = 3;
const WIDE = "(min-width: 2800px)";
// shortcut: the width of one pill with its name is estimated, so the speed is only roughly the one asked for; measure if it must be exact.
const NAME_PX = 130;

/** Splits into `n` rows, dealing one by one (so each row mixes popular and rare names). */
function deal<T>(list: T[], n: number): T[][] {
  const rows: T[][] = Array.from({ length: n }, () => []);
  list.forEach((x, i) => rows[i % n].push(x));
  return rows;
}

// A black mark turns white on a dark page; a light mark turns dark on a light one. Neither gets a plate behind it.
export const toneOf = (a: Agent) => a.tone ?? (a.dark ? "light" : undefined);

export function AgentStream({ agents, options: o, selected, onSelect, href, label }: { agents: Agent[]; options: StreamOptions; selected: string | null; onSelect: (id: string) => void; href: (a: Agent) => string; label: string }) {
  const [copies, setCopies] = useState(COPIES);
  useEffect(() => {
    const wide = matchMedia(WIDE);
    const set = () => setCopies(wide.matches ? COPIES + 1 : COPIES);
    set();
    wide.addEventListener("change", set);
    return () => wide.removeEventListener("change", set);
  }, []);
  const style = { "--copies": copies, "--tilt": `${o.tilt}deg`, "--fade": `${o.fade}%`, "--gap": `${o.gap}px`, "--hover": o.hover, "--s": `${o.size}px` } as CSSProperties;
  return (
    <div className="agent-stream" style={style} data-tilted={o.tilt > 0 || undefined} data-pause={o.pause || undefined}>
      {deal(agents, o.rows).map((row, i) => (
        <Band key={i} row={row} copies={copies} o={o} reverse={i % 2 === 1} selected={selected} onSelect={onSelect} href={href} label={label} />
      ))}
    </div>
  );
}

function Band({ row, copies, o, reverse, selected, onSelect, href, label }: { row: Agent[]; copies: number; o: StreamOptions; reverse: boolean; selected: string | null; onSelect: (id: string) => void; href: (a: Agent) => string; label: string }) {
  const box = useRef<HTMLDivElement>(null);
  const mark = Math.round(o.size * 0.72);
  const seconds = (row.length * (o.size + o.gap + (o.names ? NAME_PX : 0))) / o.speed;
  return (
    <div
      ref={box}
      className="agent-band"
      data-reverse={reverse || undefined}
      style={{ "--dur": `${seconds.toFixed(1)}s` } as CSSProperties}
      // Only keyboard focus stops the loop: a mouse click leaves it running.
      onFocusCapture={(e) => { if (box.current && e.target.matches(":focus-visible")) box.current.dataset.focus = "true"; }}
      onBlurCapture={(e) => { if (box.current && !box.current.contains(e.relatedTarget as Node | null)) delete box.current.dataset.focus; }}
    >
      <ul aria-label={label} className="agent-track">
        {Array.from({ length: copies }, (_, copy) =>
          row.map((a) => (
            <li key={`${copy}-${a.id}`} aria-hidden={copy > 0 || undefined}>
              <a
                href={href(a)}
                className="agent-chip"
                data-on={selected === a.id || undefined}
                data-nameless={o.names ? undefined : true}
                tabIndex={copy > 0 ? -1 : undefined}
                aria-current={copy === 0 && selected === a.id ? "true" : undefined}
                aria-label={o.names ? undefined : a.name}
                draggable={false}
                onClick={(e) => {
                  if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
                  e.preventDefault();
                  onSelect(a.id);
                }}
              >
                {a.logo ? (
                  <img src={a.logo} alt="" width={mark} height={mark} loading="lazy" decoding="async" draggable={false} className="agent-logo" data-tone={toneOf(a)} data-mode={o.logo} />
                ) : (
                  <span aria-hidden="true" className="agent-logo agent-initial">{a.name.slice(0, 1)}</span>
                )}
                {o.names && <span className="agent-name">{a.name}</span>}
              </a>
            </li>
          )),
        )}
      </ul>
    </div>
  );
}
