'use client';

// A band of logos that loops sideways forever (licence: site/THIRD_PARTY_NOTICES.md, React Bits section).
// Changed from the original so the band costs the page nothing while it runs: the loop is a CSS
// animation of one transform (.logo-loop in index.css), so it runs on the compositor and never wakes the
// main thread. It rests while off screen or in a hidden tab, stops under the pointer and while keyboard
// focus is inside (and then becomes an ordinary scrollable row, so a focused logo is always in view),
// and with reduced motion it is a static, wrapped grid. The second copy of the logos is hidden from
// assistive technology and from the Tab key.
import { useEffect, useRef, type ReactNode } from 'react';

export interface LogoItem {
  name: string;
  href: string;
  logo?: string;
  dark?: boolean;
}

interface LogoLoopProps {
  items: LogoItem[];
  /** Seconds for one trip along the band. */
  duration?: number;
  reverse?: boolean;
  /** The name of the list, for assistive technology. */
  label: string;
}

function Logo({ item, copy }: { item: LogoItem; copy: boolean }): ReactNode {
  return (
    <li aria-hidden={copy || undefined} inert={copy || undefined} className="logo-loop-item">
      <a
        href={item.href}
        tabIndex={copy ? -1 : undefined}
        className="group flex w-36 flex-col items-center gap-2.5 rounded-2xl border border-line bg-paper px-2 pt-4 pb-3 text-center shadow-sm transition-[border-color,box-shadow,translate] duration-300 ease-out-soft hover:border-cobalt/40 hover:shadow-glow-soft motion-safe:hover:-translate-y-0.5"
      >
        <span className={`grid size-14 shrink-0 place-items-center overflow-hidden rounded-2xl ${item.dark ? 'bg-ink' : 'bg-paper'}`}>
          {item.logo ? (
            <img src={item.logo} alt="" width={56} height={56} loading="lazy" decoding="async" className="size-full object-contain" />
          ) : (
            <span aria-hidden="true" className="text-lg font-semibold text-ink-2">{item.name.slice(0, 1)}</span>
          )}
        </span>
        <span className="w-full truncate text-sm font-medium text-ink">{item.name}</span>
      </a>
    </li>
  );
}

export default function LogoLoop({ items, duration = 70, reverse = false, label }: LogoLoopProps) {
  const box = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = box.current;
    if (!el) return;
    let visible = false;
    const apply = () => { el.dataset.paused = visible && !document.hidden ? 'false' : 'true'; };
    const io = new IntersectionObserver(([entry]) => { visible = entry.isIntersecting; apply(); });
    io.observe(el);
    document.addEventListener('visibilitychange', apply);
    return () => { io.disconnect(); document.removeEventListener('visibilitychange', apply); };
  }, []);

  return (
    <div
      ref={box}
      data-paused="true"
      data-reverse={reverse || undefined}
      className="logo-loop"
      style={{ '--loop-duration': `${duration}s` } as React.CSSProperties}
      onFocusCapture={() => { if (box.current) box.current.dataset.focus = 'true'; }}
      onBlurCapture={(e) => { if (box.current && !box.current.contains(e.relatedTarget as Node | null)) delete box.current.dataset.focus; }}
    >
      <ul aria-label={label} className="logo-loop-track">
        {items.map((item) => <Logo key={item.name} item={item} copy={false} />)}
        {items.map((item) => <Logo key={`${item.name}-copy`} item={item} copy />)}
      </ul>
    </div>
  );
}
