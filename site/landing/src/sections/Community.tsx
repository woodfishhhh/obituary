import { useId } from "react";
import type { Copy } from "@/copy";
import { repo } from "@/copy";
import { cn } from "@/lib/utils";

interface Community {
  stars: number;
  contributors: { login: string; url: string; avatar: string; contributions: number }[];
  starHistory: string | null;
}

// Written by scripts/community.mjs at build time; absent in a bare checkout,
// which leaves only the open slots.
const generated = import.meta.glob<Community>("../generated/community.json", { eager: true, import: "default" });
const data: Community = Object.values(generated)[0] ?? { stars: 0, contributors: [], starHistory: null };

const contributing = `${repo}/blob/main/CONTRIBUTING.md`;
const local = (path: string) => `${import.meta.env.BASE_URL}${path}`;

// An open place on the wall: a dashed circle and a plus, blue to yellow.
function OpenPlace({ dim }: { dim: boolean }) {
  // React's ids carry punctuation that url(#…) references do not accept everywhere.
  const id = `open-place-${useId().replace(/[^\w-]/g, "")}`;
  const paint = `url(#${id})`;
  return (
    <svg
      viewBox="0 0 64 64"
      aria-hidden="true"
      className={cn("aspect-square w-full max-w-16 transition-opacity", dim ? "opacity-50 group-hover:opacity-90" : "group-hover:opacity-80")}
    >
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" style={{ stopColor: "var(--color-cobalt)" }} />
          <stop offset="0.5" style={{ stopColor: "var(--color-sky)" }} />
          <stop offset="1" style={{ stopColor: "var(--color-gold-deep)" }} />
        </linearGradient>
      </defs>
      <circle cx="32" cy="32" r="30" fill="var(--color-paper)" stroke={paint} strokeWidth="2" strokeDasharray="5 5" strokeLinecap="round" />
      <path d="M32 22v20M22 32h20" stroke={paint} strokeWidth="3" strokeLinecap="round" />
    </svg>
  );
}

export function Community({ c }: { c: Copy }) {
  const m = c.community;
  // Always leave room on the wall: at least three open places, more to round off the row.
  const open = 3 + ((8 - ((data.contributors.length + 3) % 8)) % 8);
  const charts = [
    data.starHistory && { src: data.starHistory, title: m.stars, alt: m.starHistoryAlt },
  ].filter((x): x is { src: string; title: string; alt: string } => Boolean(x));
  return (
    <section aria-labelledby="community-head" className="relative border-t border-line bg-paper-2 py-24 md:pt-32">
      <div className="mx-auto max-w-[1240px] px-6 md:px-10">
        <div className="mb-12 grid gap-6 lg:grid-cols-2 lg:items-end">
          <h2 id="community-head" className="max-w-[16ch] text-[clamp(2rem,3.4vw,3.25rem)] leading-[1.08] font-semibold tracking-[-0.02em]">
            {m.head}
          </h2>
          <p className="max-w-[48ch] text-lg text-ink-2">
            {m.lead}{" "}
            <a href={contributing} className="font-medium text-cobalt underline-offset-4 hover:underline">
              {m.howTo}
            </a>
          </p>
        </div>

        <ul className="grid grid-cols-4 gap-3 sm:grid-cols-6 lg:grid-cols-8">
          {data.contributors.map((p) => (
            <li key={p.login} className="min-w-0">
              <a
                href={p.url}
                className="group flex flex-col items-center gap-2 rounded-2xl p-2 transition-colors hover:bg-paper focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cobalt"
              >
                <img
                  src={local(p.avatar)}
                  alt=""
                  width={96}
                  height={96}
                  loading="lazy"
                  className="aspect-square w-full max-w-16 rounded-full bg-paper shadow-sm ring-2 ring-paper"
                />
                <span className="w-full truncate text-center text-xs text-ink-2 group-hover:text-ink">{p.login}</span>
              </a>
            </li>
          ))}
          {Array.from({ length: open }, (_, i) => (
            <li key={`open-${i}`} className={cn("min-w-0", i >= 3 && "hidden lg:block")}>
              <a
                href={contributing}
                aria-hidden={i > 0 || undefined}
                tabIndex={i > 0 ? -1 : undefined}
                className="group flex flex-col items-center gap-2 rounded-2xl p-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cobalt"
              >
                <OpenPlace dim={i > 0} />
                <span className={cn("w-full truncate text-center text-xs font-medium", i === 0 ? "text-cobalt" : "invisible")}>{m.next}</span>
              </a>
            </li>
          ))}
        </ul>

        {charts.length > 0 ? (
          <div className="mt-14 grid grid-cols-1 gap-4 md:grid-cols-2">
            {charts.map((chart) => (
              <figure key={chart.src} className="flex min-w-0 flex-col gap-3 rounded-2xl border border-line bg-paper p-4">
                <figcaption className="text-sm font-medium text-ink-2">{chart.title}</figcaption>
                <img src={local(chart.src)} alt={chart.alt} loading="lazy" className="h-auto w-full" />
              </figure>
            ))}
          </div>
        ) : null}
      </div>
    </section>
  );
}
