import type { Copy } from "@/copy";
import { Flow } from "./Flow";
import Motion from "./Motion";

export default function FlowLazy({ c }: { c: Copy }) {
  return <Motion><Flow c={c} /></Motion>;
}
