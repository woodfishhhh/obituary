import { useEffect, useRef, useState } from "react";
import { CheckIcon, ChevronDownIcon } from "lucide-react";
import type { Copy } from "@/copy";
import { localeOf, locales, type Lang } from "@/locales";
import { siteLink } from "@/lib/links";
import { cn } from "@/lib/utils";

// Remembered so the page stops suggesting another language once a visitor has chosen one.
export const chosenKey = "obit-lang";

export function Flag({ id, className }: { id: Lang; className?: string }) {
  return <img src={`${import.meta.env.BASE_URL}flags/${localeOf(id).flag}.svg`} alt="" width={20} height={15} className={cn("h-[15px] w-5 shrink-0 rounded-[3px] object-cover ring-1 ring-ink/15", className)} />;
}

// A <details> menu: it opens and its links work with scripts off. With scripts, Escape and a click
// elsewhere close it and Escape returns the focus to the button.
export function LanguageMenu({ c }: { c: Copy }) {
  const box = useRef<HTMLDetailsElement>(null);
  const [open, setOpen] = useState(false);
  const here = localeOf(c.lang);

  useEffect(() => {
    if (!open) return;
    const close = () => { if (box.current) box.current.open = false; };
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      close();
      box.current?.querySelector("summary")?.focus();
    };
    const onPointer = (e: PointerEvent) => { if (!box.current?.contains(e.target as Node)) close(); };
    const onFocus = (e: FocusEvent) => { if (!box.current?.contains(e.target as Node)) close(); };
    document.addEventListener("keydown", onKey);
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("focusin", onFocus);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("focusin", onFocus);
    };
  }, [open]);

  return (
    <details ref={box} onToggle={(e) => setOpen(e.currentTarget.open)} className="group relative">
      <summary
        aria-label={`${c.a11y.language}: ${here.name}`}
        className="flex min-h-10 cursor-pointer list-none items-center gap-2 rounded-full px-2.5 py-2 text-sm whitespace-nowrap text-ink-2 transition-colors select-none hover:bg-paper-3 hover:text-ink group-open:bg-paper-3 group-open:text-ink sm:px-3 [&::-webkit-details-marker]:hidden"
      >
        <Flag id={c.lang} />
        <span className="hidden sm:inline" lang={here.html}>{here.name}</span>
        <ChevronDownIcon aria-hidden="true" className="size-3.5 transition-transform group-open:rotate-180" />
      </summary>
      <ul aria-label={c.a11y.language} className="absolute top-full right-0 z-50 mt-2 max-h-[min(30rem,calc(100dvh-6rem))] w-60 overflow-y-auto rounded-2xl border border-line bg-paper p-1.5 shadow-nav">
        {locales.map((l) => (
          <li key={l.id}>
            <a
              href={siteLink(l.path)}
              lang={l.html}
              hrefLang={l.html}
              aria-current={l.id === c.lang ? "true" : undefined}
              onClick={() => { try { localStorage.setItem(chosenKey, l.id); } catch { /* storage blocked: the page just keeps suggesting */ } }}
              className={cn(
                "flex min-h-11 items-center gap-3 rounded-xl px-3 py-2 text-[15px] transition-colors hover:bg-paper-3",
                l.id === c.lang ? "font-semibold text-cobalt" : "text-ink-2 hover:text-ink",
              )}
            >
              <Flag id={l.id} />
              <span className="min-w-0 flex-1">{l.name}</span>
              {l.id === c.lang ? <CheckIcon aria-hidden="true" className="size-4 shrink-0" /> : null}
            </a>
          </li>
        ))}
      </ul>
    </details>
  );
}
