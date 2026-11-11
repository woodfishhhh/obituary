import { forwardRef, useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { animate, motion, useInView, useMotionValue, useReducedMotion } from "motion/react";
import { ArchiveIcon, CheckIcon, FlameIcon, FolderIcon, SparkleIcon, TerminalSquareIcon } from "lucide-react";
import CountUp from "@/components/reactbits/CountUp";
import DecryptedText from "@/components/reactbits/DecryptedText";
import ElectricBorder from "@/components/reactbits/ElectricBorder";
import StarBorder from "@/components/reactbits/StarBorder";
import type { Copy } from "@/copy";
import { brand } from "@/lib/brand";
import { cn } from "@/lib/utils";

// One delete, told as a loop: an agent types rm -rf, Obituary denies it and
// offers the obit equivalent, the folder lands in the graveyard, and then it is
// either resurrected or purged. Each round moves to the next agent and
// alternates the ending. Under reduced motion the finished state stays put.
type Phase = "type" | "toObit" | "intercept" | "toGrave" | "rest" | "branch" | "settle";
const SCHEDULE: readonly [Phase, number][] = [
  ["type", 1300],
  ["toObit", 800],
  ["intercept", 1700],
  ["toGrave", 800],
  ["rest", 1300],
  ["branch", 900],
  ["settle", 1800],
];
const TRAVEL: Partial<Record<Phase, true>> = {
  toObit: true,
  toGrave: true,
  branch: true,
};
const at = (phase: Phase) => SCHEDULE.findIndex(([p]) => p === phase);
const RM = "rm -rf build";
const OBIT_RM = "obit rm -rf build";

const Node = forwardRef<HTMLDivElement, { className?: string; children: ReactNode }>(function Node({ className, children }, ref) {
  return (
    <div
      ref={ref}
      className={cn(
        "relative z-10 flex items-center gap-3 rounded-2xl border border-line bg-paper px-4 py-3 shadow-node transition-[box-shadow,border-color] duration-300",
        className,
      )}
    >
      {children}
    </div>
  );
});

type Point = { x: number; y: number };
type Side = "left" | "right" | "top" | "bottom";

function anchor(el: HTMLElement, box: HTMLElement, side: Side): Point {
  const r = el.getBoundingClientRect();
  const b = box.getBoundingClientRect();
  const x = side === "left" ? r.left : side === "right" ? r.right : r.left + r.width / 2;
  const y = side === "top" ? r.top : side === "bottom" ? r.bottom : r.top + r.height / 2;
  return { x: x - b.left, y: y - b.top };
}

// Horizontal hops bow sideways, the graveyard's branches drop downward.
function curve(a: Point, b: Point, vertical = false): string {
  if (vertical) {
    const dy = (b.y - a.y) / 2;
    return `M ${a.x} ${a.y} C ${a.x} ${a.y + dy}, ${b.x} ${b.y - dy}, ${b.x} ${b.y}`;
  }
  const dx = (b.x - a.x) / 2;
  return `M ${a.x} ${a.y} C ${a.x + dx} ${a.y}, ${b.x - dx} ${b.y}, ${b.x} ${b.y}`;
}

type Refs = {
  box: RefObject<HTMLDivElement | null>;
  agents: RefObject<HTMLDivElement | null>[];
  obit: RefObject<HTMLDivElement | null>;
  grave: RefObject<HTMLDivElement | null>;
  back: RefObject<HTMLDivElement | null>;
  end: RefObject<HTMLDivElement | null>;
};

// Path strings keyed "agent0".."agent2", "grave", "back", "end"; recomputed on resize.
function usePaths(refs: Refs): Record<string, string> {
  const [paths, setPaths] = useState<Record<string, string>>({});
  const measure = useCallback(() => {
    const box = refs.box.current;
    const obit = refs.obit.current;
    const grave = refs.grave.current;
    const back = refs.back.current;
    const end = refs.end.current;
    if (!box || !obit || !grave || !back || !end) return;
    const next: Record<string, string> = {};
    refs.agents.forEach((r, i) => {
      if (r.current) next[`agent${i}`] = curve(anchor(r.current, box, "right"), anchor(obit, box, "left"));
    });
    next.grave = curve(anchor(obit, box, "right"), anchor(grave, box, "left"));
    next.back = curve(anchor(grave, box, "bottom"), anchor(back, box, "top"), true);
    next.end = curve(anchor(grave, box, "bottom"), anchor(end, box, "top"), true);
    setPaths(next);
  }, [refs]);
  useLayoutEffect(() => {
    measure();
    const box = refs.box.current;
    if (!box) return;
    const observer = new ResizeObserver(measure);
    observer.observe(box);
    return () => observer.disconnect();
  }, [measure, refs.box]);
  return paths;
}

// Types the command one character at a time while the phase is "type".
function Typed({ text, running }: { text: string; running: boolean }) {
  const [n, setN] = useState(running ? 0 : text.length);
  useEffect(() => {
    if (!running) {
      setN(text.length);
      return;
    }
    setN(0);
    const id = window.setInterval(() => setN((v) => (v >= text.length ? v : v + 1)), 70);
    return () => window.clearInterval(id);
  }, [running, text]);
  return (
    <span>
      {text.slice(0, n)}
      {running ? <span className="ml-px inline-block h-[1.05em] w-[0.5ch] translate-y-[0.15em] bg-cobalt" /> : null}
    </span>
  );
}

export function Flow({ c }: { c: Copy }) {
  const f = c.flow;
  const reduce = useReducedMotion() ?? false;
  const box = useRef<HTMLDivElement>(null);
  const a0 = useRef<HTMLDivElement>(null);
  const a1 = useRef<HTMLDivElement>(null);
  const a2 = useRef<HTMLDivElement>(null);
  const obit = useRef<HTMLDivElement>(null);
  const grave = useRef<HTMLDivElement>(null);
  const back = useRef<HTMLDivElement>(null);
  const end = useRef<HTMLDivElement>(null);
  const refs = useRef<Refs>({
    box,
    agents: [a0, a1, a2],
    obit,
    grave,
    back,
    end,
  }).current;
  const paths = usePaths(refs);
  const inView = useInView(box, { amount: 0.35 });
  const running = inView && !reduce;

  // The still picture under reduced motion: the first agent, resurrected.
  const [round, setRound] = useState(0);
  const [step, setStep] = useState(reduce ? at("settle") : 0);
  const phase = SCHEDULE[step][0];
  useEffect(() => {
    if (!running) return;
    const id = window.setTimeout(() => {
      if (step + 1 < SCHEDULE.length) setStep(step + 1);
      else {
        setStep(0);
        setRound((r) => r + 1);
      }
    }, SCHEDULE[step][1]);
    return () => window.clearTimeout(id);
  }, [running, step]);

  const agent = round % 3;
  const ending: "back" | "end" = round % 2 === 0 ? "back" : "end";
  const past = (p: Phase) => step >= at(p);

  // The folder travels along whichever path the phase names.
  const pathEls = useRef<Record<string, SVGPathElement | null>>({});
  const x = useMotionValue(0);
  const y = useMotionValue(0);
  useEffect(() => {
    if (!running || !TRAVEL[phase]) return;
    const key = phase === "toObit" ? `agent${agent}` : phase === "toGrave" ? "grave" : ending;
    const el = pathEls.current[key];
    if (!el) return;
    const length = el.getTotalLength();
    const controls = animate(0, 1, {
      duration: (SCHEDULE[step][1] / 1000) * 0.85,
      ease: [0.65, 0, 0.35, 1],
      onUpdate: (t) => {
        const p = el.getPointAtLength(t * length);
        x.set(p.x);
        y.set(p.y);
      },
    });
    return () => controls.stop();
  }, [running, phase, step, agent, ending, x, y]);

  const traveling = running && TRAVEL[phase];
  const packetVisible = running && past("toObit") && !(phase === "settle");
  const activePath = phase === "toObit" ? `agent${agent}` : phase === "toGrave" ? "grave" : phase === "branch" ? ending : "";
  const commandSwapped = past("intercept");
  const inGrave = past("rest") && !(phase === "settle");
  const resurrected = running && phase === "settle" && ending === "back";
  const purged = running && phase === "settle" && ending === "end";

  return (
    <section id="why" aria-labelledby="flow-head" className="relative border-y border-line bg-paper-2 py-24 md:py-32">
      <div aria-hidden="true" className="grid-field fade-edges absolute inset-0" />
      <div className="relative mx-auto max-w-[1240px] px-6 md:px-10">
        <div className="mb-14 grid gap-6 lg:grid-cols-2 lg:items-end">
          <h2 id="flow-head" className="max-w-[18ch] text-[clamp(2rem,3.4vw,3.25rem)] leading-[1.08] font-semibold tracking-[-0.02em]">
            {f.head}
          </h2>
          <p className="max-w-[46ch] text-lg text-ink-2">{f.lead}</p>
        </div>

        <div
          ref={box}
          className="relative grid grid-cols-1 items-center gap-6 rounded-3xl border border-line bg-paper/80 p-6 backdrop-blur lg:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] lg:gap-10 md:p-12"
        >
          <svg aria-hidden="true" className="pointer-events-none absolute inset-0 hidden h-full w-full overflow-visible lg:block">
            <defs>
              <linearGradient id="flow-live" x1="0" y1="0" x2="1" y2="0">
                <stop offset="0" stopColor={brand.sky} />
                <stop offset="1" stopColor={brand.gold} />
              </linearGradient>
            </defs>
            {Object.entries(paths).map(([key, d]) => (
              <g key={key}>
                <path
                  ref={(el) => {
                    pathEls.current[key] = el;
                  }}
                  d={d}
                  fill="none"
                  stroke={brand.glint}
                  strokeWidth={2}
                  strokeDasharray="4 6"
                  strokeLinecap="round"
                />
                {running && activePath === key ? (
                  <motion.path
                    key={`${key}-${round}-${step}`}
                    d={d}
                    fill="none"
                    stroke="url(#flow-live)"
                    strokeWidth={2.5}
                    strokeLinecap="round"
                    initial={{ pathLength: 0, opacity: 1 }}
                    animate={{ pathLength: 1, opacity: 1 }}
                    transition={{
                      duration: (SCHEDULE[step][1] / 1000) * 0.85,
                      ease: [0.65, 0, 0.35, 1],
                    }}
                  />
                ) : null}
              </g>
            ))}
          </svg>

          <div className="flex flex-col gap-4">
            {f.agents.map((name, i) => {
              const active = i === agent && !reduce ? true : reduce && i === 0;
              return (
                <Node
                  key={name}
                  ref={refs.agents[i]}
                  className={cn("flex-col items-start gap-2 lg:max-w-[17rem]", active && "border-sky shadow-[0_0_0_3px_rgb(0_120_248/0.14)]")}
                >
                  <span className="flex items-center gap-3">
                    <TerminalSquareIcon className={cn("size-5 shrink-0", active ? "text-cobalt" : "text-ink-2")} aria-hidden="true" />
                    <span className="font-medium">{name}</span>
                    <span
                      className={cn(
                        // A tag on the node's top edge, outside the flow, so the name never wraps for it.
                        "absolute -top-3 right-3 inline-flex items-center gap-1 rounded-full border border-sky/40 bg-paper px-2 py-0.5 text-[0.75rem] font-medium whitespace-nowrap text-cobalt shadow-node transition-[opacity,transform] duration-300",
                        resurrected && active ? "translate-y-0 opacity-100" : "translate-y-1 opacity-0",
                      )}
                    >
                      <CheckIcon className="size-3.5" aria-hidden="true" />
                      {f.backDone}
                    </span>
                  </span>
                  {/* Every agent keeps its prompt line so the column never reflows. */}
                  <code
                    className={cn(
                      "flex min-h-[1.5rem] items-center gap-2 rounded-lg px-2 py-0.5 font-mono text-[0.8125rem] transition-colors duration-300",
                      active ? "bg-paper-2 text-ink-2" : "bg-transparent text-muted",
                    )}
                  >
                    <span className="text-muted">$</span>
                    {!active ? null : commandSwapped ? (
                      <DecryptedText
                        key={`swap-${round}`}
                        text={OBIT_RM}
                        animateOn="view"
                        sequential
                        speed={28}
                        className="text-cobalt"
                        encryptedClassName="text-sky"
                        characters="abcdefghijklmnopqrstuvwxyz-/"
                      />
                    ) : (
                      <Typed text={RM} running={running && phase === "type"} />
                    )}
                  </code>
                </Node>
              );
            })}
          </div>

          <ElectricBorder
            active={running && phase === "intercept"}
            intro={false}
            interactive={false}
            theme="light"
            color={brand.cobalt}
            glowColor={brand.sky}
            thickness={2.5}
            intensity={1.3}
            glow={1}
            chaos={0.6}
            arcs={2}
            borderRadius={16}
            className="lg:mx-6"
          >
            <Node ref={obit} className={cn("flex-col gap-2 px-8 py-6 text-center", running && phase === "intercept" && "border-transparent")}>
              <img src={`${import.meta.env.BASE_URL}icon-128.png`} alt="" width={80} height={80} className="size-20" />
              <span className="text-lg font-semibold">{f.obit}</span>
              <span className={cn("max-w-[14ch] text-sm transition-colors duration-300", running && phase === "intercept" ? "text-cobalt" : "text-muted")}>
                {f.obitNote}
              </span>
            </Node>
          </ElectricBorder>

          <div className="flex flex-col gap-4 lg:items-end lg:gap-14">
            <Node ref={grave} className={cn("flex-col items-start gap-2 lg:min-w-[16rem]", inGrave && "border-sky shadow-[0_0_0_3px_rgb(0_120_248/0.14)]")}>
              <span className="flex items-center gap-3">
                <ArchiveIcon className="size-5 shrink-0 text-cobalt" aria-hidden="true" />
                <span className="flex flex-col leading-tight">
                  <span className="font-medium">{f.grave}</span>
                  <span className="text-sm text-muted">{f.graveNote}</span>
                </span>
              </span>
              {/* The row is always laid out so the node keeps its size; it only fades. */}
              <span
                className={cn(
                  "flex items-center gap-2 rounded-lg bg-paper-2 px-2 py-0.5 text-[0.8125rem] text-ink-2 transition-opacity duration-300",
                  inGrave || reduce ? "opacity-100" : "opacity-0",
                )}
              >
                <FolderIcon className="size-3.5 text-cobalt" aria-hidden="true" />
                <span className="font-mono">build/</span>
                <span className="text-muted">
                  {f.graveLeft}{" "}
                  <CountUp
                    key={`days-${round}-${past("rest")}-${ending === "end" && past("branch") ? "down" : "up"}`}
                    from={ending === "end" && past("branch") ? 7 : 0}
                    to={ending === "end" && past("branch") ? 0 : 7}
                    duration={ending === "end" && past("branch") ? 0.7 : 0.9}
                    trigger="load"
                    className="font-semibold text-cobalt tabular-nums"
                  />{" "}
                  {f.days}
                </span>
              </span>
            </Node>
            <div className="flex flex-col gap-4 sm:flex-row">
              {/* StarBorder keeps its own root ref for the canvas, so the path anchor wraps it. */}
              <div ref={back} className="relative z-10 flex">
                <StarBorder
                  as="div"
                  theme="light"
                  color={brand.gold}
                  trailColor={brand.sky}
                  hover="none"
                  clickPulse={false}
                  sparkle={resurrected || (running && phase === "branch" && ending === "back")}
                  glow={resurrected ? 1.2 : 0.35}
                  duration={resurrected ? 1.6 : 6}
                  radius={16}
                  backgroundColor="var(--color-paper)"
                  textColor="var(--color-ink)"
                  borderColor="var(--color-line)"
                  className="w-full justify-start! gap-3! px-4! py-3! font-normal! shadow-node"
                >
                  <SparkleIcon className="size-5 shrink-0 fill-gold text-gold-deep" aria-hidden="true" />
                  <span className="flex flex-col text-left leading-tight">
                    <span className="font-medium">{f.back}</span>
                    <code className="font-mono text-xs text-muted">{f.backNote}</code>
                  </span>
                </StarBorder>
              </div>
              <Node ref={end} className={cn(purged && "border-gold-deep shadow-[0_0_0_3px_rgb(224_184_0/0.18)]")}>
                <FlameIcon className={cn("size-5 shrink-0 transition-colors duration-300", purged ? "text-gold-deep" : "text-ink-2")} aria-hidden="true" />
                <span className="flex flex-col leading-tight">
                  <span className="font-medium">{f.end}</span>
                  {/* Both notes share one grid cell, so swapping them never resizes the node. */}
                  <span className="grid text-sm text-muted">
                    <span className={cn("col-start-1 row-start-1 transition-opacity duration-300", purged ? "opacity-0" : "opacity-100")}>{f.endNote}</span>
                    <span
                      aria-hidden={!purged}
                      className={cn("col-start-1 row-start-1 text-ink-2 transition-opacity duration-300", purged ? "opacity-100" : "opacity-0")}
                    >
                      {f.endDone}
                    </span>
                  </span>
                </span>
              </Node>
            </div>
          </div>

          {/* The folder in flight; desktop only, where the paths are drawn. */}
          <motion.div
            aria-hidden="true"
            className="pointer-events-none absolute top-0 left-0 z-20 hidden lg:block"
            style={{ x, y }}
            initial={false}
            animate={{
              opacity: packetVisible ? 1 : 0,
              scale: traveling ? 1 : 0.85,
            }}
            transition={{ duration: 0.25 }}
          >
            <span className="flex -translate-x-1/2 -translate-y-1/2 items-center gap-1.5 rounded-full border border-sky/40 bg-paper px-2.5 py-1 font-mono text-xs font-medium text-cobalt shadow-[0_6px_18px_-8px_rgb(0_72_232/0.55)]">
              <FolderIcon className="size-3.5" aria-hidden="true" />
              build/
            </span>
          </motion.div>
        </div>
      </div>
    </section>
  );
}
