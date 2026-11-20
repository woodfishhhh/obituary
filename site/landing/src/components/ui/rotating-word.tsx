import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { AnimatePresence, motion, useReducedMotion, type Variants } from "motion/react";

import { cn } from "@/lib/utils";

// One word of a sentence that changes every `interval` ms: the letters of the next word rise in
// one after another while the old ones leave upward. Hovering pauses it, clicking advances it,
// and under reduced motion it stays on the first word, fading when clicked. The slot is as wide
// as the word being shown (measured from hidden copies of every word, so any language fits), and
// glides to the next width instead of jumping.

// Graphemes, so Chinese splits per character and accents stay whole.
const graphemes = (s: string) => Array.from(new Intl.Segmenter(undefined, { granularity: "grapheme" }).segment(s), (x) => x.segment);

const letter: Variants = {
  hidden: { y: "70%", opacity: 0 },
  show: { y: 0, opacity: 1, transition: { type: "spring", stiffness: 200, damping: 22 } },
  exit: { y: "-70%", opacity: 0, transition: { duration: 0.22, ease: [0.3, 0, 0.7, 0.4] } },
};
const fade: Variants = { hidden: { opacity: 0 }, show: { opacity: 1 }, exit: { opacity: 0 } };

export function RotatingWord({ words, interval = 2600, className }: { words: readonly string[]; interval?: number; className?: string }) {
  const [index, setIndex] = useState(0);
  const [paused, setPaused] = useState(false);
  const reduce = useReducedMotion() ?? false;
  useEffect(() => {
    if (reduce || paused || words.length < 2) return;
    const id = window.setInterval(() => setIndex((i) => (i + 1) % words.length), interval);
    return () => window.clearInterval(id);
  }, [reduce, paused, words.length, interval]);

  const probe = useRef<HTMLSpanElement>(null);
  const [widths, setWidths] = useState<number[]>([]);
  useLayoutEffect(() => {
    const el = probe.current;
    if (!el) return;
    const measure = () => setWidths(Array.from(el.children, (c) => Math.ceil(c.getBoundingClientRect().width)));
    measure();
    const ro = new ResizeObserver(measure);
    Array.from(el.children).forEach((c) => ro.observe(c));
    void document.fonts?.ready.then(measure);
    return () => ro.disconnect();
  }, [words]);

  const word = words[index % words.length];
  const chars = graphemes(word);
  const stagger = reduce ? 0 : Math.min(0.045, 0.2 / chars.length);
  return (
    <button
      type="button"
      tabIndex={-1}
      onClick={() => setIndex((i) => (i + 1) % words.length)}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      style={{ width: widths[index], transition: reduce || widths.length === 0 ? undefined : "width 600ms cubic-bezier(0.22, 1, 0.36, 1)" }}
      className={cn("relative inline-block max-w-full cursor-pointer overflow-hidden align-bottom whitespace-nowrap", className)}
    >
      <span ref={probe} aria-hidden="true" className="invisible absolute top-0 left-0">
        {words.map((w) => (
          // Laid out letter by letter like the visible word: loose letters lose kerning, so the width differs from plain text.
          <span key={w} className="block w-max">{graphemes(w).map((ch, i) => <span key={i} className="inline-block whitespace-pre">{ch}</span>)}</span>
        ))}
      </span>
      <AnimatePresence initial={false} mode="popLayout">
        <motion.span
          key={word}
          className="block"
          initial="hidden"
          animate="show"
          exit="exit"
          variants={{ hidden: {}, show: { transition: { delayChildren: reduce ? 0 : 0.04, staggerChildren: stagger } }, exit: { transition: { staggerChildren: stagger / 2 } } }}
        >
          {chars.map((ch, i) => (
            <motion.span
              key={i}
              className="brand-text inline-block whitespace-pre"
              // Each letter paints its own slice of the gradient: a parent's background-clip:text
              // does not reach text that is moving or fading in a layer of its own.
              style={{ backgroundSize: `${chars.length * 100}% 100%`, backgroundPosition: `${chars.length > 1 ? (i / (chars.length - 1)) * 100 : 0}% 0` }}
              variants={reduce ? fade : letter}
            >
              {ch}
            </motion.span>
          ))}
        </motion.span>
      </AnimatePresence>
    </button>
  );
}
