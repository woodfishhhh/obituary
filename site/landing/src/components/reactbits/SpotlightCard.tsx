'use client';

// A spotlight that follows the pointer (licence: site/THIRD_PARTY_NOTICES.md, React Bits section).
// What changed: the pointer position goes into CSS variables instead of React state (no render per
// mouse move); keyboard focus inside the card lights it too; a tap lights it for a moment where
// there is no hover; with reduced motion the light stays where it is instead of following.
import { useRef, type ElementType, type ReactNode } from 'react';

import { cn } from '@/lib/utils';

interface SpotlightCardProps {
  children: ReactNode;
  className?: string;
  /** The light's colour, with its own alpha. */
  spotlightColor?: string;
  as?: ElementType;
}

export default function SpotlightCard({ children, className, spotlightColor = 'rgb(0 120 248 / 0.18)', as: Tag = 'div' }: SpotlightCardProps) {
  const ref = useRef<HTMLElement>(null);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);

  const aim = (e: React.PointerEvent) => {
    const el = ref.current;
    if (!el || matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const r = el.getBoundingClientRect();
    el.style.setProperty('--mx', `${e.clientX - r.left}px`);
    el.style.setProperty('--my', `${e.clientY - r.top}px`);
  };
  const set = (on: boolean) => { if (ref.current) ref.current.dataset.lit = String(on); };

  return (
    <Tag
      ref={ref}
      data-lit="false"
      onPointerEnter={(e: React.PointerEvent) => { if (e.pointerType !== 'touch') { aim(e); set(true); } }}
      onPointerMove={(e: React.PointerEvent) => { if (e.pointerType !== 'touch') aim(e); }}
      onPointerLeave={() => set(false)}
      onPointerDown={(e: React.PointerEvent) => {
        if (e.pointerType !== 'touch') return;
        aim(e);
        set(true);
        clearTimeout(timer.current);
        timer.current = setTimeout(() => set(false), 900);
      }}
      style={{ '--spot': spotlightColor } as React.CSSProperties}
      className={cn('spot relative overflow-hidden', className)}
    >
      <span aria-hidden="true" className="spot-light pointer-events-none absolute inset-0" />
      <div className="relative">{children}</div>
    </Tag>
  );
}
