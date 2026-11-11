import { forwardRef, type HTMLAttributes } from "react";
import { cn } from "@/lib/utils";

// A plain scrolling box for the copied file tree, which expects one.
export const ScrollArea = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(function ScrollArea(
  { className, children, ...props },
  ref,
) {
  return (
    <div ref={ref} className={cn("overflow-auto", className)} {...props}>
      {children}
    </div>
  );
});
