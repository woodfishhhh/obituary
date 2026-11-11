import { type AnchorHTMLAttributes, type CSSProperties } from "react";
import { cn } from "@/lib/utils";

// The shimmer button (shimmer-button.tsx) as a link, for calls to action
// that navigate. The shimmer stops under prefers-reduced-motion.
interface ShimmerLinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  shimmerColor?: string;
  shimmerSize?: string;
  borderRadius?: string;
  shimmerDuration?: string;
  background?: string;
}

export function ShimmerLink({
  shimmerColor = "#ffffff",
  shimmerSize = "0.05em",
  shimmerDuration = "3s",
  borderRadius = "100px",
  background = "rgba(0, 0, 0, 1)",
  className,
  children,
  ...props
}: ShimmerLinkProps) {
  return (
    <a
      style={
        {
          "--spread": "90deg",
          "--shimmer-color": shimmerColor,
          "--radius": borderRadius,
          "--speed": shimmerDuration,
          "--cut": shimmerSize,
          "--bg": background,
        } as CSSProperties
      }
      className={cn(
        "group relative z-0 flex cursor-pointer items-center justify-center overflow-hidden [border-radius:var(--radius)] border border-white/10 px-6 py-3 whitespace-nowrap text-white [background:var(--bg)]",
        "transform-gpu transition-transform duration-300 ease-in-out active:translate-y-px",
        className,
      )}
      {...props}
    >
      <span aria-hidden="true" className="@container-[size] absolute inset-0 -z-30 overflow-visible blur-[2px] motion-reduce:hidden">
        <span className="animate-shimmer-slide absolute inset-0 aspect-[1] h-[100cqh] rounded-none [mask:none]">
          <span className="animate-spin-around absolute -inset-full w-auto [translate:0_0] rotate-0 [background:conic-gradient(from_calc(270deg-(var(--spread)*0.5)),transparent_0,var(--shimmer-color)_var(--spread),transparent_var(--spread))]" />
        </span>
      </span>
      {children}
      <span
        aria-hidden="true"
        className="absolute inset-0 size-full rounded-2xl shadow-shine transition-shadow duration-300 ease-in-out group-hover:shadow-shine-hover group-active:shadow-shine-active"
      />
      <span aria-hidden="true" className="absolute inset-(--cut) -z-20 [border-radius:var(--radius)] [background:var(--bg)]" />
    </a>
  );
}
