// Real screenshots of the desktop window, taken from its dev server
// (cmd/obit-gui/frontend, pnpm dev) with its mock data, at 1040 x 700 and
// scale 2. Only the Calendar and One line looks appear on the page.
import type { Lang } from "@/copy";

const files = import.meta.glob<string>("../assets/screens/*.webp", { eager: true, import: "default" });

export type Shot = "calendar" | "palette" | `${"calendar" | "palette"}-${"graveyard" | "decisions"}`;

export function shot(lang: Lang, name: Shot): string {
  const url = files[`../assets/screens/${lang}-${name}.webp`];
  if (!url) throw new Error(`missing screenshot ${lang}-${name}`);
  return url;
}

export const shotSize = { width: 2080, height: 1400 };
