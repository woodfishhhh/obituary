// What a variant is, and the type and colour constants they share. Each
// variant lives in src/variants/<id>/<id>.ts; the app loads the shipped
// ones on demand (catalog.ts), the playground imports all of them
// (variants.ts). Only the logo's colours are used: cobalt #0048E8, sky
// #0078F8, gold #F8E000, plus derived neutrals.
import type { ComponentType } from "react";
import type { ShellProps } from "./shell";

export type Tokens = Record<`--${string}`, string>;

export interface Variant {
  id: string;
  name: string;
  note: string;
  nav: "sidebar" | "topbar" | "custom";
  home: "list" | "horizon";
  dark: boolean;
  tokens: Tokens;
  // Overrides for the English UI, such as Latin typefaces for looks that
  // set Chinese in Song.
  tokensEn?: Tokens;
  // The variant's window: nav, layout and pages. A to E use the classic
  // sidebar or topbar shell (shells/Classic.tsx).
  Shell: ComponentType<ShellProps>;
}

export const cjk = '"Microsoft YaHei UI", "Microsoft YaHei", sans-serif';
export const segoe = `"Segoe UI Variable Text", "Segoe UI", ${cjk}`;
export const manrope = `"Manrope", ${cjk}`;
export const mono = '"JetBrains Mono", "Cascadia Mono", Consolas, monospace';
// The bundled serif covers the GUI's own copy; user text falls back to an
// installed Song face, so a mixed line still reads as one serif.
// Bahnschrift is the Windows DIN; DengXian sets the Chinese beside it.
export const din = '"Bahnschrift", "DengXian", "Microsoft YaHei UI", "Microsoft YaHei", sans-serif';
export const serif = '"Obit Serif", "Noto Serif SC", "Source Han Serif SC", "SimSun", serif';
// Georgia (every Windows) for English in the serif looks; Chinese in user
// text still falls back to the Song faces.
export const georgia = `Georgia, ${serif}`;

export const brand = {
  "--cobalt": "#0048E8",
  "--sky": "#0078F8",
  "--gold": "#F8E000",
  "--on-gold": "#3A3000",
  "--life-a": "#0078F8",
  "--life-b": "#F8E000",
};
