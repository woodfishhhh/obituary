import { useEffect, useRef, type CSSProperties } from "react"

import { cn } from "@/lib/utils"

// Fixed places, timings and sizes: the page renders the same stars on the server and in the browser,
// and the cycles differ in length, so the sparkles never fall into step.
const stars = [
  { x: "6%", y: "18%", delay: 0, duration: 2.6, scale: 1.1 },
  { x: "30%", y: "74%", delay: 0.9, duration: 3.1, scale: 0.7 },
  { x: "52%", y: "8%", delay: 1.7, duration: 2.8, scale: 1.3 },
  { x: "71%", y: "58%", delay: 0.4, duration: 3.4, scale: 0.9 },
  { x: "92%", y: "22%", delay: 2.2, duration: 2.5, scale: 1.0 },
  { x: "18%", y: "46%", delay: 1.3, duration: 3.7, scale: 0.6 },
  { x: "62%", y: "92%", delay: 2.8, duration: 3.0, scale: 0.8 },
  { x: "84%", y: "84%", delay: 0.2, duration: 3.3, scale: 1.2 },
]

interface SparklesTextProps {
  className?: string
  children: React.ReactNode
  /** How many of the places are used, at most 8. */
  sparklesCount?: number
  colors?: { first: string; second: string }
}

const path =
  "M9.82531 0.843845C10.0553 0.215178 10.9446 0.215178 11.1746 0.843845L11.8618 2.72026C12.4006 4.19229 12.3916 6.39157 13.5 7.5C14.6084 8.60843 16.8077 8.59935 18.2797 9.13822L20.1561 9.82534C20.7858 10.0553 20.7858 10.9447 20.1561 11.1747L18.2797 11.8618C16.8077 12.4007 14.6084 12.3916 13.5 13.5C12.3916 14.6084 12.4006 16.8077 11.8618 18.2798L11.1746 20.1562C10.9446 20.7858 10.0553 20.7858 9.82531 20.1562L9.13819 18.2798C8.59932 16.8077 8.60843 14.6084 7.5 13.5C6.39157 12.3916 4.19225 12.4007 2.72023 11.8618L0.843814 11.1747C0.215148 10.9447 0.215148 10.0553 0.843814 9.82534L2.72023 9.13822C4.19225 8.59935 6.39157 8.60843 7.5 7.5C8.60843 6.39157 8.59932 4.19229 9.13819 2.72026L9.82531 0.843845Z"

// Text with small stars that blink around it. Each star only changes opacity and transform from a CSS
// animation, so it runs on the compositor and never wakes the main thread; the stars rest while the
// text is off screen or the tab is hidden, and are not drawn with reduced motion.
export function SparklesText({ children, colors = { first: "#9E7AFF", second: "#FE8BBB" }, className, sparklesCount = 10 }: SparklesTextProps) {
  const box = useRef<HTMLSpanElement>(null)
  useEffect(() => {
    const el = box.current
    if (!el) return
    let visible = false
    const apply = () => { el.dataset.paused = visible && !document.hidden ? "false" : "true" }
    const io = new IntersectionObserver(([entry]) => { visible = entry.isIntersecting; apply() })
    io.observe(el)
    document.addEventListener("visibilitychange", apply)
    return () => { io.disconnect(); document.removeEventListener("visibilitychange", apply) }
  }, [])

  return (
    <span className={cn("inline-block", className)}>
      <span ref={box} data-paused="true" className="sparkles relative inline-block">
        {stars.slice(0, Math.min(sparklesCount, stars.length)).map((s, i) => (
          <svg
            key={i}
            aria-hidden="true"
            className="sparkle pointer-events-none absolute z-20"
            style={{ left: s.x, top: s.y, "--sp-scale": s.scale, animationDelay: `${s.delay}s`, animationDuration: `${s.duration}s` } as CSSProperties}
            width="21"
            height="21"
            viewBox="0 0 21 21"
          >
            <path d={path} fill={i % 2 ? colors.second : colors.first} />
          </svg>
        ))}
        <span>{children}</span>
      </span>
    </span>
  )
}
