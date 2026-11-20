import type { Copy } from "@/copy";
import Motion from "./Motion";
import { Story } from "./Story";

export default function StoryLazy({ c }: { c: Copy }) {
  return <Motion><Story c={c} /></Motion>;
}
