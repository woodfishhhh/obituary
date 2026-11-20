import { useEffect, useRef } from "react";

import { cn } from "@/lib/utils";

// Content that fades up the first time it scrolls into view. The hidden state exists only once the page's
// script has started (`html.js`, set in main.tsx), so with scripts off, or before they run, the text is
// simply there. One shared observer; nothing here uses an animation library, and the motion is a CSS
// transition of opacity and transform (.reveal in index.css), which reduced motion switches off.
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
  useEffect(() => { if (ref.current) watch(ref.current); }, []);
  return <div ref={ref} className={cn("reveal", className)} style={{ transitionDelay: `${delay}s` }}>{children}</div>;
}
