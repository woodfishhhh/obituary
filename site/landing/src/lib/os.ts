import { useEffect, useSyncExternalStore } from "react";
import type { Os } from "@/copy";

// The visitor's system, shared by the first screen and the install section. The built HTML and the
// first client render both say Windows (so hydration matches); once mounted, the system the browser
// reports takes over, and a choice made on the page wins over it.
export const osList: readonly Os[] = ["windows", "macos", "linux"];

function detect(): Os {
  const ua = navigator.userAgent;
  if (/Windows/i.test(ua)) return "windows";
  if (/Mac OS X|Macintosh/i.test(ua) && !/iPhone|iPad/i.test(ua)) return "macos";
  if (/Linux|X11/i.test(ua) && !/Android/i.test(ua)) return "linux";
  return "windows";
}

let detected: Os | null = null;
let chosen: Os | null = null;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((l) => l());
const subscribe = (l: () => void) => { listeners.add(l); return () => { listeners.delete(l); }; };
const snapshot = (): Os => chosen ?? detected ?? "windows";

export function useOs(): [Os, (os: Os) => void] {
  const os = useSyncExternalStore(subscribe, snapshot, (): Os => "windows");
  useEffect(() => {
    if (detected) return;
    detected = detect();
    emit();
  }, []);
  return [os, (next) => { chosen = next; emit(); }];
}
