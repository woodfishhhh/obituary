import { isMirror, urls } from "@/config";
import type { Copy } from "@/copy";
import { repo } from "@/copy";
import { siteLink } from "@/lib/links";

export function Nav({ c }: { c: Copy }) {
  const links = [
    { href: "#why", label: c.nav.why },
    { href: "#life", label: c.nav.life },
    { href: "#window", label: c.nav.window },
    { href: "#install", label: c.nav.install },
  ];
  return (
    <header className="fixed inset-x-0 top-3 z-50 px-3">
      <nav
        aria-label={c.lang === "zh" ? "主导航" : "Main"}
        className="mx-auto flex h-14 max-w-[1240px] items-center gap-2 rounded-full border border-line/80 bg-paper/75 pr-2 pl-4 shadow-nav backdrop-blur-md"
      >
        <a href={c.lang === "zh" ? siteLink("") : siteLink("en/")} className="mr-2 flex items-center gap-2 rounded-full py-1 pr-2 font-semibold text-ink">
          <img src={`${import.meta.env.BASE_URL}icon-128.png`} alt="" width={32} height={32} className="size-8" />
          <span>{c.lang === "zh" ? "讣告" : "Obituary"}</span>
        </a>
        <ul className="hidden items-center gap-1 md:flex">
          {links.map((l) => (
            <li key={l.href}>
              <a href={l.href} className="rounded-full px-2 py-2 text-sm whitespace-nowrap text-ink-2 transition-colors hover:bg-paper-3 hover:text-ink sm:px-3">
                {l.label}
              </a>
            </li>
          ))}
        </ul>
        <div className="ml-auto flex items-center gap-1">
          <a href={siteLink(c.docs)} className="rounded-full px-2 py-2 text-sm font-medium whitespace-nowrap text-ink transition-colors hover:bg-paper-3 sm:px-3">
            {c.nav.docs}
          </a>
          {isMirror || c.lang !== "zh" ? null : (
            <a href={urls.mirror} hrefLang="zh-CN" className="hidden rounded-full px-3 py-2 text-sm whitespace-nowrap text-ink-2 transition-colors hover:bg-paper-3 hover:text-ink sm:inline-flex">
              {c.footer.mirror}
            </a>
          )}
          <a
            href={siteLink(c.switchLang.href)}
            hrefLang={c.lang === "zh" ? "en" : "zh-CN"}
            className="rounded-full px-2 py-2 text-sm whitespace-nowrap text-ink-2 transition-colors hover:bg-paper-3 hover:text-ink sm:px-3"
          >
            {c.switchLang.label}
          </a>
          <a href={repo} className="rounded-full bg-ink px-3 py-2 text-sm font-medium whitespace-nowrap text-paper transition-colors hover:bg-ink-2 sm:px-4">
            GitHub
          </a>
        </div>
      </nav>
    </header>
  );
}
