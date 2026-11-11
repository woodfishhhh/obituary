import type { Copy } from "@/copy";
import { shot, shotSize } from "@/lib/shots";

export function Lifecycle({ c }: { c: Copy }) {
  const l = c.life;
  return (
    <section id="life" aria-labelledby="life-head" className="relative overflow-hidden bg-paper-2 py-24 md:py-32">
      <div aria-hidden="true" className="dot-field fade-edges absolute inset-0 opacity-70" />
      <div className="relative mx-auto grid max-w-[1240px] items-center gap-14 px-6 md:px-10 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
        <div>
          <h2 id="life-head" className="text-[clamp(2rem,3.4vw,3.25rem)] leading-[1.08] font-semibold tracking-[-0.02em]">{l.head}</h2>
          <p className="mt-5 max-w-[40ch] text-lg text-ink-2">{l.lead}</p>
          <ol className="relative mt-10 flex flex-col gap-8">
            <span aria-hidden="true" className="brand-gradient absolute top-6 bottom-6 left-[1.4rem] w-0.5 rounded-full opacity-60" />
            {l.steps.map((step, i) => (
              <li key={step.name} className="relative grid grid-cols-[3rem_minmax(0,1fr)] gap-x-5">
                <span aria-hidden="true" className="relative grid size-12 place-items-center rounded-full border border-line-2 bg-paper text-lg font-semibold text-cobalt shadow-sm">
                  {i + 1}
                </span>
                <div className="flex flex-col gap-2 pt-1">
                  <h3 className="text-2xl font-semibold tracking-[-0.01em]">{step.name}</h3>
                  <p className="text-ink-2">{step.line}</p>
                  <code className="self-start rounded-lg border border-line bg-paper px-2.5 py-1 font-mono text-sm text-ink">{step.code}</code>
                </div>
              </li>
            ))}
          </ol>
        </div>
        <figure className="overflow-hidden rounded-2xl bg-paper shadow-shot ring-1 ring-ink/5">
          <img src={shot(c.lang, "calendar-graveyard")} alt={l.alt} width={shotSize.width} height={shotSize.height} loading="lazy" decoding="async" className="block h-auto w-full" />
        </figure>
      </div>
    </section>
  );
}
