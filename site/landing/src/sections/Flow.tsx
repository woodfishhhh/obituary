import { forwardRef, useRef, type ReactNode } from "react";
import { useReducedMotion } from "motion/react";
import { ArchiveIcon, FlameIcon, SparkleIcon, TerminalSquareIcon } from "lucide-react";
import { AnimatedBeam } from "@/components/ui/animated-beam";
import type { Copy } from "@/copy";
import { brand } from "@/lib/brand";
import { cn } from "@/lib/utils";

const Node = forwardRef<HTMLDivElement, { className?: string; children: ReactNode }>(function Node({ className, children }, ref) {
  return (
    <div ref={ref} className={cn("relative z-10 flex items-center gap-3 rounded-2xl border border-line bg-paper px-4 py-3 shadow-node", className)}>
      {children}
    </div>
  );
});

export function Flow({ c }: { c: Copy }) {
  const f = c.flow;
  const reduce = useReducedMotion();
  const box = useRef<HTMLDivElement>(null);
  const a0 = useRef<HTMLDivElement>(null);
  const a1 = useRef<HTMLDivElement>(null);
  const a2 = useRef<HTMLDivElement>(null);
  const obit = useRef<HTMLDivElement>(null);
  const grave = useRef<HTMLDivElement>(null);
  const back = useRef<HTMLDivElement>(null);
  const end = useRef<HTMLDivElement>(null);
  const agentRefs = [a0, a1, a2];

  // The beam is the delete's path; under reduced motion only the still line stays.
  const beam = {
    containerRef: box,
    pathColor: brand.glint,
    pathOpacity: 0.9,
    pathWidth: 2,
    gradientStartColor: reduce ? "transparent" : brand.sky,
    gradientStopColor: reduce ? "transparent" : brand.gold,
    duration: 3.2,
    repeat: reduce ? 0 : 3,
    repeatDelay: 0.6,
  };

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

        <div ref={box} className="relative grid grid-cols-1 items-center gap-6 rounded-3xl border border-line bg-paper/80 p-6 backdrop-blur md:grid-cols-[1fr_auto_1fr] md:gap-10 md:p-12">
          <div className="flex flex-col gap-4">
            {f.agents.map((name, i) => (
              <Node key={name} ref={agentRefs[i]} className="md:max-w-[15rem]">
                <TerminalSquareIcon className="size-5 shrink-0 text-ink-2" aria-hidden="true" />
                <span className="font-medium">{name}</span>
              </Node>
            ))}
          </div>

          <Node ref={obit} className="flex-col gap-2 px-8 py-6 text-center md:mx-6">
            <img src={`${import.meta.env.BASE_URL}icon-128.png`} alt="" width={80} height={80} className="size-20" />
            <span className="text-lg font-semibold">{f.obit}</span>
            <span className="max-w-[14ch] text-sm text-muted">{f.obitNote}</span>
          </Node>

          <div className="flex flex-col gap-4 md:items-end">
            <Node ref={grave} className="md:min-w-[15rem]">
              <ArchiveIcon className="size-5 shrink-0 text-cobalt" aria-hidden="true" />
              <span className="flex flex-col leading-tight">
                <span className="font-medium">{f.grave}</span>
                <span className="text-sm text-muted">{f.graveNote}</span>
              </span>
            </Node>
            <div className="flex gap-4">
              <Node ref={back}>
                <SparkleIcon className="size-5 shrink-0 fill-gold text-gold-deep" aria-hidden="true" />
                <span className="flex flex-col leading-tight">
                  <span className="font-medium">{f.back}</span>
                  <code className="font-mono text-xs text-muted">{f.backNote}</code>
                </span>
              </Node>
              <Node ref={end}>
                <FlameIcon className="size-5 shrink-0 text-ink-2" aria-hidden="true" />
                <span className="flex flex-col leading-tight">
                  <span className="font-medium">{f.end}</span>
                  <span className="text-sm text-muted">{f.endNote}</span>
                </span>
              </Node>
            </div>
          </div>

          <div aria-hidden="true" className="hidden md:block">
            {agentRefs.map((r, i) => (
              <AnimatedBeam key={i} {...beam} fromRef={r} toRef={obit} curvature={(1 - i) * 40} delay={i * 0.25} />
            ))}
            <AnimatedBeam {...beam} fromRef={obit} toRef={grave} delay={0.9} />
            <AnimatedBeam {...beam} fromRef={grave} toRef={back} delay={1.6} curvature={-20} />
            <AnimatedBeam {...beam} fromRef={grave} toRef={end} delay={1.9} curvature={-20} gradientStopColor={reduce ? "transparent" : brand.muted} />
          </div>
        </div>
      </div>
    </section>
  );
}
