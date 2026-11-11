// A small i18n layer: t(key, ...params) over two tables. zh-CN.ts is the
// source; en.ts must have exactly its keys and signatures (type-checked).
// Plurals and interpolation live in the tables as functions.
import { createContext, useContext } from "react";
import { en } from "./en";
import { zh, type Messages } from "./zh-CN";

export type Lang = "zh" | "en";
type Key = keyof Messages;
type Args<K extends Key> = Messages[K] extends (...args: infer A) => string ? A : [];

const tables: Record<Lang, Messages> = { zh, en };

// The language of everything rendered from now on. The root keeps it in
// state and re-renders the window when it changes.
let current: Lang = systemLang();
export const lang = () => current;
export function setLang(next: Lang) {
  current = next;
  if (typeof document !== "undefined") document.documentElement.lang = next === "zh" ? "zh-CN" : "en";
}

export function t<K extends Key>(key: K, ...args: Args<K>): string {
  const entry = tables[current][key] as string | ((...a: unknown[]) => string);
  return typeof entry === "function" ? entry(...args) : entry;
}

export function systemLang(): Lang {
  const tag = typeof navigator === "undefined" ? "" : navigator.language ?? "";
  return tag.toLowerCase().startsWith("zh") ? "zh" : "en";
}
// The backend keeps the language in config.toml (Settings.language and
// effectiveLanguage). Its tags map to ours; anything else means unknown.
export const fromTag = (tag?: string): Lang | null => (tag === "zh-CN" ? "zh" : tag === "en" ? "en" : null);

// Only a fallback for when the backend cannot be asked: the last language
// it gave, so a failed start does not flip the language. Never the source.
const CACHE = "obit.language";
export function cachedLang(): Lang | null {
  try { return fromTag(localStorage.getItem(CACHE) ?? undefined); } catch { return null; }
}
export function cacheLang(next: Lang) {
  try { localStorage.setItem(CACHE, next === "zh" ? "zh-CN" : "en"); } catch { /* nothing to keep */ }
}

// The window's language, and how the 语言 setting hands it the backend's
// new effectiveLanguage after a change is applied.
export interface LangChoice { lang: Lang; adopt: (effectiveLanguage?: string) => void }
export const LangContext = createContext<LangChoice | null>(null);
export const useLangChoice = () => useContext(LangContext);
