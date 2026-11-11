import { useState } from "react";
import { BorderBeam } from "@/components/ui/border-beam";
import { brand } from "@/lib/brand";
import type { Copy, LookId, PageId } from "@/copy";
import { shot, shotSize, type Shot } from "@/lib/shots";
import { cn } from "@/lib/utils";

function Segmented<T extends string>({ label, options, value, onChange }: { label: string; options: [T, string][]; value: T; onChange: (v: T) => void }) {
  return (
    <div role="radiogroup" aria-label={label} className="inline-flex max-w-full overflow-x-auto rounded-full border border-line bg-paper p-1 shadow-sm">
      {options.map(([id, name]) => (
        <button
          key={id}
          type="button"
          role="radio"
          aria-checked={value === id}
          onClick={() => onChange(id)}
          className={cn(
            "min-h-10 rounded-full px-3 text-sm font-medium whitespace-nowrap transition-colors sm:px-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cobalt",
            value === id ? "bg-cobalt text-white shadow-sm" : "text-ink-2 hover:bg-paper-3 hover:text-ink",
          )}
        >
          {name}
        </button>
      ))}
    </div>
  );
}

export function WindowLooks({ c }: { c: Copy }) {
  const w = c.window;
  const [look, setLook] = useState<LookId>("palette");
  const [page, setPage] = useState<PageId>("upcoming");
  const name: Shot = page === "upcoming" ? look : `${look}-${page}`;
  return (
    <section id="window" aria-labelledby="window-head" className="py-24 md:py-32">
      <div className="mx-auto max-w-[1240px] px-6 md:px-10">
        <div className="mb-10 grid gap-6 lg:grid-cols-2 lg:items-end">
          <h2 id="window-head" className="max-w-[18ch] text-[clamp(2rem,3.4vw,3.25rem)] leading-[1.08] font-semibold tracking-[-0.02em]">{w.head}</h2>
          <p className="max-w-[46ch] text-lg text-ink-2">{w.lead}</p>
        </div>
        <div className="mb-6 flex flex-wrap items-center gap-3">
          <Segmented label={w.lookLabel} options={(["palette", "calendar"] as LookId[]).map((id) => [id, w.looks[id]])} value={look} onChange={setLook} />
          <Segmented label={w.pageLabel} options={(["upcoming", "graveyard", "decisions"] as PageId[]).map((id) => [id, w.pages[id]])} value={page} onChange={setPage} />
        </div>
        <div className="grid gap-8 lg:grid-cols-[minmax(0,8fr)_minmax(0,4fr)] lg:items-start">
          <figure className="relative overflow-hidden rounded-2xl bg-paper shadow-glow-soft ring-1 ring-ink/5">
            <img
              key={`${look}-${page}`}
              src={shot(c.lang, name)}
              alt={w.alt(w.looks[look], w.pages[page])}
              width={shotSize.width}
              height={shotSize.height}
              loading="lazy"
              decoding="async"
              className="block h-auto w-full"
            />
            <BorderBeam size={200} duration={9} colorFrom={brand.sky} colorTo={brand.gold} borderWidth={2} transition={{ repeat: 1 }} />
          </figure>
          <div className="flex flex-col gap-6 lg:pt-2" aria-live="polite">
            <div>
              <h3 className="text-xl font-semibold">{w.looks[look]}</h3>
              <p className="mt-2 text-ink-2">{w.notes[look]}</p>
            </div>
            <div className="border-t border-line pt-6">
              <h3 className="text-xl font-semibold">{w.pages[page]}</h3>
              <p className="mt-2 text-ink-2">{w.pageNotes[page]}</p>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
