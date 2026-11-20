import { cn } from "@/lib/utils";

// The entrance of the first screen: a CSS animation of opacity and transform that starts as soon as
// the browser has the HTML, so the text is on screen without waiting for any script (.rise in index.css).
export function Rise({ children, delay = 0, className }: { children: React.ReactNode; delay?: number; className?: string }) {
  return <div className={cn("rise", className)} style={{ animationDelay: `${delay}s` }}>{children}</div>;
}
