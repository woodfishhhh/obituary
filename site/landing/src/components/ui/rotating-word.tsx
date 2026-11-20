import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";

// One word of a sentence that changes every `interval` ms: the letters of the next word rise in one
// after another while the old ones leave upward. Hovering pauses it, clicking advances it, and under
// reduced motion it stays on the first word. The slot is as wide as the word being shown (measured from
// hidden copies of every word, so any language fits) and glides to the next width instead of jumping.
// The letters move with CSS animations of opacity and transform (.rw-in and .rw-out in index.css), so
// no animation library is needed here and the first screen carries none.

// Graphemes, so Chinese splits per character and accents stay whole.
const graphemes = (s: string) => Array.from(new Intl.Segmenter(undefined, { granularity: "grapheme" }).segment(s), (x) => x.segment);

const reducedMotion = () => typeof matchMedia !== "undefined" && matchMedia("(prefers-reduced-motion: reduce)").matches;

// Each letter paints its own slice of the gradient: a parent's background-clip:text does not reach text
// that is moving or fading in a layer of its own.
function Letters({ word, motion }: { word: string; motion: "rw-in" | "rw-out" | "" }) {
  const chars = graphemes(word);
  const stagger = Math.min(0.045, 0.2 / chars.length);
  return (
    <>
      {chars.map((ch, i) => (
        <span
          key={i}
          className={cn("brand-text inline-block whitespace-pre", motion)}
          style={{
            backgroundSize: `${chars.length * 100}% 100%`,
            backgroundPosition: `${chars.length > 1 ? (i / (chars.length - 1)) * 100 : 0}% 0`,
            animationDelay: motion ? `${(motion === "rw-in" ? 0.04 : 0) + i * (motion === "rw-in" ? stagger : stagger / 2)}s` : undefined,
          }}
        >
          {ch}
        </span>
      ))}
    </>
  );
}

export function RotatingWord({ words, interval = 2600, className }: { words: readonly string[]; interval?: number; className?: string }) {
  const [index, setIndex] = useState(0);
  // The word that is leaving, kept for as long as its exit takes.
  const [leaving, setLeaving] = useState<{ word: string; key: number } | null>(null);
  const [moved, setMoved] = useState(false);
  const [paused, setPaused] = useState(false);

  const go = (next: (i: number) => number) => {
    setLeaving({ word: words[index % words.length], key: Date.now() });
    setMoved(true);
    setIndex(next);
  };
  useEffect(() => {
    if (!leaving) return;
    const id = window.setTimeout(() => setLeaving(null), 600);
    return () => window.clearTimeout(id);
  }, [leaving]);
  useEffect(() => {
    if (paused || words.length < 2 || reducedMotion()) return;
    const id = window.setInterval(() => go((i) => (i + 1) % words.length), interval);
    return () => window.clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [paused, words.length, interval, index]);

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
  return (
    <button
      type="button"
      tabIndex={-1}
      onClick={() => go((i) => (i + 1) % words.length)}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      style={{ width: widths[index], transition: widths.length === 0 ? undefined : "width 600ms cubic-bezier(0.22, 1, 0.36, 1)" }}
      className={cn("relative inline-block max-w-full cursor-pointer overflow-hidden align-bottom whitespace-nowrap", className)}
    >
      <span ref={probe} aria-hidden="true" className="invisible absolute top-0 left-0">
        {words.map((w) => (
          // Laid out letter by letter like the visible word: loose letters lose kerning, so the width differs from plain text.
          <span key={w} className="block w-max">{graphemes(w).map((ch, i) => <span key={i} className="inline-block whitespace-pre">{ch}</span>)}</span>
        ))}
      </span>
      {leaving ? (
        <span key={`out-${leaving.key}`} aria-hidden="true" className="absolute top-0 left-0 block">
          <Letters word={leaving.word} motion="rw-out" />
        </span>
      ) : null}
      <span key={`in-${word}`} className="block">
        <Letters word={word} motion={moved ? "rw-in" : ""} />
      </span>
    </button>
  );
}
