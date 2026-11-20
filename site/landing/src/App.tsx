import { lazy, Suspense } from "react";
import type { Copy } from "@/copy";
import { FlagSprite } from "@/components/flags";
import { LangHint } from "@/components/lang-hint";
import { whenNeeded } from "@/lib/idle";
import { Hero } from "@/sections/Hero";
import { Nav } from "@/sections/Nav";

// Only the first screen is hydrated at once. The rest of the page is HTML from the start (the build
// waits for it) and its scripts load when the visitor first scrolls, touches or types, or a few
// seconds after the page has loaded (lib/idle.ts).
const Below = lazy(() => whenNeeded().then(() => import("@/sections/Below")));
const BelowFooter = lazy(() => whenNeeded().then(() => import("@/sections/BelowFooter")));

export function App({ c }: { c: Copy }) {
  return (
    <>
      <a href="#main" className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-[60] focus:rounded-full focus:bg-ink focus:px-4 focus:py-2 focus:text-paper">
        {c.a11y.skip}
      </a>
      <FlagSprite />
      <Nav c={c} />
      <main id="main">
        <Hero c={c} />
        <Suspense fallback={null}><Below c={c} /></Suspense>
      </main>
      <Suspense fallback={null}><BelowFooter c={c} /></Suspense>
      <LangHint lang={c.lang} />
    </>
  );
}
