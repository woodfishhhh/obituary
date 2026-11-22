import { Fragment, useState } from "react";
import { agents, type Agent } from "@/agents";
import { agentDocLink, initCommand } from "@/agents-data";
import { AgentStream, streamDefaults, toneOf } from "@/components/agent-stream";
import { Button } from "@/components/ui/button";
import type { Copy } from "@/copy";
import { siteLink } from "@/lib/links";
import { CopyButton } from "@/sections/Install";

// Three bands of agents sliding past; picking one shows the command that connects it, with a copy button and its page.
export function Agents({ c }: { c: Copy }) {
  const a = c.agents;
  const [picked, setPicked] = useState<string | null>(null);
  return (
    <section aria-labelledby="agents-head" className="relative overflow-hidden border-y border-line bg-paper-2 py-24 md:py-32">
      <div aria-hidden="true" className="dot-field fade-edges absolute inset-0 opacity-60" />
      <div className="relative mx-auto max-w-[1240px] px-6 md:px-10">
        <h2 id="agents-head" className="mb-12 max-w-[16ch] text-[clamp(2rem,3.4vw,3.25rem)] leading-[1.08] font-semibold tracking-[-0.02em]">{a.head}</h2>
      </div>
      <div className="relative">
        <AgentStream agents={agents} options={streamDefaults} selected={picked} onSelect={setPicked} href={(x) => siteLink(agentDocLink(x.id, c.lang))} label={a.head} />
      </div>
      <div className="relative mx-auto mt-10 max-w-[1240px] px-6 md:px-10" aria-live="polite">
        <Command c={c} agent={agents.find((x) => x.id === picked)} />
      </div>
    </section>
  );
}

function Command({ c, agent }: { c: Copy; agent?: Agent }) {
  if (!agent) {
    // Holds the panel's place before anything is picked, so choosing does not push the page down.
    return (
      <div className="flex min-h-[4.5rem] items-center justify-center rounded-3xl border border-dashed border-line-2 px-4 py-3 text-muted">
        <code className="font-mono text-[0.95rem]"><span aria-hidden="true">$ </span>obit init --agent …</code>
      </div>
    );
  }
  const command = initCommand(agent);
  return (
    <div className="flex min-h-[4.5rem] flex-wrap items-center gap-x-4 gap-y-3 rounded-3xl border border-line bg-paper/85 py-3 pr-3 pl-4 shadow-card backdrop-blur-md">
      {agent.logo && <img src={agent.logo} alt="" width={36} height={36} className="agent-logo" data-tone={toneOf(agent)} style={{ "--s": "50px" } as React.CSSProperties} />}
      <div className="min-w-0 flex-[1_1_15rem]">
        <div className="text-sm text-muted">{agent.name} · {agent.global ? c.agents.global : c.agents.project}</div>
        {/* The command wraps only between its words, never inside an option or an id. */}
        <code className="block font-mono text-[0.95rem] text-ink"><span className="text-muted" aria-hidden="true">$&nbsp;</span>{command.split(" ").map((word, i) => <Fragment key={i}>{i > 0 && " "}<span className="whitespace-nowrap">{word}</span></Fragment>)}</code>
      </div>
      <div className="flex flex-wrap gap-2">
        <CopyButton text={command} c={c.install} />
        <Button asChild>
          <a href={siteLink(agentDocLink(agent.id, c.lang))} className="min-h-11">{c.nav.docs} ↗</a>
        </Button>
      </div>
    </div>
  );
}
