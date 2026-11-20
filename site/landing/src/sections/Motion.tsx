import { MotionConfig } from "motion/react";
import type { ReactNode } from "react";

// The sections that use the animation library sit behind this: it carries "follow the visitor's
// reduced-motion setting" with it, and loads with them instead of with the first screen.
export default function Motion({ children }: { children: ReactNode }) {
  return <MotionConfig reducedMotion="user">{children}</MotionConfig>;
}
