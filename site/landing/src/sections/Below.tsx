import { lazy, Suspense } from "react";
import type { Copy } from "@/copy";
import { Agents } from "@/sections/Agents";
import { Community } from "@/sections/Community";
import { Install } from "@/sections/Install";
import { Lifecycle } from "@/sections/Lifecycle";
import { Promises } from "@/sections/Promises";
import { Values } from "@/sections/Values";
import { WindowLooks } from "@/sections/WindowLooks";

// The sections that use the animation library load after the first screen. The built pages already
// contain their HTML (scripts/prerender.mjs waits for them), so nothing is missing while they arrive.
const Flow = lazy(() => import("@/sections/FlowLazy"));
const Story = lazy(() => import("@/sections/StoryLazy"));

// Everything under the first screen, one chunk, loaded by App when the visitor needs it (lib/idle.ts).
export default function Below({ c }: { c: Copy }) {
  return (
    <>
      <Values c={c} />
      <Suspense fallback={null}><Flow c={c} /></Suspense>
      <Suspense fallback={null}><Story c={c} /></Suspense>
      <Promises c={c} />
      <Lifecycle c={c} />
      <WindowLooks c={c} />
      <Agents c={c} />
      <Install c={c} />
      <Community c={c} />
    </>
  );
}
