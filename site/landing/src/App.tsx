import { MotionConfig } from "motion/react";
import { copy, type Lang } from "@/copy";
import { Agents } from "@/sections/Agents";
import { Community } from "@/sections/Community";
import { Flow } from "@/sections/Flow";
import { Footer } from "@/sections/Footer";
import { Hero } from "@/sections/Hero";
import { Install } from "@/sections/Install";
import { Lifecycle } from "@/sections/Lifecycle";
import { Nav } from "@/sections/Nav";
import { Promises } from "@/sections/Promises";
import { Story } from "@/sections/Story";
import { Values } from "@/sections/Values";
import { WindowLooks } from "@/sections/WindowLooks";

export function App({ lang }: { lang: Lang }) {
  const c = copy[lang];
  return (
    <MotionConfig reducedMotion="user">
      <a href="#main" className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-[60] focus:rounded-full focus:bg-ink focus:px-4 focus:py-2 focus:text-paper">
        {lang === "zh" ? "跳到正文" : "Skip to content"}
      </a>
      <Nav c={c} />
      <main id="main">
        <Hero c={c} />
        <Values c={c} />
        <Flow c={c} />
        <Story c={c} />
        <Promises c={c} />
        <Lifecycle c={c} />
        <WindowLooks c={c} />
        <Agents c={c} />
        <Install c={c} />
        <Community c={c} />
      </main>
      <Footer c={c} />
    </MotionConfig>
  );
}
