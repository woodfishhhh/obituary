import { useEffect, useRef } from "react"

import { cn } from "@/lib/utils"

interface BorderBeamProps {
  /** Length of the light as a share of the turn, in px along a 1000px edge: more means a longer tail. */
  size?: number
  /** Seconds for one turn around the box. */
  duration?: number
  /** Seconds to wait before the first turn starts moving. */
  delay?: number
  colorFrom?: string
  colorTo?: string
  className?: string
  style?: React.CSSProperties
  reverse?: boolean
  /** Where the light starts on its turn, 0 to 100. */
  initialOffset?: number
  borderWidth?: number
}

// A light that travels around the border of its parent (which needs `position: relative` and a radius).
//
// It is one large square with a conic gradient that only ever rotates, inside a ring cut out of the
// parent by a static mask. A rotation is composited on the GPU, so no frame touches layout, style
// or paint on the main thread. (The version before animated `offset-distance` from script: every
// frame recalculated style and repainted a masked layer the size of the box, and the three beams
// on the page kept doing it while scrolled out of view, which used most of the main thread.)
// The turn pauses while the box is off screen or the tab is hidden, and resumes where it stopped.
export function BorderBeam({
  className,
  size = 50,
  delay = 0,
  duration = 6,
  colorFrom = "#ffaa40",
  colorTo = "#9c40ff",
  style,
  reverse = false,
  initialOffset = 0,
  borderWidth = 1,
}: BorderBeamProps) {
  const ring = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const el = ring.current
    if (!el) return
    let visible = false
    const apply = () => { el.dataset.paused = visible && !document.hidden ? "false" : "true" }
    const io = new IntersectionObserver(([entry]) => { visible = entry.isIntersecting; apply() })
    io.observe(el)
    document.addEventListener("visibilitychange", apply)
    return () => { io.disconnect(); document.removeEventListener("visibilitychange", apply) }
  }, [])

  // The tail is as wide as `size` px on a 1000px edge, i.e. size / 1000 of a turn... capped so it stays a comet.
  const tail = Math.min(140, Math.max(30, size * 0.36))
  return (
    <div
      ref={ring}
      aria-hidden="true"
      data-paused="true"
      className="beam-ring pointer-events-none absolute inset-0 rounded-[inherit]"
      style={{ padding: borderWidth }}
    >
      <span
        className={cn("beam-sweep", className)}
        style={
          {
            "--beam-from": colorFrom,
            "--beam-to": colorTo,
            "--beam-tail": `${tail}deg`,
            "--beam-duration": `${duration}s`,
            "--beam-delay": `${-delay - (initialOffset / 100) * duration}s`,
            "--beam-direction": reverse ? "reverse" : "normal",
            ...style,
          } as React.CSSProperties
        }
      />
    </div>
  )
}
