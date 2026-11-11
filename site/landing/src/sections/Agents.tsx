import { agents, type Agent, type Kind } from "@/agents";
import type { Copy } from "@/copy";
import { siteLink } from "@/lib/links";
import { cn } from "@/lib/utils";

const kinds: Kind[] = ["hook", "plugin", "rules"];

// Avatar style: the mark fills its square; a light mark gets a dark square.
function Mark({ agent }: { agent: Agent }) {
  return (
    <span className={cn("grid size-14 shrink-0 place-items-center overflow-hidden rounded-2xl", agent.dark ? "bg-ink" : "bg-paper")}>
      {agent.logo ? (
        <img src={agent.logo} alt="" width={56} height={56} loading="lazy" className="size-full object-contain" />
      ) : (
        <span aria-hidden="true" className="text-lg font-semibold text-ink-2">
          {agent.name.slice(0, 1)}
        </span>
      )}
    </span>
  );
}

export function Agents({ c }: { c: Copy }) {
  const a = c.agents;
  return (
    <section aria-labelledby="agents-head" className="relative overflow-hidden border-y border-line bg-paper-2 py-24 md:py-32">
      <div aria-hidden="true" className="dot-field fade-edges absolute inset-0 opacity-60" />
      <div className="relative mx-auto max-w-[1240px] px-6 md:px-10">
        <div className="mb-12 grid gap-6 lg:grid-cols-2 lg:items-end">
          <h2 id="agents-head" className="max-w-[16ch] text-[clamp(2rem,3.4vw,3.25rem)] leading-[1.08] font-semibold tracking-[-0.02em]">{a.head}</h2>
          <p className="max-w-[46ch] text-lg text-ink-2">{a.lead}</p>
        </div>

        <div className="flex flex-col divide-y divide-line border-y border-line">
          {kinds.map((kind) => (
            <div key={kind} className="grid grid-cols-1 gap-5 py-8 lg:grid-cols-[13rem_minmax(0,1fr)] lg:gap-10">
              <div className="flex flex-col gap-1">
                <h3 className="font-semibold">{a.kinds[kind].title}</h3>
                <p className="text-sm text-muted">{a.kinds[kind].line}</p>
              </div>
              <ul className="grid min-w-0 grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
                {agents
                  .filter((x) => x.kind === kind)
                  .map((agent) => (
                    <li
                      key={agent.id}
                      className="flex min-w-0 flex-col items-center gap-2.5 rounded-2xl border border-line bg-paper px-2 pt-4 pb-3 text-center shadow-sm transition-colors hover:border-line-2"
                    >
                      <Mark agent={agent} />
                      <span className="flex w-full min-w-0 flex-col">
                        <span className="truncate text-sm font-medium text-ink">{agent.name}</span>
                        <code className="truncate font-mono text-xs text-muted">{agent.id}</code>
                      </span>
                    </li>
                  ))}
              </ul>
            </div>
          ))}
        </div>

        <p className="mt-6 text-sm text-muted">
          {a.note}{" "}
          <a href={siteLink(`${c.docs}${a.href}`)} className="font-medium text-cobalt underline-offset-4 hover:underline">
            {a.link}
          </a>
        </p>
      </div>
    </section>
  );
}
