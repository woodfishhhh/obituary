import { useEffect, useLayoutEffect, useRef, useState, useSyncExternalStore } from "react";

import { cn } from "@/lib/utils";

// The tagline of the first screen: a sentence in a pill with one word that changes every `interval` ms.
// The letters of the next word rise in one after another while the old ones leave upward.
//
// Nothing that moves is laid out again while it moves. The letters use the CSS animations .rw-in and
// .rw-out (opacity and transform, index.css). When the word changes, the pill takes its new size at once
// and every part that moved (the pill's background, the text on each side, the slot) then glides from where
// it was on screen to where it is now with one animation of transform, so a busy main thread cannot make
// the glide stutter and nothing shifts the layout.
//
// The clock is one timeout per word. It runs only while the tagline is on screen and the tab is visible,
// and starts over from a full interval whenever it comes back, so a return never skips or double-steps.
// Hovering with a mouse pauses it, clicking advances it. Under reduced motion the word is swapped at once.

// Graphemes, so Chinese splits per character and accents stay whole.
const graphemes = (s: string) => Array.from(new Intl.Segmenter(undefined, { granularity: "grapheme" }).segment(s), (x) => x.segment);

const GLIDE_MS = 600;
const EXIT_MS = 220;
const GLIDE_EASE = "cubic-bezier(0.22, 1, 0.36, 1)";

const reducedQuery = "(prefers-reduced-motion: reduce)";
const useReducedMotion = () =>
  useSyncExternalStore(
    (notify) => {
      const m = matchMedia(reducedQuery);
      m.addEventListener("change", notify);
      return () => m.removeEventListener("change", notify);
    },
    () => matchMedia(reducedQuery).matches,
    () => false,
  );

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

type Props = { label: string; before: string; words: readonly string[]; after: string; interval?: number };

export function RotatingTagline({ label, before, words, after, interval = 2600 }: Props) {
  const [index, setIndex] = useState(0);
  // The word that is leaving, kept for as long as its exit takes.
  const [leaving, setLeaving] = useState<{ word: string; n: number } | null>(null);
  const [moved, setMoved] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [live, setLive] = useState(true);
  const reduce = useReducedMotion();
  const pill = useRef<HTMLParagraphElement>(null);
  const was = useRef<Map<HTMLElement, DOMRect> | null>(null);
  const swaps = useRef(0);

  const parts = () => Array.from(pill.current?.querySelectorAll<HTMLElement>("[data-glide]") ?? []);

  const go = () => {
    if (!reduce) {
      was.current = new Map(parts().map((el) => [el, el.getBoundingClientRect()]));
      setLeaving({ word: words[index], n: ++swaps.current });
    }
    setMoved(true);
    setIndex((index + 1) % words.length);
  };

  // After the new word is in the DOM and before it is painted: glide each part from its old place.
  useLayoutEffect(() => {
    const from = was.current;
    was.current = null;
    if (!from) return;
    const els = parts();
    els.forEach((el) => el.getAnimations().forEach((a) => a.cancel()));
    const box = els.find((el) => el.dataset.glide === "box");
    // A word that gets shorter waits for the old letters to leave before the pill closes in on it.
    const delay = box && box.getBoundingClientRect().width < (from.get(box)?.width ?? 0) ? EXIT_MS : 0;
    for (const el of els) {
      const a = from.get(el);
      const b = el.getBoundingClientRect();
      if (!a) continue;
      const dx = a.left - b.left;
      const dy = a.top - b.top;
      const sx = el === box ? a.width / b.width : 1;
      const sy = el === box ? a.height / b.height : 1;
      if (Math.abs(dx) < 0.5 && Math.abs(dy) < 0.5 && Math.abs(sx - 1) < 0.002 && Math.abs(sy - 1) < 0.002) continue;
      el.animate(
        [{ transform: `translate(${dx}px, ${dy}px) scale(${sx}, ${sy})` }, { transform: "none" }],
        { duration: GLIDE_MS, delay, easing: GLIDE_EASE, fill: "backwards" },
      );
    }
  }, [index]);

  useEffect(() => {
    if (!leaving) return;
    const id = window.setTimeout(() => setLeaving(null), 600);
    return () => window.clearTimeout(id);
  }, [leaving]);

  // On screen and in a visible tab.
  useEffect(() => {
    let seen = true;
    const sync = () => setLive(seen && document.visibilityState === "visible");
    const io = new IntersectionObserver(([e]) => { seen = e.isIntersecting; sync(); });
    io.observe(pill.current!);
    document.addEventListener("visibilitychange", sync);
    sync();
    return () => { io.disconnect(); document.removeEventListener("visibilitychange", sync); };
  }, []);

  useEffect(() => {
    if (!live || hovered || words.length < 2) return;
    const id = window.setTimeout(go, interval);
    return () => window.clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [live, hovered, index, reduce, words.length, interval]);

  const word = words[index];
  return (
    <p
      ref={pill}
      className="relative inline-flex max-w-full items-center rounded-[1.75rem] border border-transparent px-5 py-2 text-[clamp(1.1rem,2.3vw,1.65rem)] leading-snug font-medium text-ink-2 max-md:min-h-[calc(2.75em+1rem+2px)] md:px-7 md:py-2.5"
    >
      <span aria-hidden="true" data-glide="box" className="absolute -inset-px origin-top-left rounded-[1.75rem] border border-line-2 bg-paper/70 shadow-sm backdrop-blur" />
      <span className="sr-only">{label}</span>
      <span aria-hidden="true" className="relative inline-flex flex-wrap items-center justify-center gap-x-[0.4em]">
        {before ? <span data-glide="">{before}</span> : null}
        <button
          type="button"
          tabIndex={-1}
          data-glide=""
          onClick={go}
          onPointerEnter={(e) => e.pointerType === "mouse" && setHovered(true)}
          onPointerLeave={() => setHovered(false)}
          className="relative inline-block max-w-full cursor-pointer overflow-x-visible overflow-y-clip align-bottom font-semibold whitespace-nowrap"
        >
          {leaving ? (
            <span key={leaving.n} aria-hidden="true" className="absolute top-0 left-0 block">
              <Letters word={leaving.word} motion="rw-out" />
            </span>
          ) : null}
          <span key={`in-${word}`} className="block">
            <Letters word={word} motion={moved && !reduce ? "rw-in" : ""} />
          </span>
        </button>
        {after ? <span data-glide="">{after}</span> : null}
      </span>
    </p>
  );
}
