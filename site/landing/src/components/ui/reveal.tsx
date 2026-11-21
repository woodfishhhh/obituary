import { useEffect, useRef } from "react";

import { cn } from "@/lib/utils";

// Content that fades up the first time it scrolls into view. Each piece is hidden (`armed`) only when its
// own script starts watching it, so with scripts off, before they run, or while the chunk that holds the
// piece is still on its way, the text is simply there; what is already in view when the script arrives is
// never hidden, so it never blinks. One shared observer; nothing here uses an animation library, and the
// motion is a CSS transition of opacity and transform (.reveal in index.css), which reduced motion switches off.
let observer: IntersectionObserver | undefined;
const watch = (el: Element) => {
  observer ??= new IntersectionObserver(
    (entries) => entries.forEach((e) => { if (e.isIntersecting) { e.target.classList.add("in"); observer?.unobserve(e.target); } }),
    { rootMargin: "0px 0px -50px 0px" },
  );
  observer.observe(el);
};

export function Reveal({ children, delay = 0, className }: { children: React.ReactNode; delay?: number; className?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    // Above or inside the viewport: shown. Only what is still below it is hidden, until it comes into view.
    if (r.top < innerHeight) el.classList.add("in");
    else {
      el.classList.add("armed");
      watch(el);
    }
  }, []);
  return <div ref={ref} className={cn("reveal", className)} style={{ transitionDelay: `${delay}s` }}>{children}</div>;
}
