import { lazy, Suspense, useEffect, useState, type ComponentProps } from "react";

import { cn } from "@/lib/utils";

const Demo = lazy(() => import("./window-demo").then((m) => ({ default: m.WindowDemo })));

// The window demo carries the desktop front end, so it loads after the page, in the browser only.
// Until then a placeholder holds the same box (the server renders it too), which keeps the layout
// still while the window arrives.
export function Window(props: ComponentProps<typeof Demo>) {
  const { height = 700, minScale = 0.66 } = props;
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);
  const holder = <div aria-hidden="true" style={{ aspectRatio: `1040 / ${height}`, minHeight: height * minScale }} className={cn("bg-paper", props.className)} />;
  return mounted ? <Suspense fallback={holder}><Demo {...props} /></Suspense> : holder;
}
