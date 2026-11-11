// The chosen look: what the 外观 setting (and the J palette's command) reads
// and changes. Absent in a context without switching.
import { createContext, useContext } from "react";
import { looks, type Look } from "./catalog";

export interface Appearance {
  current: string;
  looks: Look[];
  loading?: string;
  choose: (id: string) => void;
}

export const AppearanceContext = createContext<Appearance | null>(null);
export const useAppearance = () => useContext(AppearanceContext);

// WebView2 keeps localStorage in the app's data folder, so the choice
// survives restarts. An id that no longer ships falls back to the default.
const KEY = "obit.appearance";
export function savedLook(fallback: string): string {
  try {
    const id = localStorage.getItem(KEY);
    return id && looks.some((l) => l.id === id) ? id : fallback;
  } catch {
    return fallback;
  }
}
export function saveLook(id: string) {
  try { localStorage.setItem(KEY, id); } catch { /* private mode: the choice lasts this session */ }
}
