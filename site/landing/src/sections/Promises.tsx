import { ArchiveRestoreIcon, GitBranchIcon, HandIcon } from "lucide-react";
import type { Copy } from "@/copy";

const icons = [GitBranchIcon, ArchiveRestoreIcon, HandIcon];

// What the agent does next to what Obituary does about it, then the three
// things it never does without you.
export function Promises({ c }: { c: Copy }) {
  const t = c.table;
  return (
    <section aria-labelledby="table-head" className="relative py-24 md:py-32">
      <div className="mx-auto max-w-[1240px] px-6 md:px-10">
        <h2 id="table-head" className="mb-10 max-w-[20ch] text-[clamp(2rem,3.4vw,3.25rem)] leading-[1.08] font-semibold tracking-[-0.02em]">
          {t.head}
        </h2>
        <div className="overflow-hidden rounded-3xl border border-line bg-paper shadow-card">
          <table className="w-full border-collapse text-left">
            <thead className="bg-paper-2 text-sm text-muted">
              <tr>
                <th scope="col" className="w-[38%] px-5 py-3 font-medium md:px-8">
                  {t.agentCol}
                </th>
                <th scope="col" className="px-5 py-3 font-medium md:px-8">
                  {t.obitCol}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {t.rows.map((r) => (
                <tr key={r.agent} className="align-top">
                  <th scope="row" className="px-5 py-5 font-normal md:px-8">
                    {/^(rm|obit) /.test(r.agent) ? (
                      <code className="rounded-md bg-paper-3 px-2 py-1 font-mono text-[13px] [overflow-wrap:anywhere] text-ink">{r.agent}</code>
                    ) : (
                      <span className="text-ink">{r.agent}</span>
                    )}
                  </th>
                  <td className="px-5 py-5 text-ink-2 md:px-8">{r.obit}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <h3 className="mt-20 mb-8 text-2xl font-semibold tracking-[-0.01em] md:text-3xl">{t.promisesHead}</h3>
        <ul className="grid grid-cols-1 gap-4 md:grid-cols-3">
          {t.promises.map((p, i) => {
            const Icon = icons[i];
            return (
              <li key={p.title} className="flex min-w-0 flex-col gap-3 rounded-2xl border border-line bg-paper-2 p-6">
                <Icon className="size-6 text-cobalt" aria-hidden="true" />
                <span className="text-lg font-semibold">{p.title}</span>
                <span className="text-ink-2">{p.line}</span>
              </li>
            );
          })}
        </ul>
      </div>
    </section>
  );
}
