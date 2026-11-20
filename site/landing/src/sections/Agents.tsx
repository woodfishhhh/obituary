import { agentDocLink, agents } from "@/agents";
import LogoLoop from "@/components/reactbits/LogoLoop";
import type { Copy } from "@/copy";
import { siteLink } from "@/lib/links";

// Three bands of nine agents, each agent in one band, running in alternate directions; each logo opens its documentation.
export function Agents({ c }: { c: Copy }) {
  const a = c.agents;
  const items = agents.map((x) => ({ name: x.name, logo: x.logo, dark: x.dark, href: siteLink(agentDocLink(x.id, c.lang)) }));
  const rows = [0, 1, 2].map((r) => items.slice(r * 9, r * 9 + 9));
  return (
    <section aria-labelledby="agents-head" className="relative overflow-hidden border-y border-line bg-paper-2 py-24 md:py-32">
      <div aria-hidden="true" className="dot-field fade-edges absolute inset-0 opacity-60" />
      <div className="relative mx-auto max-w-[1240px] px-6 md:px-10">
        <div className="mb-12 grid gap-6 lg:grid-cols-2 lg:items-end">
          <h2 id="agents-head" className="max-w-[16ch] text-[clamp(2rem,3.4vw,3.25rem)] leading-[1.08] font-semibold tracking-[-0.02em]">{a.head}</h2>
          <p className="max-w-[46ch] text-lg text-ink-2">{a.lead}</p>
        </div>
      </div>
      <div className="relative flex flex-col gap-4">
        <LogoLoop items={rows[0]} duration={80} label={a.head} />
        <LogoLoop items={rows[1]} duration={92} reverse label={a.head} />
        <LogoLoop items={rows[2]} duration={86} label={a.head} />
      </div>
    </section>
  );
}
