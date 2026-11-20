import { useEffect, useState } from "react";
import { XIcon } from "lucide-react";
import { copy } from "@/copy";
import { localeOf, matchBrowser, type Lang } from "@/locales";
import { siteLink } from "@/lib/links";
import { Flag } from "@/components/flags";
import { chosenKey } from "@/components/language-menu";

const dismissedKey = "obit-lang-hint-closed";

// A quiet offer, never a redirect: when the browser's language is one the site has and this page is
// another, a small bar says so in the visitor's own language. A click on its link changes the page;
// closing it (or choosing a language in the menu) is remembered.
export function LangHint({ lang }: { lang: Lang }) {
  const [offer, setOffer] = useState<Lang | null>(null);
  useEffect(() => {
    try {
      if (localStorage.getItem(chosenKey) || localStorage.getItem(dismissedKey)) return;
    } catch { /* storage blocked: the bar can still show, and just comes back next time */ }
    const wanted = matchBrowser(navigator.languages?.length ? navigator.languages : [navigator.language]);
    if (wanted && wanted !== lang) setOffer(wanted);
  }, [lang]);
  if (!offer) return null;

  const to = localeOf(offer);
  const h = copy[offer].a11y.hint;
  const close = () => {
    try { localStorage.setItem(dismissedKey, "1"); } catch { /* nothing to remember it in */ }
    setOffer(null);
  };
  return (
    <aside aria-label={copy[offer].a11y.language} lang={to.html} className="fixed inset-x-3 bottom-3 z-40 mx-auto flex max-w-xl items-center gap-3 rounded-2xl border border-line bg-paper/95 py-2.5 pr-2 pl-4 shadow-nav backdrop-blur-md [animation:card-in_240ms_ease-out]">
      <Flag id={offer} />
      <p className="min-w-0 flex-1 text-sm text-ink-2">
        {h.text}{" "}
        <a href={siteLink(to.path)} hrefLang={to.html} onClick={() => { try { localStorage.setItem(chosenKey, offer); } catch { /* as above */ } }} className="font-medium text-cobalt underline-offset-4 hover:underline">{h.go}</a>
      </p>
      <button type="button" onClick={close} aria-label={h.close} className="grid size-10 shrink-0 place-items-center rounded-full text-muted transition-colors hover:bg-paper-3 hover:text-ink">
        <XIcon aria-hidden="true" className="size-4" />
      </button>
    </aside>
  );
}
