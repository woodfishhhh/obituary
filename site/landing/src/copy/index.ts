import { locales, type Lang } from "../locales";
import { de } from "./de";
import { en } from "./en";
import { es } from "./es";
import { fr } from "./fr";
import { ja } from "./ja";
import { ko } from "./ko";
import { ptBR } from "./pt-br";
import { ru } from "./ru";
import type { Strings } from "./types";
import { zh } from "./zh";
import { zhTW } from "./zh-tw";

export * from "./base";
export type { Strings };
export type { Lang };

// Every sentence on the landing page, per language. Claims come from the repository's README.md
// (the English source) only; keep them in step. The CJK strings also decide which glyphs
// scripts/subset-fonts.mjs keeps (Simplified Chinese only; the others use the system's fonts).
export type Copy = Strings & { lang: Lang; docs: string };

const strings: Record<Lang, Strings> = { zh, "zh-TW": zhTW, en, ja, ko, es, fr, de, "pt-BR": ptBR, ru };
// The docs are in Chinese and English; the other languages open the English ones.
export const copy = Object.fromEntries(
  locales.map(({ id }) => [id, { ...strings[id], lang: id, docs: id === "zh" ? "docs/" : "docs/en/" }]),
) as Record<Lang, Copy>;
