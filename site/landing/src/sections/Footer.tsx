import wordmark from "@/assets/wordmark-light.webp";
import { icp, isMirror } from "@/config";
import { repo, type Copy } from "@/copy";
import { siteLink } from "@/lib/links";

// Where the seven footer links go, in the order of each language's labels.
const targets = (docs: string) => [docs, `${docs}commands`, `${docs}faq`, repo, `${repo}/releases`, `${docs}code-signing`, `${docs}privacy`];

export function Footer({ c }: { c: Copy }) {
  const f = c.footer;
  return (
    <footer className="relative overflow-hidden border-t border-line bg-paper-2">
      <div aria-hidden="true" className="absolute -bottom-40 left-1/3 size-[36rem] rounded-full bg-glint/40 blur-3xl" />
      <div className="relative mx-auto grid max-w-[1240px] gap-12 px-6 py-20 md:px-10 lg:grid-cols-[minmax(0,7fr)_minmax(0,5fr)] lg:items-end">
        <div>
          <img src={wordmark} alt={c.a11y.wordmark} width={1040} height={369} loading="lazy" className="w-full max-w-[480px]" />
          <p className="mt-8 text-[clamp(1.75rem,3vw,2.5rem)] leading-tight font-semibold tracking-[-0.02em]">{f.line}</p>
        </div>
        <nav aria-label={c.a11y.footerNav}>
          <ul className="grid grid-cols-2 gap-x-8 gap-y-3">
            {f.links.map((label, i) => (
              <li key={label}>
                <a href={siteLink(targets(c.docs)[i])} className="text-ink-2 underline-offset-4 transition-colors hover:text-cobalt hover:underline">
                  {label}
                </a>
              </li>
            ))}
          </ul>
        </nav>
      </div>
      <div className="relative mx-auto flex max-w-[1240px] flex-wrap justify-between gap-x-8 gap-y-2 border-t border-line px-6 py-6 text-sm text-muted md:px-10">
        <span>{f.legal}</span>
        {isMirror && icp ? (
          <a href="https://beian.miit.gov.cn/" className="underline-offset-4 hover:text-ink hover:underline">
            {icp}
          </a>
        ) : null}
      </div>
    </footer>
  );
}
