import { Children, cloneElement, forwardRef, isValidElement, type ButtonHTMLAttributes, type ReactElement } from "react";
import { cn } from "@/lib/utils";

// A minimal button for the copied components that expect one: `asChild`
// passes the classes to its single child (a link), `variant` and `size`
// pick from a few looks.
type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "default" | "ghost" | "link";
  size?: "default" | "sm";
  asChild?: boolean;
};

const looks = {
  default: "bg-cobalt text-white hover:bg-cobalt-deep",
  ghost: "hover:bg-ink/5",
  link: "underline-offset-4 hover:underline",
};

export const Button = forwardRef<HTMLButtonElement, Props>(function Button(
  { variant = "default", size = "default", asChild, className, children, ...props },
  ref,
) {
  const classes = cn(
    "inline-flex items-center justify-center gap-2 rounded-full text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cobalt disabled:opacity-50",
    size === "sm" ? "h-8 px-3" : "h-10 px-4",
    looks[variant],
    className,
  );
  if (asChild) {
    const child = Children.only(children);
    if (isValidElement<{ className?: string }>(child)) {
      return cloneElement(child as ReactElement<{ className?: string }>, { className: cn(classes, child.props.className) });
    }
  }
  return (
    <button ref={ref} type="button" className={classes} {...props}>
      {children}
    </button>
  );
});
