import { useEffect, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";

import { cn } from "@/lib/utils";

// One word of a sentence that rolls up to the next every `interval` ms. Hovering
// pauses it, clicking advances it, and reduced motion keeps it still until clicked.
// Every word sits invisibly in the same grid cell, so the slot is as wide as the
// longest one and the sentence around it never shifts.
export function RotatingWord({ words, interval = 2400, className }: { words: readonly string[]; interval?: number; className?: string }) {
  const [index, setIndex] = useState(0);
  const [paused, setPaused] = useState(false);
  const reduce = useReducedMotion();

  useEffect(() => {
    if (reduce || paused || words.length < 2) return;
    const id = window.setInterval(() => setIndex((i) => (i + 1) % words.length), interval);
    return () => window.clearInterval(id);
  }, [reduce, paused, words.length, interval]);

  return (
    <button
      type="button"
      tabIndex={-1}
      onClick={() => setIndex((i) => (i + 1) % words.length)}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      className={cn("relative inline-grid cursor-pointer overflow-hidden align-bottom", className)}
    >
      {words.map((w) => (
        <span key={w} className="invisible col-start-1 row-start-1 whitespace-nowrap">
          {w}
        </span>
      ))}
      <AnimatePresence initial={false} mode="popLayout">
        <motion.span
          key={words[index]}
          className="col-start-1 row-start-1 whitespace-nowrap"
          initial={reduce ? false : { y: "100%", opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          exit={reduce ? { opacity: 0, transition: { duration: 0 } } : { y: "-100%", opacity: 0 }}
          transition={{ type: "spring", stiffness: 320, damping: 30 }}
        >
          {words[index]}
        </motion.span>
      </AnimatePresence>
    </button>
  );
}
