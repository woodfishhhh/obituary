import { ArchiveRestoreIcon, ChevronDownIcon, ChevronRightIcon, FlameIcon } from "lucide-react";
import { Reveal } from "@/components/ui/reveal";
import type { Copy } from "@/copy";
import { cn } from "@/lib/utils";

// The life of a file, drawn in code: register, expire (extend or spare until then), bury, then
// resurrect or purge. Commands are product text and stay the same in every language; every
// sentence comes from the copy file.
const commands = {
  register: ["obit add debug.log 2h"],
  expire: ["obit list"],
  bury: ["obit graveyard"],
  resurrect: ["obit resurrect 3"],
  purge: ["obit purge"],
} as const;

const card =
  "group relative flex h-full min-w-0 flex-col gap-3 rounded-2xl border bg-paper p-5 shadow-card transition-[border-color,box-shadow,translate] duration-300 ease-out-soft motion-safe:hover:-translate-y-0.5 hover:shadow-glow-soft md:p-6";

function Chip({ children }: { children: string }) {
  return <code className="max-w-full self-start rounded-lg border border-line bg-paper-2 px-2.5 py-1 font-mono text-[13px] break-all text-ink">{children}</code>;
}

// The arrow to the next card: points down in the single column, right in the row.
function Next({ className }: { className?: string }) {
  return (
    <span aria-hidden="true" className={cn("absolute z-10 grid size-8 place-items-center rounded-full border border-line-2 bg-paper text-cobalt shadow-sm", className)}>
      <ChevronDownIcon className="size-4 lg:hidden" />
      <ChevronRightIcon className="hidden size-4 lg:block" />
    </span>
  );
}

export function Lifecycle({ c }: { c: Copy }) {
  const l = c.life;
  const [register, expire, bury] = l.stages;
  const [resurrect, purge] = l.outcomes;
  const stages = [
    { s: register, codes: commands.register },
    { s: expire, codes: commands.expire },
    { s: bury, codes: commands.bury },
  ];
  return (
    <section id="life" aria-labelledby="life-head" className="relative overflow-hidden bg-paper-2 py-24 md:py-32">
      <div aria-hidden="true" className="dot-field fade-edges absolute inset-0 opacity-70" />
      <div className="relative mx-auto max-w-[1240px] px-6 md:px-10">
        <div className="mx-auto max-w-[44rem] text-center">
          <h2 id="life-head" className="text-[clamp(2rem,3.4vw,3.25rem)] leading-[1.08] font-semibold tracking-[-0.02em]">{l.head}</h2>
          <p className="mx-auto mt-5 text-lg text-ink-2">{l.lead}</p>
        </div>

        <ol aria-label={l.label} className="mt-14 grid grid-cols-1 gap-x-10 gap-y-12 lg:grid-cols-4 lg:grid-rows-2 lg:gap-y-5">
          {stages.map(({ s, codes }, i) => (
            <li key={s.name} className="relative min-w-0 lg:row-span-2">
              <Reveal delay={0.08 * i} className="h-full">
                <div className={cn(card, "border-line hover:border-cobalt/40")}>
                  <span aria-hidden="true" className="grid size-10 place-items-center rounded-full border border-glint bg-soft font-mono text-base font-semibold text-cobalt">{i + 1}</span>
                  <h3 className="text-2xl font-semibold tracking-[-0.01em] hyphens-manual">{s.name}</h3>
                  <p className="text-ink-2">{s.line}</p>
                  <div className="mt-auto flex flex-col gap-2 pt-2">
                    {codes.map((code) => <Chip key={code}>{code}</Chip>)}
                    {i === 1 ? (
                      <ul className="flex flex-col gap-2 border-t border-dashed border-line-2 pt-3">
                        {(["extend", "spare"] as const).map((verb) => (
                          <li key={verb} className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                            <Chip>{`obit ${verb}`}</Chip>
                            <span className="text-sm font-medium text-cobalt">{l[verb]}</span>
                          </li>
                        ))}
                      </ul>
                    ) : null}
                  </div>
                </div>
              </Reveal>
              <Next className="-bottom-[2.4rem] left-1/2 -translate-x-1/2 lg:top-1/2 lg:right-[-2.35rem] lg:bottom-auto lg:left-auto lg:translate-x-0 lg:-translate-y-1/2" />
            </li>
          ))}

          {[
            { o: resurrect, codes: commands.resurrect, icon: <ArchiveRestoreIcon className="size-5" />, tone: "border-glint bg-soft/60 hover:border-cobalt/50", badge: "border-glint bg-paper text-cobalt" },
            { o: purge, codes: commands.purge, icon: <FlameIcon className="size-5" />, tone: "border-gold-deep/40 bg-cream/25 hover:border-gold-deep", badge: "border-gold-deep/50 bg-paper text-on-gold" },
          ].map(({ o, codes, icon, tone, badge }, i) => (
            <li key={o.name} className="relative min-w-0">
              {i === 1 ? (
                <span aria-hidden="true" className="absolute -top-[2.1rem] left-1/2 z-10 -translate-x-1/2 rounded-full border border-line-2 bg-paper px-3 py-0.5 text-xs font-medium whitespace-nowrap text-muted lg:-top-[1.45rem]">
                  {l.or}
                </span>
              ) : null}
              <Reveal delay={0.08 * (3 + i)} className="h-full">
                <div className={cn(card, tone)}>
                  <span aria-hidden="true" className={cn("grid size-10 shrink-0 place-items-center rounded-full border", badge)}>{icon}</span>
                  <h3 className="text-2xl font-semibold tracking-[-0.01em] hyphens-manual">{o.name}</h3>
                  <p className="text-ink-2">{o.line}</p>
                  <div className="mt-auto pt-2">
                    {codes.map((code) => <Chip key={code}>{code}</Chip>)}
                  </div>
                </div>
              </Reveal>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}
