import { BlurFade } from "@/components/ui/blur-fade";
import { BorderBeam } from "@/components/ui/border-beam";
import { RotatingWord } from "@/components/ui/rotating-word";
import { ShimmerLink } from "@/components/ui/shimmer-link";
import { SparklesText } from "@/components/ui/sparkles-text";
import { Window } from "@/components/window";
import type { Copy } from "@/copy";
import { brand } from "@/lib/brand";
import { siteLink } from "@/lib/links";
import { osList, useOs } from "@/lib/os";
import { cn } from "@/lib/utils";

const osNames = { windows: "Windows", macos: "macOS", linux: "Linux" } as const;

// All three sentences share one cell, so the button and the line below are as wide and as tall as
// the longest: switching the system changes the words, not the layout.
function Swap({ texts, active }: { texts: readonly string[]; active: number }) {
  return (
    <span className="inline-grid">
      {texts.map((t, i) => (
        <span key={t} aria-hidden={i === active ? undefined : true} className={cn("col-start-1 row-start-1", i === active ? undefined : "invisible")}>{t}</span>
      ))}
    </span>
  );
}

export function Hero({ c }: { c: Copy }) {
  const h = c.hero;
  const [os, setOs] = useOs();
  const active = osList.indexOf(os);
  const words = h.tagline.words;
  return (
    <section aria-labelledby="hero-head" className="relative isolate overflow-hidden pt-[4.5rem]">
      {/* A dot field fading out from the centre, soft washes in the logo's colours, and a glow the window floats on. */}
      <div aria-hidden="true" className="dot-field fade-edges absolute inset-0 -z-10" />
      <div aria-hidden="true" className="absolute -top-48 -left-40 -z-10 size-[44rem] rounded-full bg-glint/55 blur-3xl" />
      <div aria-hidden="true" className="absolute -top-24 -right-40 -z-10 size-[34rem] rounded-full bg-cream/55 blur-3xl" />
      <div aria-hidden="true" className="absolute top-[26rem] left-1/2 -z-10 h-[30rem] w-[min(78rem,120vw)] -translate-x-1/2 rounded-[50%] [mask-image:radial-gradient(closest-side,black_55%,transparent)] bg-[conic-gradient(from_200deg_at_50%_50%,var(--color-sky)_0%,var(--color-glint)_30%,var(--color-cream)_58%,var(--color-glint)_82%,var(--color-sky)_100%)] opacity-40 blur-[110px]" />

      <div className="mx-auto flex max-w-[1240px] flex-col items-center px-6 pt-12 text-center md:px-10 md:pt-16 lg:pt-20">
        <BlurFade delay={0.05} direction="up" offset={10}>
          <p className="inline-flex max-w-full items-center rounded-[1.75rem] border border-line-2 bg-paper/70 px-5 py-2 text-[clamp(1.1rem,2.3vw,1.65rem)] leading-snug font-medium text-ink-2 shadow-sm backdrop-blur md:px-7 md:py-2.5">
            <span className="sr-only">{h.tagline.label}</span>
            <span aria-hidden="true" className="inline-flex flex-wrap items-center justify-center gap-x-[0.4em]">
              {h.tagline.before}
              <RotatingWord words={words} className="font-semibold" />
              {h.tagline.after}
            </span>
          </p>
        </BlurFade>

        <BlurFade delay={0.12} direction="up" offset={14}>
          <h1 id="hero-head" className="mt-7 max-w-[18ch] text-[clamp(2.6rem,6.2vw,5.4rem)] leading-[1.04] font-semibold tracking-[-0.035em] text-balance text-ink md:mt-8">
            {h.before}
            <SparklesText className="brand-text" colors={{ first: brand.gold, second: brand.sky }} sparklesCount={8}>
              {h.sparkle}
            </SparklesText>
            {h.after}
          </h1>
        </BlurFade>

        <BlurFade delay={0.2} direction="up" offset={14}>
          <p className="mt-6 max-w-[40ch] text-lg leading-relaxed text-balance text-ink-2 md:text-xl">{h.line}</p>
        </BlurFade>

        <BlurFade delay={0.28} direction="up" offset={14}>
          <div className="mt-9 flex flex-wrap items-center justify-center gap-3">
            <ShimmerLink href="#install" background="var(--color-cobalt)" shimmerColor={brand.gold} shimmerDuration="2.6s" className="h-12 px-7 text-base font-medium">
              <Swap texts={osList.map((o) => h.primary[o])} active={active} />
            </ShimmerLink>
            <a
              href={siteLink(c.docs)}
              className="inline-flex h-12 items-center rounded-full border border-line-2 bg-paper/70 px-7 text-base font-medium text-ink backdrop-blur transition-colors hover:border-cobalt/40 hover:bg-soft"
            >
              {h.secondary}
            </a>
          </div>
          <div role="radiogroup" aria-label={h.systemLabel} className="mt-5 inline-flex rounded-full border border-line bg-paper/80 p-0.5 text-sm backdrop-blur">
            {osList.map((o) => (
              <button
                key={o}
                type="button"
                role="radio"
                aria-checked={os === o}
                onClick={() => setOs(o)}
                className={cn("min-h-9 rounded-full px-3.5 font-medium whitespace-nowrap transition-colors", os === o ? "bg-ink text-paper" : "text-ink-2 hover:bg-paper-3 hover:text-ink")}
              >
                {osNames[o]}
              </button>
            ))}
          </div>
          <p className="mt-3 text-sm text-muted"><Swap texts={osList.map((o) => h.terms[o])} active={active} /></p>
        </BlurFade>
      </div>

      {/* The app's own calendar, running on sample data: hover a file, click to extend it, drag it to another day. */}
      <div className="mx-auto mt-12 max-w-[1180px] px-4 pb-20 md:mt-14 md:px-8 md:pb-28">
        <BlurFade delay={0.4} direction="up" offset={28} duration={0.7} blur="10px">
          <figure className="relative overflow-hidden rounded-2xl bg-paper shadow-[0_50px_120px_-40px_rgb(0_72_232/0.5)] ring-1 ring-ink/10 md:rounded-3xl">
            <Window c={c} look="calendar" page="upcoming" interactive height={600} minScale={0.72} />
            <BorderBeam size={220} duration={9} colorFrom={brand.sky} colorTo={brand.gold} borderWidth={2} />
          </figure>
          <ul className="mt-6 flex flex-wrap items-center justify-center gap-x-6 gap-y-2 text-sm text-muted">
            {c.demo.tips.map((tip, i) => (
              <li key={tip} className="inline-flex items-center gap-2">
                <span aria-hidden="true" className="grid size-5 place-items-center rounded-full border border-line-2 bg-paper text-[11px] font-semibold text-cobalt">{i + 1}</span>
                {tip}
              </li>
            ))}
          </ul>
        </BlurFade>
      </div>
    </section>
  );
}
