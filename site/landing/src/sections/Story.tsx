import type { ReactNode } from "react";
import { useReducedMotion } from "motion/react";
import { FileIcon, FolderIcon, SparkleIcon } from "lucide-react";
import { BlurFade } from "@/components/ui/blur-fade";
import { File, Folder, Tree } from "@/components/ui/file-tree";
import { AnimatedSpan, Terminal, TypingAnimation } from "@/components/ui/terminal";
import { emptyVarRun, graveNotice, guardRun, type Copy } from "@/copy";
import { cn } from "@/lib/utils";

function Card({ className, title, line, children, i }: { className?: string; title: string; line: string; children: ReactNode; i: number }) {
  return (
    <BlurFade inView delay={0.08 * i} className={cn("flex", className)}>
      <article className="flex w-full flex-col overflow-hidden rounded-3xl border border-line bg-paper shadow-card">
        <div className="relative min-h-[15rem] flex-1 border-b border-line bg-paper-2 p-5">{children}</div>
        <div className="flex flex-col gap-2 p-6">
          <h3 className="text-xl leading-snug font-semibold tracking-[-0.01em] text-ink">{title}</h3>
          <p className="text-ink-2">{line}</p>
        </div>
      </article>
    </BlurFade>
  );
}

function Tag({ tone, children }: { tone: "rule" | "proposal" | "tracked"; children: ReactNode }) {
  const tones = {
    rule: "bg-soft text-cobalt",
    proposal: "bg-cream/70 text-on-gold",
    tracked: "bg-paper-3 text-muted",
  };
  return <span className={cn("ml-2 rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap", tones[tone])}>{children}</span>;
}

function Mess({ m }: { m: Copy["story"]["cards"]["mess"] }) {
  const folder = <FolderIcon className="size-4 text-cobalt" aria-hidden="true" />;
  const file = <FileIcon className="size-4 text-ink-2" aria-hidden="true" />;
  return (
    <Tree className="rounded-xl bg-paper p-2 text-sm" initialExpandedItems={["shop"]} indicator>
      <Folder element="shop" value="shop">
        <File value="src" fileIcon={folder} isSelectable={false}><span>src/</span><Tag tone="tracked">{m.tracked}</Tag></File>
        <File value="tr" fileIcon={folder}><span>test-results/</span><Tag tone="rule">{m.rule}</Tag></File>
        <File value="pw" fileIcon={folder}><span>.playwright-mcp/</span><Tag tone="rule">{m.rule}</Tag></File>
        <File value="log" fileIcon={file}><span>debug.log</span><Tag tone="rule">{m.rule}</Tag></File>
        <File value="notes" fileIcon={file}><span>notes-draft.md</span><Tag tone="proposal">{m.proposal}</Tag></File>
      </Folder>
    </Tree>
  );
}

function Notice({ call }: { call: string }) {
  const parts = graveNotice.split(/(#5|`obit resurrect 5`)/);
  return (
    <div className="flex h-full flex-col justify-center gap-3 font-mono text-[13px] leading-relaxed">
      <p className="self-start rounded-xl bg-paper px-3 py-2 text-ink-2 ring-1 ring-line">
        <span className="font-sans text-xs text-muted">{call}</span> <span className="text-ink">debug.log</span>
      </p>
      <p className="flex gap-2 rounded-xl bg-paper p-3 text-ink-2 shadow-notice ring-1 ring-glint">
        <SparkleIcon className="mt-0.5 size-4 shrink-0 fill-gold text-gold-deep" aria-hidden="true" />
        <span>
          {parts.map((p, i) =>
            p === "#5" || p === "`obit resurrect 5`" ? (
              <mark key={i} className="rounded bg-soft px-1 text-cobalt">{p}</mark>
            ) : (
              <span key={i}>{p}</span>
            ),
          )}
        </span>
      </p>
    </div>
  );
}

// The real run as readable steps: a comment naming each step, the command,
// and what it printed. labels: [unused, the denial, obit rm, the graveyard, the resurrect].
function GuardRun({ labels }: { labels: readonly string[] }) {
  const reduce = useReducedMotion();
  const [rm, obitRm, graveyard, resurrect] = guardRun;
  const comment = (text: string) => <span className="font-sans text-muted"># {text}</span>;
  const lines: { key: string; node: ReactNode; typed?: string }[] = [
    { key: "c0", node: null, typed: rm.cmd },
    { key: "l1", node: <span className="font-sans font-medium text-on-gold"># {labels[1]}</span> },
    { key: "o0", node: <span className="block rounded-md border-l-2 border-gold bg-cream/40 py-1 pl-2 text-ink-2">{rm.out}</span> },
    { key: "l2", node: comment(labels[2]) },
    { key: "c1", node: null, typed: obitRm.cmd },
    { key: "l3", node: comment(labels[3]) },
    { key: "c2", node: null, typed: graveyard.cmd },
    { key: "o2", node: <span className="whitespace-pre text-ink-2">{graveyard.out}</span> },
    { key: "l4", node: comment(labels[4]) },
    { key: "c3", node: null, typed: resurrect.cmd },
    { key: "o3", node: <span className="text-cobalt">{resurrect.out}</span> },
  ];
  if (reduce) {
    return (
      <pre className="h-full overflow-auto rounded-xl border border-line bg-paper p-4 font-mono text-[12.5px] leading-relaxed whitespace-pre-wrap text-ink-2 [overflow-wrap:anywhere]">
        {lines.map((l) => (
          <span key={l.key} className="block">
            {l.typed ? (
              <>
                <span className="text-sky">$ </span>
                <span className="text-ink">{l.typed}</span>
              </>
            ) : (
              l.node
            )}
          </span>
        ))}
      </pre>
    );
  }
  return (
    <Terminal className="h-full overflow-x-auto border-line bg-paper font-mono text-[12.5px] [&_code]:whitespace-pre-wrap [&_code]:[overflow-wrap:anywhere]">
      {lines.map((l) =>
        l.typed ? (
          <TypingAnimation key={l.key} className="text-ink before:text-sky before:content-['$_']" duration={26}>
            {l.typed}
          </TypingAnimation>
        ) : (
          <AnimatedSpan key={l.key} className="min-w-0">
            {l.node}
          </AnimatedSpan>
        ),
      )}
    </Terminal>
  );
}

function EmptyVar() {
  return (
    <div className="flex h-full flex-col justify-center gap-3 font-mono text-[13px] leading-relaxed">
      <p className="rounded-xl bg-paper px-3 py-2 text-ink ring-1 ring-line">
        <span className="text-sky">$ </span>
        {emptyVarRun.cmd}
      </p>
      <p className="rounded-xl bg-paper px-3 py-2 text-bad ring-1 ring-line">{emptyVarRun.out}</p>
    </div>
  );
}

export function Story({ c }: { c: Copy }) {
  const s = c.story.cards;
  return (
    <section aria-labelledby="story-head" className="py-24 md:py-32">
      <div className="mx-auto max-w-[1240px] px-6 md:px-10">
        <h2 id="story-head" className="mb-14 max-w-[20ch] text-[clamp(2rem,3.4vw,3.25rem)] leading-[1.08] font-semibold tracking-[-0.02em]">
          {c.story.head}
        </h2>
        <div className="grid grid-cols-1 gap-5 md:grid-cols-6">
          <Card i={0} className="md:col-span-3" title={s.mess.title} line={s.mess.line}>
            <Mess m={s.mess} />
          </Card>
          <Card i={1} className="md:col-span-3" title={s.notice.title} line={s.notice.line}>
            <Notice call={s.notice.call} />
          </Card>
          <Card i={2} className="md:col-span-4" title={s.guard.title} line={s.guard.line}>
            <GuardRun labels={s.guard.steps} />
          </Card>
          <Card i={3} className="md:col-span-2" title={s.shell.title} line={s.shell.line}>
            <EmptyVar />
          </Card>
        </div>
      </div>
    </section>
  );
}
