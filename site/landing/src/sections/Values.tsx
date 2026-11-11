import type { Copy } from "@/copy";

// What you get without doing anything, and where to go when you want more.
export function Values({ c }: { c: Copy }) {
  return (
    <section aria-label={c.lang === "zh" ? "讣告能做什么" : "What Obituary does"} className="relative border-t border-line bg-paper">
      <ol className="mx-auto grid max-w-[1240px] grid-cols-1 gap-10 px-6 py-16 md:grid-cols-2 md:gap-16 md:px-10 md:py-24">
        {c.flow.values.map((v, i) => (
          <li key={v.title} className="flex min-w-0 gap-5">
            <span
              aria-hidden="true"
              className="grid size-11 shrink-0 place-items-center rounded-full border border-glint bg-soft font-mono text-lg font-semibold text-cobalt"
            >
              {i + 1}
            </span>
            <span className="flex min-w-0 flex-col gap-1.5">
              <span className="text-xl font-semibold">{v.title}</span>
              <span className="text-ink-2">{v.line}</span>
            </span>
          </li>
        ))}
      </ol>
    </section>
  );
}
