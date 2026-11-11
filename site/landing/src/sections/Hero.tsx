import { BorderBeam } from "@/components/ui/border-beam";
import { ShimmerLink } from "@/components/ui/shimmer-link";
import { SparklesText } from "@/components/ui/sparkles-text";
import type { Copy } from "@/copy";
import { brand } from "@/lib/brand";
import { siteLink } from "@/lib/links";
import { shot, shotSize } from "@/lib/shots";

export function Hero({ c }: { c: Copy }) {
  const h = c.hero;
  return (
    <section aria-labelledby="hero-head" className="relative isolate overflow-hidden pt-[4.5rem]">
      {/* A dot field fading out from the centre, and two soft washes in the logo's colours. */}
      <div aria-hidden="true" className="dot-field fade-edges absolute inset-0 -z-10" />
      <div aria-hidden="true" className="absolute -top-40 -left-32 -z-10 size-[42rem] rounded-full bg-glint/50 blur-3xl" />
      <div aria-hidden="true" className="absolute -right-24 bottom-0 -z-10 size-[30rem] rounded-full bg-cream/45 blur-3xl" />

      <div className="mx-auto grid min-h-[calc(100svh-4.5rem)] max-w-[1240px] items-center gap-12 px-6 pt-10 pb-20 md:px-10 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:gap-16 lg:pt-6">
        <div className="flex flex-col items-start gap-6">
          <h1 id="hero-head" className="max-w-[13ch] text-[clamp(2.75rem,5.6vw,4.75rem)] leading-[1.04] font-semibold tracking-[-0.03em] text-ink">
            {h.before}
            <SparklesText className="brand-text" colors={{ first: brand.gold, second: brand.sky }} sparklesCount={8}>
              {h.sparkle}
            </SparklesText>
            {h.after}
          </h1>
          <p className="max-w-[34ch] text-lg leading-relaxed text-balance! text-ink-2 md:text-xl">{h.line}</p>
          <div className="mt-2 flex flex-wrap items-center gap-3">
            <ShimmerLink href="#install" background="var(--color-cobalt)" shimmerColor={brand.gold} shimmerDuration="2.6s" className="h-12 px-7 text-base font-medium">
              {h.primary}
            </ShimmerLink>
            <a
              href={siteLink(c.docs)}
              className="inline-flex h-12 items-center rounded-full border border-line-2 bg-paper/70 px-7 text-base font-medium text-ink backdrop-blur transition-colors hover:border-cobalt/40 hover:bg-soft"
            >
              {h.secondary}
            </a>
          </div>
          <p className="text-sm text-muted">{h.terms}</p>
        </div>

        {/* The product's two faces: One line in front of the Calendar. */}
        <div className="relative w-full pb-10 lg:pb-16">
          <figure className="relative ml-auto w-[93%] overflow-hidden rounded-2xl bg-paper shadow-shot-back ring-1 ring-ink/5">
            <img src={shot(c.lang, "calendar")} alt={h.altBack} width={shotSize.width} height={shotSize.height} className="block h-auto w-full" fetchPriority="high" />
          </figure>
          <figure className="absolute -bottom-2 left-0 w-[80%] overflow-hidden rounded-2xl bg-paper shadow-glow ring-1 ring-ink/5 lg:bottom-0">
            <img src={shot(c.lang, "palette")} alt={h.altFront} width={shotSize.width} height={shotSize.height} className="block h-auto w-full" fetchPriority="high" />
            <BorderBeam size={160} duration={7} colorFrom={brand.sky} colorTo={brand.gold} borderWidth={2} transition={{ repeat: 2 }} />
          </figure>
        </div>
      </div>
    </section>
  );
}
