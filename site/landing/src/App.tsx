import { lazy, Suspense } from "react";
import type { Copy } from "@/copy";
import { Agents } from "@/sections/Agents";
import { Community } from "@/sections/Community";
import { Footer } from "@/sections/Footer";
import { Hero } from "@/sections/Hero";
import { Install } from "@/sections/Install";
import { FlagSprite } from "@/components/flags";
import { LangHint } from "@/components/lang-hint";
import { Lifecycle } from "@/sections/Lifecycle";
import { Nav } from "@/sections/Nav";
import { Promises } from "@/sections/Promises";
import { Values } from "@/sections/Values";
import { WindowLooks } from "@/sections/WindowLooks";

// The sections that use the animation library load after the first screen. The built pages already
// contain their HTML (scripts/prerender.mjs waits for them), so nothing is missing while they arrive.
const Flow = lazy(() => import("@/sections/FlowLazy"));
const Story = lazy(() => import("@/sections/StoryLazy"));

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
        <Values c={c} />
        <Suspense fallback={null}><Flow c={c} /></Suspense>
        <Suspense fallback={null}><Story c={c} /></Suspense>
        <Promises c={c} />
        <Lifecycle c={c} />
        <WindowLooks c={c} />
        <Agents c={c} />
        <Install c={c} />
        <Community c={c} />
      </main>
      <Footer c={c} />
      <LangHint lang={c.lang} />
    </>
  );
}
